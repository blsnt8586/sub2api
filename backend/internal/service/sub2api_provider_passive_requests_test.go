//go:build unit

package service

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPassiveRequestProjectionReplacesLegacyActiveProbe(t *testing.T) {
	now := time.Now().UTC()
	latency := 120
	old := now.Add(-time.Hour)
	p := DefaultProviderPassivePolicy()
	h := &Sub2APIProviderProbeTargetHealth{Status: "healthy", Enabled: false, LastRunAt: &old, LastCheckedAt: &old, LatencyMS: &latency, TestModel: providerProbeStringPtr("old test model")}
	applyPassiveRequestHealth(h, nil, nil, p, now)
	require.Equal(t, "user_requests", h.MonitoringSource)
	require.Equal(t, "unknown", h.Status)
	require.True(t, h.Enabled)
	require.Nil(t, h.LastRunAt)
	require.Nil(t, h.LastCheckedAt)
	require.Nil(t, h.TestModel)
	require.Empty(t, h.Buckets)
	events := []providerPassiveRequest{{At: now, Failure: true, Model: "actual user model"}, {At: old, Latency: &latency}}
	o := &ProviderPassiveObservation{Requests: 3, Failures: 3, Status: "temporarily_unschedulable", LastRequestAt: &now}
	applyPassiveRequestHealth(h, o, events, p, now)
	require.Equal(t, "unhealthy", h.Status)
	require.Equal(t, "actual user model", *h.TestModel)
	require.Equal(t, 1, h.ConsecutiveFailures)
	require.Equal(t, "healthy", h.Buckets[0].Status)
	require.Equal(t, "unhealthy", h.Buckets[1].Status)
	require.Nil(t, h.TrafficP95LatencyMS) // No latency evidence is not zero latency.
	require.Equal(t, now, *h.LastCheckedAt)
}

func TestPassiveRequestCooldownSurvivesQuietWindow(t *testing.T) {
	now := time.Now().UTC()
	until := now.Add(time.Minute)
	o := &ProviderPassiveObservation{Status: "unknown", CooldownUntil: &until, Reason: "passive health: 3 failures"}
	h := &Sub2APIProviderProbeTargetHealth{}
	applyPassiveRequestHealth(h, o, nil, DefaultProviderPassivePolicy(), now)
	require.Equal(t, "unhealthy", h.Status)
	applyPassiveRequestHealth(h, o, nil, DefaultProviderPassivePolicy(), until.Add(time.Second))
	require.Equal(t, "unknown", h.Status)
}

// Temporary tables on one private connection isolate every SQL check from
// imported production data. No real requests, account settings or logs change.
func TestPassiveRequestsPostgresAttributionAndTimelines(t *testing.T) {
	dsn := os.Getenv("PROVIDER_PASSIVE_TEST_DSN")
	if dsn == "" {
		t.Skip("requires PROVIDER_PASSIVE_TEST_DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE accounts(id bigint primary key,provider_id bigint,deleted_at timestamptz,temp_unschedulable_until timestamptz,temp_unschedulable_reason text);
 CREATE TEMP TABLE sub2api_provider_probe_targets(account_id bigint,provider_id bigint,route_changed_at timestamptz);
 CREATE TEMP TABLE usage_logs(id bigserial primary key,account_id bigint,user_id bigint,request_id text,created_at timestamptz,stream boolean,duration_ms int,first_token_ms int,model text);
 CREATE TEMP TABLE ops_error_logs(id bigserial primary key,account_id bigint,user_id bigint,request_id text,created_at timestamptz,
 time_to_first_token_ms bigint,response_latency_ms bigint,upstream_latency_ms bigint,duration_ms int,model text,error_message text,
 is_business_limited boolean,status_code int,error_phase text,error_owner text,error_type text,upstream_status_code int,network_error_type text,error_source text);
 INSERT INTO accounts(id,provider_id) VALUES(1,18),(2,18),(3,99);
 `)
	require.NoError(t, err)
	now := time.Now().UTC()
	_, err = db.ExecContext(ctx, `INSERT INTO sub2api_provider_probe_targets VALUES(1,18,$1)`, now.Add(-5*time.Minute))
	require.NoError(t, err)
	success := func(account int, key string, at time.Time, first any) {
		_, err := db.ExecContext(ctx, `INSERT INTO usage_logs(account_id,user_id,request_id,created_at,stream,duration_ms,first_token_ms,model) VALUES($1,1,$2,$3,true,120000,$4,'real-model')`, account, key, at, first)
		require.NoError(t, err)
	}
	failure := func(account int, key string, code int, at time.Time) {
		_, err := db.ExecContext(ctx, `INSERT INTO ops_error_logs(account_id,user_id,request_id,created_at,status_code,error_phase,error_owner,error_type,response_latency_ms,error_message) VALUES($1,1,$2,$3,$4,'upstream','provider','connection_error',30000,'connection failed')`, account, key, at, code)
		require.NoError(t, err)
	}
	success(1, "retry-success", now.Add(-time.Minute), 100)
	success(1, "missing-ttft", now.Add(-30*time.Second), nil)
	failure(1, "retry-success", 502, now.Add(-2*time.Minute)) // Superseded on same account.
	failure(1, "dedup", 502, now.Add(-time.Minute))
	failure(1, "dedup", 502, now.Add(-55*time.Second))
	failure(1, "failover", 502, now.Add(-40*time.Second))
	success(2, "failover", now.Add(-35*time.Second), 150) // Does not erase account 1 failure.
	failure(1, "cancelled", 499, now.Add(-20*time.Second))
	success(1, "before-route-change", now.Add(-6*time.Minute), 100000)
	success(1, "future", now.Add(time.Minute), 100000)
	failure(3, "other-provider", 502, now.Add(-time.Minute))
	s := &Sub2APIProviderProbeService{db: db}
	p := DefaultProviderPassivePolicy()
	list, err := s.PassiveObservations(ctx, 18, p, now)
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.Equal(t, 4, list[0].Requests)
	require.Equal(t, 2, list[0].Failures)
	require.Equal(t, 100, list[0].P95) // Long successful stream uses TTFT, not total duration.
	require.Equal(t, 1, list[0].LatencySamples)
	events, err := s.passiveRequestEvents(ctx, 18, now.Add(-time.Hour), now, 60)
	require.NoError(t, err)
	require.Len(t, events[1], 4)
	require.Len(t, events[2], 1)
	require.NotContains(t, events, int64(3))
	require.Nil(t, events[1][0].Latency)
	require.Equal(t, "real-model", events[1][0].Model)
	limited, err := s.passiveRequestEvents(ctx, 18, now.Add(-time.Hour), now, 2)
	require.NoError(t, err)
	require.Len(t, limited[1], 2)
	require.Len(t, limited[2], 1)
}
