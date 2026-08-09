//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
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
		Platform:    PlatformCanvas,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  apiKey,
		},
	}
}

func TestForwardCanvasVideoCreationUsesThirdPartyBaseURLAndAPIKey(t *testing.T) {
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
	result, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointCreate, "", body, "application/json")
	require.NoError(t, err)

	// 转发到 base_url 的 /v1/videos/generations（AVI2API 统一创建路径）
	require.Equal(t, "https://zz1cc.cc.cd/v1/videos/generations", upstream.lastReq.URL.String())
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

func TestForwardCanvasVideoStatusUsesGetAndTaskID(t *testing.T) {
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
	_, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointStatus, "task_abc", nil, "")
	require.NoError(t, err)

	require.Equal(t, "https://zz1cc.cc.cd/v1/videos/task_abc", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodGet, upstream.lastReq.Method)
	require.Equal(t, "Bearer jm-secret", upstream.lastReq.Header.Get("Authorization"))
	require.Contains(t, recorder.Body.String(), "completed") // [CUSTOM] normalizeCanvasVideoResponse converts "succeeded" → "completed"
}

// TestForwardCanvasVideoMultipartPassthrough 覆盖参考素材模式（首尾帧/参考图/
// 参考视频/参考音频）：multipart body 必须逐字节透传，Content-Type 连 boundary
// 一起原样转发，且 model/duration 能从 form 文本字段解析出来供选号与计费使用。
func TestForwardCanvasVideoMultipartPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("model", "veo-3.1"))
	require.NoError(t, mw.WriteField("prompt", "a cat walks"))
	require.NoError(t, mw.WriteField("duration", "6"))
	startFrame, err := mw.CreateFormFile("start_frame", "first.png")
	require.NoError(t, err)
	_, err = startFrame.Write([]byte("PNG_START_FRAME_BYTES"))
	require.NoError(t, err)
	endFrame, err := mw.CreateFormFile("end_frame", "last.png")
	require.NoError(t, err)
	_, err = endFrame.Write([]byte("PNG_END_FRAME_BYTES"))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	multipartBody := buf.Bytes()
	multipartCT := mw.FormDataContentType()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/generations", bytes.NewReader(multipartBody))
	c.Request.Header.Set("Content-Type", multipartCT)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusAccepted,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_mp_1","status":"queued"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")
	result, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointCreate, "", multipartBody, multipartCT)
	require.NoError(t, err)

	// 创建路径统一为 /videos/generations
	require.Equal(t, "https://zz1cc.cc.cd/v1/videos/generations", upstream.lastReq.URL.String())
	// Content-Type 带 boundary 原样透传，否则上游无法解析 multipart
	require.Equal(t, multipartCT, upstream.lastReq.Header.Get("Content-Type"))
	// body 逐字节透传：首尾帧二进制完整到达上游
	require.Equal(t, multipartBody, upstream.lastBody)
	require.Contains(t, string(upstream.lastBody), "PNG_START_FRAME_BYTES")
	require.Contains(t, string(upstream.lastBody), "PNG_END_FRAME_BYTES")

	// multipart 文本字段解析：model 供选号。
	require.Equal(t, "veo-3.1", result.Model)
	// 创建路径只记录 task id 用于账号绑定，不在此扣费（异步任务此刻仅排队，
	// 最终可能失败）。计费字段（VideoCount/VideoSeconds）由 status 轮询命中终态成功时填充。
	require.Equal(t, 0, result.VideoCount)
	require.Equal(t, 0, result.VideoSeconds)
	require.Equal(t, "task_mp_1", result.ResponseID)
}

// TestForwardCanvasVideoStatusBillsOnTerminalSuccess 验证完成时扣费：
// status 轮询命中上游终态成功（视频用 "completed"）时填充计费字段。
func TestForwardCanvasVideoStatusBillsOnTerminalSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_done_1", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body: io.NopCloser(strings.NewReader(
			`{"id":"task_done_1","status":"completed","model":"veo-3.1","result":{"data":[{"url":"https://x/y.mp4","duration":6}]}}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")

	result, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointStatus, "task_done_1", nil, "")
	require.NoError(t, err)
	require.Equal(t, "veo-3.1", result.Model)
	require.Equal(t, 1, result.VideoCount)   // 终态成功 → 按次计费
	require.Equal(t, 6, result.VideoSeconds) // 从 result.data[0].duration 取
	require.Equal(t, "task_done_1", result.ResponseID)
}

// TestForwardCanvasVideoStatusNoBillWhileProcessing 验证处理中的轮询不计费。
func TestForwardCanvasVideoStatusNoBillWhileProcessing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/task_run_1", nil)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"id":"task_run_1","status":"processing","model":"veo-3.1"}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "jm-secret")

	result, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointStatus, "task_run_1", nil, "")
	require.NoError(t, err)
	require.Equal(t, 0, result.VideoCount) // 未终态 → 不计费
}

func TestForwardCanvasVideoRejectsNonJimengAccount(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := &Account{ID: 1, Platform: PlatformGrok, Type: AccountTypeAPIKey}
	_, err := svc.ForwardCanvasVideo(context.Background(), newTestGinContext(), account, CanvasVideoEndpointCreate, "", []byte(`{}`), "application/json")
	require.Error(t, err)
}

func TestForwardCanvasVideoRejectsMissingAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader([]byte(`{"model":"video-ds-2.0"}`)))

	svc := &OpenAIGatewayService{}
	account := newJimengAccount("https://zz1cc.cc.cd/v1", "")
	_, err := svc.ForwardCanvasVideo(context.Background(), c, account, CanvasVideoEndpointCreate, "", []byte(`{"model":"video-ds-2.0"}`), "application/json")
	require.Error(t, err)
}

func TestCanvasVideoEndpointBehavior(t *testing.T) {
	cases := []struct {
		endpoint     CanvasVideoEndpoint
		method       string
		requiresBody bool
		isGeneration bool
	}{
		{CanvasVideoEndpointCreate, http.MethodPost, true, true},
		{CanvasVideoEndpointStatus, http.MethodGet, false, false},
		{CanvasVideoEndpointCancel, http.MethodPost, true, false},
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
			require.Equal(t, tc.want, extractCanvasVideoTaskID([]byte(tc.body)))
		})
	}
}

func TestCanvasVideoTaskSessionHash(t *testing.T) {
	require.Empty(t, CanvasVideoTaskSessionHash(""))
	require.Empty(t, CanvasVideoTaskSessionHash("  "))
	h1 := CanvasVideoTaskSessionHash("task_abc")
	require.NotEmpty(t, h1)
	require.True(t, strings.HasPrefix(h1, "canvas-video:"))
	// 同一 task ID 稳定映射
	require.Equal(t, h1, CanvasVideoTaskSessionHash("task_abc"))
	// 不同 task ID 应不同
	require.NotEqual(t, h1, CanvasVideoTaskSessionHash("task_xyz"))
}

func newTestGinContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader([]byte(`{}`)))
	return c
}
