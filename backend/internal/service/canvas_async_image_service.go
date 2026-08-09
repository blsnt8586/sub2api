package service

// canvas 异步图像任务协议层（AIV2API /v1/tasks/* 端点）。
//
// 端点清单：
//   POST /v1/tasks/images         创建异步图像任务（仅文生图；AsyncImageRequest，additionalProperties:false）
//   GET  /v1/tasks/{id}           统一任务查询（kind=image|video|audio，同时修复 task_poller 恒 404 缺陷）
//   POST /v1/tasks/{id}/cancel    统一取消
//
// 注意：图生图（带参考图）上游没有异步版本，走同步 /v1/images/edits + Idempotency-Key，
// 由 canvas-api 的 image-edit Adapter 处理，不经过本文件。

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// CanvasAsyncImageEndpoint 标识 AIV2API 异步图像任务的端点类型。
type CanvasAsyncImageEndpoint string

const (
	// CanvasAsyncImageCreate 创建异步图像任务：POST /v1/tasks/images。
	CanvasAsyncImageCreate CanvasAsyncImageEndpoint = "task_create"
	// CanvasAsyncImageStatus 查询任务状态：GET /v1/tasks/{id}。
	// 该端点为统一端点，同时服务 video / audio 任务，路由路径为 /tasks/:id。
	CanvasAsyncImageStatus CanvasAsyncImageEndpoint = "task_status"
	// CanvasAsyncImageCancel 取消排队中的任务：POST /v1/tasks/{id}/cancel。
	CanvasAsyncImageCancel CanvasAsyncImageEndpoint = "task_cancel"
)

func (e CanvasAsyncImageEndpoint) httpMethod() string {
	switch e {
	case CanvasAsyncImageCreate, CanvasAsyncImageCancel:
		return "POST"
	default:
		return "GET"
	}
}

func (e CanvasAsyncImageEndpoint) requiresRequestBody() bool {
	return e == CanvasAsyncImageCreate
}

func (e CanvasAsyncImageEndpoint) isGeneration() bool {
	return e == CanvasAsyncImageCreate
}

// ForwardCanvasAsyncImage 转发异步图像请求到 AIV2API 上游（经 sub2api 路由）。
//
// 实现完全对齐 ForwardCanvasAudio：使用 s.httpUpstream.Do 经代理发出，
// 复用 detachUpstreamContext / handleOpenAIUpstreamTransportError /
// ReadUpstreamResponseBody 等既有基础设施。计费由 handler 层负责。
func (s *OpenAIGatewayService) ForwardCanvasAsyncImage(
	ctx context.Context,
	c *gin.Context,
	account *Account,
	endpoint CanvasAsyncImageEndpoint,
	taskID string,
	body []byte,
	contentType string,
) (*OpenAIForwardResult, error) {
	startTime := time.Now()
	if account == nil {
		return nil, fmt.Errorf("avi2api account is required")
	}
	if !account.IsCanvas() {
		return nil, fmt.Errorf("account platform %s is not supported for avi2api async image", account.Platform)
	}
	if account.Type != AccountTypeAPIKey {
		return nil, fmt.Errorf("avi2api account type %s is not supported; only api_key is allowed", account.Type)
	}

	apiKey := account.GetCanvasAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("avi2api api_key account missing api_key credential")
	}

	targetURL, err := s.canvasAsyncImageURL(account, endpoint, taskID)
	if err != nil {
		return nil, err
	}

	var bodyReader *bytes.Reader
	if endpoint.requiresRequestBody() {
		bodyReader = bytes.NewReader(body)
	} else {
		bodyReader = bytes.NewReader(nil)
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
	if ik := c.GetHeader("Idempotency-Key"); ik != "" {
		upstreamReq.Header.Set("Idempotency-Key", ik)
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
	requestModel := extractAsyncImageModel(body)

	if resp.StatusCode >= 400 {
		return s.handleCanvasVideoErrorResponse(ctx, resp, c, account, requestIDHeader, requestModel)
	}

	respBody, err := ReadUpstreamResponseBody(resp.Body, s.cfg, c, openAITooLargeError)
	if err != nil {
		return nil, err
	}
	writeCanvasAsyncImageResponse(c, resp, respBody)

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
		// 异步任务此刻仅排队，最终可能失败。扣费移到 status 轮询命中终态成功时
		// （applyCanvasAsyncCompletionBilling），失败/取消不计费。
		result.ResponseID = extractAsyncImageTaskID(respBody)
	} else if endpoint == CanvasAsyncImageStatus {
		// 图像按次计费，无产物时长维度，withSeconds=false。
		applyCanvasAsyncCompletionBilling(result, respBody, taskID, false)
	}
	return result, nil
}

// canvasAsyncImageURL 构建异步图像任务的上游 URL。
func (s *OpenAIGatewayService) canvasAsyncImageURL(account *Account, endpoint CanvasAsyncImageEndpoint, taskID string) (string, error) {
	baseURL := strings.TrimSpace(account.GetCanvasBaseURL())
	if baseURL == "" {
		baseURL = avi2api.DefaultBaseURL
	}
	validated, err := s.validateCanvasBaseURL(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid avi2api base_url: %w", err)
	}
	switch endpoint {
	case CanvasAsyncImageCreate:
		return avi2api.BuildAsyncImageTaskURL(validated)
	case CanvasAsyncImageStatus:
		if taskID == "" {
			return "", fmt.Errorf("task_id is required for async image status query")
		}
		return avi2api.BuildTaskStatusURL(validated, taskID)
	case CanvasAsyncImageCancel:
		if taskID == "" {
			return "", fmt.Errorf("task_id is required for async image cancel")
		}
		return avi2api.BuildTaskCancelURL(validated, taskID)
	default:
		return "", fmt.Errorf("unsupported avi2api async image endpoint: %s", endpoint)
	}
}

// writeCanvasAsyncImageResponse 透传上游响应头与 body 到客户端。
func writeCanvasAsyncImageResponse(c *gin.Context, resp *http.Response, body []byte) {
	for key, vals := range resp.Header {
		for _, v := range vals {
			c.Header(key, v)
		}
	}
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), body)
}

// extractAsyncImageTaskID 从任务创建响应体中提取上游 task id。
func extractAsyncImageTaskID(body []byte) string {
	if id := gjson.GetBytes(body, "id").String(); id != "" {
		return id
	}
	return gjson.GetBytes(body, "task_id").String()
}

// extractAsyncImageModel 从请求体中提取 model 字段。
func extractAsyncImageModel(body []byte) string {
	return gjson.GetBytes(body, "model").String()
}

// CanvasAsyncImageTaskSessionHash 为异步图像任务 ID 生成粘性会话 hash，
// 使状态查询 / 取消复用创建任务时选定的账号。
func CanvasAsyncImageTaskSessionHash(taskID string) string {
	return "canvas_async_image_task:" + taskID
}

// BindCanvasAsyncImageTaskAccount 将异步图像任务 ID 与账号绑定为粘性会话。
func (s *OpenAIGatewayService) BindCanvasAsyncImageTaskAccount(ctx context.Context, groupID *int64, taskID string, accountID int64) error {
	return s.BindStickySession(ctx, groupID, CanvasAsyncImageTaskSessionHash(taskID), accountID)
}

// ExtractCanvasAsyncImageModel 从异步图像请求体中提取 model 字段，供 handler 层调用。
func ExtractCanvasAsyncImageModel(body []byte) string {
	return extractAsyncImageModel(body)
}
