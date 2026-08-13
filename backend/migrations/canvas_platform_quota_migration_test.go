//go:build unit

package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCanvasPlatformQuotaMigrationAlignsConstraint(t *testing.T) {
	content, err := FS.ReadFile("223_user_platform_quotas_add_canvas.sql")
	require.NoError(t, err)

	sql := strings.ToLower(string(content))
	require.Contains(t, sql, "drop constraint if exists user_platform_quotas_platform_check")
	require.Contains(t, sql, "add constraint user_platform_quotas_platform_check")
	require.Contains(t, sql, "'canvas'")
	require.NotContains(t, sql, "drop table")
}
