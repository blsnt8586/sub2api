package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	sub2APIProviderProbeOperationTTL    = 3 * time.Minute
	sub2APIProviderOptimizeOperationTTL = 12 * time.Minute
)

// Sub2APIProviderOperationGate serializes operations that observe or mutate a
// Provider's remote API-key group. Account probes, remote binding refreshes and
// optimization runs must not overlap, otherwise a temporary candidate group can
// be persisted or reported as the account's stable route.
//
// The local map protects a single process. Redis (with a Postgres advisory-lock
// fallback) protects multi-instance deployments. Control-plane health checks do
// not use this gate because they never switch or consume an account route.
type Sub2APIProviderOperationGate struct {
	lockCache LeaderLockCache
	db        *sql.DB

	instanceID string
	mu         sync.Mutex
	running    map[int64]struct{}
}

func NewSub2APIProviderOperationGate(lockCache LeaderLockCache, db *sql.DB) *Sub2APIProviderOperationGate {
	return &Sub2APIProviderOperationGate{
		lockCache:  lockCache,
		db:         db,
		instanceID: uuid.NewString(),
		running:    make(map[int64]struct{}),
	}
}

// TryAcquire returns immediately when another account-route operation owns the
// Provider. The returned release function is idempotent.
func (g *Sub2APIProviderOperationGate) TryAcquire(ctx context.Context, providerID int64, ttl time.Duration) (func(), bool) {
	if g == nil {
		return func() {}, true
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if providerID <= 0 {
		return nil, false
	}
	if ttl <= 0 {
		ttl = sub2APIProviderProbeOperationTTL
	}

	g.mu.Lock()
	if _, exists := g.running[providerID]; exists {
		g.mu.Unlock()
		return nil, false
	}
	g.running[providerID] = struct{}{}
	g.mu.Unlock()

	key := fmt.Sprintf("sub2api:provider-operation:%d", providerID)
	owner := fmt.Sprintf("%s:%s", g.instanceID, uuid.NewString())
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	distributedRelease, acquired := tryAcquireSingletonLeaderLock(lockCtx, g.lockCache, g.db, key, owner, ttl)
	cancel()
	if !acquired {
		g.releaseLocal(providerID)
		return nil, false
	}

	var once sync.Once
	return func() {
		once.Do(func() {
			distributedRelease()
			g.releaseLocal(providerID)
		})
	}, true
}

func (g *Sub2APIProviderOperationGate) releaseLocal(providerID int64) {
	g.mu.Lock()
	delete(g.running, providerID)
	g.mu.Unlock()
}
