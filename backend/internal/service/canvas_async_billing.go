package service

// canvas 异步任务「完成时计费」协议层。
//
// 二开背景：canvas 三个异步端点（图像 /v1/tasks、视频 /v1/videos、音频 /v1/audio）
// 原本在「提交（create）成功返回 2xx」时就扣费，但异步任务此刻只是排队，
// 最终可能失败/取消。失败不该扣费，于是把扣费从提交时挪到「轮询命中终态成功」时。
//
// 计费权威仍在 sub2api 网关：canvas-api 的服务端 worker 会持续轮询
// GET /v1/tasks/{id}（经 sub2api），任务终态必被某次轮询命中，
// 那次轮询在此判定终态并填充计费字段，由 handler 触发一次扣费。
//
// 幂等由 handler 侧用稳定的 task 级 request_id 保证（见 canvasAsyncBillingRequestID），
// 同一任务被轮询几十次也只扣一次。本文件只负责「从上游响应体判定终态 + 填充计费字段」，
// 全部逻辑走上游既有 API 表面，不改动上游共享计费核心。

import (
	"strings"

	"github.com/tidwall/gjson"
)

// canvasAsyncBillingRequestIDPrefix 是完成计费幂等 key 的前缀。
//
// 覆盖到 ctx 的 ClientRequestID 后，resolveUsageBillingRequestID 会走最高优先级
// 返回 "client:canvas_async_task:{taskID}"，对同一任务的每次轮询恒定，
// 从而命中 usage_billing_dedup 的 (request_id, api_key_id) 唯一约束只扣一次。
const canvasAsyncBillingRequestIDPrefix = "canvas_async_task:"

// CanvasAsyncBillingRequestID 为异步任务生成 task 级稳定的计费幂等标识。
// taskID 为空时返回空串，调用方据此跳过计费（无从保证幂等）。
func CanvasAsyncBillingRequestID(taskID string) string {
	id := strings.TrimSpace(taskID)
	if id == "" {
		return ""
	}
	return canvasAsyncBillingRequestIDPrefix + id
}

// canvasAsyncTerminalSuccess 判定上游任务状态是否为「终态成功」。
//
// 状态取值差异（已在 canvas-api transport.go 印证）：
//   - 图像 / 音频上游用 "succeeded"
//   - 视频上游用 "completed"
//
// 两者都算成功；其余（processing/queued/pending/failed/cancelled/…）一律不计费。
func canvasAsyncTerminalSuccess(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "succeeded", "completed":
		return true
	default:
		return false
	}
}

// canvasAsyncStatusModel 从轮询响应体提取 model；缺失时回退到 fallback。
//
// 轮询请求体为空，requestModel 取不到，必须从上游响应体的 model 字段拿。
// 计费与选号都需要非空模型名。
func canvasAsyncStatusModel(respBody []byte, fallback string) string {
	if gjson.ValidBytes(respBody) {
		if m := strings.TrimSpace(gjson.GetBytes(respBody, "model").String()); m != "" {
			return m
		}
	}
	return strings.TrimSpace(fallback)
}

// canvasAsyncStatusSeconds 从轮询响应体提取产物时长（秒），用于使用记录展示。
//
// 仅用于展示（VideoDurationSeconds），不进 canvas 平台扣费金额（金额只按 VideoCount 按次）。
// 上游统一形状 result.data[0].duration 单位为秒；取不到返回 0。
func canvasAsyncStatusSeconds(respBody []byte) int {
	if !gjson.ValidBytes(respBody) {
		return 0
	}
	secs := gjson.GetBytes(respBody, "result.data.0.duration").Int()
	if secs <= 0 {
		return 0
	}
	return int(secs)
}

// applyCanvasAsyncCompletionBilling 在轮询命中终态成功时，把计费字段填入 result。
//
// 返回 true 表示本次轮询判定为终态成功、已填充计费字段，handler 应触发一次扣费；
// 返回 false 表示非终态或非成功（含 failed/cancelled/processing），不计费。
//
// taskID 为该异步任务的上游 ID，用于计费幂等 key（handler 侧使用）与账号绑定回填。
// withSeconds 为 true 时额外解析产物时长填入 VideoSeconds（视频/音频用；图像传 false）。
func applyCanvasAsyncCompletionBilling(result *OpenAIForwardResult, respBody []byte, taskID string, withSeconds bool) bool {
	if result == nil {
		return false
	}
	status := ""
	if gjson.ValidBytes(respBody) {
		status = gjson.GetBytes(respBody, "status").String()
	}
	if !canvasAsyncTerminalSuccess(status) {
		return false
	}

	model := canvasAsyncStatusModel(respBody, result.Model)
	result.Model = model
	result.BillingModel = model
	result.UpstreamModel = model
	result.ResponseID = strings.TrimSpace(taskID)
	// canvas 按次计费统一用 VideoCount（避免与图片 token 计费路径混淆），一任务一次。
	result.VideoCount = 1
	if withSeconds {
		if secs := canvasAsyncStatusSeconds(respBody); secs > 0 {
			result.VideoSeconds = secs
		}
	}
	return true
}
