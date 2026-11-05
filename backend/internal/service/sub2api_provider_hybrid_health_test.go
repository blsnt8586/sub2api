//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/sub2apiproviderprobetargetrun"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	cache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

func TestHybridObserveUsesAccountThresholdsAndFreshEvidence(t *testing.T) {
	now := time.Now().UTC()
	p := DefaultProviderAccountHealthPolicy()
	require.Equal(t, 25000, p.DegradedLatencyMS)
	require.Equal(t, 2, p.FailureThreshold)
	require.Equal(t, 3, p.AccountFailureThreshold)
	require.Equal(t, 300, p.WindowSeconds)
	require.Equal(t, 300, p.RecoveryDelaySeconds)
	h := ProviderAccountHealthState{Policy: p, ObserveSince: now.Add(-10 * time.Minute)}
	failure := func(user int64, seconds int) providerPassiveRequest {
		return providerPassiveRequest{UserID: user, RequestID: fmt.Sprintf("u%d-r%d", user, seconds), At: now.Add(-time.Duration(seconds) * time.Second), Failure: true}
	}
	slow := 60001
	fast := 100
	cases := []struct {
		name      string
		events    []providerPassiveRequest
		threshold int
		trip      bool
	}{
		{"no traffic", nil, 3, false},
		{"one failure waits with default", []providerPassiveRequest{failure(1, 1)}, 3, false},
		{"two failures for one user do not quarantine account", []providerPassiveRequest{failure(1, 1), failure(1, 2)}, 3, false},
		{"three failures for one user quarantine account", []providerPassiveRequest{failure(1, 1), failure(1, 2), failure(1, 3)}, 3, true},
		{"three failures for different users quarantine account", []providerPassiveRequest{failure(1, 1), failure(2, 2), failure(3, 3)}, 3, true},
		{"custom threshold waits", []providerPassiveRequest{failure(1, 1), failure(1, 2), failure(1, 3)}, 4, false},
		{"success does not erase account failures", []providerPassiveRequest{failure(1, 4), failure(1, 3), {UserID: 1, RequestID: "ok", At: now.Add(-2 * time.Second), Latency: &fast}, failure(1, 1)}, 3, true},
		{"single timeout does not quarantine", []providerPassiveRequest{{UserID: 1, RequestID: "slow", At: now.Add(-time.Second), Latency: &slow}}, 3, false},
		{"three timeouts quarantine", []providerPassiveRequest{{UserID: 1, RequestID: "slow-1", At: now.Add(-3 * time.Second), Latency: &slow}, {UserID: 1, RequestID: "slow-2", At: now.Add(-2 * time.Second), Latency: &slow}, {UserID: 1, RequestID: "slow-3", At: now.Add(-time.Second), Latency: &slow}}, 3, true},
		{"old failures ignored", []providerPassiveRequest{failure(1, 301), failure(1, 302), failure(1, 303)}, 3, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.Policy.AccountFailureThreshold = tc.threshold
			_, reason := hybridObserve(h, tc.events, now)
			require.Equal(t, tc.trip, reason != "")
		})
	}
	h.ObserveSince = now
	_, reason := hybridObserve(h, []providerPassiveRequest{failure(1, 1), failure(1, 2), failure(1, 3)}, now)
	require.Empty(t, reason, "successful recovery must not relatch old failures")
	other := h
	other.ObserveSince = now.Add(-time.Hour)
	other.Policy.AccountFailureThreshold = 1
	_, reason = hybridObserve(other, []providerPassiveRequest{failure(1, 1)}, now)
	require.NotEmpty(t, reason, "each account uses its own settings")
}

func TestHybridPolicyValidation(t *testing.T) {
	p := DefaultProviderAccountHealthPolicy()
	require.NoError(t, p.Validate())
	p.TimeoutMS = 100
	require.Error(t, p.Validate())
	p = DefaultProviderAccountHealthPolicy()
	p.RecoverySuccesses = 0
	require.Error(t, p.Validate())
	p = DefaultProviderAccountHealthPolicy()
	p.ProbeIntervalSeconds = 1
	require.Error(t, p.Validate())
	p = DefaultProviderAccountHealthPolicy()
	p.RecoveryDelaySeconds = 0
	require.Error(t, p.Validate())
}

type hybridTestAccountRepo struct {
	Sub2APIAccountRepository
	db *sql.DB
}

type providerHealthCacheSimulation struct {
	seen     map[string]struct{}
	failures int
}

func (c *providerHealthCacheSimulation) ObserveProviderRequestHealth(_ context.Context, e ProviderRequestHealthEvent) error {
	if c.seen == nil {
		c.seen = map[string]struct{}{}
	}
	if _, ok := c.seen[e.RequestID]; ok {
		return nil
	}
	c.seen[e.RequestID] = struct{}{}
	if e.Failed {
		c.failures++
	}
	return nil
}
func (*providerHealthCacheSimulation) ProviderSessionAvoidances(context.Context, string, time.Time) ([]ProviderSessionAvoidance, error) {
	return nil, nil
}
func (c *providerHealthCacheSimulation) ProviderRecentFailures(context.Context, int64, string, time.Time, int) (int, error) {
	return c.failures, nil
}

func (r hybridTestAccountRepo) GetByID(ctx context.Context, id int64) (*Account, error) {
	a := &Account{ID: id, Sub2APITestModel: providerProbeStringPtr("test-model")}
	var providerID int64
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT provider_id,status,schedulable,extra,COALESCE(temp_unschedulable_reason,''),provider_api_key_id,remote_group_id FROM accounts WHERE id=$1`, id).Scan(&providerID, &a.Status, &a.Schedulable, &raw, &a.TempUnschedulableReason, &a.ProviderAPIKeyID, &a.RemoteGroupID)
	if err == nil {
		err = json.Unmarshal(raw, &a.Extra)
	}
	a.ProviderID = &providerID
	return a, err
}

func TestHybridHealthPostgresQuarantineRecoveryAndOwnership(t *testing.T) {
	dsn := os.Getenv("PROVIDER_PASSIVE_TEST_DSN")
	if dsn == "" {
		t.Skip("requires PROVIDER_PASSIVE_TEST_DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE accounts(id bigint primary key,provider_id bigint,provider_api_key_id bigint,remote_group_id bigint,
 status text DEFAULT 'active',schedulable boolean DEFAULT true,extra jsonb DEFAULT '{}'::jsonb,deleted_at timestamptz,
 temp_unschedulable_until timestamptz,temp_unschedulable_reason text DEFAULT '',updated_at timestamptz DEFAULT NOW());
 CREATE TEMP TABLE scheduler_outbox(event_type text,account_id bigint);
 CREATE TEMP TABLE provider_account_hybrid_health(account_id bigint primary key,provider_id bigint,policy jsonb,revision bigint default 1,
 phase text default 'observing',observe_since timestamptz default NOW(),quarantine_token text default '',admin_generation text default '',reason text default '',
 recovery_successes int default 0,last_probe_at timestamptz,last_probe_status text default '',last_probe_latency_ms int,next_probe_at timestamptz,updated_at timestamptz default NOW());
 INSERT INTO accounts(id,provider_id) VALUES(1,18),(2,18),(3,18);`)
	require.NoError(t, err)
	s := &Sub2APIProviderProbeService{db: db, accountRepo: hybridTestAccountRepo{db: db}}
	require.NoError(t, s.initializeAccountHealth(ctx, 18))
	states := func() map[int64]ProviderAccountHealthState {
		v, e := s.accountHealthStates(ctx, 18)
		require.NoError(t, e)
		return v
	}
	block := func(id int64) string {
		var b string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT COALESCE(temp_unschedulable_reason,'') FROM accounts WHERE id=$1`, id).Scan(&b))
		return b
	}
	now := time.Now().UTC()
	latency := 100
	require.NoError(t, s.enterHybridRecovery(ctx, states()[1], "3 failures", now))
	h := states()[1]
	require.Equal(t, "recovering", h.Phase)
	require.Equal(t, "observing", states()[2].Phase)
	require.Equal(t, "hybrid health: "+h.Token, block(1))
	require.WithinDuration(t, now.Add(5*time.Minute), *h.NextProbeAt, time.Second)
	var status string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM accounts WHERE id=1`).Scan(&status))
	require.Equal(t, "error", status)
	var blocked bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT temp_unschedulable_until>NOW()+interval '1 year' FROM accounts WHERE id=1`).Scan(&blocked))
	require.True(t, blocked, "time passing must not automatically admit user traffic")
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, h, true, &latency, "", now.Add(time.Minute)))
	require.Equal(t, 1, states()[1].RecoverySuccesses)
	require.NotEmpty(t, block(1))
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, h, true, &latency, "duplicate completion", now.Add(time.Minute)))
	require.Equal(t, 1, states()[1].RecoverySuccesses, "a duplicate completion must not count twice")
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, states()[1], false, &latency, "probe failed", now.Add(2*time.Minute)))
	require.Zero(t, states()[1].RecoverySuccesses)
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, states()[1], true, &latency, "", now.Add(3*time.Minute)))
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, states()[1], true, &latency, "", now.Add(4*time.Minute)))
	require.Equal(t, "observing", states()[1].Phase)
	require.Empty(t, block(1))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM accounts WHERE id=1`).Scan(&status))
	require.Equal(t, "active", status)
	// Recovery tests use only a quarantined account and stop immediately after
	// the configured consecutive successes. No external model requests occur.
	probe := &hybridProbeRunnerStub{}
	s.accountTest = probe
	s.probeRepo = &hybridProbeRepoStub{}
	_, err = db.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET observe_since=NOW()-interval '1 hour' WHERE account_id=2`)
	require.NoError(t, err)
	target := &ent.Sub2APIProviderProbeTarget{ID: 2, ProviderID: 18, AccountID: 2}
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Zero(t, probe.calls)
	require.NoError(t, s.enterHybridRecovery(ctx, states()[2], "test incident", now))
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Zero(t, probe.calls, "quarantined account must wait before active probing")
	_, err = db.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET next_probe_at=NOW()-interval '1 second' WHERE account_id=2`)
	require.NoError(t, err)
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Equal(t, 1, probe.calls)
	require.Equal(t, 1, states()[2].RecoverySuccesses)
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Equal(t, 1, probe.calls, "honor next probe time")
	_, err = db.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET next_probe_at=NOW()-interval '1 second' WHERE account_id=2`)
	require.NoError(t, err)
	probe.fail = true
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Equal(t, 2, probe.calls)
	require.Zero(t, states()[2].RecoverySuccesses)
	probe.fail = false
	for i := 0; i < 2; i++ {
		_, err = db.ExecContext(ctx, `UPDATE provider_account_hybrid_health SET next_probe_at=NOW()-interval '1 second' WHERE account_id=2`)
		require.NoError(t, err)
		require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	}
	require.Equal(t, "observing", states()[2].Phase)
	require.Empty(t, block(2))
	require.Equal(t, 4, probe.calls)
	require.NoError(t, s.runHybridRecoveryProbe(ctx, states()[2], target))
	require.Equal(t, 4, probe.calls, "healthy account never actively probes")
	// Stale result cannot undo a later incident.
	require.NoError(t, s.enterHybridRecovery(ctx, states()[1], "next incident", now.Add(5*time.Minute)))
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, h, true, &latency, "", now.Add(6*time.Minute)))
	require.Equal(t, "recovering", states()[1].Phase)
	// Policy update invalidates in-flight responses.
	old := states()[1]
	p := old.Policy
	p.RecoverySuccesses = 3
	_, err = s.SaveAccountHealthPolicy(ctx, 18, 1, p)
	require.NoError(t, err)
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, old, true, &latency, "", now.Add(7*time.Minute)))
	require.Zero(t, states()[1].RecoverySuccesses)
	// Manual disable/status wins even over an otherwise successful probe.
	current := states()[1]
	_, err = db.ExecContext(ctx, `UPDATE accounts SET status='disabled',schedulable=false,extra='{"sub2api_admin_account_state_generation":"manual"}' WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, current, true, &latency, "", now.Add(8*time.Minute)))
	require.Zero(t, states()[1].RecoverySuccesses)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM accounts WHERE id=1`).Scan(&status))
	require.Equal(t, "disabled", status)
	// A changed group cannot be released using the old group's result.
	_, err = db.ExecContext(ctx, `UPDATE accounts SET remote_group_id=99 WHERE id=1`)
	require.NoError(t, err)
	require.NoError(t, s.finishHybridRecoveryProbe(ctx, current, true, &latency, "", now.Add(9*time.Minute)))
	require.Equal(t, "recovering", states()[1].Phase)
	// Explicit policy disable releases only this health monitor's own latch.
	p.Enabled = false
	_, err = s.SaveAccountHealthPolicy(ctx, 18, 1, p)
	require.NoError(t, err)
	require.Empty(t, block(1))
	require.Equal(t, "observing", states()[1].Phase)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM accounts WHERE id=1`).Scan(&status))
	require.Equal(t, "disabled", status)
	// A foreign block is never adopted or cleared.
	_, err = db.ExecContext(ctx, `UPDATE accounts SET temp_unschedulable_reason='admin pause',temp_unschedulable_until=NOW()+interval '1 hour' WHERE id=2`)
	require.NoError(t, err)
	require.NoError(t, s.enterHybridRecovery(ctx, states()[2], "failure", now))
	require.Equal(t, "observing", states()[2].Phase)
	p2 := states()[2].Policy
	p2.Enabled = false
	_, err = s.SaveAccountHealthPolicy(ctx, 18, 2, p2)
	require.NoError(t, err)
	require.Equal(t, "admin pause", block(2))
	providerID := int64(18)
	account3 := &Account{ID: 3, ProviderID: &providerID}
	healthCache := &providerHealthCacheSimulation{}
	runtime := &ProviderRequestHealthService{db: db, cache: healthCache, policies: cache.New(time.Minute, time.Minute)}
	makeFailure := func(requestID string) {
		requestCtx := context.WithValue(ctx, ctxkey.UserID, int64(700))
		ginCtx, _ := gin.CreateTestContext(nil)
		ginCtx.Request = httptest.NewRequest("POST", "/v1/messages", nil).WithContext(requestCtx)
		ginCtx.Set("provider_health_request_nonce", requestID)
		started := time.Now().UTC()
		runtime.Observe(requestCtx, ginCtx, account3, 5, "same-session", started, false, true, nil, &UpstreamFailoverError{StatusCode: 502})
	}
	makeFailure("failure-1")
	makeFailure("failure-1") // duplicate observation of one logical request
	require.Equal(t, "observing", states()[3].Phase)
	makeFailure("failure-2")
	require.Equal(t, "observing", states()[3].Phase)
	makeFailure("failure-3")
	require.Equal(t, "recovering", states()[3].Phase)
	require.NotEmpty(t, block(3))
	require.Equal(t, 3, healthCache.failures)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT status FROM accounts WHERE id=3`).Scan(&status))
	require.Equal(t, "error", status)
	require.True(t, states()[3].NextProbeAt.After(time.Now().Add(4*time.Minute)))
	// Restart/read retains independent account settings.
	restarted := &Sub2APIProviderProbeService{db: db}
	persisted, err := restarted.accountHealthStates(ctx, 18)
	require.NoError(t, err)
	require.Equal(t, 3, persisted[1].Policy.RecoverySuccesses)
	require.False(t, persisted[2].Policy.Enabled)
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scheduler_outbox`).Scan(&count))
	require.GreaterOrEqual(t, count, 4)
}

type hybridProbeRunnerStub struct {
	calls int
	fail  bool
}

func (r *hybridProbeRunnerStub) RunProbeTestBackground(_ context.Context, _ int64, model string) (*ScheduledTestResult, error) {
	r.calls++
	status := "success"
	if r.fail {
		status = "failed"
	}
	return &ScheduledTestResult{Status: status, LatencyMs: 50, FinishedAt: time.Now().UTC()}, nil
}

type hybridProbeRepoStub struct{ Sub2APIProviderProbeRepository }

func (r *hybridProbeRepoStub) CreateTargetRun(_ context.Context, i *Sub2APIProviderProbeTargetRunInput) (*ent.Sub2APIProviderProbeTargetRun, error) {
	return &ent.Sub2APIProviderProbeTargetRun{TargetID: i.TargetID, AccountID: i.AccountID, Status: sub2apiproviderprobetargetrun.Status(i.Status), LatencyMs: i.LatencyMS, FinishedAt: i.FinishedAt}, nil
}
func (r *hybridProbeRepoStub) MarkTargetRun(context.Context, int64, time.Time, string) error {
	return nil
}
