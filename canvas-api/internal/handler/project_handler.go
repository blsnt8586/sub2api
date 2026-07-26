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

// maxProjectBytes 限制单个画布 body 大小。生成的图片/视频存对象存储，
// 画布 data 里只放引用，正常远小于此值；上限用于防御异常大 payload。
const maxProjectBytes = 8 << 20 // 8 MiB

// ProjectStore 是项目 handler 依赖的存储接口（便于测试替换）。
type ProjectStore interface {
	ListProjects(ctx context.Context, userID int64) ([]json.RawMessage, error)
	GetProject(ctx context.Context, userID int64, clientID string) (json.RawMessage, error)
	UpsertProject(ctx context.Context, userID int64, clientID, title string, data []byte) error
	DeleteProject(ctx context.Context, userID int64, clientID string) (bool, error)
}

// Projects 承载画布项目的 CRUD handler。
type Projects struct {
	store ProjectStore
}

// NewProjects 构造项目 handler。
func NewProjects(store ProjectStore) *Projects {
	return &Projects{store: store}
}

// projectMeta 从前端 CanvasProject 里抽取列表/索引所需的最小字段。
type projectMeta struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// List 返回当前用户的所有画布（完整对象数组）。
func (h *Projects) List(c *gin.Context) {
	items, err := h.store.ListProjects(c.Request.Context(), auth.UserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": items})
}

// Get 返回单个画布的完整对象。
func (h *Projects) Get(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("id"))
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "project id is required"})
		return
	}
	data, err := h.store.GetProject(c.Request.Context(), auth.UserID(c), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "GET_FAILED", "message": err.Error()})
		return
	}
	if data == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "project not found"})
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", data)
}

// Save 插入或更新一个画布。body 是完整的前端 CanvasProject 对象；
// id 优先取 URL 参数（PUT），否则取 body 内的 id（POST）。
func (h *Projects) Save(c *gin.Context) {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxProjectBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "READ_FAILED", "message": "failed to read body"})
		return
	}
	if len(body) > maxProjectBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "TOO_LARGE", "message": "project payload too large"})
		return
	}
	if !json.Valid(body) {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_JSON", "message": "body is not valid JSON"})
		return
	}

	var meta projectMeta
	if err := json.Unmarshal(body, &meta); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_SHAPE", "message": "cannot parse project fields"})
		return
	}
	clientID := strings.TrimSpace(c.Param("id"))
	if clientID == "" {
		clientID = strings.TrimSpace(meta.ID)
	}
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "project id is required"})
		return
	}

	if err := h.store.UpsertProject(c.Request.Context(), auth.UserID(c), clientID, meta.Title, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"id": clientID})
}

// Delete 删除一个画布。
func (h *Projects) Delete(c *gin.Context) {
	clientID := strings.TrimSpace(c.Param("id"))
	if clientID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_ID", "message": "project id is required"})
		return
	}
	ok, err := h.store.DeleteProject(c.Request.Context(), auth.UserID(c), clientID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DELETE_FAILED", "message": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "project not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
