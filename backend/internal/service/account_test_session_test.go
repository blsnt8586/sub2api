//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func accountTestSessionHash(h http.Header, body []byte) string {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	c.Request.Header = h.Clone()
	return (&OpenAIGatewayService{}).GenerateSessionHash(c, body)
}

func TestAccountTestWindowIDAloneDoesNotIsolateStickySession(t *testing.T) {
	body := []byte(`{"model":"gpt-6.1-sol","instructions":"Answer only.","input":[{"role":"user","content":[{"type":"input_text","text":"17 + 25 ="}]}]}`)
	first, second := make(http.Header), make(http.Header)
	applyOpenAICodexProbeHeaders(first)
	applyOpenAICodexProbeHeaders(second)
	require.NotEqual(t, first.Get("X-Codex-Window-ID"), second.Get("X-Codex-Window-ID"))
	require.NotEmpty(t, accountTestSessionHash(first, body))
	require.Equal(t, accountTestSessionHash(first, body), accountTestSessionHash(second, body))

	applyFreshAccountTestSessionHeaders(first)
	applyFreshAccountTestSessionHeaders(second)
	require.NotEqual(t, accountTestSessionHash(first, body), accountTestSessionHash(second, body))

	// Proxies commonly discard underscore headers. The session-id alias must
	// still prevent a fallback to the otherwise identical request content.
	first.Del("session_id")
	first.Del("conversation_id")
	second.Del("session_id")
	second.Del("conversation_id")
	require.NotEqual(t, accountTestSessionHash(first, body), accountTestSessionHash(second, body))
}

func TestOpenAIAccountTestsUseFreshSessions(t *testing.T) {
	for _, mode := range []string{"manual", "provider_probe"} {
		for _, protocol := range []string{"apikey_responses", "oauth_responses", "apikey_chat"} {
			t.Run(mode+"/"+protocol, func(t *testing.T) {
				account := &Account{
					ID:          901,
					Platform:    PlatformOpenAI,
					Type:        AccountTypeAPIKey,
					Concurrency: 1,
					Credentials: map[string]any{
						"api_key":                 "test-key",
						"base_url":                "https://relay.example.com/v1",
						"header_override_enabled": true,
						"header_overrides": map[string]any{
							"session-id":        "fixed-session-override",
							"x-codex-window-id": "fixed-window-override",
						},
					},
					Extra: map[string]any{openai_compat.ExtraKeyResponsesSupported: protocol != "apikey_chat"},
				}
				if protocol == "oauth_responses" {
					account.Type = AccountTypeOAuth
					account.Credentials["access_token"] = "test-token"
				}
				stream := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"42\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"
				if protocol == "apikey_chat" {
					stream = "data: {\"choices\":[{\"delta\":{\"content\":\"42\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"
				}
				upstream := &queuedHTTPUpstream{}
				for range 2 {
					upstream.responses = append(upstream.responses, newJSONResponse(http.StatusOK, stream))
				}
				svc := &AccountTestService{
					httpUpstream:        upstream,
					tlsFPProfileService: &TLSFingerprintProfileService{},
					cfg:                 &config.Config{},
				}
				var bodies [][]byte
				var hashes []string
				for range 2 {
					c, _ := newTestContext()
					ctx := context.Background()
					if mode == "provider_probe" {
						// An identical challenge can recur. Session isolation must
						// not depend on the random arithmetic question changing.
						ctx, _ = fixedAccountProbeChallengeContext()
					}
					c.Request = c.Request.WithContext(ctx)
					require.NoError(t, svc.testOpenAIAccountConnection(c, account, "gpt-6.1-sol", "", ""))
					req := upstream.requests[len(upstream.requests)-1]
					body, err := io.ReadAll(req.Body)
					require.NoError(t, err)
					bodies = append(bodies, body)
					sessionID := req.Header.Get("session-id")
					_, err = uuid.Parse(sessionID)
					require.NoError(t, err)
					require.Equal(t, sessionID, req.Header.Get("session_id"))
					require.Equal(t, sessionID, req.Header.Get("conversation_id"))
					require.Equal(t, sessionID, req.Header.Get("X-Codex-Window-ID"))
					require.NotEqual(t, "fixed-session-override", sessionID)
					auth := "Bearer test-key"
					if protocol == "oauth_responses" {
						auth = "Bearer test-token"
					}
					require.Equal(t, auth, req.Header.Get("Authorization"))
					hashes = append(hashes, accountTestSessionHash(req.Header, body))
				}
				require.Equal(t, bodies[0], bodies[1], "session rotation must not change the test payload")
				require.NotEmpty(t, hashes[0])
				require.NotEqual(t, hashes[0], hashes[1], "independent tests must not share an upstream sticky binding")
			})
		}
	}
}

func TestFreshAccountTestSessionReplacesAllHeaderCasings(t *testing.T) {
	h := http.Header{
		"Session_ID":        {"old-a"},
		"SESSION_ID":        {"old-b"},
		"SESSION-ID":        {"old-c"},
		"Conversation_ID":   {"old-d"},
		"X-Codex-Window-ID": {"old-e"},
		"Authorization":     {"Bearer test-key"},
	}
	applyFreshAccountTestSessionHeaders(h)
	for _, name := range []string{"session-id", "session_id", "conversation_id", "x-codex-window-id"} {
		count := 0
		for existing := range h {
			if strings.EqualFold(existing, name) {
				count++
			}
		}
		require.Equal(t, 1, count, "duplicate header variants: %s", name)
	}
	require.Equal(t, "Bearer test-key", h.Get("Authorization"))
	applyFreshAccountTestSessionHeaders(nil)
}
