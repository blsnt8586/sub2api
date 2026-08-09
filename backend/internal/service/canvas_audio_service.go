package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// CanvasAudioEndpoint 表示 AIV2API 音频任务的端点类型。
type CanvasAudioEndpoint string

const (
	JimengAudioGeneration CanvasAudioEndpoint = "audio_generation"
	CanvasAudioStatus     CanvasAudioEndpoint = "audio_status"
	CanvasAudioCancel     CanvasAudioEndpoint = "audio_cancel"
)

func (e CanvasAudioEndpoint) httpMethod() string {
	switch e {
	case JimengAudioGeneration, CanvasAudioCancel:
		return "POST"
	default:
		return "GET"
	}
}

func (e CanvasAudioEndpoint) requiresRequestBody() bool {
	return e == JimengAudioGeneration
}

func (e CanvasAudioEndpoint) isGeneration() bool {
	return e == JimengAudioGeneration
}

// ForwardCanvasAudio 转发音频请求到 AIV2API 上游。
//
// 三种模型：dialogue-v3（TTS）、music-v1（音乐生成）、sound-effects-v2（音效生成）。
// 请求均为 application/json，body 原样透传，Idempotency-Key 透传。
func (s *OpenAIGatewayService) ForwardCanvasAudio(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint CanvasAudioEndpoint,
	taskID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("avi2api account is required")
	}
	if !account.IsCanvas() {
		return nil, fmt.Errorf("account platform %s is not supported for avi2api audio", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("avi2api account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetCanvasAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("avi2api api_key account missing api_key credential")
	}

	targetURL, err := s.canvasAudioURL(account, endpoint, taskID)
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
	upstreamReq.Header.Set("User-Agent", "sub2api-avi2api/1.0")
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
	requestInfo := avi2api.ParseAudioRequest(body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleCanvasVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeCanvasAudioResponse(c, resp, respBody, s.responseHeaderFilter)

	result := &OpenAIForwardResult{
		RequestID:       requestIDHeader,
		Model:           requestModel,
		BillingModel:    requestModel,
		UpstreamModel:   requestModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}
	if endpoint.isGeneration() {
		// 提交时只记录 task id 用于账号绑定（粘性会话），不在此扣费——
		// 异步任务此刻仅排队，最终可能失败。扣费移到 status 轮询命中终态成功时。
		result.ResponseID = extractCanvasAudioTaskID(respBody)
	} else if endpoint == CanvasAudioStatus {
		// 音频计费：按次（VideoCount=1）+ 展示时长（VideoSeconds）。
		applyCanvasAsyncCompletionBilling(result, respBody, taskID, true)
	}
	return result, nil
}

func (s *OpenAIGatewayService) canvasAudioURL(account *Account, endpoint CanvasAudioEndpoint, taskID string) (string, error) {
	baseURL := strings.TrimSpace(account.GetCanvasBaseURL())
	if baseURL == "" {
		baseURL = avi2api.DefaultBaseURL
	}
	validated, err := s.validateCanvasBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid avi2api base_url: %w", err)
	}
	switch endpoint {
	case JimengAudioGeneration:
		return avi2api.BuildAudioGenerationsURL(validated)
	case CanvasAudioStatus:
		if taskID == "" {
			return "", fmt.Errorf("task_id is required for audio status query")
		}
		return avi2api.BuildAudioStatusURL(validated, taskID)
	case CanvasAudioCancel:
		if taskID == "" {
			return "", fmt.Errorf("task_id is required for audio cancel")
		}
		return avi2api.BuildAudioCancelURL(validated, taskID)
	default:
		return "", fmt.Errorf("unsupported avi2api audio endpoint: %s", endpoint)
	}
}

func writeCanvasAudioResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	c.Data(resp.StatusCode, contentType, body)
}

func extractCanvasAudioTaskID(body []byte) string {
	if !gjson.ValidBytes(body) {
		return ""
	}
	return strings.TrimSpace(gjson.GetBytes(body, "id").String())
}

// ExtractCanvasAudioModel 从音频请求体中提取 model 字段，供 handler 层调用。
//
// AIV2API 的 SoundEffectsAudioRequest 只把 prompt 列为 required，model 缺省时
// 上游按 sound-effects-v2 处理。此处保持原样返回空串，由调用方决定是否兜底。
func ExtractCanvasAudioModel(body []byte) string {
	return avi2api.ParseAudioRequest(body).Model
}

// CanvasAudioDefaultModel 是 model 缺省时的兜底模型，对齐 AIV2API 的
// SoundEffectsAudioRequest.model 默认值。选号与计费需要一个非空模型名。
const CanvasAudioDefaultModel = "sound-effects-v2"

// CanvasAudioTaskSessionHash 为音频任务 ID 生成粘性会话 hash，
// 使后续的状态查询 / 取消复用创建任务时选中的账号。
func CanvasAudioTaskSessionHash(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}
	return "avi2api-audio:" + DeriveSessionHashFromSeed(taskID)
}

// BindCanvasAudioTaskAccount 将音频任务 ID 与账号绑定为粘性会话。
func (s *OpenAIGatewayService) BindCanvasAudioTaskAccount(ctx context.Context, groupID *int64, taskID string, accountID int64) error {
	return s.BindStickySession(ctx, groupID, CanvasAudioTaskSessionHash(taskID), accountID)
}
