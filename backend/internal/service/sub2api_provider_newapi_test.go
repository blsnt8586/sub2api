//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
	"github.com/stretchr/testify/require"
)

func TestNewAPIProviderAuthenticationValidation(t *testing.T) {
	s := &Sub2APIProviderService{encryptor: providerAuthTestEncryptor{}, providerTokenKeyConfigured: true}
	token := "browser-session-secret"
	_, err := s.prepareCreateProviderAuth(&CreateProviderInput{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessToken: &token})
	require.ErrorContains(t, err, "credentials are missing")
	prepared, err := s.prepareCreateProviderAuth(&CreateProviderInput{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessToken: &token, RefreshToken: &token})
	require.NoError(t, err)
	require.Equal(t, "encrypted:browser-session-secret", *prepared.accessEncrypted)
	require.Equal(t, "encrypted:browser-session-secret", *prepared.refreshEncrypted)
	require.NotNil(t, prepared.expiresAt)
	existing := &ent.Sub2APIProvider{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessTokenEncrypted: prepared.accessEncrypted, RefreshTokenEncrypted: prepared.refreshEncrypted}
	update, err := s.prepareUpdateProviderAuth(existing, &UpdateProviderInput{})
	require.NoError(t, err)
	require.Nil(t, update.AccessTokenEncrypted)
	require.False(t, update.TokenPairUpdated)
	rotated := "rotated"
	update, err = s.prepareUpdateProviderAuth(existing, &UpdateProviderInput{AccessToken: &rotated})
	require.ErrorContains(t, err, "credentials are missing")
	rotatedExisting := &ent.Sub2APIProvider{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessTokenEncrypted: prepared.accessEncrypted, RefreshTokenEncrypted: prepared.refreshEncrypted}
	update, err = s.prepareUpdateProviderAuth(rotatedExisting, &UpdateProviderInput{AccessToken: &rotated})
	require.ErrorContains(t, err, "credentials are missing")
	update, err = s.prepareUpdateProviderAuth(existing, &UpdateProviderInput{AccessToken: &rotated, RefreshToken: &rotated})
	require.NoError(t, err)
	require.Equal(t, "encrypted:rotated", *update.AccessTokenEncrypted)
	require.Equal(t, "encrypted:rotated", *update.RefreshTokenEncrypted)
	noCookieExisting := &ent.Sub2APIProvider{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessTokenEncrypted: prepared.accessEncrypted}
	update, err = s.prepareUpdateProviderAuth(noCookieExisting, &UpdateProviderInput{})
	require.NoError(t, err)
	require.False(t, update.ClearRefreshToken)
	for _, input := range []CreateProviderInput{{ProviderType: "newapi", AuthMode: "token_pair", AccessToken: &token, RefreshToken: &token}, {ProviderType: "sub2api", AuthMode: "access_token", AccessToken: &token}} {
		_, err = s.prepareCreateProviderAuth(&input)
		require.ErrorIs(t, err, ErrInvalidProviderAuth)
	}
	s.providerTokenKeyConfigured = false
	_, err = s.prepareCreateProviderAuth(&CreateProviderInput{ProviderType: "newapi", AuthMode: domain.Sub2APIProviderAuthModeBrowserSession, AccessToken: &token, RefreshToken: &token})
	require.ErrorContains(t, err, "provider_token_key")
}

func TestNewAPIProviderIdentity(t *testing.T) {
	for _, tt := range []struct {
		protocol, mode, identity string
		valid                    bool
	}{
		{"newapi", "password", "username", true}, {"newapi", "access_token", "42", true}, {"newapi", "browser_session", "42", true}, {"newapi", "browser_session", "user@example.com", false}, {"newapi", "access_token", "user@example.com", false}, {"newapi", "access_token", "0", false}, {"newapi", "access_token", "-1", false},
		{"sub2api", "password", "user@example.com", true}, {"sub2api", "password", "username", false}, {"sub2api", "token_pair", "user@example.com", true}, {"newapi", "token_pair", "user@example.com", false},
	} {
		t.Run(tt.protocol+tt.mode+tt.identity, func(t *testing.T) {
			err := validateProviderIdentity(tt.protocol, tt.mode, tt.identity)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestNewAPIProviderOverviewUsesActualConsumption(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"success":true,"data":{"quota_per_unit":500000}}`))
		case "/api/user/self":
			require.Equal(t, "Bearer management-secret", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":42,"quota":1000000,"used_quota":750000}}`))
		case "/api/user/self/groups":
			_, _ = w.Write([]byte(`{"success":true,"data":{"default":{"ratio":0.3,"desc":"effective"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	encrypted := "encrypted:management-secret"
	p := &ent.Sub2APIProvider{ID: 5, ProviderType: domain.ProviderTypeNewAPI, BaseURL: server.URL, Email: "42", AuthMode: "access_token", AccessTokenEncrypted: &encrypted}
	client, err := newAuthedSub2APIProviderClient(context.Background(), p, &providerAuthTestRepo{}, sub2api.NewTokenCache(), providerAuthTestEncryptor{})
	require.NoError(t, err)
	groups, err := client.GetGroups(context.Background(), "")
	require.NoError(t, err)
	overview, err := collectSub2APIProviderRemoteOverview(context.Background(), p.ID, client, groups, 2, "test", p.CreatedAt)
	require.NoError(t, err)
	require.Equal(t, 1.0, overview.Balance)
	require.Equal(t, .75, overview.Usage.TotalConsumed)
	require.Zero(t, overview.Usage.TotalRecharged)
	require.False(t, overview.Usage.FundingSummaryAvailable)
	require.False(t, overview.Usage.DashboardAvailable)
	require.Equal(t, .3, overview.Groups[0].EffectiveMultiplier)
}
