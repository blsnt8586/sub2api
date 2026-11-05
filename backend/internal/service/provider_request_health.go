package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// ProviderRequestHealthCache is deliberately separate from GatewayCache so the
// existing scheduler/cache contract and all third-party implementations survive.
// Atomic implementations must fence stale concurrent completions and deduplicate
// retries of one logical request. Nothing in this port changes billing or output.
type providerRequestHealthTrackedKey struct{}

func WithProviderRequestHealth(ctx context.Context) context.Context {
	return context.WithValue(ctx, providerRequestHealthTrackedKey{}, true)
}
func NewProviderHealthRequestID() string { return uuid.NewString() }

type ProviderRequestHealthCache interface {
	ObserveProviderRequestHealth(context.Context, ProviderRequestHealthEvent) error
	ProviderSessionAvoidances(context.Context, string, time.Time) ([]ProviderSessionAvoidance, error)
	ProviderRecentFailures(context.Context, int64, string, time.Time, int) (int, error)
}

type ProviderRequestHealthEvent struct {
	AccountID, UserID              int64
	Generation, Session, RequestID string
	StartedAt, CompletedAt         time.Time
	Failed                         bool
	Policy                         ProviderAccountHealthPolicy
}
type ProviderSessionAvoidance struct {
	AccountID  int64
	Generation string
}
type providerRuntimePolicy struct {
	Policy     ProviderAccountHealthPolicy
	Generation string
	Since      time.Time
}

// Policy reads are bounded, coalesced, and cached briefly. No database query per
// candidate or token, no unbounded background goroutine, no external model tests.
type ProviderRequestHealthService struct {
	db       *sql.DB
	cache    ProviderRequestHealthCache
	policies *cache.Cache
	sf       singleflight.Group
}

func NewProviderRequestHealthService(db *sql.DB, gatewayCache GatewayCache) *ProviderRequestHealthService {
	c, _ := gatewayCache.(ProviderRequestHealthCache)
	return &ProviderRequestHealthService{db: db, cache: c, policies: cache.New(5*time.Second, time.Minute)}
}

func (s *ProviderRequestHealthService) policy(ctx context.Context, accountID int64) (providerRuntimePolicy, bool) {
	if s == nil || s.db == nil || s.cache == nil || accountID <= 0 {
		return providerRuntimePolicy{}, false
	}
	key := fmt.Sprint(accountID)
	if v, ok := s.policies.Get(key); ok {
		p := v.(providerRuntimePolicy)
		return p, p.Policy.Enabled
	}
	v, err, _ := s.sf.Do(key, func() (any, error) {
		if v, ok := s.policies.Get(key); ok {
			return v, nil
		}
		readCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
		defer cancel()
		var p providerRuntimePolicy
		var raw []byte
		err := s.db.QueryRowContext(readCtx, `SELECT h.policy,h.revision::text || ':' || h.observe_since::text || ':' || COALESCE(a.extra->>'sub2api_admin_account_state_generation','') || ':' || COALESCE(a.provider_api_key_id,0)::text || ':' || COALESCE(a.remote_group_id,0)::text,h.observe_since FROM provider_account_hybrid_health h JOIN accounts a ON a.id=h.account_id AND a.provider_id=h.provider_id WHERE h.account_id=$1 AND a.deleted_at IS NULL`, accountID).Scan(&raw, &p.Generation, &p.Since)
		if errors.Is(err, sql.ErrNoRows) {
			s.policies.SetDefault(key, p)
			return p, nil
		}
		if err != nil {
			return nil, err
		}
		p.Policy = DefaultProviderAccountHealthPolicy()
		if err = json.Unmarshal(raw, &p.Policy); err != nil {
			return nil, err
		}
		if err = p.Policy.Validate(); err != nil {
			return nil, err
		}
		s.policies.SetDefault(key, p)
		return p, nil
	})
	if err != nil {
		return providerRuntimePolicy{}, false
	}
	p := v.(providerRuntimePolicy)
	return p, p.Policy.Enabled
}

func providerSessionKey(userID, groupID int64, session string) string {
	// Even clients using the same session string cannot affect other users.
	// Empty client sessions use a per-user scope, never a shared anonymous scope.
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%s", userID, groupID, session)))
	return fmt.Sprintf("%d:%s", userID, hex.EncodeToString(hash[:]))
}

func (s *ProviderRequestHealthService) Exclusions(ctx context.Context, groupID int64, session string, existing map[int64]struct{}) map[int64]struct{} {
	userID, _ := ctx.Value(ctxkey.UserID).(int64)
	tracked, _ := ctx.Value(providerRequestHealthTrackedKey{}).(bool)
	if !tracked {
		return existing
	}
	if s == nil || s.cache == nil || userID <= 0 {
		return existing
	}
	readCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	items, err := s.cache.ProviderSessionAvoidances(readCtx, providerSessionKey(userID, groupID, session), time.Now())
	if err != nil || len(items) == 0 {
		return existing
	} // Cache outages must not take down scheduling.
	out := make(map[int64]struct{}, len(existing)+len(items))
	for id := range existing {
		out[id] = struct{}{}
	}
	for _, item := range items {
		p, enabled := s.policy(readCtx, item.AccountID)
		if enabled && p.Generation == item.Generation {
			out[item.AccountID] = struct{}{}
		}
	}
	return out
}

func providerHealthFailure(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) {
		return false
	}
	var f *UpstreamFailoverError
	if errors.As(err, &f) {
		return !f.IsCredentialFailure() && !f.RequestScopedTransient && f.Scope != GatewayFailureScopeRequest && f.Scope != GatewayFailureScopeProvider && f.StatusCode >= 500
	}
	var committed *openAICommittedStreamError
	if errors.As(err, &committed) {
		return committed.eligible && committed.status >= 500
	}
	var ne net.Error
	return errors.As(err, &ne) // Only typed upstream failures; never match arbitrary user error text.
}

func (s *ProviderRequestHealthService) Observe(ctx context.Context, c *gin.Context, a *Account, groupID int64, session string, started time.Time, completed bool, stream bool, latency *int, err error) {
	if s == nil || c == nil || a == nil || a.ProviderID == nil || errors.Is(ctx.Err(), context.Canceled) {
		return
	}
	observeCtx := ctx
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		observeCtx = context.WithoutCancel(ctx)
	}
	userID, _ := ctx.Value(ctxkey.UserID).(int64)
	if userID <= 0 {
		return
	}
	p, enabled := s.policy(observeCtx, a.ID)
	if !enabled || started.Before(p.Since) {
		return
	}
	failed := providerHealthFailure(err)
	if err != nil && !failed {
		return
	}
	if !failed && !completed {
		return
	}
	if !failed && latency == nil && stream {
		return
	} // No TTFT is unknown, never a zero-ms success.
	if latency != nil && *latency > p.Policy.TimeoutMS {
		failed = true
	}
	// The account policy counts the first-response boundary. A completed
	// response with a timely first token is not a failure because generation
	// continued for longer afterward.
	requestID, _ := c.Get("provider_health_request_nonce")
	if requestID == nil {
		requestID = uuid.NewString()
		c.Set("provider_health_request_nonce", requestID)
	}
	writeCtx, cancel := context.WithTimeout(observeCtx, 150*time.Millisecond)
	defer cancel()
	if e := s.cache.ObserveProviderRequestHealth(writeCtx, ProviderRequestHealthEvent{AccountID: a.ID, UserID: userID, Generation: p.Generation, Session: providerSessionKey(userID, groupID, session), RequestID: requestID.(string), StartedAt: started, CompletedAt: time.Now(), Failed: failed, Policy: p.Policy}); e != nil {
		logger.LegacyPrintf("service.provider_health", "request health evidence unavailable: account=%d err=%v", a.ID, e)
		return
	}
	if failed {
		readCtx, readCancel := context.WithTimeout(observeCtx, 150*time.Millisecond)
		count, e := s.cache.ProviderRecentFailures(readCtx, a.ID, p.Generation, time.Now(), p.Policy.WindowSeconds)
		readCancel()
		if e != nil {
			logger.LegacyPrintf("service.provider_health", "recent account failures unavailable: account=%d err=%v", a.ID, e)
			return
		}
		if count >= p.Policy.AccountFailureThreshold {
			quarantineCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			defer cancel()
			if e := s.quarantineAccount(quarantineCtx, a, p, count); e != nil {
				logger.LegacyPrintf("service.provider_health", "account quarantine failed: account=%d err=%v", a.ID, e)
			}
		}
	}
}

func (s *ProviderRequestHealthService) quarantineAccount(ctx context.Context, a *Account, p providerRuntimePolicy, count int) error {
	if a.ProviderID == nil {
		return nil
	}
	h, err := scanHybridState(s.db.QueryRowContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE account_id=$1`, a.ID))
	if err != nil {
		return err
	}
	reason := fmt.Sprintf("%d failed or timed-out requests within %d seconds", count, p.Policy.WindowSeconds)
	return (&Sub2APIProviderProbeService{db: s.db}).enterHybridRecoveryWithGeneration(ctx, h, p.Generation, reason, time.Now().UTC())
}

func (s *RateLimitService) providerHealthManaged(ctx context.Context, a *Account) bool {
	if s == nil || s.providerHealth == nil || a == nil || a.ProviderID == nil {
		return false
	}
	tracked, _ := ctx.Value(providerRequestHealthTrackedKey{}).(bool)
	if !tracked {
		return false
	}
	_, enabled := s.providerHealth.policy(ctx, a.ID)
	return enabled
}

// Adapter hooks keep the shared protocol implementations and billing untouched.
func (s *OpenAIGatewayService) ObserveProviderRequestHealth(ctx context.Context, c *gin.Context, a *Account, groupID *int64, session string, started time.Time, r *OpenAIForwardResult, err error) {
	if s == nil || s.rateLimitService == nil {
		return
	}
	h := s.rateLimitService.providerHealth
	var latency *int
	stream := true
	if r != nil {
		stream = r.Stream
		latency = r.FirstTokenMs
		if !stream {
			n := int(r.Duration.Milliseconds())
			latency = &n
		}
	}
	h.Observe(ctx, c, a, derefGroupID(groupID), session, started, r != nil && !r.ClientDisconnect && (r.UpstreamTerminalEvent == "" || r.UpstreamTerminalEvent == "response.completed"), stream, latency, err)
}
func (s *GatewayService) ObserveProviderRequestHealth(ctx context.Context, c *gin.Context, a *Account, groupID *int64, session string, started time.Time, r *ForwardResult, err error) {
	if s == nil || s.rateLimitService == nil {
		return
	}
	h := s.rateLimitService.providerHealth
	var latency *int
	stream := true
	if r != nil {
		stream = r.Stream
		latency = r.FirstTokenMs
		if !stream {
			n := int(r.Duration.Milliseconds())
			latency = &n
		}
	}
	h.Observe(ctx, c, a, derefGroupID(groupID), session, started, r != nil && !r.ClientDisconnect, stream, latency, err)
}

// The native timeout handler remains authoritative for unmonitored accounts.
func (s *RateLimitService) HandleFirstOutputTimeout(ctx context.Context, a *Account, model string) bool {
	if s.providerHealthManaged(ctx, a) {
		return false
	}
	return s.HandleStreamTimeout(ctx, a, model)
}
