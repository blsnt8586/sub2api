package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/blsnt8586/canvas-api/internal/auth"
)

// Health 是无需鉴权的存活探针。
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "canvas-api"})
}

// Me 回显当前鉴权用户，用于验证 JWT 链路是否打通（Phase 1 的验收端点）。
func Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"user_id": auth.UserID(c),
		"email":   auth.Email(c),
	})
}
