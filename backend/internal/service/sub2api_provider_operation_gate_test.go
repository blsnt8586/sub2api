//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestSub2APIProviderOperationGateSerializesOneProvider(t *testing.T) {
	gate := NewSub2APIProviderOperationGate(nil, nil)
	release, acquired := gate.TryAcquire(context.Background(), 11, time.Minute)
	require.True(t, acquired)

	_, acquired = gate.TryAcquire(context.Background(), 11, time.Minute)
	require.False(t, acquired)

	otherRelease, acquired := gate.TryAcquire(context.Background(), 12, time.Minute)
	require.True(t, acquired, "different Providers must remain independent")
	otherRelease()

	release()
	release() // release is idempotent
	reacquiredRelease, acquired := gate.TryAcquire(context.Background(), 11, time.Minute)
	require.True(t, acquired)
	reacquiredRelease()
}

func TestSub2APIProviderOperationGateSerializesAcrossInstances(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	first := NewSub2APIProviderOperationGate(cache, nil)
	second := NewSub2APIProviderOperationGate(cache, nil)

	release, acquired := first.TryAcquire(context.Background(), 21, time.Minute)
	require.True(t, acquired)
	_, acquired = second.TryAcquire(context.Background(), 21, time.Minute)
	require.False(t, acquired)

	release()
	secondRelease, acquired := second.TryAcquire(context.Background(), 21, time.Minute)
	require.True(t, acquired)
	secondRelease()
}

func TestSub2APIProviderOperationGateStaleReleaseCannotDeleteNewOwner(t *testing.T) {
	cache := &fakeLeaderLockCache{}
	first := NewSub2APIProviderOperationGate(cache, nil)
	second := NewSub2APIProviderOperationGate(cache, nil)
	// Reuse an instance prefix to prove the per-acquisition UUID, rather than
	// merely the process UUID, protects a replacement lease.
	first.instanceID = "same-instance"
	second.instanceID = "same-instance"

	const providerID int64 = 31
	key := "sub2api:provider-operation:31"
	staleRelease, acquired := first.TryAcquire(context.Background(), providerID, time.Minute)
	require.True(t, acquired)
	oldOwner := cache.heldBy(key)

	// Simulate Redis TTL expiry while the stale process is still finishing.
	cache.mu.Lock()
	delete(cache.owners, key)
	delete(cache.ttls, key)
	cache.mu.Unlock()

	newRelease, acquired := second.TryAcquire(context.Background(), providerID, time.Minute)
	require.True(t, acquired)
	newOwner := cache.heldBy(key)
	require.NotEqual(t, oldOwner, newOwner)

	staleRelease()
	require.Equal(t, newOwner, cache.heldBy(key), "stale compare-and-delete must preserve the replacement lease")
	newRelease()
}

func TestScheduledTargetProbeYieldsWithoutWritingAResult(t *testing.T) {
	gate := NewSub2APIProviderOperationGate(nil, nil)
	releaseOptimize, acquired := gate.TryAcquire(context.Background(), 41, sub2APIProviderOptimizeOperationTTL)
	require.True(t, acquired)

	probe := &Sub2APIProviderProbeService{operationGate: gate}
	run, acquired, err := probe.runTargetIfProviderAvailable(context.Background(), &ent.Sub2APIProviderProbeTarget{ProviderID: 41})
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, run)

	releaseOptimize()
	releaseProbe, acquired := gate.TryAcquire(context.Background(), 41, sub2APIProviderProbeOperationTTL)
	require.True(t, acquired)
	optimizer := &Sub2APIOptimizeScheduleService{operationGate: gate}
	_, acquired = optimizer.tryAcquire(context.Background(), 41)
	require.False(t, acquired)
	// Scheduled probing releases its per-target operation before it submits the
	// unhealthy evidence to auto-optimization.
	releaseProbe()
	releaseAutoOptimize, acquired := optimizer.tryAcquire(context.Background(), 41)
	require.True(t, acquired)
	releaseAutoOptimize()
}

func TestRemoteGroupSyncYieldsWhileOptimizerOwnsProvider(t *testing.T) {
	gate := NewSub2APIProviderOperationGate(nil, nil)
	releaseOptimize, acquired := gate.TryAcquire(context.Background(), 51, sub2APIProviderOptimizeOperationTTL)
	require.True(t, acquired)
	defer releaseOptimize()

	provider := &Sub2APIProviderService{operationGate: gate}
	require.NotPanics(t, func() {
		// Other dependencies are intentionally nil: contended synchronization must
		// return cached state before login or persistence is attempted.
		provider.syncRemoteGroups(context.Background(), &ent.Sub2APIProvider{ID: 51}, []Account{{ID: 1}})
	})
}
