//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanvasPlatformQuotaMigrationAlignsConstraint(t *testing.T) {
	prepareContent, err := FS.ReadFile("192a_user_platform_quotas_allow_canvas_rename.sql")
	require.NoError(t, err)

	prepareSQL := strings.ToLower(string(prepareContent))
	require.Contains(t, prepareSQL, "'jimeng'")
	require.Contains(t, prepareSQL, "'canvas'")
	require.Contains(t, prepareSQL, "not valid")
	require.Contains(t, prepareSQL, "validate constraint user_platform_quotas_platform_check")

	content, err := FS.ReadFile("223_user_platform_quotas_add_canvas.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "drop constraint if exists user_platform_quotas_platform_check")
	require.Contains(t, sql, "add constraint user_platform_quotas_platform_check")
	require.Contains(t, sql, "'canvas'")
	require.NotContains(t, sql, "'jimeng'")
	require.NotContains(t, sql, "drop table")
}
