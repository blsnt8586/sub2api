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
)

type remoteOverviewProviderRepo struct {
	providerAuthTestRepo
	provider *ent.Sub2APIProvider
}

type remoteOverviewCacheStub struct {
	items    map[int64]*Sub2APIProviderRemoteOverview
	failures []string
}

func (s *remoteOverviewCacheStub) GetMany(_ context.Context, providerIDs []int64) (map[int64]*Sub2APIProviderRemoteOverview, error) {
	result := make(map[int64]*Sub2APIProviderRemoteOverview)
	for _, providerID := range providerIDs {
		if overview := s.items[providerID]; overview != nil {
			result[providerID] = overview
		}
	}
	return result, nil
}

func (s *remoteOverviewCacheStub) StoreSuccess(_ context.Context, overview *Sub2APIProviderRemoteOverview) error {
	if s.items == nil {
		s.items = make(map[int64]*Sub2APIProviderRemoteOverview)
	}
	copy := *overview
	s.items[overview.ProviderID] = &copy
	return nil
}

func (s *remoteOverviewCacheStub) StoreFailure(_ context.Context, _ int64, _ string, _ time.Time, errorMessage string) error {
	s.failures = append(s.failures, errorMessage)
	return nil
}

func (r *remoteOverviewProviderRepo) GetByID(context.Context, int64) (*ent.Sub2APIProvider, error) {
	return r.provider, nil
}

func TestGetRemoteOverviewMergesCustomRatesAndSortsByEffectiveMultiplier(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer access-live" {
			t.Fatalf("Authorization=%q, want imported access token", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":88.5}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":1,"name":"Standard","description":"Default route","rate_multiplier":1,"platform":"openai","status":"active"},{"id":2,"name":"Economy","rate_multiplier":0.5,"platform":"anthropic","status":"active"}]}`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"code":0,"data":{"1":0.25}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	access, refresh := "encrypted:access-live", "encrypted:refresh-live"
	expiresAt := time.Now().Add(time.Hour)
	groupsPath := "/api/v1/groups/available"
	repo := &remoteOverviewProviderRepo{provider: &ent.Sub2APIProvider{
		ID:                    77,
		BaseURL:               server.URL,
		AuthMode:              domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted:  &access,
		RefreshTokenEncrypted: &refresh,
		AccessTokenExpiresAt:  &expiresAt,
		APIPathGroups:         &groupsPath,
	}}
	cache := &remoteOverviewCacheStub{}
	service := &Sub2APIProviderService{
		repo:                repo,
		tokenCache:          sub2api.NewTokenCache(),
		encryptor:           providerAuthTestEncryptor{},
		remoteOverviewCache: cache,
	}

	overview, err := service.GetRemoteOverview(context.Background(), 77)
	if err != nil {
		t.Fatalf("GetRemoteOverview: %v", err)
	}
	if overview.ProviderID != 77 || overview.Balance != 88.5 || !overview.RateOverridesAvailable {
		t.Fatalf("overview summary=%+v", overview)
	}
	if !overview.Available || overview.Source != Sub2APIProviderRemoteOverviewSourceManual || overview.LastAttemptSource != Sub2APIProviderRemoteOverviewSourceManual {
		t.Fatalf("overview collection metadata=%+v", overview)
	}
	if cached := cache.items[77]; cached == nil || cached.Balance != overview.Balance {
		t.Fatalf("cached overview=%+v, want successful live snapshot", cached)
	}
	if len(overview.Groups) != 2 {
		t.Fatalf("groups=%d, want 2", len(overview.Groups))
	}
	if first := overview.Groups[0]; first.ID != 1 || first.DefaultMultiplier != 1 || first.EffectiveMultiplier != 0.25 || !first.HasCustomRate {
		t.Fatalf("first group=%+v, want custom Standard rate sorted first", first)
	}
	if second := overview.Groups[1]; second.ID != 2 || second.EffectiveMultiplier != 0.5 || second.HasCustomRate {
		t.Fatalf("second group=%+v, want default Economy rate", second)
	}
}

func TestControlProbeAssetFailureDoesNotChangeAvailability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/keys":
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[],"total":0}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":1,"name":"Default","rate_multiplier":1}]}`))
		case "/api/v1/auth/me":
			http.Error(w, `{"message":"wallet temporarily unavailable"}`, http.StatusServiceUnavailable)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	access, refresh := "encrypted:access-live", "encrypted:refresh-live"
	expiresAt := time.Now().Add(time.Hour)
	keysPath, groupsPath := "/api/v1/keys", "/api/v1/groups/available"
	provider := &ent.Sub2APIProvider{
		ID: 79, BaseURL: server.URL, AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &access, RefreshTokenEncrypted: &refresh, AccessTokenExpiresAt: &expiresAt,
		APIPathKeys: &keysPath, APIPathGroups: &groupsPath,
	}
	repo := &remoteOverviewProviderRepo{provider: provider}
	cache := &remoteOverviewCacheStub{}
	probeService := &Sub2APIProviderProbeService{
		providerRepo:        repo,
		tokenCache:          sub2api.NewTokenCache(),
		encryptor:           providerAuthTestEncryptor{},
		remoteOverviewCache: cache,
	}
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}
	status := probeService.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)
	if status != "healthy" {
		t.Fatalf("control status=%q, want healthy despite asset failure", status)
	}
	if result.Details["asset_snapshot_status"] != "failed" || len(cache.failures) != 1 {
		t.Fatalf("asset status=%v failures=%v", result.Details["asset_snapshot_status"], cache.failures)
	}
}

func TestControlProbeCollectsAssetsWhenKeysEndpointFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/api/v1/keys":
			http.Error(w, `{"message":"keys temporarily unavailable"}`, http.StatusServiceUnavailable)
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":1,"name":"Default","rate_multiplier":1}]}`))
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":42.5}}`))
		case "/api/v1/groups/rates":
			_, _ = w.Write([]byte(`{"code":0,"data":{}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	access, refresh := "encrypted:access-live", "encrypted:refresh-live"
	expiresAt := time.Now().Add(time.Hour)
	keysPath, groupsPath := "/api/v1/keys", "/api/v1/groups/available"
	provider := &ent.Sub2APIProvider{
		ID: 80, BaseURL: server.URL, AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &access, RefreshTokenEncrypted: &refresh, AccessTokenExpiresAt: &expiresAt,
		APIPathKeys: &keysPath, APIPathGroups: &groupsPath,
	}
	cache := &remoteOverviewCacheStub{}
	probeService := &Sub2APIProviderProbeService{
		providerRepo:        &remoteOverviewProviderRepo{provider: provider},
		tokenCache:          sub2api.NewTokenCache(),
		encryptor:           providerAuthTestEncryptor{},
		remoteOverviewCache: cache,
	}
	result := &Sub2APIProviderProbeRunInput{ProviderID: provider.ID, Details: map[string]any{}}
	status := probeService.runControl(context.Background(), provider, &ent.Sub2APIProviderProbeConfig{TimeoutSeconds: 5}, result)
	if status != "degraded" {
		t.Fatalf("control status=%q, want degraded for keys failure", status)
	}
	if cached := cache.items[provider.ID]; cached == nil || cached.Balance != 42.5 {
		t.Fatalf("cached overview=%+v, want asset snapshot despite keys failure", cached)
	}
	if result.Details["asset_snapshot_status"] != "updated" {
		t.Fatalf("asset status=%v, want updated", result.Details["asset_snapshot_status"])
	}
}

func TestGetRemoteOverviewFallsBackWhenOptionalRatesEndpointIsUnavailable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/me":
			_, _ = w.Write([]byte(`{"code":0,"data":{"balance":10}}`))
		case "/api/v1/groups/available":
			_, _ = w.Write([]byte(`{"code":0,"data":[{"id":3,"name":"Legacy","rate_multiplier":0.8}]}`))
		case "/api/v1/groups/rates":
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	access, refresh := "encrypted:access-live", "encrypted:refresh-live"
	expiresAt := time.Now().Add(time.Hour)
	repo := &remoteOverviewProviderRepo{provider: &ent.Sub2APIProvider{
		ID: 78, BaseURL: server.URL, AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &access, RefreshTokenEncrypted: &refresh, AccessTokenExpiresAt: &expiresAt,
	}}
	service := &Sub2APIProviderService{repo: repo, tokenCache: sub2api.NewTokenCache(), encryptor: providerAuthTestEncryptor{}}

	overview, err := service.GetRemoteOverview(context.Background(), 78)
	if err != nil {
		t.Fatalf("GetRemoteOverview: %v", err)
	}
	if overview.RateOverridesAvailable || len(overview.Groups) != 1 || overview.Groups[0].EffectiveMultiplier != 0.8 {
		t.Fatalf("fallback overview=%+v", overview)
	}
}
