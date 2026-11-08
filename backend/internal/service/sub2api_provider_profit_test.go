//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type providerProfitUsageRepoStub struct {
	UsageLogRepository
	values    map[int64]ProviderAccountLocalProfit
	err       error
	sampledAt time.Time
}

type providerDailySnapshotReaderStub struct {
	UsageLogRepository
	values map[int64]ProviderDailySnapshot
}

func (s *providerDailySnapshotReaderStub) GetProviderDailySnapshots(_ context.Context, _ int64, _ []int64, _ time.Time) (map[int64]ProviderDailySnapshot, error) {
	return s.values, nil
}

func (s *providerProfitUsageRepoStub) GetProviderAccountLocalProfit(_ context.Context, ids []int64, at time.Time) (map[int64]ProviderAccountLocalProfit, error) {
	s.sampledAt = at
	return s.values, s.err
}

func TestCollectProviderProfitSnapshotUsesLocalRequestsForEveryPeriod(t *testing.T) {
	accounts := []Account{{ID: 1, Name: "beta"}, {ID: 2, Name: "alpha"}}
	repo := &providerProfitUsageRepoStub{values: map[int64]ProviderAccountLocalProfit{
		1: {AccountID: 1, CostAvailable: true, TodayRevenue: 4, TodayProfit: 3, MonthRevenue: 10, MonthProfit: 6, CumulativeRevenue: 50, CumulativeProfit: 30},
		// A loss is preserved rather than clamped to zero.
		2: {AccountID: 2, CostAvailable: true, TodayRevenue: 1, TodayProfit: -2, MonthRevenue: 5, MonthProfit: -1, CumulativeRevenue: 20, CumulativeProfit: 4},
	}}
	at := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	got := collectProviderProfitSnapshot(context.Background(), accounts, repo, at)
	require.True(t, got.Available)
	require.True(t, got.MonthProfitAvailable)
	require.True(t, got.CumulativeProfitAvailable)
	require.Equal(t, "local_request_logs", got.CostSource)
	require.Nil(t, got.RemoteError)
	require.Equal(t, at, repo.sampledAt)
	require.True(t, got.RangeStart.IsZero()) // No implicit 30-day cutoff.
	require.Equal(t, 5.0, got.TodayRevenue)
	require.Equal(t, 4.0, got.TodayEstimatedCost)
	require.Equal(t, 1.0, got.TodayGrossProfit)
	require.InDelta(t, 0.2, got.TodayGrossMargin, 1e-9)
	require.Equal(t, 5.0, got.MonthGrossProfit)
	require.InDelta(t, 1.0/3, got.MonthGrossMargin, 1e-9)
	require.Equal(t, 34.0, got.CumulativeGrossProfit)
	require.Equal(t, 70.0, got.TotalRevenue)
	require.Equal(t, 36.0, got.TotalEstimatedCost)
	require.Equal(t, got.CumulativeGrossProfit, got.TotalGrossProfit)
	require.Equal(t, "alpha", got.Accounts[0].AccountName)
	require.Equal(t, -2.0, got.Accounts[0].TodayGrossProfit)
	require.False(t, got.Accounts[0].RemoteCostAvailable)
}

func TestProviderDailySnapshotsRemainAuthoritativeAfterUsageLogPruning(t *testing.T) {
	repo := &providerDailySnapshotReaderStub{values: map[int64]ProviderDailySnapshot{
		1: {AccountID: 1, HasData: true, CostAvailable: true, BaseCostAvailable: true, TodayTokens: 15, TotalTokens: 115, TodayRevenue: 2, MonthRevenue: 12, TotalRevenue: 22, TodayProfit: 1, MonthProfit: 6, CumulativeProfit: 11},
	}}
	accounts := []Account{{ID: 1, Name: "snapshot account"}}
	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	snapshots, ok := readProviderDailySnapshots(context.Background(), 7, accounts, repo, at)
	require.True(t, ok)

	usage := providerDailyUsageSnapshot(snapshots)
	require.Equal(t, int64(15), usage.TodayTokens)
	require.Equal(t, int64(115), usage.TotalTokens)
	profit := providerDailyProfitSnapshot(accounts, snapshots, at)
	require.Equal(t, 1.0, profit.TodayGrossProfit)
	require.Equal(t, 6.0, profit.MonthGrossProfit)
	require.Equal(t, 11.0, profit.CumulativeGrossProfit)
	require.Equal(t, "provider_daily_snapshots", profit.CostSource)
}

func TestProviderDailySnapshotsDoNotTreatUnknownHistoricalCostAsZero(t *testing.T) {
	accounts := []Account{{ID: 1, Name: "legacy account"}}
	snapshots := map[int64]ProviderDailySnapshot{
		1: {AccountID: 1, HasData: true, CostAvailable: true, BaseCostAvailable: false, TotalRevenue: 100, CumulativeProfit: 100},
	}
	profit := providerDailyProfitSnapshot(accounts, snapshots, time.Now())
	require.False(t, profit.Available)
	require.False(t, profit.MonthProfitAvailable)
	require.False(t, profit.CumulativeProfitAvailable)
	require.NotNil(t, profit.Error)
}

func TestCollectProviderProfitSnapshotDoesNotAssumeMissingMultiplier(t *testing.T) {
	repo := &providerProfitUsageRepoStub{values: map[int64]ProviderAccountLocalProfit{
		1: {AccountID: 1, CumulativeRevenue: 10, CostAvailable: false},
	}}
	got := collectProviderProfitSnapshot(context.Background(), []Account{{ID: 1}}, repo, time.Now())
	require.False(t, got.Available)
	require.False(t, got.MonthProfitAvailable)
	require.False(t, got.CumulativeProfitAvailable)
	require.NotNil(t, got.Error)
	require.False(t, got.Accounts[0].EstimatedCostAvailable)
	require.NotNil(t, got.Accounts[0].Error)
}

func TestCollectProviderProfitSnapshotRepositoryFailureIsUnavailable(t *testing.T) {
	repo := &providerProfitUsageRepoStub{err: errors.New("database unavailable")}
	got := collectProviderProfitSnapshot(context.Background(), []Account{{ID: 1}}, repo, time.Now())
	require.False(t, got.Available)
	require.Contains(t, *got.Error, "database unavailable")
	require.Nil(t, got.RemoteError)
}

func TestCollectProviderProfitSnapshotZeroTrafficAndEmptyProvider(t *testing.T) {
	repo := &providerProfitUsageRepoStub{values: map[int64]ProviderAccountLocalProfit{1: {AccountID: 1, CostAvailable: true}}}
	for _, accounts := range [][]Account{nil, {{ID: 1}}} {
		got := collectProviderProfitSnapshot(context.Background(), accounts, repo, time.Now())
		require.True(t, got.Available)
		require.True(t, got.MonthProfitAvailable)
		require.True(t, got.CumulativeProfitAvailable)
		require.Zero(t, got.CumulativeGrossProfit)
		require.Zero(t, got.CumulativeGrossMargin)
	}
}

func TestCachedProviderProfitDoesNotDependOnRemoteAssets(t *testing.T) {
	accounts := &controlBindingAccountRepoStub{accounts: []Account{{ID: 1, Name: "local"}}}
	usage := &providerProfitUsageRepoStub{values: map[int64]ProviderAccountLocalProfit{
		1: {AccountID: 1, CostAvailable: true, TodayRevenue: 2, TodayProfit: 1, CumulativeRevenue: 10, CumulativeProfit: 4},
	}}
	original := &Sub2APIProviderRemoteOverview{ProviderID: 7, Available: true, Balance: 123,
		Profit: &Sub2APIProviderProfitSummary{TotalGrossProfit: 999, CostSource: "remote"}}
	cache := &remoteOverviewCacheStub{items: map[int64]*Sub2APIProviderRemoteOverview{7: original}}
	svc := &Sub2APIProviderService{accountRepo: accounts, usageRepo: usage, remoteOverviewCache: cache}
	got, err := svc.GetCachedRemoteOverviews(context.Background(), []int64{7, 8})
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, 123.0, got[0].Balance)
	require.True(t, got[0].Available)
	require.Equal(t, 4.0, got[0].Profit.CumulativeGrossProfit)
	require.False(t, got[1].Available)                        // Unknown wallet stays unknown.
	require.True(t, got[1].Profit.Available)                  // Local profits still work.
	require.Equal(t, 999.0, original.Profit.TotalGrossProfit) // No shared-cache mutation.
}
