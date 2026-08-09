package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerTaskRoutes 注册 AIV2API 统一任务路由（限 platform=canvas）。
//
// 端点清单：
//   POST /v1/tasks/images       创建异步图像任务（文生图，AsyncImageRequest，无参考图字段）
//   GET  /v1/tasks/:id          统一任务查询（kind=image|video|audio）
//   POST /v1/tasks/:id/cancel   统一任务取消
//
// 历史背景：canvas-api task_poller 原先打 GET {baseURL}/v1/tasks/{id}，
// 但 sub2api 没有此路由，导致轮询恒 404、任务 25~50 分钟后判定 poll_timeout。
// 本文件注册这三条路由，同时修复该缺陷。
//
// 其他平台（openai/grok/gemini 等）走 tasksUnsupported 返回 404，
// 不干扰既有的 /images/tasks/:task_id（上游 AsyncImage，限 OpenAI/Grok）。
func registerTaskRoutes(
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
	tasksUnsupported := func(c *gin.Context) {
		service.MarkOpsClientBusinessLimited(c, service.OpsClientBusinessLimitedReasonLocalFeatureGate)
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"message": "Tasks endpoint not available for this platform",
				"type":    "not_found_error",
				"code":    "tasks_endpoint_not_found",
			},
		})
	}

	// tasksImageCreateHandler 处理 POST /v1/tasks/images（创建异步图像任务）
	tasksImageCreateHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAsyncImageCreation(c)
			return
		}
		tasksUnsupported(c)
	}

	// tasksStatusHandler 处理 GET /v1/tasks/:id（统一任务查询，含 video / audio）
	tasksStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAsyncImageStatus(c)
			return
		}
		tasksUnsupported(c)
	}

	// tasksCancelHandler 处理 POST /v1/tasks/:id/cancel（统一任务取消）
	tasksCancelHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAsyncImageCancel(c)
			return
		}
		tasksUnsupported(c)
	}

	// /v1 前缀路由（标准 OpenAI 兼容路径，经 gateway RouterGroup）
	gateway.POST("/tasks/images", tasksImageCreateHandler)
	gateway.GET("/tasks/:id", tasksStatusHandler)
	gateway.POST("/tasks/:id/cancel", tasksCancelHandler)

	// 根路径别名（带完整中间件链）
	r.POST("/tasks/images", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, tasksImageCreateHandler)
	r.GET("/tasks/:id", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, tasksStatusHandler)
	r.POST("/tasks/:id/cancel", bodyLimit, clientRequestID, opsErrorLogger, endpointNorm, apiKeyAuth, compositeTarget, requireGroup, tasksCancelHandler)
}
