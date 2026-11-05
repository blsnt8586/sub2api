//go:build unit

package repository

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGetProviderAccountCostUsesHistoricalCostAndMultiplierSnapshots(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	start := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT account_id,")).
		WithArgs(sqlmock.AnyArg(), start, end, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"account_id", "request_count", "input_tokens", "output_tokens", "cache_tokens", "estimated_cost", "today_estimated_cost", "month_estimated_cost", "cumulative_estimated_cost",
		}).AddRow(7, 3, 100, 40, 25, 4.5, 1.25, 2.5, 40.0))

	repo := &usageLogRepository{sql: db}
	got, err := repo.GetProviderAccountCost(context.Background(), []int64{7}, start, end)
	require.NoError(t, err)
	require.Equal(t, service.ProviderAccountCost{
		AccountID: 7, RequestCount: 3, InputTokens: 100, OutputTokens: 40, CacheTokens: 25,
		EstimatedCost: 4.5, TodayEstimatedCost: 1.25, MonthEstimatedCost: 2.5, CumulativeEstimatedCost: 40,
	}, got[7])
	require.NoError(t, mock.ExpectationsWereMet())
}
