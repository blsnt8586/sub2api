package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/Wei-Shaw/sub2api/internal/util/responseheaders"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// ExtractCanvasVideoModel 从请求体中提取 model 字段，供 handler 层调用。
// 兼容 JSON（无参考模式）与 multipart（参考素材 / 首尾帧模式）两种请求格式。
func ExtractCanvasVideoModel(contentType string, body []byte) string {
	return avi2api.ExtractVideoModel(contentType, body)
}

// CanvasVideoTaskSessionHash 为视频任务 ID 生成粘性会话 hash，
// 使后续的状态查询 / 取消复用创建任务时选中的账号。
func CanvasVideoTaskSessionHash(taskID string) string {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return ""
	}
	return "canvas-video:" + DeriveSessionHashFromSeed(taskID)
}

// BindCanvasVideoTaskAccount 将Canvas视频任务 ID 与账号绑定为粘性会话。
func (s *OpenAIGatewayService) BindCanvasVideoTaskAccount(ctx context.Context, groupID *int64, taskID string, accountID int64) error {
	return s.BindStickySession(ctx, groupID, CanvasVideoTaskSessionHash(taskID), accountID)
}

// extractCanvasVideoTaskID 从创建响应中提取任务 ID，兼容多种字段命名。
func extractCanvasVideoTaskID(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	for _, path := range []string{"id", "request_id", "task_id", "data.id", "data.request_id", "data.task_id", "video.id"} {
		if id := strings.TrimSpace(gjson.GetBytes(body, path).String()); id != "" {
			return id
		}
	}
	return ""
}

// normalizeCanvasVideoResponse 将 AIV2API的响应格式规范化为 OpenAI-compatible 格式。
//
// 新状态模型（2026-07-29）：
//
//	queued     → 本地排队，透传
//	processing → 准备/提交/查询中（合并原 reserving/uploading/submitted/polling），透传
//	succeeded  → 映射为 completed（infinite-canvas 识别 completed）
//	failed / cancelled → 透传
func normalizeCanvasVideoResponse(body []byte) []byte {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body // 非 JSON，直接返回
	}

	parsed := gjson.ParseBytes(body)

	// 只对包含 status 字段的响应做规范化（任务状态查询/创建响应）
	status := parsed.Get("status").String()
	if status == "" {
		return body // 无 status 字段，可能是其他类型响应
	}

	// 构建规范化后的 map（保留原字段 + 添加标准字段）
	var original map[string]any
	if err := json.Unmarshal(body, &original); err != nil {
		return body // 解析失败，返回原样
	}

	normalized := make(map[string]any)
	for k, v := range original {
		normalized[k] = v
	}

	// 1. 状态映射：succeeded → completed（infinite-canvas 用 completed 判断完成）
	//    queued / processing / failed / cancelled 直接透传
	if status == "succeeded" {
		normalized["status"] = "completed"
	}

	// 2. URL 提取：result.data[0].url → 顶层 video_url（OpenAI 标准字段）
	//    同时保留 result 结构以兼容 AIV2API 原生客户端
	resultDataURL := parsed.Get("result.data.0.url").String()
	if resultDataURL != "" {
		normalized["video_url"] = resultDataURL
		// 同时添加 result_url 别名（某些客户端可能用这个）
		normalized["result_url"] = resultDataURL
	}

	// 3. 添加 object 字段（OpenAI 风格）
	if _, exists := normalized["object"]; !exists {
		normalized["object"] = "video.task"
	}

	// 4. Seedance（Ark Plan v3）兼容：把 URL 同时挂到 content.video_url。
	//    按 Ark Plan 协议轮询的客户端（如 infinite-canvas 的 seedance 分支）只认
	//    这个位置或顶层 video_url；顶层已在步骤 2 写入，这里补齐嵌套形状。
	//    上游若自带 content 字段则不覆盖，避免破坏原生语义。
	if resultDataURL != "" {
		if _, exists := normalized["content"]; !exists {
			normalized["content"] = map[string]any{
				"video_url": resultDataURL,
			}
		}
	}

	// 5. 重新序列化
	result, err := json.Marshal(normalized)
	if err != nil {
		return body // 序列化失败，返回原样
	}

	return result
}

// writeCanvasVideoResponse 将上游响应写回客户端，JSON 响应会先做 OpenAI 兼容性规范化。
func writeCanvasVideoResponse(c *gin.Context, resp *http.Response, body []byte, filter *responseheaders.CompiledHeaderFilter) {
	if c == nil || resp == nil {
		return
	}
	writeOpenAIPassthroughResponseHeaders(c.Writer.Header(), resp.Header, filter)
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}

	// 对 JSON 响应做格式规范化（二进制视频内容直接透传）
	if strings.Contains(contentType, "application/json") || contentType == "" {
		body = normalizeCanvasVideoResponse(body)
	}

	c.Data(resp.StatusCode, contentType, body)
}

// handleCanvasVideoErrorResponse 处理上游 4xx/5xx 错误，判定 failover 或直接透传错误。
func (s *OpenAIGatewayService) handleCanvasVideoErrorResponse(
	ctx context.Context,
	resp *http.Response,
	c *gin.Context,
	account *Account,
	requestIDHeader string,
	requestedModel string,
) (*OpenAIForwardResult, error) {
	body := s.readUpstreamErrorBody(resp)
	upstreamMsg := sanitizeUpstreamErrorMessage(strings.TrimSpace(extractUpstreamErrorMessage(body)))
	if upstreamMsg == "" {
		upstreamMsg = fmt.Sprintf("canvas upstream returned status %d", resp.StatusCode)
	}

	upstreamDetail := ""
	if s.cfg != nil && s.cfg.Gateway.LogUpstreamErrorBody {
		maxBytes := s.cfg.Gateway.LogUpstreamErrorBodyMaxBytes
		if maxBytes <= 0 {
			maxBytes = 2048
		}
		upstreamDetail = truncateString(string(body), maxBytes)
	}
	setOpsUpstreamError(c, resp.StatusCode, upstreamMsg, upstreamDetail)

	if status, errType, errMsg, matched := applyErrorPassthroughRule(
		c,
		account.Platform,
		resp.StatusCode,
		body,
		http.StatusBadGateway,
		"upstream_error",
		"Upstream request failed",
	); matched {
		MarkResponseCommitted(c)
		writeGrokMediaErrorResponse(c, status, errType, errMsg)
		return nil, fmt.Errorf("upstream error: %d (passthrough rule matched) message=%s", resp.StatusCode, upstreamMsg)
	}

	if !account.ShouldHandleErrorCode(resp.StatusCode) {
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: resp.StatusCode,
			UpstreamRequestID:  requestIDHeader,
			Kind:               "http_error",
			Message:            upstreamMsg,
			Detail:             upstreamDetail,
		})
		MarkResponseCommitted(c)
		writeGrokMediaErrorResponse(c, http.StatusInternalServerError, "upstream_error", "Upstream gateway error")
		return nil, fmt.Errorf("upstream error: %d (not in custom error codes) message=%s", resp.StatusCode, upstreamMsg)
	}

	s.handleOpenAIAccountUpstreamError(ctx, account, resp.StatusCode, resp.Header, body, requestedModel)
	kind := "http_error"
	if s.shouldFailoverUpstreamError(resp.StatusCode) {
		kind = "failover"
	}
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		Platform:           account.Platform,
		AccountID:          account.ID,
		AccountName:        account.Name,
		UpstreamStatusCode: resp.StatusCode,
		UpstreamRequestID:  requestIDHeader,
		Kind:               kind,
		Message:            upstreamMsg,
		Detail:             upstreamDetail,
	})
	if kind == "failover" {
		return nil, &UpstreamFailoverError{
			StatusCode:             resp.StatusCode,
			ResponseBody:           body,
			RetryableOnSameAccount: account.IsPoolMode() && account.IsPoolModeRetryableStatus(resp.StatusCode),
		}
	}

	MarkResponseCommitted(c)
	writeGrokMediaErrorResponse(c, resp.StatusCode, grokMediaErrorType(resp.StatusCode), upstreamMsg)
	return nil, fmt.Errorf("upstream error: %d %s", resp.StatusCode, upstreamMsg)
}

// ValidateCanvasVideoRequest 在转发前校验视频请求参数，返回非 nil 表示请求非法。
// 已知模型会按各自的 duration/size/resolution 约束和参考模式规则做本地校验；
// 未知模型直接放行，让上游返回错误（保持前向兼容）。
func ValidateCanvasVideoRequest(contentType string, body []byte) *avi2api.ValidationError {
	return avi2api.ValidateVideoRequest(contentType, body)
}

// ValidateCanvasAudioRequest 在转发前校验音频请求参数。
func ValidateCanvasAudioRequest(body []byte) *avi2api.ValidationError {
	return avi2api.ValidateAudioRequest(body)
}
