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

func TestProviderRequestHealthCacheScopesSessionAvoidanceAndCountsRequests(t *testing.T) {
	mr := miniredis.RunT(t)
	c := &gatewayCache{rdb: redis.NewClient(&redis.Options{Addr: mr.Addr()})}
	ctx := context.Background()
	p := service.DefaultProviderAccountHealthPolicy()
	p.FailureThreshold = 2
	p.AccountFailureThreshold = 3
	p.SessionAvoidSeconds = 300
	base := time.Now().UTC().Truncate(time.Millisecond)
	event := func(user int64, session, request string, at time.Time, failed bool) service.ProviderRequestHealthEvent {
		return service.ProviderRequestHealthEvent{AccountID: 9, UserID: user, Generation: "g1", Session: session, RequestID: request, StartedAt: at, CompletedAt: at.Add(time.Millisecond), Failed: failed, Policy: p}
	}
	for _, e := range []service.ProviderRequestHealthEvent{
		event(1, "u1", "u1-1", base, true),
		event(1, "u1", "u1-2", base.Add(time.Second), true),
	} {
		require.NoError(t, c.ObserveProviderRequestHealth(ctx, e))
	}
	avoid, err := c.ProviderSessionAvoidances(ctx, "u1", base.Add(2*time.Second))
	require.NoError(t, err)
	require.Len(t, avoid, 1)
	require.Equal(t, int64(9), avoid[0].AccountID)
	require.Equal(t, 2, mustRecentFailures(t, c, ctx, base.Add(2*time.Second), p.WindowSeconds))
	require.NoError(t, c.ObserveProviderRequestHealth(ctx, event(1, "u1", "u1-2", base.Add(time.Second), true)))
	require.Equal(t, 2, mustRecentFailures(t, c, ctx, base.Add(2*time.Second), p.WindowSeconds), "retry of one logical request counts once")

	// A success on the same session clears only that session's streak and avoid.
	require.NoError(t, c.ObserveProviderRequestHealth(ctx, event(1, "u1", "u1-ok", base.Add(3*time.Second), false)))
	avoid, err = c.ProviderSessionAvoidances(ctx, "u1", base.Add(4*time.Second))
	require.NoError(t, err)
	require.Empty(t, avoid)
	require.Equal(t, 2, mustRecentFailures(t, c, ctx, base.Add(4*time.Second), p.WindowSeconds), "a successful request does not erase earlier account failures")

	require.NoError(t, c.ObserveProviderRequestHealth(ctx, event(1, "u1", "u1-3", base.Add(5*time.Second), true)))
	require.Equal(t, 3, mustRecentFailures(t, c, ctx, base.Add(6*time.Second), p.WindowSeconds), "one user can reach the account threshold")
	require.Equal(t, 0, mustRecentFailures(t, c, ctx, base.Add(6*time.Minute), p.WindowSeconds), "the window is rolling")

	// A request may start before the rolling window and only fail after a long
	// upstream timeout. Account health uses failure time, so it still counts.
	late := event(2, "u2", "late-timeout", base.Add(-6*time.Minute), true)
	late.CompletedAt = base.Add(7 * time.Minute)
	late.RequestID = "late-timeout"
	require.NoError(t, c.ObserveProviderRequestHealth(ctx, late))
	require.Equal(t, 1, mustRecentFailures(t, c, ctx, base.Add(7*time.Minute), p.WindowSeconds))
}

func mustRecentFailures(t *testing.T, c *gatewayCache, ctx context.Context, now time.Time, window int) int {
	t.Helper()
	n, err := c.ProviderRecentFailures(ctx, 9, "g1", now, window)
	require.NoError(t, err)
	return n
}
