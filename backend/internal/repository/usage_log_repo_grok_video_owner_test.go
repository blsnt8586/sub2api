package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFindGrokVideoUsageAccountIDScopesDurableOwner(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := newUsageLogRepositoryWithSQL(nil, db)

	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT ul.account_id
		FROM usage_logs ul
		JOIN accounts a ON a.id = ul.account_id
		WHERE ul.user_id = $1
		  AND ul.api_key_id = $2
		  AND (ul.request_id = $3 OR ul.request_id = $4 OR ul.upstream_request_id = $3)
		  AND (ul.billing_mode = 'video' OR ul.video_count > 0)
		  AND a.platform = 'grok'
		  AND ($5 = 0 OR ul.group_id = $5)
		ORDER BY ul.id DESC
		LIMIT 1`)).
		WithArgs(int64(1), int64(2), "task-3", "grok-video:task-3", int64(4)).
		WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(int64(15)))

	accountID, err := repo.FindGrokVideoUsageAccountID(context.Background(), "task-3", 1, 2, 4)
	require.NoError(t, err)
	require.Equal(t, int64(15), accountID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindGrokVideoUsageAccountIDReturnsStickyMiss(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := newUsageLogRepositoryWithSQL(nil, db)

	mock.ExpectQuery("SELECT ul.account_id").WillReturnError(sql.ErrNoRows)
	accountID, err := repo.FindGrokVideoUsageAccountID(context.Background(), "missing", 1, 2, 4)
	require.ErrorIs(t, err, service.ErrStickySessionNotFound)
	require.Zero(t, accountID)
	require.NoError(t, mock.ExpectationsWereMet())
}
