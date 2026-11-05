//go:build unit

package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type boundedFailoverUpstream struct {
	service.HTTPUpstream
	mu   sync.Mutex
	ids  []int64
	mode string
}

func (u *boundedFailoverUpstream) Do(_ *http.Request, _ string, id int64, _ int) (*http.Response, error) {
	u.mu.Lock()
	u.ids = append(u.ids, id)
	u.mu.Unlock()
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}}}
	text := "recovered"
	terminal := `{"type":"response.completed","response":{"id":"resp_ok","model":"gpt-5.1","output":[],"usage":{"input_tokens":0,"output_tokens":0}}}`
	if id == 1 {
		text = " "
		terminal = `{"type":"response.failed","response":{"error":{"code":"upstream_error","message":"The service is temporarily unavailable. Please retry later."}}}`
		if u.mode == "after_output" {
			text = "hello"
		}
		if u.mode == "http_502" {
			resp.StatusCode = 502
			resp.Body = io.NopCloser(strings.NewReader(`{"error":{"type":"server_error","message":"bad gateway"}}`))
			return resp, nil
		}
	}
	stream := `data: {"type":"response.created","response":{"id":"resp_test"}}` + "\n\n" +
		`data: {"type":"response.output_text.delta","delta":"` + text + `"}` + "\n\n"
	if id == 1 && u.mode == "http2" {
		resp.Body = io.NopCloser(io.MultiReader(strings.NewReader(stream), boundedFailoverErrorReader{}))
	} else {
		resp.Body = io.NopCloser(strings.NewReader(stream + "data: " + terminal + "\n\n"))
	}
	return resp, nil
}

func (u *boundedFailoverUpstream) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

type boundedFailoverErrorReader struct{}

func (boundedFailoverErrorReader) Read([]byte) (int, error) {
	return 0, errors.New("stream error: stream ID 1; INTERNAL_ERROR; received from peer")
}

func TestOpenAIResponsesFourthAttemptUsesAnotherAccount(t *testing.T) {
	for _, mode := range []string{"http_502", "whitespace_failure", "http2", "after_output"} {
		t.Run(mode, func(t *testing.T) {
			upstream := &boundedFailoverUpstream{mode: mode}
			accounts := make([]service.Account, 2)
			for i := range accounts {
				accounts[i] = service.Account{
					ID: int64(i + 1), Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
					Status: service.StatusActive, Schedulable: true, Priority: i,
					Credentials: map[string]any{
						"api_key": "test-key", "base_url": "https://relay.example.com",
						"pool_mode": true, "pool_mode_retry_count": 10,
						"pool_mode_retry_status_codes": []any{502},
					},
				}
			}
			h := newOpenAIResponsesFailoverTestHandler(t, upstream, accounts...)
			c, rec := newOpenAIResponsesFailoverTestContext(t, context.Background())
			c.Request.Body = io.NopCloser(bytes.NewBufferString(`{"model":"gpt-5.1","stream":true,"input":"hello"}`))
			h.Responses(c)
			upstream.mu.Lock()
			ids := append([]int64(nil), upstream.ids...)
			upstream.mu.Unlock()
			if mode == "after_output" {
				require.Equal(t, []int64{1}, ids)
				require.Contains(t, rec.Body.String(), "hello")
			} else {
				require.Equal(t, []int64{1, 1, 1, 2}, ids)
				require.Contains(t, rec.Body.String(), "recovered")
				require.NotContains(t, rec.Body.String(), "temporarily unavailable")
				require.NotContains(t, rec.Body.String(), "response.failed")
			}
		})
	}
}
