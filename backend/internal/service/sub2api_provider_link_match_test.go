//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type providerLinkRepoStub struct {
	Sub2APIProviderRepository
	provider *ent.Sub2APIProvider
}

func (s *providerLinkRepoStub) GetByID(context.Context, int64) (*ent.Sub2APIProvider, error) {
	return s.provider, nil
}

func (s *providerLinkRepoStub) UpdateAuthError(context.Context, int64, *string) error {
	return nil
}

func (s *providerLinkRepoStub) PersistTokenPair(context.Context, int64, string, string, time.Time, time.Time) error {
	return nil
}

type providerLinkAccountRepoStub struct {
	Sub2APIAccountRepository
	account         *Account
	linkedAccountID int64
	linkedProvider  int64
	linkedRemoteKey int64
}

func (s *providerLinkAccountRepoStub) GetByID(context.Context, int64) (*Account, error) {
	return s.account, nil
}

func (s *providerLinkAccountRepoStub) UpdateProviderLink(_ context.Context, accountID, providerID, providerAPIKeyID int64) error {
	s.linkedAccountID = accountID
	s.linkedProvider = providerID
	s.linkedRemoteKey = providerAPIKeyID
	return nil
}

func TestFindProviderRemoteAPIKeyIDMatchesCompatibleRepresentations(t *testing.T) {
	localKey := "sk-78eb5b6a47f8e11223344556677889900aabbccddeeff"
	tests := []struct {
		name      string
		remoteKey sub2api.APIKey
	}{
		{name: "full key", remoteKey: sub2api.APIKey{ID: 11, Key: localKey}},
		{name: "trimmed full key", remoteKey: sub2api.APIKey{ID: 11, Key: "  " + localKey + "\n"}},
		{name: "legacy api_key field", remoteKey: sub2api.APIKey{ID: 11, LegacyKey: localKey}},
		{name: "missing sk prefix", remoteKey: sub2api.APIKey{ID: 11, Key: localKey[3:]}},
		{name: "long unique prefix", remoteKey: sub2api.APIKey{ID: 11, KeyPrefix: localKey[:18]}},
		{name: "masked ellipsis", remoteKey: sub2api.APIKey{ID: 11, MaskedKey: localKey[:16] + "..." + localKey[len(localKey)-4:]}},
		{name: "masked stars in key field", remoteKey: sub2api.APIKey{ID: 11, Key: localKey[:16] + "********" + localKey[len(localKey)-4:]}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := findProviderRemoteAPIKeyID(localKey, []sub2api.APIKey{tt.remoteKey})
			require.NoError(t, err)
			require.NotNil(t, id)
			assert.Equal(t, int64(11), *id)
		})
	}
}

func TestFindProviderRemoteAPIKeyIDRejectsWeakOrAmbiguousMatches(t *testing.T) {
	localKey := "sk-78eb5b6a47f8e11223344556677889900aabbccddeeff"

	t.Run("short prefix", func(t *testing.T) {
		id, err := findProviderRemoteAPIKeyID(localKey, []sub2api.APIKey{{ID: 1, KeyPrefix: "sk-78eb5"}})
		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("different key", func(t *testing.T) {
		id, err := findProviderRemoteAPIKeyID(localKey, []sub2api.APIKey{{ID: 1, Key: "sk-different-key-value"}})
		require.NoError(t, err)
		assert.Nil(t, id)
	})

	t.Run("ambiguous masked keys", func(t *testing.T) {
		prefix := localKey[:18]
		id, err := findProviderRemoteAPIKeyID(localKey, []sub2api.APIKey{
			{ID: 1, KeyPrefix: prefix},
			{ID: 2, MaskedKey: prefix + "..."},
		})
		assert.Nil(t, id)
		assert.True(t, errors.Is(err, errProviderRemoteAPIKeyAmbiguous))
	})
}

func TestFindProviderRemoteAPIKeyIDPrefersFullMatchOverMaskedCandidates(t *testing.T) {
	localKey := "sk-78eb5b6a47f8e11223344556677889900aabbccddeeff"
	id, err := findProviderRemoteAPIKeyID(localKey, []sub2api.APIKey{
		{ID: 1, Key: localKey},
		{ID: 2, KeyPrefix: localKey[:18]},
	})
	require.NoError(t, err)
	require.NotNil(t, id)
	assert.Equal(t, int64(1), *id)
}

func TestLinkAccountPasswordAuthRefreshesCachedIdentityAndMatchesLegacyField(t *testing.T) {
	const (
		providerID = int64(17)
		accountID  = int64(29)
		remoteID   = int64(41)
		apiKey     = "sk-78eb5b6a47f8e11223344556677889900aabbccddeeff"
	)
	loginCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			loginCalls++
			var request sub2api.LoginRequest
			require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
			assert.Equal(t, "owner@example.com", request.Email)
			assert.Equal(t, "current-password", request.Password)
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"fresh-access","refresh_token":"fresh-refresh","expires_in":3600}}`))
		case "/api/v1/keys":
			assert.Equal(t, "Bearer fresh-access", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"code":0,"data":{"items":[{"id":41,"api_key":"` + apiKey[3:] + `"}],"total":1}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	providerRepo := &providerLinkRepoStub{provider: &ent.Sub2APIProvider{
		ID: providerID, BaseURL: server.URL, Email: "owner@example.com", PasswordEncrypted: "current-password",
		AuthMode: domain.Sub2APIProviderAuthModePassword,
	}}
	accountRepo := &providerLinkAccountRepoStub{account: &Account{
		ID: accountID, Name: "linked account", Platform: PlatformOpenAI,
		Credentials: map[string]any{"api_key": "  " + apiKey + "\n"},
	}}
	tokenCache := sub2api.NewTokenCache()
	tokenCache.SetTokenPair(providerID, "old-user-access", "old-user-refresh", time.Hour)
	svc := &Sub2APIProviderService{repo: providerRepo, accountRepo: accountRepo, tokenCache: tokenCache}

	linked, err := svc.LinkAccount(context.Background(), providerID, accountID)
	require.NoError(t, err)
	require.NotNil(t, linked)
	assert.Equal(t, 1, loginCalls)
	assert.Equal(t, accountID, accountRepo.linkedAccountID)
	assert.Equal(t, providerID, accountRepo.linkedProvider)
	assert.Equal(t, remoteID, accountRepo.linkedRemoteKey)
	require.NotNil(t, linked.ProviderAPIKeyID)
	assert.Equal(t, remoteID, *linked.ProviderAPIKeyID)
}
