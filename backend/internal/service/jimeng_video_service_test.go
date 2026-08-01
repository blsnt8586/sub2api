//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func newJimengAccount(baseURL, apiKey string) *Account {
	return &Account{
		ID:          701,
		Name:        "jimeng",
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  apiKey,
		},
	}
}

func TestForwardJimengVideoCreationUsesThirdPartyBaseURLAndAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"video-ds-2.0-fast","prompt":"cinematic","seconds":"15","aspect_ratio":"9:16"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"jm-req-1"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_abc","status":"pending"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")
	result, err := svc.ForwardJimengVideo(context.Background(), c, account, JimengVideoEndpointCreate, "", body, "application/json")
	require.NoError(t, err)

	// 转发到第三方 base_url 的 /v1/videos
	require.Equal(t, "https://zz1cc.cc.cd/v1/videos", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodPost, upstream.lastReq.Method)
	// 用 api_key 作为 Bearer
	require.Equal(t, "Bearer jm-secret", upstream.lastReq.Header.Get("Authorization"))
	require.Equal(t, "application/json", upstream.lastReq.Header.Get("Content-Type"))
	// body 原样透传：model / seconds(字符串) / aspect_ratio 保持不变
	require.Equal(t, "video-ds-2.0-fast", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "15", gjson.GetBytes(upstream.lastBody, "seconds").String())
	require.Equal(t, gjson.String, gjson.GetBytes(upstream.lastBody, "seconds").Type)
	require.Equal(t, "9:16", gjson.GetBytes(upstream.lastBody, "aspect_ratio").String())

	require.Equal(t, "video-ds-2.0-fast", result.Model)
	require.Equal(t, "task_abc", result.ResponseID)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "task_abc")
}

func TestForwardJimengVideoStatusUsesGetAndTaskID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_abc", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_abc","status":"succeeded"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newJimengAccount("https://zz1cc.cc.cd", "jm-secret")
	_, err := svc.ForwardJimengVideo(context.Background(), c, account, JimengVideoEndpointStatus, "task_abc", nil, "")
	require.NoError(t, err)

	require.Equal(t, "https://zz1cc.cc.cd/v1/videos/task_abc", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "Bearer jm-secret", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, recorder.Body.String(), "completed") // [CUSTOM] normalizeJimengVideoResponse converts "succeeded" → "completed"
}

func TestForwardJimengVideoContentUsesContentPath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_abc/content", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"video/mp4"}},
		Body:       io.NopCloser(strings.NewReader("BINARY_MP4_DATA")),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")
	_, err := svc.ForwardJimengVideo(context.Background(), c, account, JimengVideoEndpointContent, "task_abc", nil, "")
	require.NoError(t, err)

	require.Equal(t, "https://zz1cc.cc.cd/v1/videos/task_abc/content", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Contains(t, recorder.Header().Get("Content-Type"), "video/mp4")
	require.Equal(t, "BINARY_MP4_DATA", recorder.Body.String())
}

func TestForwardJimengVideoRejectsNonJimengAccount(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey}
	_, err := svc.ForwardJimengVideo(context.Background(), newTestGinContext(), account, JimengVideoEndpointCreate, "", []byte(`{}`), "application/json")
	require.Error(t, err)
}

func TestForwardJimengVideoRejectsMissingAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader([]byte(`{"model":"video-ds-2.0"}`)))

	svc := &OpenAIGatewayService{}
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "")
	_, err := svc.ForwardJimengVideo(context.Background(), c, account, JimengVideoEndpointCreate, "", []byte(`{"model":"video-ds-2.0"}`), "application/json")
	require.Error(t, err)
}

func TestJimengVideoEndpointBehavior(t *testing.T) {
	cases := []struct {
		endpoint     JimengVideoEndpoint
		method       string
		requiresBody bool
		isGeneration bool
	}{
		{JimengVideoEndpointCreate, http.MethodPost, true, true},
		{JimengVideoEndpointStatus, http.MethodGet, false, false},
		{JimengVideoEndpointContent, http.MethodGet, false, false},
	}
	for _, tc := range cases {
		t.Run(string(tc.endpoint), func(t *testing.T) {
			require.Equal(t, tc.method, tc.endpoint.httpMethod())
			require.Equal(t, tc.requiresBody, tc.endpoint.requiresRequestBody())
			require.Equal(t, tc.isGeneration, tc.endpoint.isGeneration())
		})
	}
}

func TestExtractJimengVideoTaskID(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "id field", body: `{"id":"task_1"}`, want: "task_1"},
		{name: "request_id field", body: `{"request_id":"task_2"}`, want: "task_2"},
		{name: "nested data.id", body: `{"data":{"id":"task_3"}}`, want: "task_3"},
		{name: "missing", body: `{"status":"pending"}`, want: ""},
		{name: "invalid json", body: `nope`, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, extractJimengVideoTaskID([]byte(tc.body)))
		})
	}
}

func TestJimengVideoTaskSessionHash(t *testing.T) {
	require.Empty(t, JimengVideoTaskSessionHash(""))
	require.Empty(t, JimengVideoTaskSessionHash("  "))
	h1 := JimengVideoTaskSessionHash("task_abc")
	require.NotEmpty(t, h1)
	require.True(t, strings.HasPrefix(h1, "jimeng-video:"))
	// 同一 task ID 稳定映射
	require.Equal(t, h1, JimengVideoTaskSessionHash("task_abc"))
	// 不同 task ID 应不同
	require.NotEqual(t, h1, JimengVideoTaskSessionHash("task_xyz"))
}

func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader([]byte(`{}`)))
	return c
}
