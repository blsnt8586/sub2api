//go:build unit

package handler

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestToolDownloadsHandlerServesLocalFilesAndRanges(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("a", 64)
	name := "CodexPlusPlus-v1-windows-x64-setup.exe"
	contents := []byte("a verified local installer")
	dir := filepath.Join(root, "files", id)
	require.NoError(t, os.MkdirAll(dir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), contents, 0600))
	asset := service.ToolDownloadAsset{Name: name, Platform: "windows", Architecture: "x64", Format: "exe", Size: int64(len(contents)), URL: "/api/v1/tool-downloads/files/" + id + "/" + name, Digest: fmt.Sprintf("sha256:%x", sha256.Sum256(contents))}
	next := time.Now().Add(24 * time.Hour)
	snapshot := service.ToolDownloadsSnapshot{Version: "v1", Available: true, NextCheckAt: &next, Assets: []service.ToolDownloadAsset{asset}}
	raw, err := json.Marshal(map[string]any{"schema": 1, "snapshot": snapshot, "releases": []any{map[string]any{"id": id, "snapshot": snapshot}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0600))
	// Both catalog entries start with cached fixtures; no live network in tests.
	ccRoot := filepath.Join(root, "cc-switch")
	require.NoError(t, os.MkdirAll(filepath.Join(ccRoot, "files", id), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(ccRoot, "manifest.json"), raw, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(ccRoot, "files", id, name), contents, 0600))
	t.Setenv("TOOL_DOWNLOADS_DIR", root)
	s := service.NewToolDownloadsService(nil)
	t.Cleanup(s.Stop)
	h := NewToolDownloadsHandler(s)
	r := gin.New()
	r.GET("/files/:release/:name", h.Download)
	r.HEAD("/files/:release/:name", h.Download)
	base := "/files/" + id + "/" + name
	request := func(method, path, byteRange string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	full := request("GET", base, "")
	require.Equal(t, 200, full.Code)
	require.Equal(t, contents, full.Body.Bytes())
	require.Contains(t, full.Header().Get("Content-Disposition"), "attachment;")
	require.Contains(t, full.Header().Get("Content-Disposition"), name)
	require.Empty(t, full.Header().Get("Location"))
	require.Equal(t, "bytes", full.Header().Get("Accept-Ranges"))
	partial := request("GET", base, "bytes=2-8")
	require.Equal(t, http.StatusPartialContent, partial.Code)
	require.Equal(t, contents[2:9], partial.Body.Bytes())
	require.Equal(t, fmt.Sprintf("bytes 2-8/%d", len(contents)), partial.Header().Get("Content-Range"))
	head := request("HEAD", base, "")
	require.Equal(t, 200, head.Code)
	require.Empty(t, head.Body.Bytes())
	missing := request("GET", "/files/"+id+"/manifest.json", "")
	require.Equal(t, 404, missing.Code)
	invalid := request("GET", "/files/invalid/"+name, "")
	require.Equal(t, 404, invalid.Code)
	etagReq := httptest.NewRequest("GET", base, nil)
	etagReq.Header.Set("If-None-Match", full.Header().Get("ETag"))
	etagResp := httptest.NewRecorder()
	r.ServeHTTP(etagResp, etagReq)
	require.Equal(t, 304, etagResp.Code)
	require.Empty(t, etagResp.Body.Bytes())
	require.NoError(t, os.Remove(filepath.Join(dir, name)))
	require.NoError(t, os.Remove(filepath.Join(ccRoot, "files", id, name)))
	deleted := request("GET", base, "")
	require.Equal(t, 404, deleted.Code)
}
