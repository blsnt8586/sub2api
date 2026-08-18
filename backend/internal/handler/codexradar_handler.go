package handler

// 本文件为二开新增：Codex 雷达（第三方 codexradar.com 数据）代理接口。
// 用户与管理员共用，挂在已认证路由组下。数据来源为第三方社区站点，本平台仅做
// 代理缓存 + 署名转载，功能默认关闭（opt-in），由 codex_radar_enabled 开关控制。
// 与上游解耦——独立 handler + 独立 service，上游文件无 hook。详见 CUSTOM-CHANGES.md。

import (
	"encoding/json"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// CodexRadarHandler 处理 Codex 雷达图片与摘要的代理读取。
type CodexRadarHandler struct {
	radarService   *service.CodexRadarService
	settingService *service.SettingService
}

// NewCodexRadarHandler 创建 Codex 雷达 handler（wire 注入）。
func NewCodexRadarHandler(
	radarService *service.CodexRadarService,
	settingService *service.SettingService,
) *CodexRadarHandler {
	return &CodexRadarHandler{
		radarService:   radarService,
		settingService: settingService,
	}
}

// enabled 返回功能开关是否启用。默认关闭（opt-in，第三方数据来源需显式启用）。
func (h *CodexRadarHandler) enabled(c *gin.Context) bool {
	if h.settingService == nil {
		return false
	}
	return h.settingService.IsCodexRadarEnabled(c.Request.Context())
}

// codexRadarSummaryResponse 是摘要接口的响应体：第三方原始 JSON + 本平台附加的
// 来源/署名元信息（前端据此渲染「非本站提供」免责说明与跳转链接）。
type codexRadarSummaryResponse struct {
	Enabled                bool   `json:"enabled"`
	Available              bool   `json:"available"`
	Source                 string `json:"source"`      // 第三方来源站点首页
	Attribution            string `json:"attribution"` // 第三方数据署名
	FetchedAt              string `json:"fetched_at"`  // 本平台缓存时间（RFC3339），空=尚无数据
	RefreshIntervalSeconds int    `json:"refresh_interval_seconds"`
	Data                   any    `json:"data"`
}

// Summary 返回缓存的第三方状态摘要 + 来源元信息。
// GET /api/v1/codexradar/summary
func (h *CodexRadarHandler) Summary(c *gin.Context) {
	if !h.enabled(c) {
		response.Forbidden(c, "Codex radar feature is disabled")
		return
	}
	h.radarService.EnsureFresh(c.Request.Context())
	snap := h.radarService.SummarySnapshot()

	resp := codexRadarSummaryResponse{
		Enabled:                true,
		Available:              false,
		Source:                 service.CodexRadarSourceSite,
		Attribution:            service.CodexRadarAttribution,
		RefreshIntervalSeconds: 3600,
	}
	data := h.radarService.DataSnapshot()
	if data.Available {
		resp.Available = true
		resp.FetchedAt = data.FetchedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.Data = struct {
			Recommendations json.RawMessage `json:"recommendations"`
			Intelligence    json.RawMessage `json:"intelligence"`
		}{
			Recommendations: json.RawMessage(data.Recommendations),
			Intelligence:    json.RawMessage(data.Intelligence),
		}
	} else if snap.Available {
		resp.Available = true
		// 旧 current.json 快照兼容，便于滚动升级期间已有缓存继续可读。
		resp.FetchedAt = snap.FetchedAt.UTC().Format("2006-01-02T15:04:05Z07:00")
		resp.Data = json.RawMessage(snap.JSON)
	}
	response.Success(c, resp)
}

// Image 直出缓存的漫画摘要图（走本平台域名，避免让终端用户浏览器直连第三方）。
// GET /api/v1/codexradar/image
func (h *CodexRadarHandler) Image(c *gin.Context) {
	if !h.enabled(c) {
		response.Forbidden(c, "Codex radar feature is disabled")
		return
	}
	h.radarService.EnsureFresh(c.Request.Context())
	img := h.radarService.ImageSnapshot()
	if !img.Available {
		response.NotFound(c, "Codex radar image not available yet")
		return
	}

	// ETag 协商缓存：命中则回 304，省流量。
	if img.ETag != "" {
		c.Header("ETag", img.ETag)
		if match := c.GetHeader("If-None-Match"); match != "" && match == img.ETag {
			c.Status(http.StatusNotModified)
			return
		}
	}
	// 私有缓存：数据日更两次，缓存 1 小时避免重复下载大图（跨境链路下载 2.2MB 很慢）；
	// 配合上面的 ETag 协商，过期后也多半回 304 而非重传整图。随开关关闭自然失效。
	c.Header("Cache-Control", "private, max-age=3600")
	c.Data(http.StatusOK, img.ContentType, img.Bytes)
}
