package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ptrStringOrNil 若 s 非空返回其指针，否则返回 nil。
func ptrStringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// forwardGrokResponsesViaAPIKey 处理 api_key 类型的 Grok 账号（第三方 xAI 兼容端点）。
//
// 与 OAuth 订阅转发（forwardGrokResponses）刻意分离，对齐 OpenAI 的
// oauth / api_key 双路径设计：
//   - URL 校验走通用可配置 allowlist（grokAPIKeyResponsesURL），而非 xAI 官方
//     域名硬白名单，第三方兼容端点才能真正放行；
//   - 鉴权用 credentials.api_key 作为 Bearer，不经过 GrokTokenProvider 刷新；
//   - 不做 OAuth 订阅专属的配额快照 / tempUnschedule（那些依赖 xAI 订阅响应头）。
//
// body sanitize（剔除 xAI 不支持的字段/工具）与响应处理为无害通用逻辑，直接复用。
func (s *OpenAIGatewayService) forwardGrokResponsesViaAPIKey(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	body []byte,
	originalModel string,
	reqStream bool,
	startTime time.Time,
) (*OpenAIForwardResult, error) {
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("grok account type %s is not supported by api key forwarding", account.Type)
	}

	upstreamModel := account.GetMappedModel(originalModel)
	if strings.TrimSpace(upstreamModel) == "" {
		upstreamModel = "grok-4.3"
	}
	patchedBody, err := patchGrokResponsesBody(body, upstreamModel)
	if err != nil {
		return nil, err
	}

	apiKey := strings.TrimSpace(account.GetCredential("api_key"))
	if apiKey == "" {
		return nil, fmt.Errorf("grok api_key account missing api_key credential")
	}

	targetURL, err := s.grokAPIKeyResponsesURL(account)
	if err != nil {
		return nil, err
	}

	upstreamCtx, releaseUpstreamCtx := detachUpstreamContext(ctx)
	defer releaseUpstreamCtx()
	upstreamReq, err := buildGrokAPIKeyResponsesRequest(upstreamCtx, c, targetURL, patchedBody, apiKey)
	if err != nil {
		return nil, err
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

	if resp.StatusCode >= 400 {
		respBody := s.readUpstreamErrorBody(resp)
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		upstreamMsg := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(respBody))
		if upstreamMsg == "" {
			upstreamMsg = fmt.Sprintf("xAI-compatible upstream returned status %d", resp.StatusCode)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
			Kind:               "failover",
			Message:            upstreamMsg,
		})
		s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, respBody, upstreamModel)
		if s.shouldFailoverUpstreamError(resp.StatusCode) {
			return nil, &UpstreamFailoverError{
				StatusCode:             resp.StatusCode,
				ResponseBody:           respBody,
				RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
			}
		}
		return s.handleErrorResponse(ctx, resp, c, account, patchedBody, upstreamModel)
	}

	var usage *OpenAIUsage
	var firstTokenMs *int
	responseID := ""
	if reqStream {
		streamResult, err := s.handleStreamingResponse(ctx, resp, c, account, startTime, originalModel, upstreamModel)
		if err != nil {
			return nil, err
		}
		usage = streamResult.usage
		firstTokenMs = streamResult.firstTokenMs
		responseID = strings.TrimSpace(streamResult.responseID)
	} else {
		nonStreamResult, err := s.handleNonStreamingResponse(ctx, resp, c, account, originalModel, upstreamModel)
		if err != nil {
			return nil, err
		}
		usage = nonStreamResult.usage
		responseID = strings.TrimSpace(nonStreamResult.responseID)
	}

	if usage == nil {
		usage = &OpenAIUsage{}
	}
	return &OpenAIForwardResult{
		RequestID:       firstNonEmpty(resp.Header.Get("x-request-id"), resp.Header.Get("xai-request-id")),
		ResponseID:      responseID,
		Usage:           *usage,
		Model:           originalModel,
		UpstreamModel:   upstreamModel,
		ReasoningEffort: ptrStringOrNil(normalizeOpenAIReasoningEffort(gjson.GetBytes(patchedBody, "reasoning.effort").String())),
		Stream:          reqStream,
		OpenAIWSMode:    false,
		ResponseHeaders: resp.Header.Clone(),
		Duration:        time.Since(startTime),
		FirstTokenMs:    firstTokenMs,
	}, nil
}

// grokAPIKeyResponsesURL 用通用可配置 allowlist 校验第三方 base_url，并拼接 /responses。
// 与 OAuth 路径的 xai.BuildResponsesURL 不同——后者只放行 xAI 官方域名。
func (s *OpenAIGatewayService) grokAPIKeyResponsesURL(account *Account) (string, error) {
	baseURL := strings.TrimSpace(account.GetGrokBaseURL())
	if baseURL == "" {
		baseURL = xai.DefaultBaseURL
	}
	validated, err := s.validateGrokAPIKeyBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid grok base_url: %w", err)
	}
	return strings.TrimRight(validated, "/") + "/responses", nil
}

// validateGrokAPIKeyBaseURL 复用与 OpenAI api_key 相同的通用 URL 校验策略：
// 未开启 allowlist 时仅做格式校验；开启时按管理员配置的 UpstreamHosts 放行。
// nil-safe：cfg 缺省时退化为格式校验，便于单测与最小配置部署。
func (s *OpenAIGatewayService) validateGrokAPIKeyBaseURL(raw string) (string, error) {
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

func buildGrokAPIKeyResponsesRequest(ctx context.Context, c *gin.Context, targetURL string, body []byte, apiKey string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "sub2api-grok/1.0")
	if c != nil {
		if v := c.GetHeader("OpenAI-Beta"); strings.TrimSpace(v) != "" {
			req.Header.Set("OpenAI-Beta", v)
		}
	}
	return req, nil
}
