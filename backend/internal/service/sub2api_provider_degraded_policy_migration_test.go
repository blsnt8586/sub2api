//go:build unit

package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestHybridDegradedPolicyMigrationPreservesSettingsAndRecoveryOwnership(t *testing.T) {
	dsn := os.Getenv("PROVIDER_PASSIVE_TEST_DSN")
	if dsn == "" {
		t.Skip("requires PROVIDER_PASSIVE_TEST_DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE provider_account_hybrid_health(
 account_id bigint primary key,policy jsonb,revision bigint DEFAULT 1,phase text DEFAULT 'observing',
 observe_since timestamptz DEFAULT NOW()-interval '1 hour',quarantine_token text DEFAULT '',recovery_successes int DEFAULT 1,
 next_probe_at timestamptz DEFAULT NOW()+interval '5 minutes',updated_at timestamptz DEFAULT NOW());
 INSERT INTO provider_account_hybrid_health(account_id,policy,phase,quarantine_token) VALUES
 (1,'{"enabled":false,"degraded_latency_ms":40000,"account_failure_threshold":7}','observing',''),
 (2,'{"enabled":true,"degraded_threshold":0}','observing',''),
 (3,'{"enabled":true,"degraded_threshold":4,"account_degraded_threshold":9}','observing',''),
 (4,'{"enabled":true}','recovering','owned-quarantine');`)
	require.NoError(t, err)
	var beforeSince, beforeProbe time.Time
	require.NoError(t, db.QueryRowContext(ctx, `SELECT observe_since,next_probe_at FROM provider_account_hybrid_health WHERE account_id=4`).Scan(&beforeSince, &beforeProbe))
	content, err := migrations.FS.ReadFile("268_provider_account_degraded_request_policy.sql")
	require.NoError(t, err)
	result, err := db.ExecContext(ctx, string(content))
	require.NoError(t, err)
	affected, err := result.RowsAffected()
	require.NoError(t, err)
	require.EqualValues(t, 3, affected)
	policy := func(id int) ProviderAccountHealthPolicy {
		var raw []byte
		require.NoError(t, db.QueryRowContext(ctx, `SELECT policy FROM provider_account_hybrid_health WHERE account_id=$1`, id).Scan(&raw))
		p := DefaultProviderAccountHealthPolicy()
		require.NoError(t, json.Unmarshal(raw, &p))
		return p
	}
	p := policy(1)
	require.False(t, p.Enabled)
	require.Equal(t, 40000, p.DegradedLatencyMS)
	require.Equal(t, 7, p.AccountFailureThreshold)
	require.Equal(t, 2, p.DegradedThreshold)
	require.Equal(t, 5, p.AccountDegradedThreshold)
	require.Zero(t, policy(2).DegradedThreshold, "explicit disabled rule is retained")
	require.Equal(t, 5, policy(2).AccountDegradedThreshold)
	require.Equal(t, 4, policy(3).DegradedThreshold)
	require.Equal(t, 9, policy(3).AccountDegradedThreshold)
	var token, phase string
	var afterSince, afterProbe time.Time
	var streak int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT quarantine_token,phase,observe_since,next_probe_at,recovery_successes FROM provider_account_hybrid_health WHERE account_id=4`).Scan(&token, &phase, &afterSince, &afterProbe, &streak))
	require.Equal(t, "owned-quarantine", token)
	require.Equal(t, "recovering", phase)
	require.True(t, beforeSince.Equal(afterSince))
	require.True(t, beforeProbe.Equal(afterProbe))
	require.Zero(t, streak, "old slow-but-successful probes cannot finish a new stricter recovery streak")
	result, err = db.ExecContext(ctx, string(content))
	require.NoError(t, err)
	affected, err = result.RowsAffected()
	require.NoError(t, err)
	require.Zero(t, affected, "migration is idempotent")
}
