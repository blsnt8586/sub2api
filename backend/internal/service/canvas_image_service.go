package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/gin-gonic/gin"
)

// CanvasImageEndpoint 标识 AVI2API 图像接口的两种操作。
type CanvasImageEndpoint string

const (
	// CanvasImageEndpointGenerations 文生图：POST /v1/images/generations。
	CanvasImageEndpointGenerations CanvasImageEndpoint = "generations"
	// CanvasImageEndpointEdits 图生图：POST /v1/images/edits（multipart）。
	CanvasImageEndpointEdits CanvasImageEndpoint = "edits"
)

// ExtractCanvasImageModel 从图像请求体中提取 model 字段，供 handler 层调用。
func ExtractCanvasImageModel(contentType string, body []byte) string {
	return avi2api.ExtractImageModel(contentType, body)
}

// ForwardCanvasImages 将图像请求转发到 AIV2API 上游。
//
// 设计对齐 ForwardCanvasVideo 的 api_key 第三方路径：
//   - 仅支持 api_key 账号（base_url + api_key）；
//   - base_url 走通用可配置 allowlist 校验；
//   - 鉴权用 credentials.api_key 作为 Bearer；
//   - body 原样透传（AIV2API 图像接口为 OpenAI Images 兼容格式，无需改写）；
//   - 响应体透传给客户端，同时提取 data 数组长度作为计费用图片数量。
func (s *OpenAIGatewayService) ForwardCanvasImages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint CanvasImageEndpoint,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("avi2api account is required")
	}
	if !account.IsCanvas() {
		return nil, fmt.Errorf("account platform %s is not supported for avi2api images", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("avi2api account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetCanvasAPIKey()
	if apiKey == "" {
		// 缺 api_key 的账号返回 account 级 failover 错误（而非普通 error），
		// 让上层循环跳过该账号继续调度同组内其他账号；整组均不可用时才耗尽退出。
		return nil, newCanvasImageAccountUnusableError(
			fmt.Sprintf("avi2api account %d missing api_key credential", account.ID),
		)
	}

	targetURL, err := s.canvasImageURL(account, endpoint)
	if err != nil {
		return nil, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := http.NewRequestWithContext(upstreamCtx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	upstreamReq.Header.Set("Authorization", "Bearer "+apiKey)
	upstreamReq.Header.Set("Accept", "application/json")
	upstreamReq.Header.Set("User-Agent", "sub2api-canvas/1.0")
	// 透传 Idempotency-Key：AIV2API 对 /images/generations、/images/edits 等
	// 创建类接口强制要求该头（缺失即报 "Idempotency-Key is required..."）。
	// 与 video/audio 转发保持一致，也避免 failover 重试重复创建付费任务。
	if idempotencyKey := c.GetHeader("Idempotency-Key"); idempotencyKey != "" {
		upstreamReq.Header.Set("Idempotency-Key", idempotencyKey)
	}
	ct := strings.TrimSpace(contentType)
	if ct == "" {
		ct = "application/json"
	}
	upstreamReq.Header.Set("Content-Type", ct)

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
	requestInfo := avi2api.ParseImageRequest(ct, body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleCanvasVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeCanvasVideoResponse(c, resp, respBody, s.responseHeaderFilter)

	// 图片数量：优先用响应 data 数组长度，回退到请求侧 N。
	imageCount := avi2api.CountImages(respBody)
	if imageCount <= 0 {
		imageCount = requestInfo.N
	}
	if imageCount <= 0 {
		imageCount = 1
	}

	result := &OpenAIForwardResult{
		RequestID:       requestIDHeader,
		Model:           requestModel,
		BillingModel:    requestModel,
		UpstreamModel:   requestModel,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		ImageCount:      imageCount,
		ImageSize:       NormalizeImageBillingTierOrDefault(requestInfo.Size),
		ImageInputSize:  requestInfo.Size,
	}
	usage, _ := extractOpenAIUsageFromJSONBytes(respBody)
	result.Usage = usage
	return result, nil
}

// canvasImageURLBuilder 占位符已移除。

// canvasImageURL 构建 AVI2API 图像目标 URL：先用通用 allowlist 校验 base_url，再拼接端点路径。
func (s *OpenAIGatewayService) canvasImageURL(account *Account, endpoint CanvasImageEndpoint) (string, error) {
	baseURL := strings.TrimSpace(account.GetCanvasBaseURL())
	if baseURL == "" {
		baseURL = avi2api.DefaultBaseURL
	}
	validated, err := s.validateCanvasBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid avi2api base_url: %w", err)
	}
	switch endpoint {
	case CanvasImageEndpointGenerations:
		return avi2api.BuildImagesGenerationsURL(validated)
	case CanvasImageEndpointEdits:
		return avi2api.BuildImagesEditsURL(validated)
	default:
		return "", fmt.Errorf("unsupported avi2api image endpoint: %s", endpoint)
	}
}

// newCanvasImageAccountUnusableError 构造一个 account 级的 failover 错误，用于所选账号
// 无法服务图像请求的场景（原生Canvas无图像接口 / 缺 api_key）。StatusCode 取 502，
// 使 failover 耗尽时映射为通用的 "Upstream service temporarily unavailable"；Scope 为
// account 表示仅该账号不可用，应切换到同组其他账号而非中止整个请求。
func newCanvasImageAccountUnusableError(reason string) *UpstreamFailoverError {
	return &UpstreamFailoverError{
		StatusCode:        http.StatusBadGateway,
		ResponseBody:      []byte(reason),
		Stage:             GatewayFailureStageInference,
		Scope:             GatewayFailureScopeAccount,
		NextAccountAction: NextAccountRetry,
	}
}
