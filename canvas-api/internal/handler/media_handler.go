package handler

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/blsnt8586/canvas-api/internal/auth"
	"github.com/blsnt8586/canvas-api/internal/repository"
)

// maxMediaBytes 限制单个生成内容上传大小。图片一般 < 10MB，6s 720p 视频 ~15MB，
// 上限放到 64MB 覆盖较长/较高清视频，同时防御异常大 payload。
const maxMediaBytes = 64 << 20 // 64 MiB

// presignTTL 是预签名下载 URL 的有效期。前端每次打开画布用 storageKey 重新换取，
// 所以不必太长；7 天足够覆盖一次会话内的反复加载。
const presignTTL = 7 * 24 * time.Hour

// MediaStore 是媒体元数据存储接口（便于测试替换）。
type MediaStore interface {
	Upsert(ctx context.Context, userID int64, m repository.MediaRecord) error
	GetByKey(ctx context.Context, userID int64, mediaKey string) (*repository.MediaRecord, error)
	List(ctx context.Context, userID int64, kind string, limit, offset int) ([]repository.MediaRecord, error)
	SetPinned(ctx context.Context, userID int64, mediaKey string, pinned bool) (bool, error)
	Delete(ctx context.Context, userID int64, mediaKey string) (string, error)
}

// ObjectStore 是对象存储接口（S3 兼容）。
type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, data []byte) error
	PresignGet(ctx context.Context, key string, expire time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

// Media 承载生成内容（图片/视频/音频）的存取 handler。
type Media struct {
	store MediaStore
	s3    ObjectStore
}

// NewMedia 构造媒体 handler。s3 为 nil 时（未配置 Wasabi）上传类接口返回 503。
func NewMedia(store MediaStore, s3 ObjectStore) *Media {
	return &Media{store: store, s3: s3}
}

// s3KeyFor 由 userID + mediaKey 推导出稳定的 S3 对象键。
// mediaKey 形如 "image:abc"，冒号换成斜杠 → "users/1/image/abc"。
func s3KeyFor(userID int64, mediaKey string) string {
	safe := strings.ReplaceAll(mediaKey, ":", "/")
	return fmt.Sprintf("users/%d/%s", userID, safe)
}

// Upload 接收 multipart 上传：file（字节）+ key（前端 storageKey，可选）+ kind + 元数据。
// 存 S3 → 入库 → 返回 { key, url }。
func (h *Media) Upload(c *gin.Context) {
	if h.s3 == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "STORAGE_DISABLED", "message": "object storage is not configured"})
		return
	}
	userID := auth.UserID(c)

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxMediaBytes+1)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "NO_FILE", "message": "file is required"})
		return
	}
	if fileHeader.Size > maxMediaBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"code": "TOO_LARGE", "message": "media payload too large"})
		return
	}

	f, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "OPEN_FAILED", "message": "cannot open uploaded file"})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "READ_FAILED", "message": "cannot read uploaded file"})
		return
	}

	kind := normalizeKind(c.PostForm("kind"))
	mediaKey := strings.TrimSpace(c.PostForm("key"))
	if mediaKey == "" {
		// 前端未带 key 时，用 kind 生成一个（与前端 storageKey 同构：kind:uuid）。
		mediaKey = kind + ":" + strings.ReplaceAll(fileHeader.Filename, "/", "_")
	}
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = c.PostForm("mimeType")
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	s3Key := s3KeyFor(userID, mediaKey)
	if err := h.s3.Put(c.Request.Context(), s3Key, contentType, data); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": "UPLOAD_FAILED", "message": err.Error()})
		return
	}

	rec := repository.MediaRecord{
		MediaKey:   mediaKey,
		Kind:       kind,
		S3Key:      s3Key,
		MimeType:   contentType,
		Bytes:      int64(len(data)),
		Width:      optInt(c.PostForm("width")),
		Height:     optInt(c.PostForm("height")),
		DurationMs: optInt(c.PostForm("durationMs")),
	}
	if err := h.store.Upsert(c.Request.Context(), userID, rec); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "SAVE_FAILED", "message": err.Error()})
		return
	}

	url, err := h.s3.PresignGet(c.Request.Context(), s3Key, presignTTL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": "PRESIGN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": mediaKey, "url": url, "bytes": rec.Bytes, "mimeType": contentType})
}

// ResolveURL 用 media_key 换取一个新的预签名下载 URL。
// 前端每次打开画布/资产时调用它，把稳定的 storageKey 变成可显示的临时 URL。
func (h *Media) ResolveURL(c *gin.Context) {
	if h.s3 == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": "STORAGE_DISABLED", "message": "object storage is not configured"})
		return
	}
	mediaKey := strings.TrimSpace(c.Param("key"))
	if mediaKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_KEY", "message": "media key is required"})
		return
	}
	rec, err := h.store.GetByKey(c.Request.Context(), auth.UserID(c), mediaKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "GET_FAILED", "message": err.Error()})
		return
	}
	if rec == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "media not found"})
		return
	}
	url, err := h.s3.PresignGet(c.Request.Context(), rec.S3Key, presignTTL)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"code": "PRESIGN_FAILED", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": mediaKey, "url": url, "kind": rec.Kind, "mimeType": rec.MimeType})
}

// List 分页返回用户的生成内容（附带每条的预签名 URL）。
func (h *Media) List(c *gin.Context) {
	userID := auth.UserID(c)
	kind := normalizeKindFilter(c.Query("kind"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	offset, _ := strconv.Atoi(c.Query("offset"))

	items, err := h.store.List(c.Request.Context(), userID, kind, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "LIST_FAILED", "message": err.Error()})
		return
	}

	out := make([]gin.H, 0, len(items))
	for _, m := range items {
		entry := gin.H{
			"key": m.MediaKey, "kind": m.Kind, "mimeType": m.MimeType,
			"bytes": m.Bytes, "width": m.Width, "height": m.Height,
			"durationMs": m.DurationMs, "pinned": m.Pinned, "createdAt": m.CreatedAt,
		}
		if h.s3 != nil {
			if url, err := h.s3.PresignGet(c.Request.Context(), m.S3Key, presignTTL); err == nil {
				entry["url"] = url
			}
		}
		out = append(out, entry)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

// Pin 标记/取消收藏（收藏的内容不受生命周期清理影响）。
func (h *Media) Pin(c *gin.Context) {
	mediaKey := strings.TrimSpace(c.Param("key"))
	if mediaKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_KEY", "message": "media key is required"})
		return
	}
	pinned := c.Query("pinned") != "false" // 默认 true，?pinned=false 取消
	ok, err := h.store.SetPinned(c.Request.Context(), auth.UserID(c), mediaKey, pinned)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "PIN_FAILED", "message": err.Error()})
		return
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "media not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": mediaKey, "pinned": pinned})
}

// Delete 删除一条生成内容（同时删对象存储里的字节）。
func (h *Media) Delete(c *gin.Context) {
	mediaKey := strings.TrimSpace(c.Param("key"))
	if mediaKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": "INVALID_KEY", "message": "media key is required"})
		return
	}
	s3Key, err := h.store.Delete(c.Request.Context(), auth.UserID(c), mediaKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "DELETE_FAILED", "message": err.Error()})
		return
	}
	if s3Key == "" {
		c.JSON(http.StatusNotFound, gin.H{"code": "NOT_FOUND", "message": "media not found"})
		return
	}
	// 先删元数据（上面已删）再删字节；字节删除失败只记不阻断（可由生命周期兜底）。
	if h.s3 != nil {
		_ = h.s3.Delete(c.Request.Context(), s3Key)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// normalizeKind 归一化上传的 kind，非法值回退到 image。
func normalizeKind(kind string) string {
	switch strings.TrimSpace(kind) {
	case "video":
		return "video"
	case "audio":
		return "audio"
	default:
		return "image"
	}
}

// normalizeKindFilter 归一化列表过滤的 kind，空/非法表示"全部"。
func normalizeKindFilter(kind string) string {
	switch strings.TrimSpace(kind) {
	case "image", "video", "audio":
		return strings.TrimSpace(kind)
	default:
		return ""
	}
}

// optInt 把可选的表单数字字段解析成 *int；空或非法返回 nil。
func optInt(s string) *int {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return nil
	}
	return &n
}
