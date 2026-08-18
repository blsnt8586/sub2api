package sub2api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func writeSub2APIResponse(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data}); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}

func TestLoginKeepsRefreshTokenAndExpiry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("path=%q, want login path", r.URL.Path)
		}
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode login: %v", err)
		}
		if req.Email != "admin@example.com" || req.Password != "secret" {
			t.Fatalf("unexpected credentials: %+v", req)
		}
		writeSub2APIResponse(t, w, map[string]any{
			"access_token":  "access-1",
			"refresh_token": "refresh-1",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if client.Token != "access-1" || client.RefreshToken != "refresh-1" {
		t.Fatalf("token pair=(%q,%q), want access-1/refresh-1", client.Token, client.RefreshToken)
	}
	if client.TokenExpiresIn != time.Hour {
		t.Fatalf("TokenExpiresIn=%s, want 1h", client.TokenExpiresIn)
	}
}

func TestNewClientNormalizesTrailingSlashBeforeAPIRequests(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("path=%q, want normalized login path", r.URL.Path)
		}
		writeSub2APIResponse(t, w, map[string]any{
			"access_token": "access-1",
			"expires_in":   3600,
		})
	}))
	defer server.Close()

	client := NewClient(server.URL+"/", "admin@example.com", "secret")
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login with trailing-slash base URL: %v", err)
	}
	if client.BaseURL != server.URL {
		t.Fatalf("BaseURL=%q, want %q", client.BaseURL, server.URL)
	}
}

func TestNewClientNormalizesKnownSub2APIPageAndAPIPaths(t *testing.T) {
	tests := map[string]string{
		"https://provider.example.com/keys":         "https://provider.example.com",
		"https://provider.example.com/api/v1/keys/": "https://provider.example.com",
		"https://provider.example.com/prefix/keys":  "https://provider.example.com/prefix",
	}
	for raw, want := range tests {
		if got := NewClient(raw, "", "").BaseURL; got != want {
			t.Errorf("NewClient(%q).BaseURL=%q, want %q", raw, got, want)
		}
	}
}

func TestClientConfigureProxyRoutesRequestsThroughProxy(t *testing.T) {
	var requests atomic.Int32
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.URL.Host != "upstream.invalid" || r.URL.Path != "/api/v1/auth/login" {
			t.Fatalf("proxied request URL=%q, want upstream.invalid login endpoint", r.URL.String())
		}
		writeSub2APIResponse(t, w, map[string]any{
			"access_token": "proxied-access",
			"expires_in":   3600,
		})
	}))
	defer proxyServer.Close()

	client := NewClient("http://upstream.invalid", "admin@example.com", "secret")
	if err := client.ConfigureProxy(proxyServer.URL); err != nil {
		t.Fatalf("ConfigureProxy: %v", err)
	}
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login through proxy: %v", err)
	}
	if requests.Load() != 1 || client.Token != "proxied-access" {
		t.Fatalf("proxy requests=%d token=%q, want 1/proxied-access", requests.Load(), client.Token)
	}
}

func TestClientConfigureProxyRejectsInvalidURLWithoutDirectFallback(t *testing.T) {
	client := NewClient("http://upstream.invalid", "admin@example.com", "secret")
	original := client.HTTPClient
	if err := client.ConfigureProxy("not-a-proxy-url"); err == nil {
		t.Fatal("ConfigureProxy should reject an invalid proxy URL")
	}
	if client.HTTPClient != original {
		t.Fatal("invalid proxy configuration must not replace the existing client")
	}
}

func TestClientDoesNotPersistHTMLWhenJSONEndpointReturnsFrontendPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!doctype html><html><body>frontend</body></html>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	err := client.Login(context.Background())
	if err == nil || strings.Contains(err.Error(), "<!doctype") || strings.Contains(err.Error(), "frontend") {
		t.Fatalf("Login error=%v, want bounded non-HTML protocol error", err)
	}
	if !strings.Contains(err.Error(), "unexpected upstream response") {
		t.Fatalf("Login error=%v, want unexpected response classification", err)
	}
	var unexpected *ErrUnexpectedResponse
	if !errors.As(err, &unexpected) {
		t.Fatalf("Login error=%v, want ErrUnexpectedResponse", err)
	}
}

func TestEnsureLoggedInRefreshesExpiredAccessToken(t *testing.T) {
	var refreshRequests atomic.Int32
	var loginRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshRequests.Add(1)
			var req RefreshTokenRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode refresh: %v", err)
			}
			if req.RefreshToken != "refresh-old" {
				t.Fatalf("refresh_token=%q, want refresh-old", req.RefreshToken)
			}
			writeSub2APIResponse(t, w, map[string]any{
				"access_token":  "access-new",
				"refresh_token": "refresh-new",
				"expires_in":    3600,
			})
		case "/api/v1/auth/login":
			loginRequests.Add(1)
			http.Error(w, "login should not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(7, "access-old", "refresh-old", time.Hour)
	cache.EvictAccess(7)
	client := NewClient(server.URL, "admin@example.com", "secret")
	if err := client.EnsureLoggedIn(context.Background(), 7, cache); err != nil {
		t.Fatalf("EnsureLoggedIn: %v", err)
	}
	if client.Token != "access-new" || client.RefreshToken != "refresh-new" {
		t.Fatalf("token pair=(%q,%q), want access-new/refresh-new", client.Token, client.RefreshToken)
	}
	if refreshRequests.Load() != 1 || loginRequests.Load() != 0 {
		t.Fatalf("refresh requests=%d login requests=%d, want 1/0", refreshRequests.Load(), loginRequests.Load())
	}
}

func TestEnsureLoggedInFallsBackToPasswordWhenRefreshFails(t *testing.T) {
	var refreshRequests atomic.Int32
	var loginRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshRequests.Add(1)
			http.Error(w, "expired refresh token", http.StatusUnauthorized)
		case "/api/v1/auth/login":
			loginRequests.Add(1)
			writeSub2APIResponse(t, w, map[string]any{
				"access_token": "legacy-access",
				"expires_in":   3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(8, "access-old", "refresh-expired", time.Hour)
	cache.EvictAccess(8)
	client := NewClient(server.URL, "admin@example.com", "secret")
	if err := client.EnsureLoggedIn(context.Background(), 8, cache); err != nil {
		t.Fatalf("EnsureLoggedIn: %v", err)
	}
	if client.Token != "legacy-access" || client.RefreshToken != "" {
		t.Fatalf("token pair=(%q,%q), want legacy-access with no refresh token", client.Token, client.RefreshToken)
	}
	if refreshRequests.Load() != 1 || loginRequests.Load() != 1 {
		t.Fatalf("refresh requests=%d login requests=%d, want 1/1", refreshRequests.Load(), loginRequests.Load())
	}
}

func TestImportedTokenAuthNeverFallsBackToPassword(t *testing.T) {
	var refreshRequests atomic.Int32
	var loginRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/auth/refresh":
			refreshRequests.Add(1)
			http.Error(w, "expired refresh token", http.StatusUnauthorized)
		case "/api/v1/auth/login":
			loginRequests.Add(1)
			http.Error(w, "password login must not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(80, "access-old", "refresh-expired", time.Hour)
	cache.EvictAccess(80)
	client := NewClient(server.URL, "admin@example.com", "unused")
	client.ConfigureImportedTokenAuth(nil)
	err := client.EnsureLoggedIn(context.Background(), 80, cache)
	var interactionRequired *ErrAuthInteractionRequired
	if !errors.As(err, &interactionRequired) {
		t.Fatalf("EnsureLoggedIn error=%v, want ErrAuthInteractionRequired", err)
	}
	if refreshRequests.Load() != 1 || loginRequests.Load() != 0 {
		t.Fatalf("refresh requests=%d login requests=%d, want 1/0", refreshRequests.Load(), loginRequests.Load())
	}
}

func TestImportedTokenRefreshPersistsRotatedPairBeforeUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSub2APIResponse(t, w, map[string]any{
			"access_token":  "access-new",
			"refresh_token": "refresh-new",
			"expires_in":    3600,
		})
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(81, "access-old", "refresh-old", time.Hour)
	cache.EvictAccess(81)
	var persisted TokenPair
	client := NewClient(server.URL, "admin@example.com", "unused")
	client.ConfigureImportedTokenAuth(func(_ context.Context, pair TokenPair) error {
		persisted = pair
		return nil
	})
	if err := client.EnsureLoggedIn(context.Background(), 81, cache); err != nil {
		t.Fatalf("EnsureLoggedIn: %v", err)
	}
	if persisted.AccessToken != "access-new" || persisted.RefreshToken != "refresh-new" {
		t.Fatalf("persisted pair=(%q,%q), want rotated pair", persisted.AccessToken, persisted.RefreshToken)
	}
	if client.Token != "access-new" || client.RefreshToken != "refresh-new" {
		t.Fatalf("client pair=(%q,%q), want rotated pair", client.Token, client.RefreshToken)
	}
}

func TestCloudflareChallengeErrorDoesNotRetainHTML(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("cf-mitigated", "challenge")
		w.Header().Set("cf-ray", "ray-test-123")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("<html>Just a moment " + strings.Repeat("sensitive-html", 1000) + "</html>"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	err := client.ProbeHealth(context.Background(), "/health")
	var challenge *ErrCloudflareChallenge
	if !errors.As(err, &challenge) {
		t.Fatalf("ProbeHealth error=%v, want ErrCloudflareChallenge", err)
	}
	if challenge.RayID != "ray-test-123" || strings.Contains(err.Error(), "sensitive-html") {
		t.Fatalf("challenge error leaked body or lost ray id: %v", err)
	}
}

func requireMethod(t *testing.T, r *http.Request, expected string) {
	t.Helper()
	if r.Method != expected {
		t.Fatalf("method=%q, want %q", r.Method, expected)
	}
}

func TestAuthenticatedRequestRefreshesAfter401AndRetriesOnce(t *testing.T) {
	var keyRequests atomic.Int32
	var refreshRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/keys":
			keyRequests.Add(1)
			switch r.Header.Get("Authorization") {
			case "Bearer access-old":
				http.Error(w, "expired", http.StatusUnauthorized)
			case "Bearer access-new":
				writeSub2APIResponse(t, w, map[string]any{
					"items": []APIKey{{ID: 1, Name: "key-1"}},
					"total": 1,
				})
			default:
				http.Error(w, "unexpected authorization", http.StatusUnauthorized)
			}
		case "/api/v1/auth/refresh":
			refreshRequests.Add(1)
			writeSub2APIResponse(t, w, map[string]any{
				"access_token":  "access-new",
				"refresh_token": "refresh-new",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(9, "access-old", "refresh-old", time.Hour)
	client := NewClient(server.URL, "admin@example.com", "secret")
	if err := client.EnsureLoggedIn(context.Background(), 9, cache); err != nil {
		t.Fatalf("EnsureLoggedIn: %v", err)
	}
	keys, err := client.GetAPIKeys(context.Background(), "/api/v1/keys")
	if err != nil {
		t.Fatalf("GetAPIKeys: %v", err)
	}
	if len(keys) != 1 || client.Token != "access-new" || client.RefreshToken != "refresh-new" {
		t.Fatalf("keys=%d token pair=(%q,%q)", len(keys), client.Token, client.RefreshToken)
	}
	if keyRequests.Load() != 2 || refreshRequests.Load() != 1 {
		t.Fatalf("key requests=%d refresh requests=%d, want 2/1", keyRequests.Load(), refreshRequests.Load())
	}
}

func TestConcurrent401UsesRefreshTokenOnlyOnce(t *testing.T) {
	var oldAccessRequests atomic.Int32
	var refreshRequests atomic.Int32
	var closeOldReady sync.Once
	oldReady := make(chan struct{})
	releaseRefresh := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/groups/available":
			if r.Header.Get("Authorization") == "Bearer access-old" {
				if oldAccessRequests.Add(1) == 2 {
					closeOldReady.Do(func() { close(oldReady) })
				}
				http.Error(w, "expired", http.StatusUnauthorized)
				return
			}
			if r.Header.Get("Authorization") != "Bearer access-new" {
				http.Error(w, "unexpected authorization", http.StatusUnauthorized)
				return
			}
			writeSub2APIResponse(t, w, []Group{{ID: 1, Name: "default"}})
		case "/api/v1/auth/refresh":
			refreshRequests.Add(1)
			<-releaseRefresh
			writeSub2APIResponse(t, w, map[string]any{
				"access_token":  "access-new",
				"refresh_token": "refresh-new",
				"expires_in":    3600,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cache := NewTokenCache()
	cache.SetTokenPair(10, "access-old", "refresh-old", time.Hour)
	clients := []*Client{
		NewClient(server.URL, "admin@example.com", "secret"),
		NewClient(server.URL, "admin@example.com", "secret"),
	}
	for _, client := range clients {
		if err := client.EnsureLoggedIn(context.Background(), 10, cache); err != nil {
			t.Fatalf("EnsureLoggedIn: %v", err)
		}
	}

	errs := make(chan error, len(clients))
	for _, client := range clients {
		go func(client *Client) {
			_, err := client.GetGroups(context.Background(), "/api/v1/groups/available")
			errs <- err
		}(client)
	}
	select {
	case <-oldReady:
		close(releaseRefresh)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for both clients to receive 401")
	}
	for range clients {
		if err := <-errs; err != nil {
			t.Fatalf("GetGroups: %v", err)
		}
	}
	if refreshRequests.Load() != 1 {
		t.Fatalf("refresh requests=%d, want 1", refreshRequests.Load())
	}
}

func TestGetAPIKeysFetchesEveryRemotePage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.URL.Query().Get("page_size"); got != "100" {
			t.Fatalf("page_size=%q, want 100", got)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		start := (page - 1) * 20
		end := start + 20
		if end > 45 {
			end = 45
		}
		items := make([]APIKey, 0, end-start)
		for id := start + 1; id <= end; id++ {
			items = append(items, APIKey{ID: int64(id), Name: "key-" + strconv.Itoa(id)})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"items": items, "total": 45},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.Token = "token"
	keys, err := client.GetAPIKeys(context.Background(), "/api/v1/keys")
	if err != nil {
		t.Fatalf("GetAPIKeys: %v", err)
	}
	if len(keys) != 45 {
		t.Fatalf("len(keys)=%d, want 45", len(keys))
	}
	if requests != 3 {
		t.Fatalf("requests=%d, want 3", requests)
	}
}

func TestGetAPIKeysStopsWhenUpstreamIgnoresPagination(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{
				"items": []APIKey{{ID: 1}, {ID: 2}},
				"total": 100,
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin@example.com", "secret")
	client.Token = "token"
	keys, err := client.GetAPIKeys(context.Background(), "/api/v1/keys")
	if err != nil {
		t.Fatalf("GetAPIKeys: %v", err)
	}
	if len(keys) != 2 || requests != 2 {
		t.Fatalf("len(keys)=%d requests=%d, want 2 keys and 2 requests", len(keys), requests)
	}
}

func TestGetCurrentUserBalanceUsesAuthenticatedEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/me" {
			t.Fatalf("path=%q, want /api/v1/auth/me", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer remote-access" {
			t.Fatalf("Authorization=%q, want remote access token", got)
		}
		writeSub2APIResponse(t, w, map[string]any{"balance": 123.45, "email": "ignored@example.com"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	client.Token = "remote-access"
	balance, err := client.GetCurrentUserBalance(context.Background())
	if err != nil {
		t.Fatalf("GetCurrentUserBalance: %v", err)
	}
	if balance != 123.45 {
		t.Fatalf("balance=%v, want 123.45", balance)
	}
}

func TestGetGroupRatesNormalizesNullAndReadsStringKeys(t *testing.T) {
	responses := []string{
		`{"code":0,"data":{"7":0.35,"9":1.2}}`,
		`{"code":0,"data":null}`,
	}
	requestIndex := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/groups/rates" {
			t.Fatalf("path=%q, want /api/v1/groups/rates", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(responses[requestIndex]))
		requestIndex++
	}))
	defer server.Close()

	client := NewClient(server.URL, "", "")
	client.Token = "remote-access"
	rates, err := client.GetGroupRates(context.Background())
	if err != nil {
		t.Fatalf("GetGroupRates: %v", err)
	}
	if rates["7"] != 0.35 || rates["9"] != 1.2 {
		t.Fatalf("rates=%v, want overrides for groups 7 and 9", rates)
	}
	rates, err = client.GetGroupRates(context.Background())
	if err != nil {
		t.Fatalf("GetGroupRates null: %v", err)
	}
	if rates == nil || len(rates) != 0 {
		t.Fatalf("null rates=%v, want non-nil empty map", rates)
	}
}
