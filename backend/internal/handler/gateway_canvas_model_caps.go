package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"
)

// canvasModelCapsSchemaVersion 随 wire 格式变更递增。
// 前端拿到不认识的版本时保留内置兜底表，不会误用无法解析的结构。
const canvasModelCapsSchemaVersion = 1

// canvasModelCapsResponse 是 /v1/canvas/model-caps 的响应体。
//
// 该端点存在的唯一目的是消除 caps 的双份维护：avi2api registry 是唯一权威，
// infinite-canvas 前端（独立仓库）在运行时拉取本端点覆盖其内置兜底表。
// 前端不能改成构建期代码生成——它是独立 git 仓库，构建时拿不到 Go 源码。
type canvasModelCapsResponse struct {
	Object        string               `json:"object"`
	SchemaVersion int                  `json:"schemaVersion"`
	Caps          avi2api.ModelCapsDTO `json:"caps"`
}

// caps 是进程内不变量：registry 是编译期常量，序列化结果与请求无关。
// 首次请求时算一次 body 和 ETag，后续直接复用。
var (
	canvasModelCapsOnce sync.Once
	canvasModelCapsBody []byte
	canvasModelCapsETag string
)

func canvasModelCapsPayload() ([]byte, string) {
	canvasModelCapsOnce.Do(func() {
		body, err := json.Marshal(canvasModelCapsResponse{
			Object:        "sub2api.canvas_model_caps",
			SchemaVersion: canvasModelCapsSchemaVersion,
			Caps:          avi2api.AllModelCapsDTO(),
		})
		if err != nil {
			// registry 是纯数据结构，Marshal 不可能失败；兜底成空对象而非 panic，
			// 避免一个元数据端点把网关拖崩。
			canvasModelCapsBody = []byte(`{"object":"sub2api.canvas_model_caps","schemaVersion":0,"caps":{}}`)
			canvasModelCapsETag = `"caps-unavailable"`
			return
		}
		canvasModelCapsBody = body
		sum := sha256.Sum256(body)
		canvasModelCapsETag = `"` + hex.EncodeToString(sum[:16]) + `"`
	})
	return canvasModelCapsBody, canvasModelCapsETag
}

// CanvasModelCaps 返回 avi2api 全部已注册模型的参数约束。
//
// 仅需 API Key 鉴权（中间件已完成），不涉及分组、配额与计费：响应对所有 Key
// 完全相同，且不读取任何用户数据。因此也在 apiKeyAuth 的 skipBilling 列表里，
// 额度耗尽的 Key 依然能拉到 caps——否则前端会退回内置兜底表，反而更容易发错参数。
func (h *GatewayHandler) CanvasModelCaps(c *gin.Context) {
	body, etag := canvasModelCapsPayload()

	// caps 只随后端版本变化，给一个较长的 max-age；ETag 让版本更新后立刻失效。
	c.Header("Cache-Control", "public, max-age=300")
	c.Header("ETag", etag)

	if match := c.GetHeader("If-None-Match"); match != "" && match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", body)
}
