package middleware

import (
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

func ProvideAPIKeyAuthMiddleware(
	apiKeyService *service.APIKeyService,
	subscriptionService *service.SubscriptionService,
	smartGroupService *service.APIKeySmartGroupService,
	cfg *config.Config,
) APIKeyAuthMiddleware {
	return APIKeyAuthMiddleware(apiKeyAuthWithSubscriptionAndSmartGroups(apiKeyService, subscriptionService, smartGroupService, cfg))
}

// JWTAuthMiddleware JWT 认证中间件类型
type JWTAuthMiddleware gin.HandlerFunc

// OptionalJWTAuthMiddleware 可选 JWT 认证中间件类型：匿名放行，带 token 严格校验
type OptionalJWTAuthMiddleware gin.HandlerFunc

// AdminAuthMiddleware 管理员认证中间件类型
type AdminAuthMiddleware gin.HandlerFunc

// APIKeyAuthMiddleware API Key 认证中间件类型
type APIKeyAuthMiddleware gin.HandlerFunc

// ProviderSet 中间件层的依赖注入
var ProviderSet = wire.NewSet(
	NewJWTAuthMiddleware,
	NewOptionalJWTAuthMiddleware,
	NewAdminAuthMiddleware,
	ProvideAPIKeyAuthMiddleware,
	NewAuditLogMiddleware,
	NewStepUpAuthMiddleware,
)
