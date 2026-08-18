package admin

import (
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Sub2APIOptimizeScheduleHandler 处理上游定时优化配置的 HTTP 请求
type Sub2APIOptimizeScheduleHandler struct {
	scheduleService *service.Sub2APIOptimizeScheduleService
}

// NewSub2APIOptimizeScheduleHandler 创建 Handler 实例
func NewSub2APIOptimizeScheduleHandler(scheduleService *service.Sub2APIOptimizeScheduleService) *Sub2APIOptimizeScheduleHandler {
	return &Sub2APIOptimizeScheduleHandler{scheduleService: scheduleService}
}

// Get 获取指定 Provider 的定时优化配置（含最近日志）
// GET /api/v1/admin/sub2api-providers/:id/optimize-schedule
func (h *Sub2APIOptimizeScheduleHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	info, err := h.scheduleService.GetByProviderID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	// info 为 nil 表示尚未配置，返回 null
	response.Success(c, info)
}

// ListLogs returns paginated Provider-owned optimization audit history.
// GET /api/v1/admin/sub2api-providers/:id/optimize-logs
func (h *Sub2APIOptimizeScheduleHandler) ListLogs(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || providerID <= 0 {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	filter := service.OptimizeLogFilter{
		Trigger:  strings.TrimSpace(c.Query("trigger")),
		Status:   strings.TrimSpace(c.Query("status")),
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Page:     1,
		PageSize: 20,
	}
	if value := strings.TrimSpace(c.Query("page")); value != "" {
		filter.Page, err = strconv.Atoi(value)
		if err != nil || filter.Page < 1 {
			response.BadRequest(c, "Invalid page")
			return
		}
	}
	if value := strings.TrimSpace(c.Query("page_size")); value != "" {
		filter.PageSize, err = strconv.Atoi(value)
		if err != nil || filter.PageSize < 1 {
			response.BadRequest(c, "Invalid page_size")
			return
		}
		if filter.PageSize > 100 {
			filter.PageSize = 100
		}
	}
	if value := strings.TrimSpace(c.Query("account_id")); value != "" {
		accountID, parseErr := strconv.ParseInt(value, 10, 64)
		if parseErr != nil || accountID <= 0 {
			response.BadRequest(c, "Invalid account_id")
			return
		}
		filter.AccountID = &accountID
	}
	if value := strings.TrimSpace(c.Query("from")); value != "" {
		from, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			response.BadRequest(c, "Invalid from time; expected RFC3339")
			return
		}
		filter.From = &from
	}
	if value := strings.TrimSpace(c.Query("to")); value != "" {
		to, parseErr := time.Parse(time.RFC3339, value)
		if parseErr != nil {
			response.BadRequest(c, "Invalid to time; expected RFC3339")
			return
		}
		filter.To = &to
	}

	items, total, err := h.scheduleService.ListLogs(c.Request.Context(), providerID, filter)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, total, filter.Page, filter.PageSize)
}

// UpsertScheduleRequest 创建/更新定时配置请求
type UpsertScheduleRequest struct {
	CronExpr string `json:"cron_expr" binding:"required"`
	Enabled  bool   `json:"enabled"`
}

// Upsert 创建或更新定时优化配置
// PUT /api/v1/admin/sub2api-providers/:id/optimize-schedule
func (h *Sub2APIOptimizeScheduleHandler) Upsert(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	var req UpsertScheduleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	info, err := h.scheduleService.Upsert(c.Request.Context(), id, req.CronExpr, req.Enabled)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, info)
}

// Delete 删除定时优化配置
// DELETE /api/v1/admin/sub2api-providers/:id/optimize-schedule
func (h *Sub2APIOptimizeScheduleHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	if err := h.scheduleService.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Schedule deleted successfully"})
}

// RunNow 立即手动触发一次优化
// POST /api/v1/admin/sub2api-providers/:id/optimize-schedule/run
func (h *Sub2APIOptimizeScheduleHandler) RunNow(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	info, err := h.scheduleService.RunNow(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, info)
}

// OptimizeAccount 手动优化单个账号（走与定时任务一致的智能引擎：倍率上限 + 连通测试 + 回滚）。
// POST /api/v1/admin/sub2api-providers/:id/accounts/:account_id/optimize
func (h *Sub2APIOptimizeScheduleHandler) OptimizeAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	detail, err := h.scheduleService.OptimizeAccountManually(c.Request.Context(), id, accountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, detail)
}

// OptimizeAll 手动批量优化该上游下所有满足前置条件的账号（同一智能引擎）。
// POST /api/v1/admin/sub2api-providers/:id/optimize-all
func (h *Sub2APIOptimizeScheduleHandler) OptimizeAll(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	details, err := h.scheduleService.OptimizeAllManually(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	optimized, skipped, failed := 0, 0, 0
	for _, d := range details {
		switch d.Status {
		case "optimized":
			optimized++
		case "skipped":
			skipped++
		default:
			failed++
		}
	}
	response.Success(c, gin.H{
		"results":   details,
		"total":     len(details),
		"optimized": optimized,
		"skipped":   skipped,
		"failed":    failed,
	})
}

// UpdateAccountSettingsRequest 更新账号定时优化设置请求。
// 全量覆盖语义：enabled 独立控制是否参与，倍率上限/测试模型即使 enabled=false 也照常保留。
type UpdateAccountSettingsRequest struct {
	Enabled       *bool    `json:"enabled"`
	MinMultiplier *float64 `json:"min_multiplier"`
	MaxMultiplier *float64 `json:"max_multiplier"`
	TestModel     *string  `json:"test_model"`
}

// UpdateAccountSettings 更新关联账号的定时优化设置（倍率上限、测试模型）
// PUT /api/v1/admin/sub2api-providers/:id/accounts/:account_id/optimize-settings
func (h *Sub2APIOptimizeScheduleHandler) UpdateAccountSettings(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	accountID, err := strconv.ParseInt(c.Param("account_id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid account ID")
		return
	}

	var req UpdateAccountSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	enabled := req.Enabled != nil && *req.Enabled
	if err := h.scheduleService.UpdateAccountOptimizeSettings(c.Request.Context(), providerID, accountID, enabled, req.MinMultiplier, req.MaxMultiplier, req.TestModel); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Account optimize settings updated"})
}
