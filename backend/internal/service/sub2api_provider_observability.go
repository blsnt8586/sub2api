package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ProviderAttributableErrorSQL deliberately excludes user validation, billing,
// cancellation, model support and platform capacity failures. Alias must be o.
const ProviderAttributableErrorSQL = `COALESCE(o.is_business_limited, false) = false
 AND COALESCE(o.status_code, 0) <> 499
 AND o.error_phase = 'upstream'
 AND COALESCE(o.error_owner, 'provider') = 'provider'
 AND COALESCE(o.error_type, '') NOT IN ('overloaded_error','model_not_found','invalid_request_error','content_policy_violation','cancelled','canceled')
 AND (COALESCE(o.upstream_status_code, o.status_code, 0) IN (401,403)
      OR COALESCE(o.upstream_status_code, o.status_code, 0) >= 500
      OR COALESCE(o.network_error_type, '') <> ''
      OR o.error_source IN ('upstream_network','upstream_stream')
      OR o.error_type IN ('stream_error','timeout_error','connection_error'))`

type ProviderPassivePolicy struct {
	Enforce           bool    `json:"enforce"`
	WindowSeconds     int     `json:"window_seconds"`
	MinimumSamples    int     `json:"minimum_samples"`
	DegradedLatencyMS int     `json:"degraded_latency_ms"`
	BlockLatencyMS    int     `json:"block_latency_ms"`
	DegradedErrorRate float64 `json:"degraded_error_rate"`
	BlockErrorRate    float64 `json:"block_error_rate"`
	FailureThreshold  int     `json:"failure_threshold"`
	CooldownSeconds   int     `json:"cooldown_seconds"`
	RecoveryCount     int     `json:"recovery_count"`
}

func DefaultProviderPassivePolicy() ProviderPassivePolicy {
	return ProviderPassivePolicy{WindowSeconds: 600, MinimumSamples: 3, DegradedLatencyMS: 5000, BlockLatencyMS: 60000, DegradedErrorRate: 20, BlockErrorRate: 50, FailureThreshold: 3, CooldownSeconds: 600, RecoveryCount: 3}
}
func (p ProviderPassivePolicy) Validate() error {
	if p.WindowSeconds < 60 || p.WindowSeconds > 86400 || p.MinimumSamples < 1 || p.MinimumSamples > 10000 || p.DegradedLatencyMS < 100 || p.BlockLatencyMS < p.DegradedLatencyMS || p.BlockLatencyMS > 600000 || p.DegradedErrorRate < 0 || p.DegradedErrorRate > 100 || p.BlockErrorRate < p.DegradedErrorRate || p.BlockErrorRate > 100 || p.FailureThreshold < 1 || p.FailureThreshold > 10000 || p.CooldownSeconds < 30 || p.CooldownSeconds > 86400 || p.RecoveryCount < 1 || p.RecoveryCount > 100 {
		return invalidProviderProbeConfig("invalid passive health thresholds")
	}
	return nil
}
func (s *Sub2APIProviderProbeService) PassivePolicy(ctx context.Context, id int64) (ProviderPassivePolicy, error) {
	p := DefaultProviderPassivePolicy()
	if s.db == nil {
		return p, nil
	}
	var raw []byte
	err := s.db.QueryRowContext(ctx, `SELECT policy FROM provider_passive_health_policies WHERE provider_id=$1`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(raw, &p)
	return p, err
}
func (s *Sub2APIProviderProbeService) SavePassivePolicy(ctx context.Context, id int64, p ProviderPassivePolicy) error {
	if err := p.Validate(); err != nil {
		return err
	}
	if _, err := s.providerRepo.GetByID(ctx, id); err != nil {
		return ErrProviderNotFound
	}
	if s.db == nil {
		return fmt.Errorf("observability database unavailable")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO provider_passive_health_policies(provider_id,policy) VALUES($1,$2::jsonb) ON CONFLICT(provider_id) DO UPDATE SET policy=EXCLUDED.policy,updated_at=NOW()`, id, string(raw))
	return err
}

type ProviderPassiveObservation struct {
	AccountID      int64      `json:"account_id"`
	Requests       int        `json:"requests"`
	Failures       int        `json:"failures"`
	P95            int        `json:"p95_ms"`
	LatencySamples int        `json:"latency_samples"`
	LastRequestAt  *time.Time `json:"last_request_at,omitempty"`
	LastFailureAt  *time.Time `json:"last_failure_at,omitempty"`
	CooldownUntil  *time.Time `json:"cooldown_until,omitempty"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
}

func passiveObservationStatus(p ProviderPassivePolicy, o ProviderPassiveObservation) string {
	if o.Requests < p.MinimumSamples {
		return "unknown"
	}
	rate := float64(o.Failures) / float64(o.Requests) * 100
	if o.Failures >= p.FailureThreshold || rate >= p.BlockErrorRate || o.P95 > p.BlockLatencyMS {
		return "temporarily_unschedulable"
	}
	if rate >= p.DegradedErrorRate || o.P95 > p.DegradedLatencyMS {
		return "degraded"
	}
	return "healthy"
}
func (s *Sub2APIProviderProbeService) PassiveObservations(ctx context.Context, id int64, p ProviderPassivePolicy, now time.Time) ([]ProviderPassiveObservation, error) {
	if s.db == nil {
		return []ProviderPassiveObservation{}, nil
	}
	// Collapse duplicate error events per request; a successful request supersedes
	// its intermediate error on the same account. Failover on another account still
	// counts as evidence against the original account.
	q := providerPassiveEventsSQL + `SELECT a.id,COUNT(e.account_id),COUNT(*) FILTER(WHERE e.failure),
 COALESCE(percentile_cont(0.95) WITHIN GROUP(ORDER BY e.latency) FILTER(WHERE NOT e.failure),0),
 COUNT(e.latency) FILTER(WHERE NOT e.failure),MAX(e.created_at),MAX(e.created_at) FILTER(WHERE e.failure),a.temp_unschedulable_until,COALESCE(a.temp_unschedulable_reason,'')
 FROM accounts a LEFT JOIN events e ON e.account_id=a.id
 WHERE a.provider_id=$1 AND a.deleted_at IS NULL
 GROUP BY a.id ORDER BY a.id`
	rows, err := s.db.QueryContext(ctx, q, id, now.Add(-time.Duration(p.WindowSeconds)*time.Second), now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ProviderPassiveObservation, 0)
	for rows.Next() {
		var o ProviderPassiveObservation
		var p95 float64
		if err = rows.Scan(&o.AccountID, &o.Requests, &o.Failures, &p95, &o.LatencySamples, &o.LastRequestAt, &o.LastFailureAt, &o.CooldownUntil, &o.Reason); err != nil {
			return nil, err
		}
		o.P95 = int(p95)
		o.Status = passiveObservationStatus(p, o)
		out = append(out, o)
	}
	return out, rows.Err()
}

type ProviderPassiveCooldownWriter interface {
	ApplyProviderPassiveCooldown(context.Context, int64, int64, time.Time, time.Time, string) error
}

func (s *Sub2APIProviderProbeService) evaluatePassiveHealth(ctx context.Context, id int64, now time.Time) error {
	p, err := s.PassivePolicy(ctx, id)
	if err != nil || !p.Enforce {
		return err
	}
	obs, err := s.PassiveObservations(ctx, id, p, now)
	if err != nil {
		return err
	}
	writer, ok := s.accountRepo.(ProviderPassiveCooldownWriter)
	if !ok {
		return nil
	}
	for _, o := range obs {
		if o.Status != "temporarily_unschedulable" || o.LastRequestAt == nil {
			continue
		}
		// Anchor cooldown to evidence, never extend it on every polling tick. Once
		// it expires the normal scheduler admits real traffic again; old failures
		// cannot re-open the same cooldown.
		evidenceAt := o.LastRequestAt
		if o.LastFailureAt != nil && o.P95 <= p.BlockLatencyMS {
			evidenceAt = o.LastFailureAt
		}
		until := evidenceAt.Add(time.Duration(p.CooldownSeconds) * time.Second)
		if !until.After(now) {
			continue
		}
		if err = writer.ApplyProviderPassiveCooldown(ctx, id, o.AccountID, now, until, fmt.Sprintf("passive health: %d/%d failures; P95 %d ms", o.Failures, o.Requests, o.P95)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sub2APIProviderProbeService) applyLiveProviderTraffic(ctx context.Context, h *Sub2APIProviderHealth) {
	if h == nil {
		return
	}
	states, err := s.accountHealthStates(ctx, h.ProviderID)
	if err != nil {
		return
	}
	now := time.Now().UTC()
	events, err := s.passiveRequestEvents(ctx, h.ProviderID, now.Add(-24*time.Hour), now, 10000)
	if err != nil {
		return
	}
	obs := make([]ProviderPassiveObservation, 0, len(states))
	for _, state := range states {
		o, _ := hybridObserve(state, events[state.AccountID], now)
		if state.Phase == "recovering" {
			o.Status = "temporarily_unschedulable"
		}
		obs = append(obs, o)
	}
	h.TrafficRequestCount = 0
	h.TrafficFailureCount = 0
	h.TrafficSuccessRate = nil
	h.TrafficP95LatencyMS = nil
	h.PassiveHealthStatus = "unknown"
	maxP95 := 0
	for _, o := range obs {
		h.TrafficRequestCount += o.Requests
		h.TrafficFailureCount += o.Failures
		if o.P95 > maxP95 {
			maxP95 = o.P95
		}
	}
	if h.TrafficRequestCount > 0 {
		rate := float64(h.TrafficRequestCount-h.TrafficFailureCount) / float64(h.TrafficRequestCount) * 100
		h.TrafficSuccessRate = &rate
		h.TrafficP95LatencyMS = &maxP95
		h.PassiveHealthStatus = "healthy"
		for _, o := range obs {
			if o.Status == "temporarily_unschedulable" {
				h.PassiveHealthStatus = o.Status
				break
			}
			if o.Status == "degraded" {
				h.PassiveHealthStatus = "degraded"
			}
		}
	}
}
