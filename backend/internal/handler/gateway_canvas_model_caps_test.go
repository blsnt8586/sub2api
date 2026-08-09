package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 走真实 engine 而非直接调 handler：c.Status() 只在 gin 内部登记状态码，
// 要等处理链收尾时 WriteHeaderNow() 才落到 ResponseWriter。裸调 handler
// 拿不到 304，测出来的会是 recorder 的初始值 200。
func serveCanvasModelCaps(ifNoneMatch string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/v1/sub2api/canvas/model-caps", (&GatewayHandler{}).CanvasModelCaps)

	req := httptest.NewRequest(http.MethodGet, "/v1/sub2api/canvas/model-caps", nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// 端点不依赖任何 service，也不读 API Key —— 返回的是编译期就固定的静态注册表。
func TestCanvasModelCapsReturnsFullRegistry(t *testing.T) {
	w := serveCanvasModelCaps("")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Object        string               `json:"object"`
		SchemaVersion int                  `json:"schemaVersion"`
		Caps          avi2api.ModelCapsDTO `json:"caps"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))

	require.Equal(t, "sub2api.canvas_model_caps", body.Object)
	require.Equal(t, canvasModelCapsSchemaVersion, body.SchemaVersion)

	// 数量必须与注册表一致，缺一个就意味着前端下拉会少一个模型。
	require.Len(t, body.Caps.Video, len(avi2api.AllVideoModels()))
	require.Len(t, body.Caps.Image, len(avi2api.AllImageModels()))
	require.Len(t, body.Caps.Audio, len(avi2api.AllAudioModels()))
}

// 前端靠 ETag 做冷启动后的廉价复验；头缺失会导致每次启动都全量传输。
func TestCanvasModelCapsSetsCacheHeaders(t *testing.T) {
	w := serveCanvasModelCaps("")
	require.NotEmpty(t, w.Header().Get("ETag"))
	require.Contains(t, w.Header().Get("Cache-Control"), "max-age")
}

func TestCanvasModelCapsHonorsIfNoneMatch(t *testing.T) {
	etag := serveCanvasModelCaps("").Header().Get("ETag")
	require.NotEmpty(t, etag)

	second := serveCanvasModelCaps(etag)
	require.Equal(t, http.StatusNotModified, second.Code)
	require.Empty(t, second.Body.Bytes(), "304 不应带 body")
}

// ETag 由内容派生：注册表不变则跨请求稳定，否则前端无法判断缓存是否失效。
func TestCanvasModelCapsETagIsContentDerivedAndStable(t *testing.T) {
	require.Equal(t, serveCanvasModelCaps("").Header().Get("ETag"), serveCanvasModelCaps("").Header().Get("ETag"))
}
