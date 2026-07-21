package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/leonardo"
	"github.com/gin-gonic/gin"
)

// LeonardoImageEndpoint 标识 Leonardo 图像接口的两种操作。
type LeonardoImageEndpoint string

const (
	// LeonardoImageEndpointGenerations 文生图：POST /v1/images/generations。
	LeonardoImageEndpointGenerations LeonardoImageEndpoint = "generations"
	// LeonardoImageEndpointEdits 图生图：POST /v1/images/edits（multipart）。
	LeonardoImageEndpointEdits LeonardoImageEndpoint = "edits"
)

// ExtractLeonardoImageModel 从图像请求体中提取 model 字段，供 handler 层调用。
func ExtractLeonardoImageModel(contentType string, body []byte) string {
	return leonardo.ExtractImageModel(contentType, body)
}

// ForwardJimengImages 将即梦平台下 Leonardo vendor 账号的图像请求转发到上游。
//
// 设计对齐 ForwardJimengVideo 的 api_key 第三方路径：
//   - 仅支持即梦平台 + Leonardo vendor 的 api_key 账号（base_url + api_key）；
//   - base_url 走通用可配置 allowlist 校验；
//   - 鉴权用 credentials.api_key 作为 Bearer；
//   - body 原样透传（Leonardo 图像接口为 OpenAI Images 兼容格式，无需改写）；
//   - 响应体透传给客户端，同时提取 data 数组长度作为计费用图片数量。
func (s *OpenAIGatewayService) ForwardJimengImages(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint LeonardoImageEndpoint,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("jimeng account is required")
	}
	if !account.IsJimeng() {
		return nil, fmt.Errorf("account platform %s is not supported for jimeng images", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("jimeng account type %s is not supported; only api_key is allowed", account.Type)
	}
	// 原生即梦账号无图像接口。返回 account 级 failover 错误（而非普通 error），
	// 让上层循环跳过该账号继续调度同组内的 Leonardo 账号；整组均不可用时才耗尽退出。
	if !account.IsJimengLeonardo() {
		return nil, newJimengImageAccountUnusableError(
			fmt.Sprintf("jimeng images require the leonardo vendor; account %d is native jimeng which has no images API", account.ID),
		)
	}

	apiKey := account.GetJimengAPIKey()
	if apiKey == "" {
		return nil, newJimengImageAccountUnusableError(
			fmt.Sprintf("jimeng leonardo account %d missing api_key credential", account.ID),
		)
	}

	targetURL, err := s.leonardoImageURL(account, endpoint)
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
	upstreamReq.Header.Set("User-Agent", "sub2api-jimeng/1.0")
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
	requestInfo := leonardo.ParseImageRequest(ct, body)
	requestModel := requestInfo.Model

	if resp.StatusCode >= 400 {
		return s.handleJimengVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeJimengVideoResponse(c, resp, respBody, s.responseHeaderFilter)

	// 图片数量：优先用响应 data 数组长度，回退到请求侧 N。
	imageCount := leonardo.CountImages(respBody)
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

// leonardoImageURLBuilder 占位符已移除。

// leonardoImageURL 构建 Leonardo 图像目标 URL：先用通用 allowlist 校验 base_url，再拼接端点路径。
func (s *OpenAIGatewayService) leonardoImageURL(account *Account, endpoint LeonardoImageEndpoint) (string, error) {
	baseURL := strings.TrimSpace(account.GetJimengBaseURL())
	if baseURL == "" {
		baseURL = leonardo.DefaultBaseURL
	}
	validated, err := s.validateJimengBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid leonardo base_url: %w", err)
	}
	switch endpoint {
	case LeonardoImageEndpointGenerations:
		return leonardo.BuildImagesGenerationsURL(validated)
	case LeonardoImageEndpointEdits:
		return leonardo.BuildImagesEditsURL(validated)
	default:
		return "", fmt.Errorf("unsupported leonardo image endpoint: %s", endpoint)
	}
}

// newJimengImageAccountUnusableError 构造一个 account 级的 failover 错误，用于所选账号
// 无法服务图像请求的场景（原生即梦无图像接口 / 缺 api_key）。StatusCode 取 502，
// 使 failover 耗尽时映射为通用的 "Upstream service temporarily unavailable"；Scope 为
// account 表示仅该账号不可用，应切换到同组其他账号而非中止整个请求。
func newJimengImageAccountUnusableError(reason string) *UpstreamFailoverError {
	return &UpstreamFailoverError{
		StatusCode:        http.StatusBadGateway,
		ResponseBody:      []byte(reason),
		Stage:             GatewayFailureStageInference,
		Scope:             GatewayFailureScopeAccount,
		NextAccountAction: NextAccountRetry,
	}
}
