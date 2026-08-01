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
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
)

// JimengVideoEndpoint 标识即梦视频接口的三种操作。
type JimengVideoEndpoint string

const (
	// JimengVideoEndpointCreate 创建视频任务：POST /v1/videos。
	JimengVideoEndpointCreate JimengVideoEndpoint = "create"
	// JimengVideoEndpointStatus 查询任务状态：GET /v1/videos/{task_id}。
	JimengVideoEndpointStatus JimengVideoEndpoint = "status"
	// JimengVideoEndpointContent 下载视频内容：GET /v1/videos/{task_id}/content。
	JimengVideoEndpointContent JimengVideoEndpoint = "content"
	// JimengVideoEndpointCancel 取消视频任务：POST /v1/videos/{task_id}/cancel（Leonardo vendor 专用）。
	JimengVideoEndpointCancel JimengVideoEndpoint = "cancel"
)

// httpMethod 返回该端点对应的 HTTP 方法。
func (e JimengVideoEndpoint) httpMethod() string {
	switch e {
	case JimengVideoEndpointCreate, JimengVideoEndpointCancel:
		return http.MethodPost
	default:
		return http.MethodGet
	}
}

// requiresRequestBody 创建任务与取消任务需要携带请求体（cancel 发空 body）。
func (e JimengVideoEndpoint) requiresRequestBody() bool {
	return e == JimengVideoEndpointCreate || e == JimengVideoEndpointCancel
}

// isGeneration 标识是否为生成型请求（用于计费判定）。
func (e JimengVideoEndpoint) isGeneration() bool {
	return e == JimengVideoEndpointCreate
}

// ForwardJimengVideo 将即梦视频请求转发到第三方兼容端点。
//
// 设计对齐 Grok/OpenAI 的 api_key 第三方路径：
//   - 仅支持 api_key 账号（base_url + api_key），不涉及 OAuth；
//   - base_url 走通用可配置 allowlist 校验（jimengVideoURL），而非硬白名单；
//   - 鉴权用 credentials.api_key 作为 Bearer；
//   - body 原样透传（保留 model / seconds 字符串 / aspect_ratio），不做字段改写，
//     避免触发即梦上游的 "model name empty" / "seconds must be string" 校验错误。
func (s *OpenAIGatewayService) ForwardJimengVideo(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint JimengVideoEndpoint,
	taskID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("jimeng account is required")
	}
	if !account.IsJimeng() {
		return nil, fmt.Errorf("account platform %s is not supported for jimeng video", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("jimeng account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetJimengAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("jimeng api_key account missing api_key credential")
	}

	targetURL, err := s.jimengVideoURL(account, endpoint, taskID)
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
	requestInfo := jimeng.ParseVideoRequest(body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleJimengVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeJimengVideoResponse(c, resp, respBody, s.responseHeaderFilter)

	result := &OpenAIForwardResult{
		RequestID:       requestIDHeader,
		Model:           requestModel,
		BillingModel:    requestModel,
		UpstreamModel:   requestModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
	}
	if endpoint.isGeneration() {
		result.ResponseID = extractJimengVideoTaskID(respBody)
		// 视频计费：使用 VideoCount/VideoSeconds 而非 ImageCount，
		// 避免与图片计费路径混淆。
		result.VideoCount = 1
		if requestInfo.Seconds != "" {
			var secs int
			if _, err := fmt.Sscanf(requestInfo.Seconds, "%d", &secs); err == nil && secs > 0 {
				result.VideoSeconds = secs
			}
		}
	}
	usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
	result.Usage = usage
	return result, nil
}

// jimengVideoURL 构建目标 URL：先用通用 allowlist 校验 base_url，再拼接端点路径。
func (s *OpenAIGatewayService) jimengVideoURL(account *Account, endpoint JimengVideoEndpoint, taskID string) (string, error) {
	baseURL := strings.TrimSpace(account.GetJimengBaseURL())
	if baseURL == "" {
		baseURL = jimeng.DefaultBaseURL
	}
	validated, err := s.validateJimengBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid jimeng base_url: %w", err)
	}
	// Leonardo vendor（实为 AIV2API）：视频接口与原生即梦不同（创建走 /v1/videos/generations，
	// 无 /content 端点——MP4 URL 在状态响应的 result.data[0].url 中）。对外客户端契约
	// 仍是即梦的 POST /v1/videos + GET /v1/videos/{id} + POST /v1/videos/{id}/cancel，
	// 此处仅翻译上游真实路径。
	if account.IsJimengLeonardo() {
		switch endpoint {
		case JimengVideoEndpointCreate:
			return leonardo.BuildVideosGenerationsURL(validated)
		case JimengVideoEndpointStatus:
			return leonardo.BuildVideoStatusURL(validated, taskID)
		case JimengVideoEndpointCancel:
			return leonardo.BuildVideoCancelURL(validated, taskID)
		case JimengVideoEndpointContent:
			return "", fmt.Errorf("leonardo vendor does not support the video content endpoint; read the MP4 URL from result.data[0].url in the status response")
		default:
			return "", fmt.Errorf("unsupported jimeng video endpoint: %s", endpoint)
		}
	}
	switch endpoint {
	case JimengVideoEndpointCreate:
		return jimeng.BuildVideosURL(validated)
	case JimengVideoEndpointStatus:
		return jimeng.BuildVideoStatusURL(validated, taskID)
	case JimengVideoEndpointContent:
		return jimeng.BuildVideoContentURL(validated, taskID)
	default:
		return "", fmt.Errorf("unsupported jimeng video endpoint: %s", endpoint)
	}
}

// validateJimengBaseURL 复用与 Grok/OpenAI api_key 相同的通用 URL 校验策略：
// 未开启 allowlist 时仅做格式校验；开启时按管理员配置的 UpstreamHosts 放行。
// nil-safe：cfg 缺省时退化为格式校验，便于单测与最小配置部署。
func (s *OpenAIGatewayService) validateJimengBaseURL(raw string) (string, error) {
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
