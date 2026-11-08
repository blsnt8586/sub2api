package sub2api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newAPIReply(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func TestNewAPIAuthenticationCompatibilityAndCache(t *testing.T) {
	for _, modern := range []bool{false, true} {
		t.Run(fmt.Sprint(modern), func(t *testing.T) {
			var logins atomic.Int32
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/status":
					newAPIReply(w, map[string]any{"quota_per_unit": 500000, "quota_display_type": "CNY", "price": 7.3})
				case "/api/user/login":
					logins.Add(1)
					var input map[string]string
					require.NoError(t, json.NewDecoder(r.Body).Decode(&input))
					require.Equal(t, "alice", input["username"])
					require.Equal(t, "password", input["password"])
					if modern {
						newAPIReply(w, map[string]any{"access_token": "bearer-session", "user": map[string]any{"id": 42}})
					} else {
						http.SetCookie(w, &http.Cookie{Name: "session", Value: "cookie-session", Path: "/"})
						newAPIReply(w, map[string]any{"id": 42})
					}
				case "/api/user/self":
					require.Equal(t, "42", r.Header.Get("New-Api-User"))
					if modern {
						require.Equal(t, "Bearer bearer-session", r.Header.Get("Authorization"))
					} else {
						cookie, err := r.Cookie("session")
						require.NoError(t, err)
						require.Equal(t, "cookie-session", cookie.Value)
					}
					newAPIReply(w, map[string]any{"id": 42, "quota": 1000000, "used_quota": 250000, "group": "default"})
				default:
					t.Errorf("unexpected request: %s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer server.Close()
			cache := NewTokenCache()
			for range 2 {
				c := NewClient(server.URL, "alice", "password")
				require.NoError(t, c.ConfigureNewAPI(""))
				require.NoError(t, c.EnsureLoggedIn(context.Background(), 1, cache))
				profile, err := c.GetCurrentUserProfile(context.Background())
				require.NoError(t, err)
				require.Equal(t, 2.0, profile.Balance)
				require.Equal(t, .5, *profile.TotalConsumed)
				require.Equal(t, "default", c.newAPI.session.UserGroup)
				require.Zero(t, profile.TotalRecharged)
			}
			require.EqualValues(t, 1, logins.Load())
			cache.Evict(1)
			c := NewClient(server.URL, "alice", "password")
			require.NoError(t, c.ConfigureNewAPI(""))
			require.NoError(t, c.EnsureLoggedIn(context.Background(), 1, cache))
			require.EqualValues(t, 2, logins.Load())
		})
	}
}

func TestNewAPIBrowserSessionRefreshesAndPersistsRotation(t *testing.T) {
	var persistedAccess, persistedCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/auth/refresh":
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "new_api_refresh=old-cookie", r.Header.Get("Cookie"))
			require.Equal(t, "http://"+r.Host, r.Header.Get("Origin"))
			http.SetCookie(w, &http.Cookie{Name: "new_api_refresh", Value: "rotated-cookie", Path: "/api/user/auth", HttpOnly: true})
			newAPIReply(w, map[string]any{"access_token": "fresh-token", "access_expires_at": time.Now().Add(time.Hour).Unix(), "user": map[string]any{"id": 42}})
		case "/api/user/self":
			if r.Header.Get("Authorization") == "Bearer old-token" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			require.Equal(t, "Bearer fresh-token", r.Header.Get("Authorization"))
			newAPIReply(w, map[string]any{"id": 42, "quota": 1000000, "used_quota": 250000})
		case "/api/status":
			newAPIReply(w, map[string]any{"quota_per_unit": 500000})
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPIBrowserSession("old-token", "old-cookie", func(_ context.Context, access, cookie string, _ time.Time) error {
		persistedAccess, persistedCookie = access, cookie
		return nil
	}))
	require.NoError(t, c.EnsureLoggedIn(context.Background(), 9, NewTokenCache()))
	require.Equal(t, "fresh-token", persistedAccess)
	require.Equal(t, "rotated-cookie", persistedCookie)
}

func TestNewAPITokenIdentityAndErrorRedaction(t *testing.T) {
	for _, test := range []struct {
		name    string
		id      int
		success bool
		wantErr string
	}{
		{"valid", 42, true, ""}, {"wrong user", 43, true, "does not match"}, {"business failure", 42, false, "success=false"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "GET", r.Method)
				if r.URL.Path == "/api/status" {
					newAPIReply(w, map[string]any{"quota_per_unit": 500000})
					return
				}
				require.Equal(t, "Bearer management-secret", r.Header.Get("Authorization"))
				require.Equal(t, "42", r.Header.Get("New-Api-User"))
				_ = json.NewEncoder(w).Encode(map[string]any{"success": test.success, "message": "echo management-secret", "data": map[string]any{"id": test.id, "quota": 0, "used_quota": 0}})
			}))
			defer server.Close()
			c := NewClient(server.URL, "42", "")
			require.NoError(t, c.ConfigureNewAPI("management-secret"))
			err := c.EnsureLoggedIn(context.Background(), 1, NewTokenCache())
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantErr)
				require.NotContains(t, err.Error(), "management-secret")
			}
		})
	}
}

func TestNewAPIGroupsKeysAndExactLink(t *testing.T) {
	var reveals atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/self/groups":
			newAPIReply(w, map[string]any{"cheap": map[string]any{"ratio": .3, "desc": "effective user ratio"}, "default": map[string]any{"ratio": 1}, "auto": map[string]any{"ratio": "auto"}})
		case "/api/token/":
			require.NotEmpty(t, r.URL.Query().Get("p"))
			require.Equal(t, "100", r.URL.Query().Get("page_size"))
			newAPIReply(w, map[string]any{"items": []any{map[string]any{"id": 7, "name": "test", "key": "abcd**********wxyz", "group": "cheap", "status": 1}}, "total": 1})
		case "/api/token/7/key":
			require.Equal(t, "POST", r.Method)
			reveals.Add(1)
			newAPIReply(w, map[string]any{"key": "abcd-full-secret-wxyz"})
		default:
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management-secret"))
	groups, err := c.GetGroups(context.Background(), "/api/v1/groups")
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, "cheap", groups[0].Name)
	require.Equal(t, .3, groups[0].RateMultiplier)
	require.Empty(t, groups[0].Platform)
	require.LessOrEqual(t, groups[0].ID, int64(1<<53))
	require.Equal(t, groups[0].ID, newAPIGroupID("cheap"))
	keys, err := c.GetAPIKeys(context.Background(), "/api/v1/keys")
	require.NoError(t, err)
	require.Len(t, keys, 1)
	require.Equal(t, groups[0].ID, keys[0].GroupID)
	require.Empty(t, keys[0].Key)
	require.EqualValues(t, 0, reveals.Load())
	resolved, err := c.ResolveAPIKeysForLink(context.Background(), "sk-abcd-full-secret-wxyz", keys)
	require.NoError(t, err)
	require.Equal(t, "abcd-full-secret-wxyz", resolved[0].Key)
	require.Empty(t, resolved[0].MaskedKey)
	require.EqualValues(t, 1, reveals.Load())
	_, err = c.ResolveAPIKeysForLink(context.Background(), "sk-other-unrelated-secret", keys)
	require.NoError(t, err)
	require.EqualValues(t, 1, reveals.Load())
	require.Error(t, c.UpdateAPIKeyGroup(context.Background(), "", 7, groups[0].ID))
	_, err = c.GetAPIKeysUsage(context.Background(), []int64{7})
	require.ErrorIs(t, err, errNewAPIUnsupportedStats)
}

func TestNewAPIIncompleteInventoryFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newAPIReply(w, map[string]any{"items": []any{}, "total": 2})
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management"))
	_, err := c.GetAPIKeys(context.Background(), "")
	require.ErrorContains(t, err, "inventory incomplete")
}

func TestNewAPIInteractiveLoginAndHTMLFailClosed(t *testing.T) {
	for _, body := range []string{`{"success":true,"data":{"require_2fa":true}}`, `{"success":false,"message":"captcha-secret"}`, `<html>session-secret</html>`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(body)) }))
		c := NewClient(server.URL, "alice", "password")
		require.NoError(t, c.ConfigureNewAPI(""))
		err := c.EnsureLoggedIn(context.Background(), 1, nil)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "secret")
		server.Close()
	}
}

func TestNewAPIExpiredPasswordSessionRetriesOnce(t *testing.T) {
	var logins atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			newAPIReply(w, map[string]any{"quota_per_unit": 500000})
		case "/api/user/login":
			logins.Add(1)
			newAPIReply(w, map[string]any{"access_token": "fresh", "user": map[string]any{"id": 42}})
		case "/api/user/self":
			if r.Header.Get("Authorization") != "Bearer fresh" {
				w.WriteHeader(401)
				return
			}
			newAPIReply(w, map[string]any{"id": 42, "quota": 0, "used_quota": 0})
		}
	}))
	defer server.Close()
	c := NewClient(server.URL, "alice", "password")
	require.NoError(t, c.ConfigureNewAPI(""))
	c.newAPI.session = newAPISession{UserID: 42, Token: "expired", ExpiresAt: time.Now().Add(time.Hour)}
	_, err := c.GetCurrentUserProfile(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, logins.Load())
}

func TestNewAPIPaginationAcrossPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		items := []any{}
		if r.URL.Query().Get("p") == "1" {
			for i := 1; i <= 100; i++ {
				items = append(items, map[string]any{"id": i, "key": strings.Repeat("x", 32), "group": "default", "status": 1})
			}
		} else {
			items = append(items, map[string]any{"id": 101, "key": "last-token", "group": "default", "status": 1})
		}
		newAPIReply(w, map[string]any{"items": items, "total": 101})
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management"))
	keys, err := c.GetAPIKeys(context.Background(), "")
	require.NoError(t, err)
	require.Len(t, keys, 101)
}

func TestNewAPIMissingRatioAndEmptyInventory(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/token/" {
			newAPIReply(w, map[string]any{"items": nil, "total": 0})
			return
		}
		newAPIReply(w, map[string]any{"unknown": map[string]any{"ratio": nil}})
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management"))
	_, err := c.GetGroups(context.Background(), "")
	require.ErrorContains(t, err, "no valid fixed ratio")
	keys, err := c.GetAPIKeys(context.Background(), "")
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestNewAPIPublicGroupsAreNeverPricingFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self/groups", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management"))
	_, err := c.GetGroups(context.Background(), "")
	require.Error(t, err)
}

func TestNewAPIEmptyTokenGroupInheritsUserGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newAPIReply(w, map[string]any{"items": []any{map[string]any{"id": 1, "key": "secret", "group": "", "status": 1}}, "total": 1})
	}))
	defer server.Close()
	c := NewClient(server.URL, "42", "")
	require.NoError(t, c.ConfigureNewAPI("management"))
	c.newAPI.session.UserGroup = "default"
	keys, err := c.GetAPIKeys(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, newAPIGroupID("default"), keys[0].GroupID)
}
