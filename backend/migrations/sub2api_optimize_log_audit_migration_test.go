//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSub2APIOptimizeLogAuditMigrationPreservesHistory(t *testing.T) {
	content, err := FS.ReadFile("228_sub2api_optimize_logs_provider_audit.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "set provider_id = schedules.provider_id")
	require.Contains(t, sql, "set trigger = 'legacy'")
	require.Contains(t, sql, "alter column schedule_id drop not null")
	require.Contains(t, sql, "on delete set null")
	require.Contains(t, sql, "foreign key (provider_id)")
	require.Contains(t, sql, "constraint_type = 'foreign key'")
	require.Contains(t, sql, "'probe_unhealthy'")
	require.Contains(t, sql, "'manual_account'")
	require.Contains(t, sql, "'manual_all'")
	require.NotContains(t, sql, "delete from sub2api_optimize_logs")
	require.NotContains(t, sql, "drop table")
}

func TestSub2APIOptimizeLogFilterIndexesUseNonTransactionalMigration(t *testing.T) {
	content, err := FS.ReadFile("229_sub2api_optimize_log_filter_indexes_notx.sql")
	require.NoError(t, err)
	sql := strings.ToLower(string(content))

	require.Equal(t, 4, strings.Count(sql, "create index concurrently if not exists"))
	require.Contains(t, sql, "(provider_id, trigger, created_at desc)")
	require.Contains(t, sql, "(provider_id, status, created_at desc)")
	require.Contains(t, sql, "using gin (detail jsonb_path_ops)")
}
