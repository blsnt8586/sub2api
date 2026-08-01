package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/jimeng"
	"github.com/Wei-Shaw/sub2api/internal/pkg/leonardo"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// JimengAudioEndpoint 表示音频任务的端点类型
type JimengAudioEndpoint string

const (
	JimengAudioGeneration JimengAudioEndpoint = "audio_generation"
	JimengAudioStatus     JimengAudioEndpoint = "audio_status"
	JimengAudioCancel     JimengAudioEndpoint = "audio_cancel"
)

func (e JimengAudioEndpoint) httpMethod() string {
	switch e {
	case JimengAudioGeneration:
		return "POST"
	case JimengAudioStatus:
		return "GET"
	case JimengAudioCancel:
		return "POST"
	default:
		return "POST"
	}
}

func (e JimengAudioEndpoint) requiresRequestBody() bool {
	return e == JimengAudioGeneration
}

func (e JimengAudioEndpoint) isGeneration() bool {
	return e == JimengAudioGeneration
}

// ForwardJimengAudio 转发音频请求到 AIV2API 上游
func (s *OpenAIGatewayService) ForwardJimengAudio(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint JimengAudioEndpoint,
	taskID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("jimeng account is required")
	}
	if !account.IsJimeng() {
		return nil, fmt.Errorf("account platform %s is not supported for jimeng audio", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("jimeng account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetJimengAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("jimeng api_key account missing api_key credential")
	}

	targetURL, err := s.jimengAudioURL(account, endpoint, taskID)
	if err != nil {
		return nil, err
	}

	var bodyReader io.Reader
	if endpoint.requiresRequestBody() {
		bodyReader = bytes.NewReader(body)
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, endpoint.httpMethod(), targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Accept", "application/json")
	upstreamReq.Header.Set("User-Agent", "sub2api-jimeng/1.0")
	// 透传 Idempotency-Key，防止 failover 重试重复创建付费任务
	if idempotencyKey := c.GetHeader("Idempotency-Key"); idempotencyKey != "" {
		upstreamReq.Header.Set("Idempotency-Key", idempotencyKey)
	}
	if endpoint.requiresRequestBody() {
		ct := strings.TrimSpace(contentType)
		if ct == "" {
			ct = "application/json"
		}
		upstreamReq.Header.Set("Content-Type", ct)
	}

	proxyURL := ""
	if account.ProxyID != nil && account.Proxy != nil {
		proxyURL = account.Proxy.URL()
	}

	upstreamStart := time.Now()
	resp, err := s.httpUpstream.Do(upstreamReq, proxyURL, account.ID, account.Concurrency)
	SetOpsLatencyMs(c, OpsUpstreamLatencyMsKey, time.Since(upstreamStart).Milliseconds())
	if err != nil {
		return nil, s.handleOpenAIUpstreamTransportError(ctx, c, account, err, false)
	}
	defer func() { _ = resp.Body.Close() }()

	requestIDHeader := firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("request-id"))
	requestInfo := jimeng.ParseAudioRequest(body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleJimengAudioErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeJimengAudioResponse(c, resp, respBody, s.responseHeaderFilter)

	result := &OpenAIForwardResult{
		RequestID:       requestIDHeader,
		Model:           requestModel,
		BillingModel:    requestModel,
		UpstreamModel:   requestModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}
	if endpoint.isGeneration() {
		result.ResponseID = extractJimengAudioTaskID(respBody)
		// 音频计费逻辑待实现
	}
	return result, nil
}

func (s *OpenAIGatewayService) jimengAudioURL(account *Account, endpoint JimengAudioEndpoint, taskID string) (string, error) {
	baseURL := strings.TrimSpace(account.GetJimengBaseURL())
	if baseURL == "" {
		baseURL = jimeng.DefaultBaseURL
	}
	validated, err := s.validateJimengBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid jimeng base_url: %w", err)
	}
	// AIV2API (Leonardo vendor) audio endpoints
	if account.IsJimengLeonardo() {
		switch endpoint {
		case JimengAudioGeneration:
			return leonardo.BuildAudioGenerationsURL(validated)
		case JimengAudioStatus:
			if taskID == "" {
				return "", fmt.Errorf("task_id is required for audio status query")
			}
			return leonardo.BuildAudioStatusURL(validated, taskID)
		case JimengAudioCancel:
			if taskID == "" {
				return "", fmt.Errorf("task_id is required for audio cancel")
			}
			return leonardo.BuildAudioCancelURL(validated, taskID)
		default:
			return "", fmt.Errorf("unsupported jimeng audio endpoint: %s", endpoint)
		}
	}
	// 原生即梦暂不支持音频
	return "", fmt.Errorf("audio endpoint not supported for native jimeng accounts")
}

func (s *OpenAIGatewayService) handleJimengAudioErrorResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestIDHeader string,
	requestModel string,
) (*OpenAIForwardResult, error) {
	// 音频和视频的错误处理逻辑完全一样，直接复用
	return s.handleJimengVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
}

func writeJimengAudioResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	// 音频响应无需特殊规范化，直接透传
	c.Data(resp.StatusCode, contentType, body)
}

func extractJimengAudioTaskID(body []byte) string {
	// 从响应体提取任务 ID（AIV2API 返回格式：{"id":"...","status":"..."}）
	if !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "id").String())
}

