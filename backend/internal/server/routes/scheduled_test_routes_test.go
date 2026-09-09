//go:build unit

package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestScheduledTestRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	noop := func(c *gin.Context) { c.Next() }
	RegisterAdminRoutes(
		router.Group("/api/v1"),
		&handler.Handlers{Admin: &handler.AdminHandlers{}},
		middleware.AdminAuthMiddleware(noop),
		middleware.AuditLogMiddleware(noop),
		middleware.StepUpAuthMiddleware(noop),
		nil,
		nil,
	)

	registered := make(map[string]bool)
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}
	for _, route := range []string{
		"GET /api/v1/admin/accounts/:id/scheduled-test-plans",
		"POST /api/v1/admin/scheduled-test-plans",
		"PUT /api/v1/admin/scheduled-test-plans/:id",
		"DELETE /api/v1/admin/scheduled-test-plans/:id",
		"GET /api/v1/admin/scheduled-test-plans/:id/results",
	} {
		require.Truef(t, registered[route], "missing scheduled-test route %s", route)
	}
}
