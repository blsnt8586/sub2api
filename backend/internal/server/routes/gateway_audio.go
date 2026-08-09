package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerAudioRoutes 注册音频生成相关路由（Canvas canvas 等）
func registerAudioRoutes(
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
	audioUnsupported := func(c *gin.Context) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Audio endpoint not available for this platform",
				"type":    "not_found_error",
				"code":    "audio_endpoint_not_found",
			},
		})
	}

	// audioGenerationHandler 处理 POST /v1/audio/generations
	audioGenerationHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAudioCreation(c)
			return
		}
		audioUnsupported(c)
	}

	// audioStatusHandler 处理 GET /v1/audio/{id}
	audioStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAudioStatus(c)
			return
		}
		audioUnsupported(c)
	}

	// audioCancelHandler 处理 POST /v1/audio/{id}/cancel
	audioCancelHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAudioCancel(c)
			return
		}
		audioUnsupported(c)
	}

	// 标准 OpenAI 兼容路由
	gateway.POST("/audio/generations", audioGenerationHandler)
	gateway.GET("/audio/:id", audioStatusHandler)
	gateway.POST("/audio/:id/cancel", audioCancelHandler)

	// 根路径别名（带完整中间件链）
	r.POST("/audio/generations", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, audioGenerationHandler)
	r.GET("/audio/:id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, audioStatusHandler)
	r.POST("/audio/:id/cancel", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, audioCancelHandler)
}
