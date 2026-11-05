package handler

import (
	"mime"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ToolDownloadsHandler struct{ downloads *service.ToolDownloadsService }

func NewToolDownloadsHandler(downloads *service.ToolDownloadsService) *ToolDownloadsHandler {
	return &ToolDownloadsHandler{downloads: downloads}
}

func (h *ToolDownloadsHandler) Download(c *gin.Context) {
	f, asset, err := h.downloads.OpenDownload(c.Param("release"), c.Param("name"))
	if err != nil {
		response.NotFound(c, "安装包暂不可用，请刷新下载列表后重试")
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		response.NotFound(c, "安装包暂不可用")
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": asset.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "public, max-age=86400, immutable")
	c.Header("ETag", `"`+strings.TrimPrefix(asset.Digest, "sha256:")+`"`)
	http.ServeContent(c.Writer, c.Request, asset.Name, stat.ModTime(), f)
}

func (h *ToolDownloadsHandler) List(c *gin.Context) {
	snapshots, err := h.downloads.Catalog(c.Request.Context())
	if err != nil {
		return
	} // The client has canceled or the request deadline elapsed.
	response.Success(c, struct {
		Tools []service.ToolDownloadsSnapshot `json:"tools"`
	}{Tools: snapshots})
}
