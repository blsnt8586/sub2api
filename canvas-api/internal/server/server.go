package server

import (
	"github.com/gin-gonic/gin"

	"github.com/blsnt8586/canvas-api/internal/auth"
	"github.com/blsnt8586/canvas-api/internal/config"
	"github.com/blsnt8586/canvas-api/internal/handler"
	"github.com/blsnt8586/canvas-api/internal/repository"
)

// New 装配 Gin 引擎：Recovery + CORS + 鉴权中间件 + 路由。
// s3 传 nil 时 media 上传/presign 接口返回 503，其余接口正常。
func New(cfg *config.Config, db *repository.DB, s3 handler.ObjectStore) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(corsMiddleware(cfg.CORSAllowedOrigins))

	// 存活探针无需鉴权。
	r.GET("/health", handler.Health)

	// /canvas/* 全部经 JWT 鉴权（共享 sub2api 的 secret）。
	// 使用 /canvas 前缀避免与 sub2api 的 /api/v1/* 在 infinite-canvas Vite proxy 里产生冲突。
	api := r.Group("/canvas")
	api.Use(auth.Middleware(cfg.JWTSecret, db))
	{
		api.GET("/me", handler.Me)

		projects := handler.NewProjects(db.Projects())
		api.GET("/projects", projects.List)
		api.POST("/projects", projects.Save)
		api.GET("/projects/:id", projects.Get)
		api.PUT("/projects/:id", projects.Save)
		api.DELETE("/projects/:id", projects.Delete)

		media := handler.NewMedia(db.Media(), s3)
		api.POST("/media/upload", media.Upload)
		api.GET("/media", media.List)
		api.GET("/media/:key/url", media.ResolveURL)
		api.POST("/media/:key/pin", media.Pin)
		api.DELETE("/media/:key", media.Delete)

		assets := handler.NewAssets(db.Assets())
		api.GET("/assets", assets.List)
		api.POST("/assets", assets.Save)
		api.PUT("/assets/:id", assets.Save)
		api.DELETE("/assets/:id", assets.Delete)

		cfg := handler.NewConfig(db.Config())
		api.GET("/config", cfg.Get)
		api.PUT("/config", cfg.Save)
	}

	return r
}
