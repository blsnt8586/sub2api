package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
)

// A separate per-account policy replaces provider-wide passive enforcement.
// Healthy accounts generate no model tests. Only a latched quarantine permits
// automatic recovery tests; an elapsed timer never admits user traffic.
type ProviderAccountHealthPolicy struct {
	Enabled                 bool `json:"enabled"`
	WindowSeconds           int  `json:"window_seconds"`
	MinimumSamples          int  `json:"minimum_samples"`
	DegradedLatencyMS       int  `json:"degraded_latency_ms"`
	TimeoutMS               int  `json:"timeout_ms"`
	FailureThreshold        int  `json:"failure_threshold"`
	AccountFailureThreshold int  `json:"account_failure_threshold"`
	SessionAvoidSeconds     int  `json:"session_avoid_seconds"`
	RecoveryDelaySeconds    int  `json:"recovery_delay_seconds"`
	ProbeIntervalSeconds    int  `json:"probe_interval_seconds"`
	ProbeTimeoutSeconds     int  `json:"probe_timeout_seconds"`
	RecoverySuccesses       int  `json:"recovery_successes"`
}

func DefaultProviderAccountHealthPolicy() ProviderAccountHealthPolicy {
	return ProviderAccountHealthPolicy{Enabled: true, WindowSeconds: 300, MinimumSamples: 3, DegradedLatencyMS: 25000, TimeoutMS: 60000, FailureThreshold: 2, AccountFailureThreshold: 3, SessionAvoidSeconds: 300, RecoveryDelaySeconds: 300, ProbeIntervalSeconds: 60, ProbeTimeoutSeconds: 60, RecoverySuccesses: 2}
}
func (p ProviderAccountHealthPolicy) Validate() error {
	if p.SessionAvoidSeconds < 30 || p.SessionAvoidSeconds > 86400 {
		return invalidProviderProbeConfig("invalid session avoidance duration")
	}
	if p.WindowSeconds < 60 || p.WindowSeconds > 86400 || p.MinimumSamples < 1 || p.MinimumSamples > 10000 || p.DegradedLatencyMS < 100 || p.TimeoutMS < p.DegradedLatencyMS || p.TimeoutMS > 600000 || p.FailureThreshold < 1 || p.FailureThreshold > 100 || p.AccountFailureThreshold < 1 || p.AccountFailureThreshold > 10000 || p.RecoveryDelaySeconds < 30 || p.RecoveryDelaySeconds > 86400 || p.ProbeIntervalSeconds < 30 || p.ProbeIntervalSeconds > 86400 || p.ProbeTimeoutSeconds < 3 || p.ProbeTimeoutSeconds > 120 || p.RecoverySuccesses < 1 || p.RecoverySuccesses > 20 {
		return invalidProviderProbeConfig("invalid account hybrid health policy")
	}
	return nil
}

type ProviderAccountHealthState struct {
	AccountID          int64                       `json:"account_id"`
	ProviderID         int64                       `json:"provider_id"`
	Policy             ProviderAccountHealthPolicy `json:"policy"`
	Revision           int64                       `json:"revision"`
	Phase              string                      `json:"phase"`
	ObserveSince       time.Time                   `json:"observe_since"`
	Token              string                      `json:"-"`
	AdminGeneration    string                      `json:"-"`
	Reason             string                      `json:"reason"`
	RecoverySuccesses  int                         `json:"recovery_successes"`
	LastProbeAt        *time.Time                  `json:"last_probe_at,omitempty"`
	LastProbeStatus    string                      `json:"last_probe_status"`
	LastProbeLatencyMS *int                        `json:"last_probe_latency_ms,omitempty"`
	NextProbeAt        *time.Time                  `json:"next_probe_at,omitempty"`
}

const hybridStateColumns = `account_id,provider_id,policy,revision,phase,observe_since,quarantine_token,admin_generation,reason,recovery_successes,last_probe_at,last_probe_status,last_probe_latency_ms,next_probe_at`

type hybridScanner interface{ Scan(...any) error }

func scanHybridState(row hybridScanner) (ProviderAccountHealthState, error) {
	var h ProviderAccountHealthState
	var raw []byte
	err := row.Scan(&h.AccountID, &h.ProviderID, &raw, &h.Revision, &h.Phase, &h.ObserveSince, &h.Token, &h.AdminGeneration, &h.Reason, &h.RecoverySuccesses, &h.LastProbeAt, &h.LastProbeStatus, &h.LastProbeLatencyMS, &h.NextProbeAt)
	if err == nil {
		h.Policy = DefaultProviderAccountHealthPolicy()
		err = json.Unmarshal(raw, &h.Policy)
	}
	return h, err
}
func (s *Sub2APIProviderProbeService) accountHealthStates(ctx context.Context, providerID int64) (map[int64]ProviderAccountHealthState, error) {
	out := map[int64]ProviderAccountHealthState{}
	if s.db == nil {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE provider_id=$1`, providerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		h, e := scanHybridState(rows)
		if e != nil {
			return nil, e
		}
		out[h.AccountID] = h
	}
	return out, rows.Err()
}
func (s *Sub2APIProviderProbeService) AccountHealthPolicy(ctx context.Context, providerID, accountID int64) (*ProviderAccountHealthState, error) {
	a, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if a.ProviderID == nil || *a.ProviderID != providerID {
		return nil, invalidProviderProbeConfig("account is not linked to provider")
	}
	states, err := s.accountHealthStates(ctx, providerID)
	if err != nil {
		return nil, err
	}
	h, ok := states[accountID]
	if !ok {
		h = ProviderAccountHealthState{AccountID: accountID, ProviderID: providerID, Policy: DefaultProviderAccountHealthPolicy(), Phase: "observing"}
	}
	return &h, nil
}

// Lock account before state in every mutation. This serializes administrator
// edits, rebinding, policy saves and stale in-flight recovery results.
func hybridLockAccount(ctx context.Context, tx *sql.Tx, providerID, accountID int64) (generation, reason, status string, schedulable bool, err error) {
	err = tx.QueryRowContext(ctx, `SELECT COALESCE(extra->>'sub2api_admin_account_state_generation','') || '|' || COALESCE(provider_api_key_id,0)::text || '|' || COALESCE(remote_group_id,0)::text,CASE WHEN temp_unschedulable_until>NOW() THEN COALESCE(temp_unschedulable_reason,'') ELSE '' END,
 CASE WHEN status='error' AND (extra @> '{"sub2api_runtime_recoverable_account_error":true}' OR extra @> '{"sub2api_probe_managed_account_status":true}') THEN 'active' ELSE status END,
 (schedulable OR (status='error' AND (extra @> '{"sub2api_runtime_recoverable_account_error":true}' OR extra @> '{"sub2api_probe_managed_account_status":true}'))) FROM accounts WHERE id=$1 AND provider_id=$2 AND deleted_at IS NULL FOR UPDATE`, accountID, providerID).Scan(&generation, &reason, &status, &schedulable)
	return
}
func (s *Sub2APIProviderProbeService) SaveAccountHealthPolicy(ctx context.Context, providerID, accountID int64, p ProviderAccountHealthPolicy) (*ProviderAccountHealthState, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if s.db == nil {
		return nil, fmt.Errorf("health database unavailable")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, _, _, _, err = hybridLockAccount(ctx, tx, providerID, accountID)
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(p)
	// Saving a changed policy invalidates in-flight results. Keep quarantine when
	// enabled; disabling monitoring only clears a block this state still owns.
	old, err := scanHybridState(tx.QueryRowContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE account_id=$1 FOR UPDATE`, accountID))
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if !p.Enabled && old.Token != "" {
		if err = hybridClearOwnedBlock(ctx, tx, accountID, old.Token); err != nil {
			return nil, err
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO provider_account_hybrid_health(account_id,provider_id,policy) VALUES($1,$2,$3::jsonb)
 ON CONFLICT(account_id) DO UPDATE SET provider_id=EXCLUDED.provider_id,policy=EXCLUDED.policy,revision=provider_account_hybrid_health.revision+1,
 phase=CASE WHEN $4 THEN provider_account_hybrid_health.phase ELSE 'observing' END,
 observe_since=CASE WHEN provider_account_hybrid_health.policy=EXCLUDED.policy THEN provider_account_hybrid_health.observe_since ELSE NOW() END,
 quarantine_token=CASE WHEN $4 THEN provider_account_hybrid_health.quarantine_token ELSE '' END,
 recovery_successes=0,next_probe_at=CASE WHEN $4 AND provider_account_hybrid_health.phase='recovering' THEN NOW()+($5 * INTERVAL '1 second') ELSE NULL END,updated_at=NOW()`, accountID, providerID, string(raw), p.Enabled, p.RecoveryDelaySeconds)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if !p.Enabled && old.Token != "" {
		s.invalidateHybridTempCache(ctx, accountID)
	}
	if s.rateLimitService != nil && s.rateLimitService.providerHealth != nil {
		s.rateLimitService.providerHealth.policies.Delete(fmt.Sprint(accountID))
	}
	return s.AccountHealthPolicy(ctx, providerID, accountID)
}
func hybridClearOwnedBlock(ctx context.Context, tx *sql.Tx, accountID int64, token string) error {
	_, err := tx.ExecContext(ctx, `WITH changed AS (UPDATE accounts SET temp_unschedulable_until=NULL,temp_unschedulable_reason='',
 status=CASE WHEN status='error' AND (extra @> '{"sub2api_runtime_recoverable_account_error":true}' OR extra @> '{"sub2api_probe_managed_account_status":true}') THEN 'active' ELSE status END,
 schedulable=CASE WHEN status='error' AND (extra @> '{"sub2api_runtime_recoverable_account_error":true}' OR extra @> '{"sub2api_probe_managed_account_status":true}') THEN true ELSE schedulable END,
 extra=COALESCE(extra,'{}'::jsonb)||'{"sub2api_runtime_recoverable_account_error":false,"sub2api_probe_managed_account_status":false}'::jsonb,updated_at=NOW() WHERE id=$1 AND temp_unschedulable_reason=$2 RETURNING id) INSERT INTO scheduler_outbox(event_type,account_id) SELECT $3,id FROM changed`, accountID, "hybrid health: "+token, SchedulerOutboxEventAccountChanged)
	return err
}
func (s *Sub2APIProviderProbeService) initializeAccountHealth(ctx context.Context, providerID int64) error {
	raw, _ := json.Marshal(DefaultProviderAccountHealthPolicy())
	_, err := s.db.ExecContext(ctx, `INSERT INTO provider_account_hybrid_health(account_id,provider_id,policy)
 SELECT id,provider_id,$2::jsonb FROM accounts WHERE provider_id=$1 AND deleted_at IS NULL
 ON CONFLICT(account_id) DO UPDATE SET provider_id=EXCLUDED.provider_id,revision=provider_account_hybrid_health.revision+1,
 observe_since=NOW(),admin_generation='',recovery_successes=0,next_probe_at=NOW()
 WHERE provider_account_hybrid_health.provider_id<>EXCLUDED.provider_id`, providerID, string(raw))
	return err
}
func hybridPolicyAsPassive(p ProviderAccountHealthPolicy) ProviderPassivePolicy {
	return ProviderPassivePolicy{WindowSeconds: p.WindowSeconds, MinimumSamples: p.MinimumSamples, DegradedLatencyMS: p.DegradedLatencyMS, BlockLatencyMS: p.TimeoutMS, FailureThreshold: p.FailureThreshold, DegradedErrorRate: 101, BlockErrorRate: 101}
}
func hybridObserve(h ProviderAccountHealthState, events []providerPassiveRequest, now time.Time) (ProviderPassiveObservation, string) {
	o := ProviderPassiveObservation{AccountID: h.AccountID}
	var latencies []int
	// passiveRequestEvents is returned newest-first for the admin timeline. Health
	// decisions must be evaluated oldest-to-newest so a successful request really
	// resets a prior timeout/failure streak.
	ordered := append([]providerPassiveRequest(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].At.Equal(ordered[j].At) {
			return ordered[i].RequestID < ordered[j].RequestID
		}
		return ordered[i].At.Before(ordered[j].At)
	})
	trigger := ""
	badRequests := map[string]struct{}{}
	since := now.Add(-time.Duration(h.Policy.WindowSeconds) * time.Second)
	if h.ObserveSince.After(since) {
		since = h.ObserveSince
	}
	for index, e := range ordered {
		if e.At.Before(since) || !e.At.Before(now) {
			continue
		}
		o.Requests++
		if e.Failure {
			o.Failures++
			t := e.At
			o.LastFailureAt = &t
		}
		if !e.Failure && e.Latency != nil {
			latencies = append(latencies, *e.Latency)
		}
		isTimeout := !e.Failure && e.Latency != nil && *e.Latency > h.Policy.TimeoutMS
		isBad := e.Failure || isTimeout
		if isBad {
			requestID := e.RequestID
			if requestID == "" {
				requestID = fmt.Sprintf("event:%d:%d", e.At.UnixNano(), index)
			}
			badRequests[requestID] = struct{}{}
		}
	}
	if len(badRequests) >= h.Policy.AccountFailureThreshold {
		trigger = fmt.Sprintf("%d failed or timed-out requests within %d seconds", len(badRequests), h.Policy.WindowSeconds)
	}
	if len(ordered) > 0 {
		for i := len(ordered) - 1; i >= 0; i-- {
			e := ordered[i]
			if e.At.Before(since) || !e.At.Before(now) {
				continue
			}
			t := e.At
			o.LastRequestAt = &t
			break
		}
	}
	if len(latencies) > 0 {
		sort.Ints(latencies)
		index := int(math.Ceil(.95*float64(len(latencies)))) - 1
		o.P95 = latencies[index]
		o.LatencySamples = len(latencies)
	}
	o.Status = "unknown"
	if o.Requests >= h.Policy.MinimumSamples {
		o.Status = "healthy"
		if o.P95 > h.Policy.DegradedLatencyMS {
			o.Status = "degraded"
		}
	}
	if trigger != "" {
		// Historical logs have no session identity and can miss recovered
		// failover attempts. They describe latency/availability, never isolate.
		o.Status = "degraded"
	}
	return o, trigger
}
func (s *Sub2APIProviderProbeService) enterHybridRecovery(ctx context.Context, h ProviderAccountHealthState, reason string, now time.Time) error {
	return s.enterHybridRecoveryWithGeneration(ctx, h, "", reason, now)
}

func (s *Sub2APIProviderProbeService) enterHybridRecoveryWithGeneration(ctx context.Context, h ProviderAccountHealthState, expectedGeneration, reason string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	generation, block, status, sched, err := hybridLockAccount(ctx, tx, h.ProviderID, h.AccountID)
	if err != nil {
		return err
	}
	// Never take ownership of another cooldown, even a shorter native breaker.
	if status != StatusActive || !sched || block != "" {
		return nil
	}
	current, err := scanHybridState(tx.QueryRowContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE account_id=$1 FOR UPDATE`, h.AccountID))
	if err != nil {
		return err
	}
	if expectedGeneration != "" {
		var actualGeneration string
		if err = tx.QueryRowContext(ctx, `SELECT h.revision::text || ':' || h.observe_since::text || ':' || COALESCE(a.extra->>'sub2api_admin_account_state_generation','') || ':' || COALESCE(a.provider_api_key_id,0)::text || ':' || COALESCE(a.remote_group_id,0)::text FROM provider_account_hybrid_health h JOIN accounts a ON a.id=h.account_id WHERE h.account_id=$1`, h.AccountID).Scan(&actualGeneration); err != nil {
			return err
		}
		if actualGeneration != expectedGeneration {
			return nil
		}
	}
	if current.Revision != h.Revision || current.Phase != "observing" || !current.Policy.Enabled {
		return nil
	}
	token := uuid.NewString()
	// Far-future latch: only successful recovery or an explicit policy disable
	// may release this ownership token. A timer expiry cannot return traffic.
	_, err = tx.ExecContext(ctx, `WITH changed AS (UPDATE accounts SET status='error',schedulable=false,extra=COALESCE(extra,'{}'::jsonb)||'{"sub2api_probe_managed_account_status":true}'::jsonb,temp_unschedulable_until='9999-01-01T00:00:00Z',temp_unschedulable_reason=$2,updated_at=NOW() WHERE id=$1 RETURNING id)
 INSERT INTO scheduler_outbox(event_type,account_id) SELECT $3,id FROM changed`, h.AccountID, "hybrid health: "+token, SchedulerOutboxEventAccountChanged)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET phase='recovering',observe_since=$5,quarantine_token=$2,admin_generation=$3,reason=$4,recovery_successes=0,last_probe_at=NULL,last_probe_status='',last_probe_latency_ms=NULL,next_probe_at=$6,updated_at=NOW() WHERE account_id=$1`, h.AccountID, token, generation, reason, now, now.Add(time.Duration(current.Policy.RecoveryDelaySeconds)*time.Second))
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Sub2APIProviderProbeService) finishHybridRecoveryProbe(ctx context.Context, h ProviderAccountHealthState, healthy bool, latency *int, reason string, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	generation, block, status, sched, err := hybridLockAccount(ctx, tx, h.ProviderID, h.AccountID)
	if err != nil {
		return err
	}
	current, err := scanHybridState(tx.QueryRowContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE account_id=$1 FOR UPDATE`, h.AccountID))
	if err != nil {
		return err
	}
	if !hybridSameTime(current.LastProbeAt, h.LastProbeAt) {
		return nil
	}
	if current.Revision != h.Revision || current.Token != h.Token || current.Phase != "recovering" || !current.Policy.Enabled || generation != h.AdminGeneration || block != "hybrid health: "+h.Token || status != StatusActive || !sched {
		return nil
	}
	streak := 0
	probeStatus := "unhealthy"
	phase := "recovering"
	token := h.Token
	if healthy {
		streak = current.RecoverySuccesses + 1
		probeStatus = "healthy"
	}
	if streak >= current.Policy.RecoverySuccesses {
		if err = hybridClearOwnedBlock(ctx, tx, h.AccountID, h.Token); err != nil {
			return err
		}
		phase = "observing"
		token = ""
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET phase=$2,quarantine_token=$3,recovery_successes=$4,last_probe_at=$5,last_probe_status=$6,last_probe_latency_ms=$7,reason=$8,next_probe_at=$9,
 observe_since=CASE WHEN $2='observing' THEN $5 ELSE observe_since END,updated_at=NOW() WHERE account_id=$1`, h.AccountID, phase, token, streak, now, probeStatus, latency, reason, now.Add(time.Duration(current.Policy.ProbeIntervalSeconds)*time.Second))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if phase == "observing" {
		s.invalidateHybridTempCache(ctx, h.AccountID)
	}
	return nil
}
func (s *Sub2APIProviderProbeService) runHybridRecoveryProbe(ctx context.Context, h ProviderAccountHealthState, target *ent.Sub2APIProviderProbeTarget) error {
	states, err := s.accountHealthStates(ctx, h.ProviderID)
	if err != nil {
		return err
	}
	current, ok := states[h.AccountID]
	if !ok || current.Phase != "recovering" || !current.Policy.Enabled || current.Token != h.Token || current.Revision != h.Revision {
		return nil
	}
	h = current
	a, err := s.accountRepo.GetByID(ctx, h.AccountID)
	if err != nil {
		return err
	}
	if a.ProviderID == nil || *a.ProviderID != h.ProviderID {
		return nil
	}
	if (!probeOwnsAbnormalAccountStatus(a) && (a.Status != StatusActive || !a.Schedulable)) || a.TempUnschedulableReason != "hybrid health: "+h.Token {
		return s.abandonHybridRecovery(ctx, h)
	}
	if hybridAccountGeneration(a) != h.AdminGeneration {
		return s.refreshHybridRecoveryIdentity(ctx, h)
	} // changed binding must pass a new recovery streak
	now := time.Now().UTC()
	if h.NextProbeAt != nil && now.Before(*h.NextProbeAt) {
		return nil
	}
	// Reuse tested challenge-response handling, timeout/cancellation, credentials
	// and the provider operation gate. Never run legacy account status projection.
	copyTarget := *target
	copyTarget.TimeoutSeconds = h.Policy.ProbeTimeoutSeconds
	copyTarget.DegradedLatencyMs = h.Policy.TimeoutMS
	if configuredAccountTestModel(a) == nil || (a.IsCanvas() || isProviderProbeMediaModel(*configuredAccountTestModel(a))) && !copyTarget.AllowMediaProbe {
		return s.finishHybridRecoveryProbe(ctx, h, false, nil, "configure a text test model or explicitly allow media recovery probes", now)
	}
	run, _, acquired, err := s.runTargetIfProviderAvailable(ctx, &copyTarget)
	if !acquired {
		return nil
	}
	if err != nil {
		return s.finishHybridRecoveryProbe(ctx, h, false, nil, trimProbeError(err), time.Now().UTC())
	}
	if run == nil {
		return nil
	}
	healthy := string(run.Status) == "healthy" && run.LatencyMs != nil && *run.LatencyMs <= h.Policy.TimeoutMS
	message := ""
	if run.ErrorMessage != nil {
		message = *run.ErrorMessage
	}
	if !healthy && message == "" {
		message = "recovery probe exceeded timeout"
	}
	return s.finishHybridRecoveryProbe(ctx, h, healthy, run.LatencyMs, message, time.Now().UTC())
}
func (s *Sub2APIProviderProbeService) runHybridAccountHealth(ctx context.Context, providerID int64) error {
	if s.db == nil {
		return nil
	}
	if err := s.initializeAccountHealth(ctx, providerID); err != nil {
		return err
	}
	states, err := s.accountHealthStates(ctx, providerID)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	targets, err := s.ensureTargets(ctx, providerID)
	if err != nil {
		return err
	}
	for _, target := range targets {
		h, ok := states[target.AccountID]
		if !ok || !h.Policy.Enabled {
			continue
		}
		if h.Phase == "observing" {
			// Runtime evidence includes every upstream attempt, including errors
			// hidden by successful failover. SQL request totals remain metrics only.
			if s.rateLimitService == nil || s.rateLimitService.providerHealth == nil {
				continue
			}
			runtime := s.rateLimitService.providerHealth
			p, enabled := runtime.policy(ctx, h.AccountID)
			if !enabled || !p.Since.Equal(h.ObserveSince) {
				continue
			}
			readCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
			failures, e := runtime.cache.ProviderRecentFailures(readCtx, h.AccountID, p.Generation, now, h.Policy.WindowSeconds)
			cancel()
			if e != nil {
				return e
			}
			if failures >= h.Policy.AccountFailureThreshold {
				reason := fmt.Sprintf("%d failed or timed-out requests within %d seconds", failures, h.Policy.WindowSeconds)
				if err = s.enterHybridRecovery(ctx, h, reason, now); err != nil {
					return err
				}
			}
			continue
		}
		if h.NextProbeAt == nil {
			_, err = s.db.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET next_probe_at=NOW()+($4 * INTERVAL '1 second'),updated_at=NOW() WHERE account_id=$1 AND phase='recovering' AND quarantine_token=$2 AND revision=$3 AND next_probe_at IS NULL`, h.AccountID, h.Token, h.Revision, h.Policy.RecoveryDelaySeconds)
			if err != nil {
				return err
			}
			continue
		}
		if !now.Before(*h.NextProbeAt) {
			s.scheduleHybridRecoveryProbe(h, target)
		}
	}
	return nil
}

func hybridAccountGeneration(a *Account) string {
	keyID, groupID := int64(0), int64(0)
	if a.ProviderAPIKeyID != nil {
		keyID = *a.ProviderAPIKeyID
	}
	if a.RemoteGroupID != nil {
		groupID = *a.RemoteGroupID
	}
	return fmt.Sprintf("%s|%d|%d", sub2APIAdminAccountStateGeneration(a), keyID, groupID)
}

func (s *Sub2APIProviderProbeService) refreshHybridRecoveryIdentity(ctx context.Context, h ProviderAccountHealthState) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	generation, block, status, sched, err := hybridLockAccount(ctx, tx, h.ProviderID, h.AccountID)
	if err != nil {
		return err
	}
	if status != StatusActive || !sched || block != "hybrid health: "+h.Token {
		return nil
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET admin_generation=$2,revision=revision+1,recovery_successes=0,next_probe_at=NOW()+($5 * INTERVAL '1 second'),observe_since=NOW(),updated_at=NOW() WHERE account_id=$1 AND revision=$3 AND quarantine_token=$4 AND phase='recovering'`, h.AccountID, generation, h.Revision, h.Token, h.Policy.RecoveryDelaySeconds)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Sub2APIProviderProbeService) scheduleHybridRecoveryProbe(h ProviderAccountHealthState, target *ent.Sub2APIProviderProbeTarget) {
	// Acquire before spawning so every scheduler tick cannot build a backlog.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	release, acquired := tryAcquireSingletonLeaderLock(ctx, s.lockCache, s.db, fmt.Sprintf("sub2api:hybrid-recovery:%d", h.AccountID), s.instanceID, time.Duration(h.Policy.ProbeTimeoutSeconds+30)*time.Second)
	cancel()
	if !acquired {
		return
	}
	copyTarget := *target
	go func() {
		defer release()
		workCtx, cancel := context.WithTimeout(context.Background(), time.Duration(h.Policy.ProbeTimeoutSeconds+10)*time.Second)
		defer cancel()
		if err := s.runHybridRecoveryProbe(workCtx, h, &copyTarget); err != nil {
			logger.LegacyPrintf("service.sub2api_provider_probe", "account recovery probe: %v", err)
		}
	}()
}

func (s *Sub2APIProviderProbeService) invalidateHybridTempCache(ctx context.Context, accountID int64) {
	if s.rateLimitService != nil && s.rateLimitService.tempUnschedCache != nil {
		if err := s.rateLimitService.tempUnschedCache.DeleteTempUnsched(ctx, accountID); err != nil {
			logger.LegacyPrintf("service.sub2api_provider_probe", "invalidate recovered account cooldown cache: %v", err)
		}
	}
}

func hybridSameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Equal(*b)
}

// Manual pause/reset wins. Drop stale recovery state without clearing a
// foreign cooldown or changing the administrator's status/schedulable choice.
func (s *Sub2APIProviderProbeService) abandonHybridRecovery(ctx context.Context, h ProviderAccountHealthState) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, block, status, sched, err := hybridLockAccount(ctx, tx, h.ProviderID, h.AccountID)
	if err != nil {
		return err
	}
	current, err := scanHybridState(tx.QueryRowContext(ctx, `SELECT `+hybridStateColumns+` FROM provider_account_hybrid_health WHERE account_id=$1 FOR UPDATE`, h.AccountID))
	if err != nil {
		return err
	}
	if current.Revision != h.Revision || current.Token != h.Token || current.Phase != "recovering" {
		return nil
	}
	if status == StatusActive && sched && block == "hybrid health: "+h.Token {
		return nil
	}
	if block == "hybrid health: "+h.Token {
		if err = hybridClearOwnedBlock(ctx, tx, h.AccountID, h.Token); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET phase='observing',observe_since=NOW(),quarantine_token='',recovery_successes=0,next_probe_at=NULL,reason='',revision=revision+1,updated_at=NOW() WHERE account_id=$1`, h.AccountID)
	if err != nil {
		return err
	}
	return tx.Commit()
}
