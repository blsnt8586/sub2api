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

func TestProviderDegradedRequestsIsolateOnlyAffectedSessionAndPreserveWindow(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	c := &gatewayCache{rdb: rdb}
	ctx := context.Background()
	p := service.DefaultProviderAccountHealthPolicy()
	base := time.Now().UTC().Truncate(time.Millisecond)
	event := func(session, request string, offset int, slow bool) service.ProviderRequestHealthEvent {
		started := base.Add(time.Duration(offset) * time.Second)
		return service.ProviderRequestHealthEvent{AccountID: 9, UserID: 1, Generation: "g1", Session: session, RequestID: request, StartedAt: started, CompletedAt: started.Add(time.Millisecond), Degraded: slow, Policy: p}
	}
	observe := func(e service.ProviderRequestHealthEvent) { require.NoError(t, c.ObserveProviderRequestHealth(ctx, e)) }
	avoid := func(session string, offset int) []service.ProviderSessionAvoidance {
		items, err := c.ProviderSessionAvoidances(ctx, session, base.Add(time.Duration(offset)*time.Second))
		require.NoError(t, err)
		return items
	}
	slowCount := func(offset int) int {
		n, err := c.ProviderRecentDegradations(ctx, 9, "g1", base.Add(time.Duration(offset)*time.Second), p.WindowSeconds)
		require.NoError(t, err)
		return n
	}
	observe(event("user1-sessionA", "slow1", 0, true))
	require.Empty(t, avoid("user1-sessionA", 1))
	observe(event("user1-sessionA", "slow2", 2, true))
	require.Len(t, avoid("user1-sessionA", 3), 1)
	require.Empty(t, avoid("user1-sessionB", 3), "other conversations remain usable")
	require.Empty(t, avoid("user2-sessionA", 3), "other users remain usable")
	require.Equal(t, 2, slowCount(3))
	require.Zero(t, mustRecentFailures(t, c, ctx, base.Add(3*time.Second), p.WindowSeconds), "slow successes are not upstream errors")
	observe(event("user1-sessionA", "slow2", 2, true))
	require.Equal(t, 2, slowCount(3), "retries count once")
	observe(event("user2-sessionA", "other-fast", 4, false))
	require.Len(t, avoid("user1-sessionA", 5), 1)
	oldFast := event("user1-sessionA", "old-fast", -1, false)
	oldFast.CompletedAt = base.Add(6 * time.Second)
	observe(oldFast)
	require.Len(t, avoid("user1-sessionA", 7), 1, "older fast completion cannot clear newer slow requests")
	hardFailure := event("user1-sessionA", "failed", 8, false)
	hardFailure.Failed = true
	hardFailure.Degraded = true // Failure takes precedence even for a malformed event.
	observe(hardFailure)
	require.Len(t, avoid("user1-sessionA", 9), 1, "a hard failure is not a fast success")
	require.Equal(t, 2, slowCount(9), "timeouts and slow successes are separate")
	observe(event("user1-sessionA", "fast", 10, false))
	require.Empty(t, avoid("user1-sessionA", 11))
	require.Equal(t, 2, slowCount(11), "fast completion does not erase account totals")
	observe(event("user1-sessionA", "slow3", 12, true))
	require.Empty(t, avoid("user1-sessionA", 13), "fast response resets only this session's consecutive streak")
	oldSlow := event("user1-sessionA", "old-slow", 1, true)
	oldSlow.CompletedAt = base.Add(14 * time.Second)
	observe(oldSlow)
	require.Empty(t, avoid("user1-sessionA", 15), "late slow completion cannot undo a newer fast success")
	require.Equal(t, 4, slowCount(15))
	observe(event("user1-sessionB", "slow4", 16, true))
	require.Equal(t, 5, slowCount(17), "account threshold accumulates across sessions")
	otherGeneration, err := c.ProviderRecentDegradations(ctx, 9, "new-policy", base.Add(17*time.Second), p.WindowSeconds)
	require.NoError(t, err)
	require.Zero(t, otherGeneration)
	require.Zero(t, slowCount(317), "rolling window expires past evidence")
	require.Empty(t, avoid("user1-sessionA", 317))
}

func TestProviderDegradedThresholdsAreIndependentAndConfigurable(t *testing.T) {
	for _, tc := range []struct {
		name                              string
		session, account, requests, avoid int
	}{
		{"custom session threshold waits", 3, 5, 2, 0},
		{"custom session threshold reached", 3, 5, 3, 1},
		{"account-only rule", 0, 5, 3, 0},
		{"session-only rule", 2, 0, 2, 1},
		{"both disabled", 0, 0, 3, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			c := &gatewayCache{rdb: rdb}
			p := service.DefaultProviderAccountHealthPolicy()
			p.DegradedThreshold, p.AccountDegradedThreshold = tc.session, tc.account
			ctx := context.Background()
			base := time.Now()
			for i := 0; i < tc.requests; i++ {
				at := base.Add(time.Duration(i) * time.Second)
				require.NoError(t, c.ObserveProviderRequestHealth(ctx, service.ProviderRequestHealthEvent{AccountID: 9, UserID: 1, Generation: "g1", Session: "s", RequestID: at.String(), StartedAt: at, CompletedAt: at, Degraded: true, Policy: p}))
			}
			items, err := c.ProviderSessionAvoidances(ctx, "s", base.Add(10*time.Second))
			require.NoError(t, err)
			require.Len(t, items, tc.avoid)
			n, err := c.ProviderRecentDegradations(ctx, 9, "g1", base.Add(10*time.Second), 300)
			require.NoError(t, err)
			if tc.session == 0 && tc.account == 0 {
				require.Zero(t, n)
			} else {
				require.Equal(t, tc.requests, n)
			}
		})
	}
}
