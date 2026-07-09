package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerVideoRoutes 注册视频生成相关路由（Grok、即梦 jimeng 等）。
// 新增视频平台时仅需在此文件中：
//  1. 在对应 handler 函数里追加 case
//  2. 如有专属路径，在本函数末尾添加路由
//
// 无需修改 gateway.go。
func registerVideoRoutes(
	gateway *gin.RouterGroup,
	r *gin.Engine,
	h *handler.Handlers,
	bodyLimit gin.HandlerFunc,
	clientRequestID gin.HandlerFunc,
	opsErrorLogger gin.HandlerFunc,
	endpointNorm gin.HandlerFunc,
	apiKeyAuth gin.HandlerFunc,
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

	// videoGenerationHandler 处理 POST /v1/videos/generations（目前仅 Grok）。
	videoGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformGrok {
			h.OpenAIGateway.GrokVideoGeneration(c)
			return
		}
		videoUnsupported(c)
	}

	// videoStatusHandler 处理 GET /v1/videos/{id}：按平台分流。
	videoStatusHandler := func(c *gin.Context) {
		switch getGroupPlatform(c) {
		case service.PlatformGrok:
			h.OpenAIGateway.GrokVideoStatus(c)
		case service.PlatformJimeng:
			h.OpenAIGateway.JimengVideoStatus(c)
		default:
			videoUnsupported(c)
		}
	}

	// jimengVideoCreateHandler 处理即梦的 POST /v1/videos（固定接口，非 /generations）。
	jimengVideoCreateHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformJimeng {
			h.OpenAIGateway.JimengVideoCreation(c)
			return
		}
		videoUnsupported(c)
	}

	// jimengVideoContentHandler 处理即梦的 GET /v1/videos/{id}/content 视频下载。
	jimengVideoContentHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformJimeng {
			h.OpenAIGateway.JimengVideoContent(c)
			return
		}
		videoUnsupported(c)
	}

	// /v1 分组路由（middleware 已由 gateway RouterGroup 统一应用）
	gateway.POST("/videos/generations", videoGenerationHandler)
	// 即梦固定接口：POST /v1/videos（创建）与 /v1/videos/{id}/content（下载）
	gateway.POST("/videos", jimengVideoCreateHandler)
	gateway.GET("/videos/:request_id", videoStatusHandler)
	gateway.GET("/videos/:request_id/content", jimengVideoContentHandler)

	// 根路径别名（不带 /v1 前缀，需显式挂中间件）
	r.POST("/videos/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, requireGroup, videoGenerationHandler)
	r.POST("/videos", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, requireGroup, jimengVideoCreateHandler)
	r.GET("/videos/:request_id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, requireGroup, videoStatusHandler)
	r.GET("/videos/:request_id/content", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, requireGroup, jimengVideoContentHandler)
}
