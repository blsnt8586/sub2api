package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/blsnt8586/canvas-api/internal/auth"
)

const maxAssetBytes = 8 << 20  // 8 MiB（图片资产的 data 含 dataUrl，可能稍大）
const maxConfigBytes = 512 << 10 // 512 KiB

// AssetStore 是资产存储接口。
type AssetStore interface {
	ListAssets(ctx context.Context, userID int64, kind string) ([]json.RawMessage, error)
	UpsertAsset(ctx context.Context, userID int64, clientID, kind, title string, data []byte) error
	DeleteAsset(ctx context.Context, userID int64, clientID string) (bool, error)
}

// ConfigStore 是配置存储接口。
type ConfigStore interface {
	GetConfig(ctx context.Context, userID int64) (json.RawMessage, error)
	SaveConfig(ctx context.Context, userID int64, data []byte) error
}

// assetMeta 从前端 Asset 对象里抽取索引字段。
type assetMeta struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Title string `json:"title"`
}

// Assets 承载资产 CRUD handler。
type Assets struct{ store AssetStore }

// NewAssets 构造资产 handler。
func NewAssets(store AssetStore) *Assets { return &Assets{store: store} }

// List 返回当前用户的全部资产，可按 ?kind=image|video|text 过滤。
func (h *Assets) List(c *gin.Context) {
	kind := normalizeAssetKind(c.Query("kind"))
	items, err := h.store.ListAssets(c.Request.Context(), auth.UserID(c), kind)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"assets": items})
}

// Save 插入或更新一个资产（body 是完整前端 Asset 对象）。
func (h *Assets) Save(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxAssetBytes+1))
	if err != nil || len(body) > maxAssetBytes {
		c.JSON(http.StatusBadRequest, gin.H{"code": "READ_FAILED", "message": "failed to read body"})
		return
	}
	if !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_JSON", "message": "body is not valid JSON"})
		return
	}
	var meta assetMeta
	_ = json.Unmarshal(body, &meta)
	clientID := strings.TrimSpace(c.Param("id"))
	if clientID == "" {
		clientID = strings.TrimSpace(meta.ID)
	}
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "asset id is required"})
		return
	}
	kind := normalizeAssetKind(meta.Kind)
	if err := h.store.UpsertAsset(c.Request.Context(), auth.UserID(c), clientID, kind, meta.Title, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": clientID})
}

// Delete 删除一个资产。
func (h *Assets) Delete(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("id"))
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "asset id is required"})
		return
	}
	ok, err := h.store.DeleteAsset(c.Request.Context(), auth.UserID(c), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DELETE_FAILED", "message": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "asset not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// normalizeAssetKind 过滤非法 kind，空/非法返回空字符串（列表时表示"全部"，保存时回退 text）。
func normalizeAssetKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "image", "video", "text":
		return strings.TrimSpace(kind)
	default:
		return ""
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

// Config 承载用户 AI 配置读写 handler。
type Config struct{ store ConfigStore }

// NewConfig 构造配置 handler。
func NewConfig(store ConfigStore) *Config { return &Config{store: store} }

// Get 取当前用户配置，不存在返回 {}。
func (h *Config) Get(c *gin.Context) {
	data, err := h.store.GetConfig(c.Request.Context(), auth.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "GET_FAILED", "message": err.Error()})
		return
	}
	if data == nil {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(`{}`))
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// Save 保存当前用户配置（body 是完整 AiConfig JSON 对象）。
func (h *Config) Save(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxConfigBytes+1))
	if err != nil || len(body) > maxConfigBytes {
		c.JSON(http.StatusBadRequest, gin.H{"code": "READ_FAILED", "message": "failed to read body"})
		return
	}
	if !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_JSON", "message": "body is not valid JSON"})
		return
	}
	if err := h.store.SaveConfig(c.Request.Context(), auth.UserID(c), body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
