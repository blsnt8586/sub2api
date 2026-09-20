//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeMigrationPreservesCanvasQuota(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	var userID int64
	require.NoError(t, tx.QueryRowContext(ctx, `INSERT INTO users (email, password_hash)
		VALUES ('canvas-migration@example.test', 'test') RETURNING id`).Scan(&userID))
	_, err := tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd)
		VALUES ($1, 'canvas', 12)`, userID)
	require.NoError(t, err)
	migration, err := dbmigrations.FS.ReadFile("238_opencode_go_platform.sql")
	require.NoError(t, err)
	for range 2 {
		_, err = tx.ExecContext(ctx, string(migration))
		require.NoError(t, err)
	}
	var limit float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT daily_limit_usd FROM user_platform_quotas
		WHERE user_id = $1 AND platform = 'canvas'`, userID).Scan(&limit))
	require.Equal(t, 12.0, limit)
	_, err = tx.ExecContext(ctx, `INSERT INTO user_platform_quotas (user_id, platform, daily_limit_usd)
		VALUES ($1, 'opencode_go', 20)`, userID)
	require.NoError(t, err)
}
