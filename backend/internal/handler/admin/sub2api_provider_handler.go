package admin

import (
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Sub2APIProviderHandler 处理 Provider 管理的 HTTP 请求
type Sub2APIProviderHandler struct {
	providerService *service.Sub2APIProviderService
}

// NewSub2APIProviderHandler 创建 Handler 实例
func NewSub2APIProviderHandler(providerService *service.Sub2APIProviderService) *Sub2APIProviderHandler {
	return &Sub2APIProviderHandler{
		providerService: providerService,
	}
}

// CreateProviderRequest 创建 Provider 请求
type CreateProviderRequest struct {
	Name         string  `json:"name" binding:"required"`
	BaseURL      string  `json:"base_url" binding:"required,url"`
	ProviderType string  `json:"provider_type" binding:"omitempty,oneof=sub2api"`
	LoginMethod  string  `json:"login_method" binding:"omitempty,oneof=http browser"`
	Email        string  `json:"email" binding:"required,email"`
	Password     string  `json:"password" binding:"required"`
	Notes        *string `json:"notes"`
}

// UpdateProviderRequest 更新 Provider 请求
type UpdateProviderRequest struct {
	Name        *string `json:"name"`
	BaseURL     *string `json:"base_url" binding:"omitempty,url"`
	LoginMethod *string `json:"login_method" binding:"omitempty,oneof=http browser"`
	Email       *string `json:"email" binding:"omitempty,email"`
	Password    *string `json:"password"`
	Status      *string `json:"status" binding:"omitempty,oneof=active inactive"`
	Notes       *string `json:"notes"`
}

// Create 创建 Provider
// POST /api/v1/admin/sub2api-providers
func (h *Sub2APIProviderHandler) Create(c *gin.Context) {
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	provider, err := h.providerService.CreateProvider(c.Request.Context(), &service.CreateProviderInput{
		Name:         req.Name,
		BaseURL:      req.BaseURL,
		ProviderType: req.ProviderType,
		LoginMethod:  req.LoginMethod,
		Email:        req.Email,
		Password:     req.Password,
		Notes:        req.Notes,
	})

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProviderFromService(provider))
}

// List 列出所有 Provider（分页）
// GET /api/v1/admin/sub2api-providers
func (h *Sub2APIProviderHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := c.Query("status")
	search := c.Query("search")

	providers, total, err := h.providerService.ListProviders(
		c.Request.Context(),
		page,
		pageSize,
		status,
		search,
	)

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := make([]dto.Provider, len(providers))
	for i, p := range providers {
		result[i] = *dto.ProviderFromService(p)
	}

	response.Paginated(c, result, int64(total), page, pageSize)
}

// GetAll 获取所有 Provider（不分页）
// GET /api/v1/admin/sub2api-providers/all
func (h *Sub2APIProviderHandler) GetAll(c *gin.Context) {
	status := c.DefaultQuery("status", "active")

	providers, err := h.providerService.ListAllProviders(
		c.Request.Context(),
		status,
	)

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	result := make([]dto.Provider, len(providers))
	for i, p := range providers {
		result[i] = *dto.ProviderFromService(p)
	}

	response.Success(c, result)
}

// GetByID 根据 ID 获取 Provider
// GET /api/v1/admin/sub2api-providers/:id
func (h *Sub2APIProviderHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	provider, err := h.providerService.GetProviderWithAccounts(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{
		"provider":       dto.ProviderFromService(provider.Provider),
		"accounts_count": provider.AccountsCount,
	})
}

// Update 更新 Provider
// PUT /api/v1/admin/sub2api-providers/:id
func (h *Sub2APIProviderHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	provider, err := h.providerService.UpdateProvider(c.Request.Context(), id, &service.UpdateProviderInput{
		Name:        req.Name,
		BaseURL:     req.BaseURL,
		LoginMethod: req.LoginMethod,
		Email:       req.Email,
		Password:    req.Password,
		Status:      req.Status,
		Notes:       req.Notes,
	})

	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, dto.ProviderFromService(provider))
}

// Delete 删除 Provider
// DELETE /api/v1/admin/sub2api-providers/:id
func (h *Sub2APIProviderHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	if err := h.providerService.DeleteProvider(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Provider deleted successfully"})
}

// DetectPaths 探测并更新 API 路径
func (h *Sub2APIProviderHandler) DetectPaths(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	result, err := h.providerService.DetectAndUpdateAPIPaths(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// TestConnection 测试 Provider 连接
func (h *Sub2APIProviderHandler) TestConnection(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	if err := h.providerService.TestProviderConnection(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Connection test successful"})
}

// GetLinkedAccounts 获取 Provider 下所有关联账号
// GET /api/v1/admin/sub2api-providers/:id/accounts
func (h *Sub2APIProviderHandler) GetLinkedAccounts(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	accounts, err := h.providerService.GetLinkedAccounts(c.Request.Context(), id, c.Query("sync") == "true")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, accounts)
}

// LinkAccountRequest 关联 Account 请求
type LinkAccountRequest struct {
	AccountID int64 `json:"account_id" binding:"required"`
}

// LinkAccount 将 Account 关联到 Provider
// POST /api/v1/admin/sub2api-providers/:id/link-account
func (h *Sub2APIProviderHandler) LinkAccount(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	var req LinkAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	result, err := h.providerService.LinkAccount(c.Request.Context(), id, req.AccountID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, result)
}

// UnlinkAccount 解除 Account 与 Provider 的关联
// DELETE /api/v1/admin/sub2api-providers/:id/accounts/:account_id
func (h *Sub2APIProviderHandler) UnlinkAccount(c *gin.Context) {
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

	if err := h.providerService.UnlinkAccount(c.Request.Context(), id, accountID); err != nil {
		response.ErrorFrom(c, err)
		return
	}

	response.Success(c, gin.H{"message": "Account unlinked successfully"})
}

// OptimizeAccount 与 OptimizeAll 已迁移到 Sub2APIOptimizeScheduleHandler，
// 与定时任务共用同一智能引擎（倍率上限 + 连通测试 + 回滚）。此处不再提供。
