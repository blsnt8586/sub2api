package routes

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// registerTaskRoutes 注册 AIV2API 1.x 统一任务兼容路由（限 platform=canvas）。
//
// 端点清单：
//
//	POST /v1/tasks/images       创建异步图像任务（文生图，AsyncImageRequest，无参考图字段）
//	GET  /v1/tasks/:id          存量图像任务查询别名
//	POST /v1/tasks/:id/cancel   存量图像任务取消别名
//
// AIV2API 2.0 已把图像任务迁移到 /v1/images/*；旧 Canvas 图片 worker 仍可能
// 持有 /tasks/* 请求，因此保留别名。handler 内部会转发到新图像上游路径。
// 视频和音频已经分别使用 /v1/videos/*、/v1/audio/*，不能经该别名查询。
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

	// tasksStatusHandler 处理 GET /v1/tasks/:id（仅存量图像任务兼容）
	tasksStatusHandler := func(c *gin.Context) {
		if getGroupPlatform(c) == service.PlatformCanvas {
			h.OpenAIGateway.CanvasAsyncImageStatus(c)
			return
		}
		tasksUnsupported(c)
	}

	// tasksCancelHandler 处理 POST /v1/tasks/:id/cancel（仅存量图像任务兼容）
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
