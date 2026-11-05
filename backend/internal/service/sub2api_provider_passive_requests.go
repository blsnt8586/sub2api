package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
)

// The same evidence feeds aggregate decisions and the per-request timeline.
// Success on the same account supersedes retry errors. Errors on another
// account still count against that account. Route changes start fresh evidence.
// Streaming latency is TTFT; an intentionally long answer is not a slow start.
const providerPassiveEventsSQL = `WITH monitored_accounts AS (
 SELECT a.id,GREATEST($2,COALESCE(MAX(t.route_changed_at),$2)) AS since
 FROM accounts a LEFT JOIN sub2api_provider_probe_targets t ON t.account_id=a.id AND t.provider_id=a.provider_id
 WHERE a.provider_id=$1 AND a.deleted_at IS NULL GROUP BY a.id
), events AS (
 SELECT u.account_id,u.user_id,u.created_at,
 CASE WHEN u.stream THEN u.first_token_ms ELSE u.duration_ms END AS latency,
 false AS failure,u.model, NULL::text AS message, 'u:' || u.id::text AS event_id,
 COALESCE(NULLIF(u.request_id,''), 'u:' || u.id::text) AS request_id
 FROM usage_logs u JOIN monitored_accounts a ON a.id=u.account_id
 WHERE u.created_at >= a.since AND u.created_at < $3
 UNION ALL
 SELECT o.account_id,MAX(o.user_id),MAX(o.created_at),
 MAX(COALESCE(o.time_to_first_token_ms,o.response_latency_ms,o.upstream_latency_ms,o.duration_ms)),
 true,MAX(o.model),MAX(o.error_message),'e:' || MIN(o.id)::text,
 COALESCE(NULLIF(MIN(o.request_id),''), MIN(o.id)::text)
 FROM ops_error_logs o JOIN monitored_accounts a ON a.id=o.account_id
 WHERE o.created_at >= a.since AND o.created_at < $3 AND ` + ProviderAttributableErrorSQL + `
 AND NOT EXISTS (SELECT 1 FROM usage_logs u WHERE u.account_id=o.account_id AND u.request_id=o.request_id AND u.created_at >= a.since AND u.created_at < $3)
 GROUP BY o.account_id,COALESCE(NULLIF(o.request_id,''),o.id::text)
) `

type providerPassiveRequest struct {
	AccountID int64
	UserID    int64
	RequestID string
	At        time.Time
	Latency   *int
	Failure   bool
	Model     string
	Message   *string
}

func (s *Sub2APIProviderProbeService) passiveRequestEvents(ctx context.Context, providerID int64, since, now time.Time, limit int) (map[int64][]providerPassiveRequest, error) {
	out := map[int64][]providerPassiveRequest{}
	if s.db == nil {
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, providerPassiveEventsSQL+`, ranked AS (
 SELECT *,ROW_NUMBER() OVER(PARTITION BY account_id ORDER BY created_at DESC,event_id DESC) AS rank FROM events
 ) SELECT account_id,user_id,created_at,latency,failure,COALESCE(model,''),message,request_id FROM ranked
 WHERE rank<=$4 ORDER BY account_id,rank`, providerID, since, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var e providerPassiveRequest
		var latency sql.NullInt64
		var userID sql.NullInt64
		if err := rows.Scan(&e.AccountID, &userID, &e.At, &latency, &e.Failure, &e.Model, &e.Message, &e.RequestID); err != nil {
			return nil, err
		}
		if userID.Valid {
			e.UserID = userID.Int64
		}
		if latency.Valid {
			v := int(latency.Int64)
			e.Latency = &v
		}
		out[e.AccountID] = append(out[e.AccountID], e)
	}
	return out, rows.Err()
}

func passiveRequestStatus(e providerPassiveRequest, degraded int) string {
	if e.Failure {
		return "unhealthy"
	}
	if e.Latency != nil && *e.Latency > degraded {
		return "degraded"
	}
	return "healthy"
}

func passiveRequestBuckets(events []providerPassiveRequest, degraded int) []Sub2APIProviderHealthBucket {
	out := make([]Sub2APIProviderHealthBucket, 0, len(events))
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		b := Sub2APIProviderHealthBucket{StartedAt: e.At, EndedAt: e.At, Status: passiveRequestStatus(e, degraded), SampleCount: 1, MaxHealthLatencyMS: e.Latency, LastError: e.Message}
		switch b.Status {
		case "healthy":
			b.HealthySamples = 1
		case "degraded":
			b.DegradedSamples = 1
		case "unhealthy":
			b.UnhealthySamples = 1
		}
		out = append(out, b)
	}
	return out
}

func applyPassiveRequestHealth(h *Sub2APIProviderProbeTargetHealth, o *ProviderPassiveObservation, events []providerPassiveRequest, p ProviderPassivePolicy, now time.Time) {
	h.MonitoringSource = "user_requests"
	h.Enabled = true // Legacy active-probe enablement never disables passive observations.
	h.Status = "unknown"
	h.PassiveHealthStatus = "unknown"
	h.LatencyMS = nil
	h.TrafficRequestCount = 0
	h.TrafficFailureCount = 0
	h.TrafficSuccessRate = nil
	h.TrafficP95LatencyMS = nil
	h.LastRequestAt = nil
	h.PassiveCooldownUntil = nil
	h.PassiveReason = ""
	h.LastCheckedAt = nil
	h.LastRunAt = nil
	h.ErrorCategory = nil
	h.ErrorMessage = nil
	h.ConsecutiveFailures = 0
	h.DegradedLatencyMS = p.DegradedLatencyMS
	h.Buckets = passiveRequestBuckets(events, p.DegradedLatencyMS)
	if len(events) > 0 {
		latest := events[0]
		h.LastCheckedAt = &latest.At
		h.LatencyMS = latest.Latency
		h.TestModel = nil
		if latest.Model != "" {
			h.TestModel = &latest.Model
		}
		if latest.Failure {
			h.ErrorMessage = latest.Message
		}
		for _, e := range events {
			if !e.Failure {
				break
			}
			h.ConsecutiveFailures++
		}
	} else {
		h.TestModel = nil
	}
	if o == nil {
		return
	}
	h.TrafficRequestCount = o.Requests
	h.TrafficFailureCount = o.Failures
	h.PassiveHealthStatus = o.Status
	h.LastRequestAt = o.LastRequestAt
	h.PassiveCooldownUntil = o.CooldownUntil
	h.PassiveReason = o.Reason
	if o.Requests > 0 {
		rate := float64(o.Requests-o.Failures) / float64(o.Requests) * 100
		h.TrafficSuccessRate = &rate
		if o.LatencySamples > 0 {
			v := o.P95
			h.TrafficP95LatencyMS = &v
		}
	}
	switch o.Status {
	case "healthy", "degraded":
		h.Status = o.Status
	case "temporarily_unschedulable":
		h.Status = "unhealthy"
	}
	// Show current exclusion even when a quiet account has too few new samples.
	if o.CooldownUntil != nil && o.CooldownUntil.After(now) && strings.HasPrefix(o.Reason, "passive health:") {
		h.Status = "unhealthy"
	}
}

func (s *Sub2APIProviderProbeService) passiveTargetHistory(ctx context.Context, target *ent.Sub2APIProviderProbeTarget, since time.Time, limit int) ([]*Sub2APIProviderProbeTargetHealth, error) {
	now := time.Now().UTC()
	state, err := s.AccountHealthPolicy(ctx, target.ProviderID, target.AccountID)
	if err != nil {
		return nil, err
	}
	byAccount, err := s.passiveRequestEvents(ctx, target.ProviderID, since, now, limit)
	if err != nil {
		return nil, fmt.Errorf("read local request history: %w", err)
	}
	out := make([]*Sub2APIProviderProbeTargetHealth, 0, len(byAccount[target.AccountID]))
	for _, e := range byAccount[target.AccountID] {
		h := &Sub2APIProviderProbeTargetHealth{ID: target.ID, ProviderID: target.ProviderID, AccountID: target.AccountID, MonitoringSource: "user_requests", Status: passiveRequestStatus(e, state.Policy.DegradedLatencyMS), LatencyMS: e.Latency, LastCheckedAt: &e.At, ErrorMessage: e.Message}
		if e.Model != "" {
			h.TestModel = &e.Model
		}
		out = append(out, h)
	}
	return out, nil
}
