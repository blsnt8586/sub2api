package admin

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Sub2APIProviderHandler 处理 Provider 管理的 HTTP 请求
type Sub2APIProviderHandler struct {
	providerService *service.Sub2APIProviderService
	probeService    *service.Sub2APIProviderProbeService
}

// NewSub2APIProviderHandler 创建 Handler 实例
func NewSub2APIProviderHandler(providerService *service.Sub2APIProviderService, probeService *service.Sub2APIProviderProbeService) *Sub2APIProviderHandler {
	return &Sub2APIProviderHandler{
		providerService: providerService,
		probeService:    probeService,
	}
}

// CreateProviderRequest 创建 Provider 请求
type CreateProviderRequest struct {
	Name         string  `json:"name" binding:"required"`
	BaseURL      string  `json:"base_url" binding:"required,url"`
	ProviderType string  `json:"provider_type" binding:"omitempty,oneof=sub2api"`
	Email        string  `json:"email" binding:"required,email"`
	Password     string  `json:"password"`
	AuthMode     string  `json:"auth_mode" binding:"omitempty,oneof=password token_pair"`
	AccessToken  *string `json:"access_token"`
	RefreshToken *string `json:"refresh_token"`
	Notes        *string `json:"notes"`
	ProxyID      *int64  `json:"proxy_id"`
}

// optionalProviderNotes distinguishes an omitted notes field (keep the current
// value) from null or an empty string (clear the current value).
type optionalProviderNotes struct {
	Set   bool
	Value *string
}

func (f *optionalProviderNotes) UnmarshalJSON(data []byte) error {
	f.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		empty := ""
		f.Value = &empty
		return nil
	}

	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	f.Value = &value
	return nil
}

// optionalProviderProxyID distinguishes an omitted field (keep current route)
// from null (switch to direct) and a numeric ID (switch to that proxy).
type optionalProviderProxyID struct {
	Set   bool
	Value *int64
}

func (f *optionalProviderProxyID) UnmarshalJSON(data []byte) error {
	f.Set = true
	if strings.TrimSpace(string(data)) == "null" {
		f.Value = nil
		return nil
	}
	var value int64
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	f.Value = &value
	return nil
}

// UpdateProviderRequest 更新 Provider 请求
type UpdateProviderRequest struct {
	Name         *string                 `json:"name"`
	BaseURL      *string                 `json:"base_url" binding:"omitempty,url"`
	Email        *string                 `json:"email" binding:"omitempty,email"`
	Password     *string                 `json:"password"`
	AuthMode     *string                 `json:"auth_mode" binding:"omitempty,oneof=password token_pair"`
	AccessToken  *string                 `json:"access_token"`
	RefreshToken *string                 `json:"refresh_token"`
	Status       *string                 `json:"status" binding:"omitempty,oneof=active inactive"`
	Notes        optionalProviderNotes   `json:"notes"`
	ProxyID      optionalProviderProxyID `json:"proxy_id"`
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
		Email:        req.Email,
		Password:     req.Password,
		AuthMode:     req.AuthMode,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		Notes:        req.Notes,
		ProxyID:      req.ProxyID,
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
		Name:         req.Name,
		BaseURL:      req.BaseURL,
		Email:        req.Email,
		Password:     req.Password,
		AuthMode:     req.AuthMode,
		AccessToken:  req.AccessToken,
		RefreshToken: req.RefreshToken,
		Status:       req.Status,
		Notes:        req.Notes.Value,
		ProxyID: service.OptionalProviderProxyID{
			Set:   req.ProxyID.Set,
			Value: req.ProxyID.Value,
		},
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

// RemoteOverview returns a live snapshot of the remote Provider account's
// wallet balance and visible group multipliers.
// GET /api/v1/admin/sub2api-providers/:id/remote-overview
func (h *Sub2APIProviderHandler) RemoteOverview(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid provider ID")
		return
	}

	overview, err := h.providerService.GetRemoteOverview(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overview)
}

// CachedRemoteOverviews returns only Redis-backed latest asset states and does
// not contact any upstream Provider.
func (h *Sub2APIProviderHandler) CachedRemoteOverviews(c *gin.Context) {
	ids, ok := parseProviderHealthOverviewIDs(c.Query("ids"))
	if !ok {
		response.BadRequest(c, "ids must contain at most 100 unique positive integers")
		return
	}
	overviews, err := h.providerService.GetCachedRemoteOverviews(c.Request.Context(), ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overviews)
}

// HealthOverview returns a 24-hour control-plane timeline for the visible cards.
func (h *Sub2APIProviderHandler) HealthOverview(c *gin.Context) {
	ids, ok := parseProviderHealthOverviewIDs(c.Query("ids"))
	if !ok {
		response.BadRequest(c, "ids must contain at most 100 unique positive integers")
		return
	}
	overviews, err := h.probeService.HealthOverview(c.Request.Context(), ids)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, overviews)
}

func parseProviderHealthOverviewIDs(raw string) ([]int64, bool) {
	if strings.TrimSpace(raw) == "" {
		return []int64{}, true
	}
	parts := strings.Split(raw, ",")
	ids := make([]int64, 0, len(parts))
	seen := make(map[int64]struct{}, len(parts))
	for _, part := range parts {
		id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
		if err != nil || id <= 0 {
			return nil, false
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
		if len(ids) > 100 {
			return nil, false
		}
	}
	return ids, true
}

// Health returns the latest passive/active health snapshot.
func (h *Sub2APIProviderHandler) Health(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	health, err := h.probeService.GetHealth(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, health)
}

func (h *Sub2APIProviderHandler) GetProbeConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	cfg, err := h.probeService.GetConfig(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *Sub2APIProviderHandler) UpdateProbeConfig(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	var req service.Sub2APIProviderProbeConfigInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.probeService.UpdateConfig(c.Request.Context(), id, &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *Sub2APIProviderHandler) RunProbe(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	health, err := h.probeService.RunNow(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, health)
}

func (h *Sub2APIProviderHandler) ProbeHistory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	limit, sinceSeconds, ok := parseProviderProbeHistoryQuery(c.Query("limit"), c.Query("since_seconds"))
	if !ok {
		response.BadRequest(c, "limit must be 1-2000 and since_seconds must be 60-86400")
		return
	}
	history, err := h.probeService.HistorySince(c.Request.Context(), id, time.Now().Add(-time.Duration(sinceSeconds)*time.Second), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

// ProbeTargets returns all independently configured business routes for a
// Provider. sync=true refreshes only the remote Key-to-group binding.
func (h *Sub2APIProviderHandler) ProbeTargets(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid provider ID")
		return
	}
	targets, err := h.probeService.ListTargets(c.Request.Context(), id, c.Query("sync") == "true")
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, targets)
}

func (h *Sub2APIProviderHandler) UpdateProbeTarget(c *gin.Context) {
	providerID, targetID, ok := parseProviderProbeTargetIDs(c)
	if !ok {
		return
	}
	var req service.Sub2APIProviderProbeTargetInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	target, err := h.probeService.UpdateTarget(c.Request.Context(), providerID, targetID, &req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, target)
}

func (h *Sub2APIProviderHandler) RunProbeTarget(c *gin.Context) {
	providerID, targetID, ok := parseProviderProbeTargetIDs(c)
	if !ok {
		return
	}
	target, err := h.probeService.RunTargetNow(c.Request.Context(), providerID, targetID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, target)
}

func (h *Sub2APIProviderHandler) ProbeTargetHistory(c *gin.Context) {
	providerID, targetID, ok := parseProviderProbeTargetIDs(c)
	if !ok {
		return
	}
	limit, sinceSeconds, valid := parseProviderProbeHistoryQuery(c.Query("limit"), c.Query("since_seconds"))
	if !valid {
		response.BadRequest(c, "limit must be 1-2000 and since_seconds must be 60-86400")
		return
	}
	history, err := h.probeService.TargetHistory(c.Request.Context(), providerID, targetID, time.Now().Add(-time.Duration(sinceSeconds)*time.Second), limit)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, history)
}

func parseProviderProbeTargetIDs(c *gin.Context) (providerID, targetID int64, ok bool) {
	providerID, providerErr := strconv.ParseInt(c.Param("id"), 10, 64)
	targetID, targetErr := strconv.ParseInt(c.Param("target_id"), 10, 64)
	if providerErr != nil || targetErr != nil || providerID <= 0 || targetID <= 0 {
		response.BadRequest(c, "Invalid provider or probe target ID")
		return 0, 0, false
	}
	return providerID, targetID, true
}

func parseProviderProbeHistoryQuery(limitRaw, sinceRaw string) (limit int, sinceSeconds int, ok bool) {
	limit, sinceSeconds = 100, 3600
	if limitRaw != "" {
		parsed, err := strconv.Atoi(limitRaw)
		if err != nil || parsed < 1 || parsed > 2000 {
			return 0, 0, false
		}
		limit = parsed
	}
	if sinceRaw != "" {
		parsed, err := strconv.Atoi(sinceRaw)
		if err != nil || parsed < 60 || parsed > 86400 {
			return 0, 0, false
		}
		sinceSeconds = parsed
	}
	return limit, sinceSeconds, true
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
