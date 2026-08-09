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

func newAVI2APIAccount(baseURL, apiKey string) *Account {
	return &Account{
		ID:          801,
		Name:        "avi2api",
		Platform:    PlatformCanvas,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  apiKey,
		},
	}
}

func TestForwardCanvasImagesGenerationsUsesAVI2APIEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}, "X-Request-Id": []string{"leo-req-1"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1,"data":[{"url":"https://cdn.example.com/a.png"}]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newAVI2APIAccount("https://avi2api.example.com/v1", "avi-secret")
	result, err := svc.ForwardCanvasImages(context.Background(), c, account, CanvasImageEndpointGenerations, body, "application/json")
	require.NoError(t, err)

	// 转发到 AVI2API 的 /v1/images/generations
	require.Equal(t, "https://avi2api.example.com/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodPost, upstream.lastReq.Method)
	// 用 api_key 作为 Bearer
	require.Equal(t, "Bearer avi-secret", upstream.lastReq.Header.Get("Authorization"))
	// body 原样透传
	require.Equal(t, "gpt-image-2", gjson.GetBytes(upstream.lastBody, "model").String())
	require.Equal(t, "a cat", gjson.GetBytes(upstream.lastBody, "prompt").String())

	// 计费字段：图片数量取响应 data 数组长度，尺寸归一化到档位
	require.Equal(t, "gpt-image-2", result.Model)
	require.Equal(t, 1, result.ImageCount)
	require.Equal(t, ImageBillingSize1K, result.ImageSize)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "a.png")
}

func TestForwardCanvasImagesEditsUsesAVI2APIEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"nano-banana-2","prompt":"edit","size":"2048x2048"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1,"data":[{"url":"a"},{"url":"b"}]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newAVI2APIAccount("https://avi2api.example.com", "avi-secret")
	result, err := svc.ForwardCanvasImages(context.Background(), c, account, CanvasImageEndpointEdits, body, "application/json")
	require.NoError(t, err)

	require.Equal(t, "https://avi2api.example.com/v1/images/edits", upstream.lastReq.URL.String())
	// 图片数量取响应 data 长度（2 张），尺寸 2048 -> 2K
	require.Equal(t, 2, result.ImageCount)
	require.Equal(t, ImageBillingSize2K, result.ImageSize)
}

func TestForwardCanvasImagesForwardsIdempotencyKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"a cat","size":"1024x1024","n":1}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	// AIV2API 对创建类接口强制要求 Idempotency-Key，网关必须原样透传给上游。
	c.Request.Header.Set("Idempotency-Key", "idem-key-123")

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"created":1,"data":[{"url":"https://cdn.example.com/a.png"}]}`)),
	}}
	svc := &OpenAIGatewayService{httpUpstream: upstream}

	account := newAVI2APIAccount("https://avi2api.example.com/v1", "avi-secret")
	_, err := svc.ForwardCanvasImages(context.Background(), c, account, CanvasImageEndpointGenerations, body, "application/json")
	require.NoError(t, err)

	// Idempotency-Key 透传到上游，否则上游报 "Idempotency-Key is required..."
	require.Equal(t, "idem-key-123", upstream.lastReq.Header.Get("Idempotency-Key"))
}

func TestForwardCanvasImagesRejectsAccountMissingAPIKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"x"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

	svc := &OpenAIGatewayService{httpUpstream: &httpUpstreamRecorder{}}
	// 缺 api_key 的账号：返回 account 级 failover 错误，让上层循环跳过该账号
	// 继续调度同组内其他账号，而非直接失败。
	account := newAVI2APIAccount("https://avi2api.example.com/v1", "")
	_, err := svc.ForwardCanvasImages(context.Background(), c, account, CanvasImageEndpointGenerations, body, "application/json")
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, GatewayFailureScopeAccount, failoverErr.Scope)
	require.NotEqual(t, NextAccountStop, failoverErr.NextAccountAction)
	// 缺凭据，不应触碰上游
	require.Nil(t, upstreamRecorder(svc).lastReq)
}

// upstreamRecorder 从 svc 取回注入的 httpUpstreamRecorder，便于断言未发起上游请求。
func upstreamRecorder(svc *OpenAIGatewayService) *httpUpstreamRecorder {
	rec, _ := svc.httpUpstream.(*httpUpstreamRecorder)
	return rec
}

func TestJimengVideoURLUsesAVI2APIPaths(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newAVI2APIAccount("https://avi2api.example.com/v1", "avi-secret")

	// 创建任务：POST {base}/v1/videos/generations
	createURL, err := svc.canvasVideoURL(account, CanvasVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://avi2api.example.com/v1/videos/generations", createURL)

	// 状态查询：GET {base}/v1/videos/{id}
	statusURL, err := svc.canvasVideoURL(account, CanvasVideoEndpointStatus, "task_abc")
	require.NoError(t, err)
	require.Equal(t, "https://avi2api.example.com/v1/videos/task_abc", statusURL)

	// 取消任务：POST {base}/v1/videos/{id}/cancel
	cancelURL, err := svc.canvasVideoURL(account, CanvasVideoEndpointCancel, "task_abc")
	require.NoError(t, err)
	require.Equal(t, "https://avi2api.example.com/v1/videos/task_abc/cancel", cancelURL)
}

func TestJimengVideoURLNormalizesBaseURLWithoutV1Suffix(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newAVI2APIAccount("https://avi2api.example.com", "avi-secret")

	createURL, err := svc.canvasVideoURL(account, CanvasVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://avi2api.example.com/v1/videos/generations", createURL)
}
