//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/stretchr/testify/require"
)

type controlBindingAccountUpdate struct {
	accountID  int64
	groupID    int64
	groupName  string
	multiplier float64
}

type controlBindingAccountRepoStub struct {
	Sub2APIAccountRepository
	accounts []Account
	updates  []controlBindingAccountUpdate
	cleared  []int64
}

func (r *controlBindingAccountRepoStub) ListByProviderID(context.Context, int64) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func (r *controlBindingAccountRepoStub) UpdateRemoteGroupBinding(_ context.Context, accountID, groupID int64, groupName string, multiplier float64) error {
	r.updates = append(r.updates, controlBindingAccountUpdate{
		accountID: accountID, groupID: groupID, groupName: groupName, multiplier: multiplier,
	})
	return nil
}

func (r *controlBindingAccountRepoStub) UpdateRemoteGroupIdentity(context.Context, int64, int64) error {
	return nil
}

func (r *controlBindingAccountRepoStub) ClearRemoteGroupBinding(_ context.Context, accountID int64) error {
	r.cleared = append(r.cleared, accountID)
	return nil
}

type controlBindingTargetUpdate struct {
	targetID  int64
	keyID     *int64
	groupID   *int64
	groupName *string
	platform  string
}

type controlBindingProbeRepoStub struct {
	Sub2APIProviderProbeRepository
	targets []*ent.Sub2APIProviderProbeTarget
	updates []controlBindingTargetUpdate
}

func (r *controlBindingProbeRepoStub) ListTargets(context.Context, int64) ([]*ent.Sub2APIProviderProbeTarget, error) {
	return r.targets, nil
}

func (r *controlBindingProbeRepoStub) UpdateTargetBinding(
	_ context.Context,
	targetID int64,
	keyID, groupID *int64,
	groupName *string,
	platform string,
) (*ent.Sub2APIProviderProbeTarget, error) {
	r.updates = append(r.updates, controlBindingTargetUpdate{
		targetID: targetID, keyID: keyID, groupID: groupID, groupName: groupName, platform: platform,
	})
	return &ent.Sub2APIProviderProbeTarget{
		ID: targetID, ProviderAPIKeyID: keyID, RemoteGroupID: groupID, RemoteGroupName: groupName, Platform: platform,
	}, nil
}

func newControlBindingProbeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/keys":
			// The embedded Key object intentionally carries the stale multiplier
			// shown in the pane before this regression was fixed.
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":501,"group_id":15,"group":{"id":15,"name":"Old snapshot","rate_multiplier":0.08}}],"total":1}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":15,"name":"ChatGPT-Pro","rate_multiplier":0.15,"platform":"openai","status":"active"}]}`))
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":30.67}}`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newControlBindingOverrideServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":501,"group_id":15,"group":{"id":15,"name":"ChatGPT-Pro","rate_multiplier":0.08}}],"total":1}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":15,"name":"ChatGPT-Pro","rate_multiplier":0.08,"platform":"openai","status":"active"}]}`))
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":30.67}}`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"code":0,"data":{"15":0.15}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newControlBindingBalanceFailureServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":501,"group_id":15}],"total":1}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":15,"name":"ChatGPT-Pro","rate_multiplier":0.08,"platform":"openai","status":"active"}]}`))
		case "/api/v1/auth/me":
			http.Error(w, `{"message":"temporary balance failure"}`, http.StatusBadGateway)
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"code":0,"data":{"15":0.15}}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newControlBindingProbeService(
	serverURL string,
	accountRepo *controlBindingAccountRepoStub,
	probeRepo *controlBindingProbeRepoStub,
	gate *Sub2APIProviderOperationGate,
) (*Sub2APIProviderProbeService, *ent.Sub2APIProvider) {
	access, refresh := "encrypted:access-live", "encrypted:refresh-live"
	expiresAt := time.Now().Add(time.Hour)
	keysPath, groupsPath := "/api/v1/keys", "/api/v1/groups/available"
	provider := &ent.Sub2APIProvider{
		ID: 91, BaseURL: serverURL, AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &access, RefreshTokenEncrypted: &refresh, AccessTokenExpiresAt: &expiresAt,
		APIPathKeys: &keysPath, APIPathGroups: &groupsPath,
	}
	cache := &remoteOverviewCacheStub{}
	return &Sub2APIProviderProbeService{
		providerRepo:        &remoteOverviewProviderRepo{provider: provider},
		probeRepo:           probeRepo,
		accountRepo:         accountRepo,
		tokenCache:          sub2api.NewTokenCache(),
		encryptor:           providerAuthTestEncryptor{},
		remoteOverviewCache: cache,
		operationGate:       gate,
	}, provider
}

func TestControlProbeSynchronizesEffectiveMultiplierIntoAccountAndPane(t *testing.T) {
	server := newControlBindingProbeServer(t)
	defer server.Close()

	keyID, oldGroupID := int64(501), int64(8)
	oldName, oldMultiplier := "Old group", 0.08
	accountRepo := &controlBindingAccountRepoStub{accounts: []Account{{
		ID: 101, Platform: "openai", ProviderAPIKeyID: &keyID,
		RemoteGroupID: &oldGroupID, RemoteGroupName: &oldName, RemoteGroupMultiplier: &oldMultiplier,
	}}}
	probeRepo := &controlBindingProbeRepoStub{targets: []*ent.Sub2APIProviderProbeTarget{{
		ID: 201, ProviderID: 91, AccountID: 101, ProviderAPIKeyID: &keyID,
		RemoteGroupID: &oldGroupID, RemoteGroupName: &oldName, Platform: "openai",
	}}}
	service, provider := newControlBindingProbeService(server.URL, accountRepo, probeRepo, NewSub2APIProviderOperationGate(nil, nil))
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}

	status := service.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)
	require.Equal(t, "healthy", status)
	require.Equal(t, "updated", result.Details["binding_sync_status"])
	require.Equal(t, 1, result.Details["binding_sync_accounts_updated"])
	require.Len(t, accountRepo.updates, 1)
	require.Equal(t, int64(15), accountRepo.updates[0].groupID)
	require.Equal(t, "ChatGPT-Pro", accountRepo.updates[0].groupName)
	require.InDelta(t, 0.15, accountRepo.updates[0].multiplier, 0.000001)
	require.Len(t, probeRepo.updates, 1)
	require.NotNil(t, probeRepo.updates[0].groupID)
	require.Equal(t, int64(15), *probeRepo.updates[0].groupID)
	require.NotNil(t, probeRepo.updates[0].groupName)
	require.Equal(t, "ChatGPT-Pro", *probeRepo.updates[0].groupName)
}

func TestControlProbeSynchronizesCustomRateOverrideIntoAccountAndPane(t *testing.T) {
	server := newControlBindingOverrideServer(t)
	defer server.Close()

	keyID := int64(501)
	accountRepo := &controlBindingAccountRepoStub{accounts: []Account{{
		ID: 101, Platform: "openai", ProviderAPIKeyID: &keyID,
	}}}
	probeRepo := &controlBindingProbeRepoStub{targets: []*ent.Sub2APIProviderProbeTarget{{
		ID: 201, ProviderID: 91, AccountID: 101, ProviderAPIKeyID: &keyID, Platform: "openai",
	}}}
	service, provider := newControlBindingProbeService(server.URL, accountRepo, probeRepo, NewSub2APIProviderOperationGate(nil, nil))
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}

	status := service.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)

	require.Equal(t, "healthy", status)
	require.Len(t, accountRepo.updates, 1)
	require.InDelta(t, 0.15, accountRepo.updates[0].multiplier, 0.000001)
}

func TestControlProbeKeepsCustomRateWhenBalanceCollectionFails(t *testing.T) {
	server := newControlBindingBalanceFailureServer(t)
	defer server.Close()

	keyID := int64(501)
	accountRepo := &controlBindingAccountRepoStub{accounts: []Account{{
		ID: 101, Platform: "openai", ProviderAPIKeyID: &keyID,
	}}}
	probeRepo := &controlBindingProbeRepoStub{targets: []*ent.Sub2APIProviderProbeTarget{{
		ID: 201, ProviderID: 91, AccountID: 101, ProviderAPIKeyID: &keyID, Platform: "openai",
	}}}
	service, provider := newControlBindingProbeService(server.URL, accountRepo, probeRepo, NewSub2APIProviderOperationGate(nil, nil))
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}

	status := service.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)

	require.Equal(t, "healthy", status)
	require.Equal(t, "failed", result.Details["asset_snapshot_status"])
	require.Len(t, accountRepo.updates, 1)
	require.InDelta(t, 0.15, accountRepo.updates[0].multiplier, 0.000001)
}

func TestLinkedAccountRefreshKeepsCustomEffectiveRate(t *testing.T) {
	server := newControlBindingOverrideServer(t)
	defer server.Close()

	keyID := int64(501)
	accounts := []Account{{ID: 101, Platform: "openai", ProviderAPIKeyID: &keyID}}
	accountRepo := &controlBindingAccountRepoStub{}
	_, provider := newControlBindingProbeService(server.URL, accountRepo, &controlBindingProbeRepoStub{}, NewSub2APIProviderOperationGate(nil, nil))
	service := &Sub2APIProviderService{
		accountRepo:   accountRepo,
		tokenCache:    sub2api.NewTokenCache(),
		encryptor:     providerAuthTestEncryptor{},
		operationGate: NewSub2APIProviderOperationGate(nil, nil),
	}

	service.syncRemoteGroups(context.Background(), provider, accounts)

	require.Len(t, accountRepo.updates, 1)
	require.InDelta(t, 0.15, accountRepo.updates[0].multiplier, 0.000001)
	require.NotNil(t, accounts[0].RemoteGroupMultiplier)
	require.InDelta(t, 0.15, *accounts[0].RemoteGroupMultiplier, 0.000001)
}

func TestControlProbeDoesNotPersistBindingsWhileOptimizerOwnsProvider(t *testing.T) {
	server := newControlBindingProbeServer(t)
	defer server.Close()

	keyID := int64(501)
	accountRepo := &controlBindingAccountRepoStub{accounts: []Account{{ID: 101, Platform: "openai", ProviderAPIKeyID: &keyID}}}
	probeRepo := &controlBindingProbeRepoStub{targets: []*ent.Sub2APIProviderProbeTarget{{ID: 201, ProviderID: 91, AccountID: 101, ProviderAPIKeyID: &keyID}}}
	gate := NewSub2APIProviderOperationGate(nil, nil)
	release, acquired := gate.TryAcquire(context.Background(), 91, sub2APIProviderOptimizeOperationTTL)
	require.True(t, acquired)
	defer release()

	service, provider := newControlBindingProbeService(server.URL, accountRepo, probeRepo, gate)
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}
	status := service.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)

	require.Equal(t, "healthy", status)
	require.Equal(t, "skipped_busy", result.Details["binding_sync_status"])
	require.Empty(t, accountRepo.updates)
	require.Empty(t, accountRepo.cleared)
	require.Empty(t, probeRepo.updates)
}
