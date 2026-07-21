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

func newLeonardoAccount(baseURL, apiKey string) *Account {
	return &Account{
		ID:          801,
		Name:        "leonardo",
		Platform:    PlatformJimeng,
		Type:        AccountTypeAPIKey,
		Concurrency: 1,
		Credentials: map[string]any{
			"base_url": baseURL,
			"api_key":  apiKey,
			"vendor":   JimengVendorLeonardo,
		},
	}
}

func TestForwardJimengImagesGenerationsUsesLeonardoEndpoint(t *testing.T) {
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

	account := newLeonardoAccount("https://leo.example.com/v1", "leo-secret")
	result, err := svc.ForwardJimengImages(context.Background(), c, account, LeonardoImageEndpointGenerations, body, "application/json")
	require.NoError(t, err)

	// 转发到 Leonardo 的 /v1/images/generations
	require.Equal(t, "https://leo.example.com/v1/images/generations", upstream.lastReq.URL.String())
	require.Equal(t, http.MethodPost, upstream.lastReq.Method)
	// 用 api_key 作为 Bearer
	require.Equal(t, "Bearer leo-secret", upstream.lastReq.Header.Get("Authorization"))
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

func TestForwardJimengImagesEditsUsesLeonardoEndpoint(t *testing.T) {
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

	account := newLeonardoAccount("https://leo.example.com", "leo-secret")
	result, err := svc.ForwardJimengImages(context.Background(), c, account, LeonardoImageEndpointEdits, body, "application/json")
	require.NoError(t, err)

	require.Equal(t, "https://leo.example.com/v1/images/edits", upstream.lastReq.URL.String())
	// 图片数量取响应 data 长度（2 张），尺寸 2048 -> 2K
	require.Equal(t, 2, result.ImageCount)
	require.Equal(t, ImageBillingSize2K, result.ImageSize)
}

func TestForwardJimengImagesRejectsNativeJimeng(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	body := []byte(`{"model":"gpt-image-2","prompt":"x"}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", bytes.NewReader(body))

	svc := &OpenAIGatewayService{httpUpstream: &httpUpstreamRecorder{}}
	// 原生即梦账号（无 vendor）不支持图像：应返回 account 级 failover 错误，
	// 让上层循环跳过该账号继续调度同组内的 Leonardo 账号，而非直接失败。
	account := newJimengAccount("https://jm.example.com/v1", "jm-secret")
	_, err := svc.ForwardJimengImages(context.Background(), c, account, LeonardoImageEndpointGenerations, body, "application/json")
	require.Error(t, err)

	var failoverErr *UpstreamFailoverError
	require.ErrorAs(t, err, &failoverErr)
	require.Equal(t, GatewayFailureScopeAccount, failoverErr.Scope)
	require.NotEqual(t, NextAccountStop, failoverErr.NextAccountAction)
	// 原生即梦无 vendor，不应触碰上游
	require.Nil(t, upstreamRecorder(svc).lastReq)
}

// upstreamRecorder 从 svc 取回注入的 httpUpstreamRecorder，便于断言未发起上游请求。
func upstreamRecorder(svc *OpenAIGatewayService) *httpUpstreamRecorder {
	rec, _ := svc.httpUpstream.(*httpUpstreamRecorder)
	return rec
}

func TestJimengVideoURLLeonardoVendorUsesGenerationsPath(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newLeonardoAccount("https://leo.example.com/v1", "leo-secret")

	// 创建任务：Leonardo 走 /v1/videos/generations（原生即梦是 /v1/videos）
	createURL, err := svc.jimengVideoURL(account, JimengVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://leo.example.com/v1/videos/generations", createURL)

	// 状态查询：两者一致，/v1/videos/{id}
	statusURL, err := svc.jimengVideoURL(account, JimengVideoEndpointStatus, "task_abc")
	require.NoError(t, err)
	require.Equal(t, "https://leo.example.com/v1/videos/task_abc", statusURL)

	// Leonardo 无 content 端点
	_, err = svc.jimengVideoURL(account, JimengVideoEndpointContent, "task_abc")
	require.Error(t, err)
}

func TestJimengVideoURLNativeVendorUnchanged(t *testing.T) {
	svc := &OpenAIGatewayService{}
	account := newJimengAccount("https://jm.example.com/v1", "jm-secret")

	createURL, err := svc.jimengVideoURL(account, JimengVideoEndpointCreate, "")
	require.NoError(t, err)
	require.Equal(t, "https://jm.example.com/v1/videos", createURL)

	contentURL, err := svc.jimengVideoURL(account, JimengVideoEndpointContent, "task_abc")
	require.NoError(t, err)
	require.Equal(t, "https://jm.example.com/v1/videos/task_abc/content", contentURL)
}
