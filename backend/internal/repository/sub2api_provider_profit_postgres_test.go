//go:build unit

package repository

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

// Temporary tables on a dedicated connection shadow the application tables.
// No account, credential or request in the local preview database is modified.
func TestProviderLocalProfitPostgresPeriodsAndMultiplier(t *testing.T) {
	dsn := os.Getenv("PROVIDER_PROFIT_TEST_DSN")
	if dsn == "" {
		t.Skip("requires PROVIDER_PROFIT_TEST_DSN")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	ctx := context.Background()
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.ExecContext(ctx, `CREATE TEMP TABLE accounts(id bigint primary key, remote_group_multiplier numeric);
 CREATE TEMP TABLE usage_logs(account_id bigint, total_cost numeric, actual_cost numeric, account_stats_cost numeric, account_rate_multiplier numeric, created_at timestamptz);
 INSERT INTO accounts VALUES (1,0.1),(2,NULL),(3,0),(4,0.2),(5,NULL);`)
	require.NoError(t, err)
	end := time.Date(2026, 11, 3, 12, 0, 0, 0, timezone.Location())
	insert := func(account int, base, revenue float64, stats any, at time.Time) {
		_, err := conn.ExecContext(ctx, `INSERT INTO usage_logs VALUES($1,$2,$3,$4,99,$5)`, account, base, revenue, stats, at)
		require.NoError(t, err)
	}
	// Local sale rate 0.2, upstream 0.1 => 10*(0.2-0.1) = 1.
	insert(1, 10, 2, nil, end.Add(-time.Hour))
	// Special account pricing is honored; account_rate_multiplier=99 ignored.
	insert(1, 10, 1, 5.0, end.AddDate(0, 0, -1))
	insert(1, 10, 0.5, nil, end.AddDate(0, -1, 0)) // Historical loss -0.5.
	insert(1, 100, 100, nil, end.Add(time.Hour))   // Future logs excluded.
	insert(2, 10, 2, nil, end.Add(-time.Hour))     // Unknown upstream rate unavailable.
	insert(3, 10, 2, nil, end.Add(-time.Hour))     // Explicit free upstream: profit 2.
	insert(4, 10, 0, nil, end.Add(-time.Hour))     // Free customer still incurs a loss.
	repo := &usageLogRepository{sql: conn}
	got, err := repo.GetProviderAccountLocalProfit(ctx, []int64{1, 2, 3, 4, 5, 1, -1}, end)
	require.NoError(t, err)
	require.Len(t, got, 5)
	require.True(t, got[1].CostAvailable)
	require.Equal(t, 2.0, got[1].TodayRevenue)
	require.InDelta(t, 1.0, got[1].TodayProfit, 1e-9)
	require.InDelta(t, 1.5, got[1].MonthProfit, 1e-9)
	require.InDelta(t, 1.0, got[1].CumulativeProfit, 1e-9)
	require.Equal(t, 3.5, got[1].CumulativeRevenue)
	require.False(t, got[2].CostAvailable)
	require.True(t, got[3].CostAvailable)
	require.Equal(t, 2.0, got[3].TodayProfit)
	require.Equal(t, -2.0, got[4].TodayProfit)
	require.True(t, got[5].CostAvailable) // No requests means zero, no assumed cost.
	require.Zero(t, got[5].CumulativeProfit)
}
