package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type ctxKey string

const (
	ctxKeyUserID ctxKey = "canvas_user_id"
	ctxKeyEmail  ctxKey = "canvas_user_email"
	ctxKeyRole   ctxKey = "canvas_user_role"
)

// AuthUser 是从 users 表读到的、用于校验 token 吊销的最小字段集。
type AuthUser struct {
	ID           int64
	Email        string
	PasswordHash string
	Status       string
}

// IsActive 报告用户是否处于可用状态。
func (u AuthUser) IsActive() bool { return u.Status == "active" }

// UserAuthReader 只读 users 表。canvas-api 从不写 users 表（解耦：users 归 sub2api 所有）。
type UserAuthReader interface {
	GetAuthUser(ctx context.Context, id int64) (*AuthUser, error)
}

// Middleware 返回 Gin 鉴权中间件：验证 sub2api 用同一 secret（HS256）签发的 access token，
// 再通过重算 token_version 指纹确认该 token 未被密码变更吊销。
func Middleware(secret string, reader UserAuthReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			abort(c, http.StatusUnauthorized, "UNAUTHORIZED", "Authorization header is required")
			return
		}
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			abort(c, http.StatusUnauthorized, "INVALID_AUTH_HEADER", "Authorization header must be 'Bearer {token}'")
			return
		}
		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			abort(c, http.StatusUnauthorized, "EMPTY_TOKEN", "Token cannot be empty")
			return
		}

		claims, err := ParseToken(tokenString, secret)
		if err != nil {
			if errors.Is(err, ErrTokenExpired) {
				abort(c, http.StatusUnauthorized, "TOKEN_EXPIRED", "Token has expired")
				return
			}
			abort(c, http.StatusUnauthorized, "INVALID_TOKEN", "Invalid token")
			return
		}

		user, err := reader.GetAuthUser(c.Request.Context(), claims.UserID)
		if err != nil || user == nil {
			abort(c, http.StatusUnauthorized, "USER_NOT_FOUND", "User not found")
			return
		}
		if !user.IsActive() {
			abort(c, http.StatusUnauthorized, "USER_INACTIVE", "User account is not active")
			return
		}
		// 重算指纹校验吊销：改密码后 password_hash 变 → 指纹变 → 旧 token 失效。
		if claims.TokenVersion != ComputeTokenVersion(user.Email, user.PasswordHash) {
			abort(c, http.StatusUnauthorized, "TOKEN_REVOKED", "Token has been revoked")
			return
		}

		c.Set(string(ctxKeyUserID), user.ID)
		c.Set(string(ctxKeyEmail), user.Email)
		c.Set(string(ctxKeyRole), claims.Role)
		c.Next()
	}
}

// UserID 从 gin.Context 取出鉴权用户 ID；未鉴权返回 0。
func UserID(c *gin.Context) int64 {
	if v, ok := c.Get(string(ctxKeyUserID)); ok {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

// Email 从 gin.Context 取出鉴权用户邮箱。
func Email(c *gin.Context) string {
	if v, ok := c.Get(string(ctxKeyEmail)); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func abort(c *gin.Context, status int, code, msg string) {
	c.AbortWithStatusJSON(status, gin.H{"code": code, "message": msg})
}
