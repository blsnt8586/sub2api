package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerVideoRoutes 注册视频生成相关路由（Grok、即梦 jimeng 等）。
//
// 路由骨架与上游 gateway.go 保持一致（细分的 /videos/{generations,edits,extensions}
// 及其 /:request_id、/:request_id/content 变体），仅在共享 handler 内按 platform 分流时
// 追加 jimeng 分支，并额外挂载 jimeng 专属的 cancel 与 Seedance 兼容路由。这样上游的
// Grok 视频行为完全保留，即梦作为增量平台叠加。[CUSTOM decoupling]
//
// 新增视频平台时仅需在此文件内的 handler 里追加 case，无需改 gateway.go。
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

	// videoGenerationHandler 处理 POST /v1/videos 与 /v1/videos/generations。
	// Grok 走生成接口；即梦 POST /v1/videos 为固定创建接口。[CUSTOM: jimeng 分支]
	videoGenerationHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok:
			h.OpenAIGateway.GrokVideoGeneration(c)
		case service.PlatformJimeng:
			h.OpenAIGateway.JimengVideoCreation(c)
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

	// videoStatusHandler 处理 GET /v1/videos/{id} 及细分变体：按平台分流。
	// composite 分组走 Grok：状态查询请求不带 model，compositeTargetPlatformMiddleware
	// 无法解析目标平台，交给调度器/选号阶段校验容量（与上游一致）。[CUSTOM: jimeng 分支]
	videoStatusHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok, service.PlatformComposite:
			h.OpenAIGateway.GrokVideoStatus(c)
		case service.PlatformJimeng:
			h.OpenAIGateway.JimengVideoStatus(c)
		default:
			videoUnsupported(c)
		}
	}

	// videoContentHandler 处理 GET /v1/videos/{id}/content 视频下载：按平台分流。
	// 与 videoStatusHandler 同口径（含 composite 走 Grok）——两者必须成对维护，
	// 否则某平台会「查得到状态但下不了片」。[CUSTOM: jimeng 分支]
	videoContentHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok, service.PlatformComposite:
			h.OpenAIGateway.GrokVideoContent(c)
		case service.PlatformJimeng:
			h.OpenAIGateway.JimengVideoContent(c)
		default:
			videoUnsupported(c)
		}
	}

	// videoCancelHandler 处理 POST /v1/videos/{id}/cancel（即梦 Leonardo vendor 专用）。[CUSTOM]
	videoCancelHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformJimeng {
			h.OpenAIGateway.JimengVideoCancel(c)
			return
		}
		videoUnsupported(c)
	}

	// seedanceTasksHandler 处理 POST /v1/contents/generations/tasks（Seedance/Ark Plan v3 原生接口）。
	// infinite-canvas 等客户端对含 "seedance" 的模型走此路径；body 在 handler 层自动转换成
	// AIV2API 风格后沿 jimeng 通道转发。仅限 jimeng 平台。[CUSTOM]
	seedanceTasksHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformJimeng {
			h.OpenAIGateway.JimengVideoCreation(c)
			return
		}
		videoUnsupported(c)
	}

	// seedanceTaskStatusHandler 处理 GET /v1/contents/generations/tasks/{id}（Seedance 状态查询）。
	// 复用 jimeng 状态处理器；normalizeJimengVideoResponse 已补入 content.video_url，
	// 使 infinite-canvas 的 Seedance 轮询路径能正确提取到视频 URL。[CUSTOM]
	seedanceTaskStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformJimeng {
			h.OpenAIGateway.JimengVideoStatus(c)
			return
		}
		videoUnsupported(c)
	}

	// /v1 分组路由（middleware 已由 gateway RouterGroup 统一应用）。
	// 路由集与上游 gateway.go 一致，尾部追加 jimeng cancel 与 Seedance 兼容路由。
	gateway.POST("/videos", videoGenerationHandler)
	gateway.POST("/videos/generations", videoGenerationHandler)
	gateway.POST("/videos/edits", videoEditHandler)
	gateway.POST("/videos/extensions", videoExtensionHandler)
	gateway.GET("/videos/generations/:request_id/content", videoContentHandler)
	gateway.GET("/videos/edits/:request_id/content", videoContentHandler)
	gateway.GET("/videos/extensions/:request_id/content", videoContentHandler)
	gateway.GET("/videos/generations/:request_id", videoStatusHandler)
	gateway.GET("/videos/edits/:request_id", videoStatusHandler)
	gateway.GET("/videos/extensions/:request_id", videoStatusHandler)
	gateway.GET("/videos/:request_id", videoStatusHandler)
	gateway.GET("/videos/:request_id/content", videoContentHandler)
	gateway.POST("/videos/:request_id/cancel", videoCancelHandler) // [CUSTOM] jimeng
	gateway.POST("/contents/generations/tasks", seedanceTasksHandler)
	gateway.GET("/contents/generations/tasks/:request_id", seedanceTaskStatusHandler)

	// 根路径别名（不带 /v1 前缀，需显式挂中间件）
	r.POST("/videos", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoGenerationHandler)
	r.POST("/videos/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoGenerationHandler)
	r.POST("/videos/edits", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoEditHandler)
	r.POST("/videos/extensions", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoExtensionHandler)
	r.GET("/videos/generations/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoContentHandler)
	r.GET("/videos/edits/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoContentHandler)
	r.GET("/videos/extensions/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoContentHandler)
	r.GET("/videos/generations/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoStatusHandler)
	r.GET("/videos/edits/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoStatusHandler)
	r.GET("/videos/extensions/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoStatusHandler)
	r.GET("/videos/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoStatusHandler)
	r.GET("/videos/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoContentHandler)
	r.POST("/videos/:request_id/cancel", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, videoCancelHandler) // [CUSTOM] jimeng
	r.POST("/contents/generations/tasks", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, seedanceTasksHandler)
	r.GET("/contents/generations/tasks/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, seedanceTaskStatusHandler)
}
