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
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
)

// CanvasVideoEndpoint 标识 AIV2API 视频接口的操作类型。
type CanvasVideoEndpoint string

const (
	// CanvasVideoEndpointCreate 创建视频任务：POST /v1/videos/generations。
	CanvasVideoEndpointCreate CanvasVideoEndpoint = "create"
	// CanvasVideoEndpointStatus 查询任务状态：GET /v1/videos/{id}。
	CanvasVideoEndpointStatus CanvasVideoEndpoint = "status"
	// CanvasVideoEndpointCancel 取消排队中的视频任务：POST /v1/videos/{id}/cancel。
	CanvasVideoEndpointCancel CanvasVideoEndpoint = "cancel"
)

// httpMethod 返回该端点对应的 HTTP 方法。
func (e CanvasVideoEndpoint) httpMethod() string {
	switch e {
	case CanvasVideoEndpointCreate, CanvasVideoEndpointCancel:
		return http.MethodPost
	default:
		return http.MethodGet
	}
}

// requiresRequestBody 创建任务与取消任务需要携带请求体（cancel 发空 body）。
func (e CanvasVideoEndpoint) requiresRequestBody() bool {
	return e == CanvasVideoEndpointCreate || e == CanvasVideoEndpointCancel
}

// isGeneration 标识是否为生成型请求（用于计费判定）。
func (e CanvasVideoEndpoint) isGeneration() bool {
	return e == CanvasVideoEndpointCreate
}

// ForwardCanvasVideo 将视频请求转发到 AIV2API 上游。
//
//   - 仅支持 api_key 账号（base_url + api_key）；
//   - body 原样透传（JSON 或 multipart），不做字段改写；
//   - Content-Type 透传（multipart 需携带 boundary）；
//   - Idempotency-Key 透传，防止 failover 重试重复扣费。
func (s *OpenAIGatewayService) ForwardCanvasVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint CanvasVideoEndpoint,
	taskID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("canvas account is required")
	}
	if !account.IsCanvas() {
		return nil, fmt.Errorf("account platform %s is not supported for canvas video", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("canvas account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetCanvasAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("canvas api_key account missing api_key credential")
	}

	targetURL, err := s.canvasVideoURL(account, endpoint, taskID)
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
	upstreamReq.Header.Set("User-Agent", "sub2api-canvas/1.0")
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
	requestInfo := avi2api.ParseVideoRequest(contentType, body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleCanvasVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeCanvasVideoResponse(c, resp, respBody, s.responseHeaderFilter)

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
		result.ResponseID = extractCanvasVideoTaskID(respBody)
	} else if endpoint == CanvasVideoEndpointStatus {
		// 视频计费：按次（VideoCount=1）+ 展示时长（VideoSeconds）。
		if applyCanvasAsyncCompletionBilling(result, respBody, taskID, "video") {
			// 响应体缺产物时长时，从模型注册表取默认时长兜底，
			// 避免 per-second 计费因 VideoSeconds=0 而错误回退到按次计费。
			if result.VideoSeconds == 0 {
				if caps := avi2api.LookupVideoModel(result.Model); caps != nil {
					result.VideoSeconds = caps.Duration.Default
				}
			}
		}
	}
	usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
	result.Usage = usage
	return result, nil
}

// canvasVideoURL 构建 AIV2API 目标 URL：先 allowlist 校验 base_url，再拼接端点路径。
// 所有 canvas 账号统一走 AIV2API 协议（/v1/videos/generations 等）。
func (s *OpenAIGatewayService) canvasVideoURL(account *Account, endpoint CanvasVideoEndpoint, taskID string) (string, error) {
	baseURL := strings.TrimSpace(account.GetCanvasBaseURL())
	if baseURL == "" {
		baseURL = avi2api.DefaultBaseURL
	}
	validated, err := s.validateCanvasBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid avi2api base_url: %w", err)
	}
	switch endpoint {
	case CanvasVideoEndpointCreate:
		return avi2api.BuildVideosGenerationsURL(validated)
	case CanvasVideoEndpointStatus:
		return avi2api.BuildVideoStatusURL(validated, taskID)
	case CanvasVideoEndpointCancel:
		return avi2api.BuildVideoCancelURL(validated, taskID)
	default:
		return "", fmt.Errorf("unsupported avi2api video endpoint: %s", endpoint)
	}
}

// validateCanvasBaseURL 复用与 Grok/OpenAI api_key 相同的通用 URL 校验策略：
// 未开启 allowlist 时仅做格式校验；开启时按管理员配置的 UpstreamHosts 放行。
// nil-safe：cfg 缺省时退化为格式校验，便于单测与最小配置部署。
func (s *OpenAIGatewayService) validateCanvasBaseURL(raw string) (string, error) {
	if s.cfg == nil || !s.cfg.Security.URLAllowlist.Enabled {
		allowInsecureHTTP := s.cfg != nil && s.cfg.Security.URLAllowlist.AllowInsecureHTTP
		return urlvalidator.ValidateURLFormat(raw, allowInsecureHTTP)
	}
	return urlvalidator.ValidateHTTPSURL(raw, urlvalidator.ValidationOptions{
		AllowedHosts:     s.cfg.Security.URLAllowlist.UpstreamHosts,
		RequireAllowlist: true,
		AllowPrivate:     s.cfg.Security.URLAllowlist.AllowPrivateHosts,
	})
}
