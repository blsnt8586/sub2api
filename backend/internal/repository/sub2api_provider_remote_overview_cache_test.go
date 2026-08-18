//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func newProviderRemoteOverviewTestCache(t *testing.T) (*sub2APIProviderRemoteOverviewCache, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &sub2APIProviderRemoteOverviewCache{rdb: rdb}, mr
}

func TestProviderRemoteOverviewCachePreservesLastSuccessAcrossFailure(t *testing.T) {
	cache, mr := newProviderRemoteOverviewTestCache(t)
	ctx := context.Background()
	sampledAt := time.Date(2026, 8, 17, 10, 0, 0, 0, time.UTC)
	overview := &service.Sub2APIProviderRemoteOverview{
		ProviderID: 7, Available: true, Balance: 12.5,
		Groups:                 []service.Sub2APIProviderRemoteGroupRate{{ID: 1, Name: "Economy", EffectiveMultiplier: 0.25}},
		RateOverridesAvailable: true,
		SampledAt:              sampledAt, Source: service.Sub2APIProviderRemoteOverviewSourceControlProbe,
		LastAttemptedAt: sampledAt, LastAttemptSource: service.Sub2APIProviderRemoteOverviewSourceControlProbe,
	}
	require.NoError(t, cache.StoreSuccess(ctx, overview))
	failureAt := sampledAt.Add(10 * time.Minute)
	require.NoError(t, cache.StoreFailure(ctx, 7, service.Sub2APIProviderRemoteOverviewSourceManual, failureAt, "wallet unavailable"))

	items, err := cache.GetMany(ctx, []int64{7, 8})
	require.NoError(t, err)
	cached := items[7]
	require.NotNil(t, cached)
	require.True(t, cached.Available)
	require.Equal(t, 12.5, cached.Balance)
	require.Equal(t, sampledAt, cached.SampledAt)
	require.Equal(t, service.Sub2APIProviderRemoteOverviewSourceControlProbe, cached.Source)
	require.Equal(t, service.Sub2APIProviderRemoteOverviewSourceManual, cached.LastAttemptSource)
	require.Equal(t, "wallet unavailable", requireString(t, cached.LastError))
	require.Equal(t, failureAt, *cached.LastErrorAt)
	require.NotContains(t, items, int64(8))
	require.InDelta(t, providerRemoteOverviewTTL.Seconds(), mr.TTL(providerRemoteOverviewKey(7)).Seconds(), 1)
}

func TestProviderRemoteOverviewCacheIgnoresOlderFailureAndSupportsFailureOnlyState(t *testing.T) {
	cache, _ := newProviderRemoteOverviewTestCache(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 17, 11, 0, 0, 0, time.UTC)
	require.NoError(t, cache.StoreSuccess(ctx, &service.Sub2APIProviderRemoteOverview{
		ProviderID: 9, Available: true, Groups: []service.Sub2APIProviderRemoteGroupRate{},
		SampledAt: now, Source: service.Sub2APIProviderRemoteOverviewSourceControlProbe,
		LastAttemptedAt: now, LastAttemptSource: service.Sub2APIProviderRemoteOverviewSourceControlProbe,
	}))
	require.NoError(t, cache.StoreFailure(ctx, 9, service.Sub2APIProviderRemoteOverviewSourceManual, now.Add(-time.Minute), "stale failure"))
	require.NoError(t, cache.StoreFailure(ctx, 10, service.Sub2APIProviderRemoteOverviewSourceControlProbe, now, "first collection failed"))

	items, err := cache.GetMany(ctx, []int64{9, 10})
	require.NoError(t, err)
	require.Nil(t, items[9].LastError)
	require.False(t, items[10].Available)
	require.Equal(t, "first collection failed", requireString(t, items[10].LastError))
}

func requireString(t *testing.T, value *string) string {
	t.Helper()
	require.NotNil(t, value)
	return *value
}
