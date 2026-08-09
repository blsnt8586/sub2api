package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerVideoRoutes 注册视频生成相关路由（Grok、AIV2API 等）。
// 新增视频平台时仅需在此文件中：
//  1. 在对应 handler 函数里追加 case
//  2. 如有专属路径，在本函数末尾添加路由
//
// 无需修改 gateway.go。
//
// 对外契约统一为 AIV2API 风格（2026-08 起，原生Canvas下线）：
//
//	POST /v1/videos/generations       创建视频任务（JSON 无参考 / multipart 带参考素材）
//	GET  /v1/videos/{id}             查询任务状态
//	POST /v1/videos/{id}/cancel      取消排队中的任务
//
// 原 `POST /v1/videos`（Sora 风格创建端点）已移除——原生Canvas上游不再接入，
// 单一创建路径避免同一资源两个入口。视频 MP4 URL 在状态响应的 result.data[0].url，
// 无 /content 端点。
func registerVideoRoutes(
	gateway *gin.RouterGroup,
	r *gin.Engine,
	h *handler.Handlers,
	bodyLimit gin.HandlerFunc,
	clientRequestID gin.HandlerFunc,
	opsErrorLogger gin.HandlerFunc,
	endpointNorm gin.HandlerFunc,
	apiKeyAuth gin.HandlerFunc,
	compositeTarget gin.HandlerFunc,
	requireGroup gin.HandlerFunc,
) {
	videoUnsupported := func(c *gin.Context) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"type":    "not_found_error",
				"message": "Videos API is not supported for this platform",
			},
		})
	}

	// videoGenerationHandler 处理 POST /v1/videos/generations：按平台分流。
	// canvas（AIV2API）与 Grok 共用此路径，语义各自实现。
	videoGenerationHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok:
			h.OpenAIGateway.GrokVideoGeneration(c)
		case service.PlatformCanvas:
			h.OpenAIGateway.CanvasVideoCreation(c)
		default:
			videoUnsupported(c)
		}
	}

	// videoEditHandler 处理 POST /v1/videos/edits（目前仅 Grok）。
	videoEditHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoEdit(c)
			return
		}
		videoUnsupported(c)
	}

	// videoExtensionHandler 处理 POST /v1/videos/extensions（目前仅 Grok）。
	videoExtensionHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoExtension(c)
			return
		}
		videoUnsupported(c)
	}

	// videoStatusHandler 处理 GET /v1/videos/{id}：按平台分流。
	// composite 分组走 Grok：状态查询请求不带 model，compositeTargetPlatformMiddleware
	// 无法解析出目标平台，交给调度器/选号阶段去校验容量（与上游一致）。
	videoStatusHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok, service.PlatformComposite:
			h.OpenAIGateway.GrokVideoStatus(c)
		case service.PlatformCanvas:
			h.OpenAIGateway.CanvasVideoStatus(c)
		default:
			videoUnsupported(c)
		}
	}

	// videoCancelHandler 处理 POST /v1/videos/{id}/cancel（目前仅 AIV2API 支持）。
	videoCancelHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasVideoCancel(c)
			return
		}
		videoUnsupported(c)
	}

	// videoContentHandler 处理 GET /v1/videos/{id}/content 视频下载（目前仅 Grok）。
	// AIV2API 无此端点——MP4 URL 直接在状态响应的 result.data[0].url。
	videoContentHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok, service.PlatformComposite:
			h.OpenAIGateway.GrokVideoContent(c)
		default:
			videoUnsupported(c)
		}
	}

	// /v1 分组路由（middleware 已由 gateway RouterGroup 统一应用）
	gateway.POST("/videos/generations", videoGenerationHandler)
	gateway.POST("/videos/edits", videoEditHandler)
	gateway.POST("/videos/extensions", videoExtensionHandler)
	gateway.GET("/videos/:request_id", videoStatusHandler)
	gateway.POST("/videos/:request_id/cancel", videoCancelHandler)
	gateway.GET("/videos/:request_id/content", videoContentHandler)

	// 根路径别名（不带 /v1 前缀，需显式挂中间件）
	r.POST("/videos/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoGenerationHandler)
	r.POST("/videos/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoEditHandler)
	r.POST("/videos/extensions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoExtensionHandler)
	r.GET("/videos/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoStatusHandler)
	r.POST("/videos/:request_id/cancel", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoCancelHandler)
	// /content 根别名保留：Grok/composite 仍需此端点；AIV2API 的 videoUnsupported 响应
	// 使客户端得到清晰错误而非 404，不会静默失败。
	r.GET("/videos/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoContentHandler)

	// seedanceTasksHandler 处理 POST /v1/contents/generations/tasks（Seedance/Ark Plan v3 原生接口）。
	// 部分客户端对含 "seedance" 的模型走此路径；body 在 handler 层自动转换成
	// AIV2API 风格后沿同一通道转发。仅限 canvas 平台。
	seedanceTasksHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasVideoCreation(c)
			return
		}
		videoUnsupported(c)
	}

	// seedanceTaskStatusHandler 处理 GET /v1/contents/generations/tasks/{id}（Seedance 状态查询）。
	// 复用状态处理器；normalizeCanvasVideoResponse 已补入 content.video_url。
	seedanceTaskStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasVideoStatus(c)
			return
		}
		videoUnsupported(c)
	}

	// Seedance/Ark Plan v3 兼容路由
	gateway.POST("/contents/generations/tasks", seedanceTasksHandler)
	gateway.GET("/contents/generations/tasks/:request_id", seedanceTaskStatusHandler)
	// 根路径别名
	r.POST("/contents/generations/tasks", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, seedanceTasksHandler)
	r.GET("/contents/generations/tasks/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, seedanceTaskStatusHandler)
}
