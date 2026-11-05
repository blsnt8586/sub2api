//go:build unit

package service

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// The upstream on server 45 sends an SSE event name without a top-level JSON
// type. Before any model output this must switch accounts, including pool-mode
// accounts whose same-account retry list does not contain 502.
func TestOpenAIUpstreamUnavailableStreamFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, outputStarted := range []bool{false, true} {
			for _, queuedHeartbeat := range []bool{false, true} {
				name := "native"
				if passthrough {
					name = "passthrough"
				}
				if outputStarted {
					name += "/after_output"
				} else {
					name += "/before_output"
				}
				if queuedHeartbeat {
					name += "/queue_heartbeat"
				}
				t.Run(name, func(t *testing.T) {
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
					if queuedHeartbeat {
						n, err := c.Writer.Write([]byte(":\n\n"))
						require.NoError(t, err)
						RecordGatewayStreamHeartbeat(c, n)
						c.Writer.Flush()
					}
					body := "event: response.created\ndata: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n"
					if outputStarted {
						body += "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"
					}
					body += "event: error\ndata: {\"error\":{\"code\":\"upstream_error\",\"message\":\"The service is temporarily unavailable. Please retry later.\",\"type\":\"upstream_error\"}}\n\n"
					resp := &http.Response{
						StatusCode: http.StatusOK,
						Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
						Body:       io.NopCloser(strings.NewReader(body)),
					}
					defer resp.Body.Close()
					svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
					account := &Account{ID: 499, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{
						"pool_mode": true, "pool_mode_retry_count": 3,
						"pool_mode_retry_status_codes": []any{400, 401, 403, 429},
					}}
					var err error
					if passthrough {
						_, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
					} else {
						_, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
					}
					require.Error(t, err)
					var failoverErr *UpstreamFailoverError
					if outputStarted {
						require.False(t, errors.As(err, &failoverErr), "must not replay a stream after model output")
						require.Contains(t, rec.Body.String(), "hello")
						events, ok := c.Get(OpsUpstreamErrorsKey)
						require.True(t, ok, "bare SSE errors after output must retain upstream attribution")
						require.Len(t, events, 1)
					} else {
						require.ErrorAs(t, err, &failoverErr)
						require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
						require.False(t, failoverErr.RetryableOnSameAccount)
						require.True(t, failoverErr.ShouldRetryNextAccount())
						if queuedHeartbeat {
							require.Equal(t, ":\n\n", rec.Body.String())
						} else {
							require.False(t, c.Writer.Written())
							require.Empty(t, rec.Body.String())
						}
					}
				})
			}
		}
	}
}
