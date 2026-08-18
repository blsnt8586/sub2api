//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/sub2api"
)

type providerAuthTestEncryptor struct{}

func (providerAuthTestEncryptor) Encrypt(plaintext string) (string, error) {
	return "encrypted:" + plaintext, nil
}

func (providerAuthTestEncryptor) Decrypt(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, "encrypted:") {
		return "", errors.New("invalid ciphertext")
	}
	return strings.TrimPrefix(ciphertext, "encrypted:"), nil
}

type providerAuthTestRepo struct {
	persistedAccess  string
	persistedRefresh string
	persistedExpiry  time.Time
	authError        *string
}

type providerProxyLookupStub struct {
	proxy *Proxy
	err   error
}

func (s *providerProxyLookupStub) GetByID(context.Context, int64) (*Proxy, error) {
	return s.proxy, s.err
}

func (*providerAuthTestRepo) Create(context.Context, *CreateSub2APIProviderInput) (*ent.Sub2APIProvider, error) {
	return nil, errors.New("not implemented")
}
func (*providerAuthTestRepo) GetByID(context.Context, int64) (*ent.Sub2APIProvider, error) {
	return nil, errors.New("not implemented")
}
func (*providerAuthTestRepo) GetByIDWithAccounts(context.Context, int64) (*ent.Sub2APIProvider, error) {
	return nil, errors.New("not implemented")
}
func (*providerAuthTestRepo) List(context.Context, *Sub2APIProviderFilters, int, int) ([]*ent.Sub2APIProvider, int, error) {
	return nil, 0, errors.New("not implemented")
}
func (*providerAuthTestRepo) ListAll(context.Context, *Sub2APIProviderFilters) ([]*ent.Sub2APIProvider, error) {
	return nil, errors.New("not implemented")
}
func (*providerAuthTestRepo) Update(context.Context, int64, *UpdateSub2APIProviderInput) (*ent.Sub2APIProvider, error) {
	return nil, errors.New("not implemented")
}
func (*providerAuthTestRepo) Delete(context.Context, int64) error {
	return errors.New("not implemented")
}
func (*providerAuthTestRepo) UpdateSyncStatus(context.Context, int64, string, *string) error {
	return errors.New("not implemented")
}
func (*providerAuthTestRepo) UpdateAPIPaths(context.Context, int64, string, string) error {
	return errors.New("not implemented")
}
func (r *providerAuthTestRepo) PersistTokenPair(_ context.Context, _ int64, access, refresh string, expiresAt, _ time.Time) error {
	r.persistedAccess = access
	r.persistedRefresh = refresh
	r.persistedExpiry = expiresAt
	return nil
}
func (r *providerAuthTestRepo) UpdateAuthError(_ context.Context, _ int64, message *string) error {
	r.authError = message
	return nil
}

func TestPrepareCreateProviderAuthRequiresStableEncryptionKey(t *testing.T) {
	access, refresh := "access", "refresh"
	svc := &Sub2APIProviderService{encryptor: providerAuthTestEncryptor{}}
	_, err := svc.prepareCreateProviderAuth(&CreateProviderInput{
		AuthMode: domain.Sub2APIProviderAuthModeTokenPair, AccessToken: &access, RefreshToken: &refresh,
	})
	if err == nil || !strings.Contains(err.Error(), "totp.encryption_key") {
		t.Fatalf("prepareCreateProviderAuth error=%v, want fixed encryption key requirement", err)
	}
}

func TestPrepareCreateProviderAuthEncryptsImportedTokens(t *testing.T) {
	access, refresh := "access-secret", "refresh-secret"
	svc := &Sub2APIProviderService{encryptor: providerAuthTestEncryptor{}, tokenEncryptionKeyConfigured: true}
	prepared, err := svc.prepareCreateProviderAuth(&CreateProviderInput{
		AuthMode: domain.Sub2APIProviderAuthModeTokenPair, AccessToken: &access, RefreshToken: &refresh,
	})
	if err != nil {
		t.Fatalf("prepareCreateProviderAuth: %v", err)
	}
	if prepared.accessEncrypted == nil || *prepared.accessEncrypted != "encrypted:access-secret" {
		t.Fatalf("access token was not encrypted: %+v", prepared.accessEncrypted)
	}
	if prepared.refreshEncrypted == nil || *prepared.refreshEncrypted != "encrypted:refresh-secret" {
		t.Fatalf("refresh token was not encrypted: %+v", prepared.refreshEncrypted)
	}
	if prepared.expiresAt == nil || !prepared.expiresAt.After(time.Now()) {
		t.Fatalf("missing conservative token expiry: %+v", prepared.expiresAt)
	}
}

func TestValidateProviderProxyRequiresActiveUnexpiredProxy(t *testing.T) {
	proxyID := int64(7)
	active := &Proxy{ID: proxyID, Status: StatusActive}
	svc := &Sub2APIProviderService{proxyRepo: &providerProxyLookupStub{proxy: active}}
	if err := svc.validateProviderProxy(context.Background(), &proxyID); err != nil {
		t.Fatalf("validate active proxy: %v", err)
	}

	expiredAt := time.Now().Add(-time.Minute)
	svc.proxyRepo = &providerProxyLookupStub{proxy: &Proxy{ID: proxyID, Status: StatusActive, ExpiresAt: &expiredAt}}
	if err := svc.validateProviderProxy(context.Background(), &proxyID); err == nil {
		t.Fatal("expired proxy should be rejected")
	}

	svc.proxyRepo = &providerProxyLookupStub{proxy: &Proxy{ID: proxyID, Status: "inactive"}}
	if err := svc.validateProviderProxy(context.Background(), &proxyID); err == nil {
		t.Fatal("inactive proxy should be rejected")
	}
}

func TestConfigureProviderProxyFailsClosedWhenEdgeIsUnavailable(t *testing.T) {
	proxyID := int64(9)
	client := sub2api.NewClient("https://provider.example.com", "admin@example.com", "secret")
	err := configureSub2APIProviderProxy(client, &ent.Sub2APIProvider{ID: 3, ProxyID: &proxyID})
	if err == nil || !strings.Contains(err.Error(), "proxy 9 is unavailable") {
		t.Fatalf("configure error=%v, want unavailable proxy", err)
	}
}

func TestAuthedProviderClientRestoresAndPersistsRotatedTokenPair(t *testing.T) {
	var loginCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"code":0,"data":{"access_token":"access-new","refresh_token":"refresh-new","expires_in":3600}}`))
		case "/api/v1/auth/login":
			loginCalls++
			http.Error(w, "login must not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	accessEncrypted, refreshEncrypted := "encrypted:access-old", "encrypted:refresh-old"
	expired := time.Now().Add(-time.Minute)
	provider := &ent.Sub2APIProvider{
		ID: 41, BaseURL: server.URL, Email: "admin@example.com", AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &accessEncrypted, RefreshTokenEncrypted: &refreshEncrypted, AccessTokenExpiresAt: &expired,
	}
	repo := &providerAuthTestRepo{}
	client, err := newAuthedSub2APIProviderClient(context.Background(), provider, repo, sub2api.NewTokenCache(), providerAuthTestEncryptor{})
	if err != nil {
		t.Fatalf("newAuthedSub2APIProviderClient: %v", err)
	}
	if loginCalls != 0 {
		t.Fatalf("password login calls=%d, want 0", loginCalls)
	}
	if client.Token != "access-new" || client.RefreshToken != "refresh-new" {
		t.Fatalf("client token pair=(%q,%q), want rotated pair", client.Token, client.RefreshToken)
	}
	if repo.persistedAccess != "encrypted:access-new" || repo.persistedRefresh != "encrypted:refresh-new" || repo.persistedExpiry.IsZero() {
		t.Fatalf("persisted token state=(%q,%q,%v)", repo.persistedAccess, repo.persistedRefresh, repo.persistedExpiry)
	}
}

func TestProviderJSONNeverIncludesStoredSecrets(t *testing.T) {
	password := "password-secret"
	access := "encrypted:access-secret"
	refresh := "encrypted:refresh-secret"
	encoded, err := json.Marshal(providerFromEnt(&ent.Sub2APIProvider{
		ID: 1, Name: "provider", BaseURL: "https://example.com", ProviderType: "sub2api", Status: "active",
		Email: "admin@example.com", PasswordEncrypted: password, AuthMode: domain.Sub2APIProviderAuthModeTokenPair,
		AccessTokenEncrypted: &access, RefreshTokenEncrypted: &refresh,
	}))
	if err != nil {
		t.Fatalf("marshal provider: %v", err)
	}
	body := string(encoded)
	for _, secret := range []string{password, access, refresh} {
		if strings.Contains(body, secret) {
			t.Fatalf("provider JSON leaked secret %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, `"has_access_token":true`) || !strings.Contains(body, `"has_refresh_token":true`) {
		t.Fatalf("provider JSON did not expose safe credential status: %s", body)
	}
}
