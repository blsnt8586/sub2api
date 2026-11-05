//go:build unit

package service

import (
	"context"
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

func TestOpenAIStreamLeadingWhitespaceCanFailoverAndIsPreservedOnSuccess(t *testing.T) {
	for _, passthrough := range []bool{false, true} {
		for _, terminal := range []string{"failed", "success", "policy", "after_text"} {
			t.Run(terminal+map[bool]string{false: "/native", true: "/passthrough"}[passthrough], func(t *testing.T) {
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
				stream := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n" +
					"data: {\"type\":\"response.output_text.delta\",\"delta\":\" \\n  \"}\n\n" +
					"data: {\"type\":\"response.content_part.done\",\"part\":{\"type\":\"output_text\",\"text\":\" \"}}\n\n" +
					"data: {\"type\":\"response.output_item.done\",\"item\":{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\" \"}]}}\n\n"
				if terminal == "success" || terminal == "after_text" {
					stream += "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n"
				}
				if terminal == "success" {
					stream += "data: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"input_tokens\":1,\"output_tokens\":2}}}\n\n"
				} else if terminal == "policy" {
					stream += "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"content_policy_violation\",\"message\":\"Request rejected by policy\"}}}\n\n"
				} else {
					stream += "data: {\"type\":\"response.failed\",\"response\":{\"error\":{\"code\":\"upstream_error\",\"message\":\"The service is temporarily unavailable. Please retry later.\"}}}\n\n"
				}
				resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(stream))}
				defer resp.Body.Close()
				svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
				account := &Account{ID: 516, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				var err error
				if passthrough {
					_, err = svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
				} else {
					_, err = svc.handleStreamingResponse(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
				}
				var failoverErr *UpstreamFailoverError
				switch terminal {
				case "failed":
					require.ErrorAs(t, err, &failoverErr)
					require.True(t, failoverErr.ShouldRetryNextAccount())
					require.Empty(t, rec.Body.String(), "failed attempt must not leak whitespace or lifecycle events")
				case "success":
					require.NoError(t, err)
					require.Contains(t, rec.Body.String(), `"delta":" \n  "`)
					require.Contains(t, rec.Body.String(), "hello")
				default:
					require.Error(t, err)
					require.False(t, errors.As(err, &failoverErr), "never replay policy errors or committed content")
					_, _, eligible := classifyOpenAIAPIKeyHealthFailure(err)
					require.Equal(t, terminal == "after_text", eligible)
				}
			})
		}
	}
}

func TestOpenAICommittedStreamReadFailureCountsWithoutReplay(t *testing.T) {
	err := newOpenAICommittedStreamReadError(errors.New("http2: stream error: INTERNAL_ERROR; received from peer"))
	status, _, eligible := classifyOpenAIAPIKeyHealthFailure(err)
	require.Equal(t, 502, status)
	require.True(t, eligible)
	var failoverErr *UpstreamFailoverError
	require.False(t, errors.As(err, &failoverErr))
	_, _, eligible = classifyOpenAIAPIKeyHealthFailure(newOpenAICommittedStreamReadError(context.Canceled))
	require.False(t, eligible)
}

func TestOpenAIPassthroughPreOutputBufferIsBounded(t *testing.T) {
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	stream := strings.Repeat("data: {\"type\":\"response.in_progress\"}\n\n", 32769)
	resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(stream))}
	defer resp.Body.Close()
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
	account := &Account{ID: 516, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	_, err := svc.handleStreamingResponsePassthrough(c.Request.Context(), resp, c, account, time.Now(), "gpt-6.1-sol", "gpt-6.1-sol")
	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Contains(t, string(failoverErr.ResponseBody), "buffer limit")
	require.Empty(t, rec.Body.String())
}
