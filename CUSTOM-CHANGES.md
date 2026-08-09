# Sub2API 自定义改动说明

> 本文档记录在上游 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 基础上的全部二次开发（fork）改动。
> 用途：换机部署 / 升级上游后快速定位与复原自定义代码。
>
> **维护原则**：本 fork 长期紧跟上游。为降低同步痛点，二开代码尽量**新增文件**、
> 并把散落的平台分支**收敛到单一权威文件**（见 [§6 解耦策略](#六解耦策略降低上游同步冲突)）。
> 每次同步上游后，对照 [§7 冲突面分级](#七冲突面分级high-conflict-必查) 逐项核对。

---

## 一、功能总览

fork 在上游之上叠加了三大功能块，外加一层解耦重构：

| 功能块 | 说明 | 冲突面 |
|--------|------|--------|
| A. Nova Image Studio iframe 对接 | 用户端 iframe 嵌入 Nova，含剪贴板授权、导航联动、`/v1/models` 免计费 | 中（改上游 View + 中间件） |
| B. AVI2API 图像/视频/音频平台（`platform=jimeng`） | 新增图像 + 视频 + 音频生成平台，走 OpenAI 网关链路，按次/按秒计费。视频支持五种请求模式（无参考 JSON + 参考图/首尾帧/参考视频/参考音频 multipart）。2026-08 重构：原生即梦下线、vendor 机制移除、`leonardo` 更名 `avi2api` | 中（新增为主，少量上游文件插桩） |
| C. 上游 Sub2API 管理 + 定时优化 | 管理端管理 `provider_type=sub2api` 上游实例，定时优化账号倍率区间 | 低（几乎全新增文件） |
| D. 平台分支解耦重构 | 把散落各处的平台 `switch/if` 收敛到 `platformColors.ts` / `domain_constants.go` | 降低未来冲突 |
| E. OpenAI/Codex 全局 system prompt 注入 | 管理端配置全局系统提示词，前置合并到 Responses `instructions`，覆盖 responses/codex/chat 三条路径 | 低（逻辑全在新增文件，上游纯追加 + 2 处网关钩子） |
| F. Codex 雷达（第三方数据代理） | 代理缓存第三方站点 codexradar.com 的 Codex 观测数据，用户+管理员共用页面，带第三方来源免责说明 | 极低（全新增文件 + opt-in 功能开关，零上游钩子） |
| G. 首页整体重构（2026-07 提交 `0f60c3edd`） | `HomeView.vue` 全量重写为深空网关风格：明暗双主题（默认亮色）、canvas 波形/星尘/剖半点阵地球、Base URL 复制组件、SDK 兼容徽章、终端三 Tab、FAQ；`landing.ts`(zh/en) 新增大量 key；`site_subtitle` 支持 JSON 多语言；router `scrollBehavior` 刷新不恢复滚动位置 | **高（HomeView.vue 与上游完全分叉，同步时保留本 fork 版本）** |

> 注：**Grok 平台是上游自带**，非本 fork 新增。fork 唯一新增的平台是**即梦（jimeng）**。

---

## 二、Nova Image Studio iframe 对接（功能块 A）

### 2.1 `/v1/models` 跳过计费（后端）

`backend/internal/server/middleware/api_key_auth.go`（约 148 行）：

```go
// skipBilling: /v1/usage 和 /v1/models 只需鉴权，跳过所有计费执行
skipBilling := c.Request.URL.Path == "/v1/usage" || c.Request.URL.Path == "/v1/models"
```

**原因**：Nova 需要拉模型列表（`/v1/models`），此接口只做鉴权、不应计费/扣订阅额度。
**冲突点**：上游只有 `/v1/usage`，同步时若该行被覆盖需补回 `/v1/models`。

### 2.2 iframe 剪贴板/全屏权限（前端）

嵌入 Nova 的 iframe（`HomeView.vue` / `CustomPageView.vue` / `RiskControlView.vue`）均加：

```html
<iframe allow="clipboard-write; clipboard-read; fullscreen" allowfullscreen></iframe>
```

**原因**：Nova 在 iframe 内复制图片时被浏览器拦截（Clipboard API 默认禁用），父页面必须显式授权。

### 2.3 嵌入页导航联动（前端）

`frontend/src/views/user/CustomPageView.vue` 监听 Nova `postMessage` 做**当前页内路由跳转**（不整页刷新）：

```ts
const NAVIGATE_TARGETS: Record<string, string> = { keys: '/keys' } // 白名单，避免任意跳转

function handleEmbeddedMessage(event: MessageEvent) {
  const data = event.data
  if (!data || typeof data !== 'object' || data.type !== 'sub2api:navigate') return
  if (event.origin !== new URL(embeddedUrl.value).origin) return  // 校验 origin
  const path = NAVIGATE_TARGETS[String(data.target)]
  if (path) void router.push(path)
}
```

**安全要点**：①消息类型固定 `sub2api:navigate`；②校验 `event.origin`；③`target` 必须命中白名单。
Nova 侧发送示例：`parent.postMessage({ type: 'sub2api:navigate', target: 'keys' }, '*')`。

---

## 三、即梦（jimeng）视频平台（功能块 B）

新增第三方视频生成平台（`base_url` + `api_key` 凭据），生图/生视频走 **OpenAI 网关链路**（`OpenAIGatewayService`），并支持**按次/按秒计费**。

### 3.1 平台常量（权威来源）

- `backend/internal/domain/constants.go` — `PlatformJimeng` 常量
- `backend/internal/service/domain_constants.go` — 转出 `PlatformJimeng`，并加入 `AllowedQuotaPlatforms`（**单一权威列表**，新增平台只改这里）
- 前端 `frontend/src/utils/platformColors.ts` — `Platform` 类型 + `ALL_PLATFORMS` 追加 `jimeng`，`platformLabel()` 映射 `jimeng → '即梦'`（**前端单一权威列表**）

### 3.2 新增文件（低冲突，直接保留）

后端：
- `backend/internal/pkg/avi2api/` — 协议客户端（`models.go` / `url.go` + 测试）。**注**：原 `pkg/jimeng/` 与 `pkg/leonardo/` 已于 2026-08 合并至此，见 3.4
- `backend/internal/handler/jimeng_video.go` — 视频生成/状态查询 handler
- `backend/internal/service/jimeng_video_service.go` / `jimeng_video_helpers.go` — 业务逻辑（含 duration → `VideoSeconds` 解析）

前端：即梦作为平台自动被各平台下拉/配额矩阵纳入（见解耦策略），无独立视图。

### 3.3 视频计费（后端，方案 A：独立价格字段）

**Ent schema** `backend/ent/schema/group.go`（改 schema 后必须 `go generate ./ent` 并提交生成代码）：

```go
field.Float("video_price_per_count")   // 每次视频价格 USD/次，nil=默认
field.Float("video_price_per_second")  // 每秒视频价格 USD/秒，非 nil 时优先于按次
```

**迁移** `backend/migrations/165_groups_add_video_pricing.sql`。

**计费模式** `backend/internal/service/channel.go`：新增 `BillingModeVideo = "video"`、
`BillingModeVideoPerSecond = "video_per_second"`（已加入 `IsValid()`）。

**成本计算** `backend/internal/service/billing_service.go` 的 `CalculateVideoCost(videoCount, videoSeconds, groupConfig, rateMultiplier)`：
优先级 **按秒 > 按次 > 默认 $0.05/次**。

**接入点（上游文件插桩，注意冲突）**：
- `openai_gateway_usage.go` `calculateOpenAIRecordUsageCost` — video 分支置于 image 分支**之前**（即梦走 OpenAI 链路，`OpenAIForwardResult.VideoCount/VideoSeconds`）
- `gateway_usage_billing.go` `calculateRecordUsageCost` — 对应 video 分支（`ForwardResult` 路径）
- `admin_group.go` CreateGroup/UpdateGroup — video 价格字段映射（`normalizePrice`）
- `server/routes/gateway.go` — `videoStatusHandler` 按平台分流即梦/Grok；`/v1/videos/*` 路由

> ⚠️ **两套 ForwardResult**：`ForwardResult`（GatewayService）与 `OpenAIForwardResult`（OpenAIGatewayService）都加了
> `VideoCount`/`VideoSeconds` 字段。即梦走 OpenAI 路径，改计费务必两边同步。

### 3.4 AVI2API 上游（图像 + 视频 + 音频）

> **2026-08 重构**：原生即梦上游已**下线**，`credentials.vendor` 机制**移除**。
> `platform=jimeng` 现在唯一对应 AVI2API 上游。原 vendor 名 `leonardo`
> 在代码中全部改为 `avi2api`。DB 里存量账号的 `credentials.vendor` 字段被忽略，
> **无需迁移**。

**设计目标**：接入 AVI2API 图像/视频/音频生成网关。平台标识仍为 `jimeng`
（DB/前端/quota 三处的值未改，避免账号+分组+配额的数据迁移），
但代码标识、管理端文案、对外契约均已统一为 AVI2API。

> **平台名遗留**：`platform=jimeng` 这个字符串值本身仍未改名。要改成 `avi2api`
> 需要迁移 accounts/groups/user_platform_quota 三张表 + 前端类型 + i18n，
> 风险与本次重构不同量级，属未决事项。

**新增文件（低冲突，直接保留）**：
- `backend/internal/pkg/avi2api/` — 协议客户端。合并了原 `pkg/leonardo` 与 `pkg/jimeng`：
  - `url.go` — 图像/视频/音频九个端点的 URL 构建
  - `models.go` — 请求解析：`ParseImageRequest`/`ParseVideoRequest`/`ParseAudioRequest`
    + `CountImages`。**视频解析同时支持 JSON 与 multipart**（见下）
- `backend/internal/service/avi2api_image_service.go` — `ForwardJimengImages`（图像转发）
- `backend/internal/service/jimeng_video_service.go` / `jimeng_video_helpers.go` — 视频转发 + 响应规范化
- `backend/internal/service/jimeng_audio_service.go` — 音频转发 + 粘性会话 + 计费
- `backend/internal/service/jimeng_video_seedance.go` — Ark Plan v3 body 翻译层
- `backend/internal/handler/jimeng_images.go` / `jimeng_video.go` / `jimeng_audio.go` — 三个 handler + 转发循环
- `backend/internal/server/routes/gateway_video.go` / `gateway_audio.go` — 路由注册

**已删除**：`pkg/leonardo/`、`pkg/jimeng/`、`service/leonardo_image_service.go`

**上游/既有文件插桩（少量，注意冲突）**：
- `backend/internal/service/account.go` — `GetJimengBaseURL()` / `GetJimengAPIKey()`
  （`GetJimengVendor()` / `IsJimengLeonardo()` / `JimengVendorLeonardo` 已删除）
- `backend/internal/server/routes/gateway.go` `imagesHandler` — `case service.PlatformJimeng`
- `backend/internal/service/openai_gateway_scheduling.go` `normalizeOpenAICompatiblePlatform`
  — jimeng 分支（`[CUSTOM]` 标记，必须保留，否则选号命中不了）
- 前端 `frontend/src/components/account/CreateAccountModal.vue` — 即梦建号块
  （vendor 选择器**已移除**，只剩 Base URL + API Key）
- 前端 `frontend/src/views/admin/groupsImagePricing.ts` — `imagePricingPlatforms` 含 `"jimeng"`
- i18n `zh|en/admin/accounts.ts` 的 `jimeng` 块 — `name` 改为 `AVI2API`，
  `vendorLabel`/`vendorNative`/`vendorLeonardo`/`vendorLeonardoHint` **已删除**

**客户端契约（对外，与 AVI2API 文档逐字对齐）**：

| 对外端点 | 上游端点 |
|---|---|
| `POST /v1/images/generations` \| `/edits` | 同名 |
| `POST /v1/videos/generations` | 同名 |
| `GET /v1/videos/{id}` | 同名 |
| `POST /v1/videos/{id}/cancel` | 同名 |
| `POST /v1/audio/generations` | 同名 |
| `GET /v1/audio/{id}` | 同名 |
| `POST /v1/audio/{id}/cancel` | 同名 |

- **`POST /v1/videos` 已移除**（原 Sora 风格创建端点）。创建统一走 `/videos/generations`，
  与 Grok 共用该路径按 platform 分流。
- `GET /v1/videos/{id}/content` 保留给 Grok/composite；jimeng 分组访问返回明确的
  "not supported" 而非 404。AVI2API 无此端点，MP4 URL 在状态响应 `result.data[0].url`。
- `POST /v1/contents/generations/tasks` + `GET .../{id}` — Seedance/Ark Plan v3 兼容别名，
  body 在 handler 层经 `ConvertSeedanceVideoCreateBody` 翻译。

**视频五种请求模式（全部支持）**：

AVI2API 无 `mode` 字段，靠 Content-Type + multipart 中出现的文件字段推断：

| 模式 | Content-Type | 必需文件字段 |
|---|---|---|
| 无参考 | `application/json` | — |
| 参考图 | `multipart/form-data` | `image` / `image[]`（≤4） |
| 首尾帧 | `multipart/form-data` | `start_frame`（+可选 `end_frame`） |
| 参考视频 | `multipart/form-data` | `video[]`（≤3，合计 ≤15s） |
| 参考音频 | `multipart/form-data` | `audio[]`（≤1）+ 陪跑图/视频 |

实现要点（`pkg/avi2api/models.go`）：
- `ParseVideoRequest(contentType, body)` 双格式：JSON 走 gjson，multipart 走
  `parseVideoMultipartFields` **只挑文本字段**（model/prompt/duration/size/resolution），
  文件字段跳过不读进内存（单字段 1 MiB 上限防 OOM）
- body **逐字节透传**，Content-Type 连 boundary 一起转发，否则上游无法解析 multipart
- **组合规则不在本层实现**：首尾帧与其他素材互斥、`end_frame` 依赖 `start_frame`、
  Seedance 音频需搭图或视频、MiniMax H3 音频需搭普通图、各类数量上限——
  全部由上游 schema（`allOf`/`not`/`maxItems`）校验，违规吃 422 传回客户端。
  本层复制只会与上游漂移。

> **历史坑**：multipart 支持之前缺失，`ParseVideoRequest` 只认 JSON，
> handler 拿到空 model 直接 `400 model is required`，四种参考模式全部不可用。
> 同时 `duration` 解不出导致 `VideoSeconds=0`，按秒计费的分组等于白送。

**字段黑名单**：`fps` 与 `watermark` 必须丢弃。AVI2API v1.1.2 全部视频 schema 都是
`additionalProperties:false` 且均无这两个字段，携带即 400。测试
`TestConvertSeedanceVideoCreateBodyDropsFPS` 锁定此行为。

**`Idempotency-Key` 透传（图像/视频/音频三条转发链路必须一致）**：
AVI2API 对所有 POST 创建类接口（`/images/generations`、`/images/edits`、
`/videos/generations`、`/audio/generations`）**强制要求** `Idempotency-Key` 请求头
（非空且 ≤255 字符），缺失即返回 `Idempotency-Key is required and must not exceed 255 characters`。
网关转发时必须把客户端传来的该头**原样带给上游**：

```go
// canvas_image_service.go / canvas_video_service.go / canvas_audio_service.go 统一模式
if idempotencyKey := c.GetHeader("Idempotency-Key"); idempotencyKey != "" {
    upstreamReq.Header.Set("Idempotency-Key", idempotencyKey)
}
```

- 前端 infinite-canvas 在 `services/api/{image,video,audio}.ts` 与 `model-plugin.ts`
  的 POST 请求头用 `nanoid()` 生成该头。
- 透传还能防止 failover 重试重复创建付费任务（同 key 命中上游幂等）。
- **历史坑（2026-08 修复）**：`canvas_image_service.go` 曾漏掉透传（video/audio 有、image 没有），
  导致生图工作台一律 `生成失败：Idempotency-Key is required...`。回归测试
  `TestForwardCanvasImagesForwardsIdempotencyKey` 锁定此行为。**同步上游或重写转发时三条链路要同增同减。**

**响应规范化**（`normalizeJimengVideoResponse`）：
- `status: succeeded` → `completed`（infinite-canvas 认 `completed`）。
  **注意**：这偏离了 AVI2API 文档的 status enum，照文档写的客户端认不出 `completed`
- `result.data[0].url` → 顶层 `video_url` + `result_url` + `content.video_url`（Ark Plan 兼容）
- 补 `object: "video.task"`

**计费**：
- 图像：`CalculateImageCost`（`ImageCount` + `ImageSize` 1K/2K/4K 档位，分组 `image_price_1k/2k/4k`）
- 视频：`CalculateJimengVideoCost`（按次 `video_price_per_count` / 按秒 `video_price_per_second`）
- 音频：按次 `VideoCount=1`；`music-v1` 的 `duration_minutes` × 60 记入 `VideoSeconds`
  （`dialogue-v3`/`sound-effects-v2` 的 `duration` 单位是秒，直接记入）
- **无新增 schema 字段、无新增迁移**

**音频选号坑**：`runJimengAudioForwardLoop` 必须用
`SelectAccountWithSchedulerForCapability(..., service.PlatformJimeng)`。
通用的 `SelectAccountForModelWithExclusions` 内部硬编码 `PlatformOpenAI`
（`openai_gateway_scheduling.go:190`），用它会一个 jimeng 账号都命中不了。

**AVI2API 上游模型清单（v1.1.2）**：
- 图像：`gpt-image-2`、`nano-banana-2`、`nano-banana-pro`、`seedream-5.0-pro`
- 视频：`seedance-2.0`/`-fast`/`-mini`、`gemini-omni-flash`、`veo-3.1`/`-fast`/`-lite`、
  `kling-3.0`、`minimax-h3`（各模型的 duration/size/resolution/generate_audio 约束不同，
  详见上游 `/openapi.json`；本层纯透传不做本地校验）
- 音频：`dialogue-v3`（TTS）、`music-v1`（音乐）、`sound-effects-v2`（音效，`model` 可省略）

### 3.5 模型能力表：单一权威 + 运行时下发

**背景**：模型参数约束（duration/size/resolution/generate_audio/参考模式）原先在
两处各维护一份 —— 后端 `pkg/avi2api/registry.go` 与前端
`infinite-canvas/web/src/lib/canvas-model-caps.ts`。靠人工同步，已经出过两类事故：

1. **约束漂移**：前端箝制逻辑把 `1080p` 与去掉后缀的 `["720","1080"]` 相比，
   永远不匹配 → 所有 canvas 视频模型的 resolution 都被箝到裸 `"720"`，被上游拒
2. **模型漏判**：`guessCapability()` 靠模型名关键词猜能力，`nano-banana-2`/
   `nano-banana-pro`/`minimax-h3`/`gemini-omni-flash`/`dialogue-v3` 五个名字不含
   任何关键词，全被归到 `text`，在三个工作台的下拉里都不出现

**方案**：后端 registry 为唯一权威，前端运行时拉取覆盖。

- 后端 `pkg/avi2api/caps_dto.go` — 对外 JSON 表示。三处必须转换：
  `DurationConstraint`（Go 靠 `Enum` 非空隐式判别 → 前端要显式 `kind` 标签）、
  `GenAudioMode`（int 枚举 → 字符串）、`ReferenceMode`（Go 字面量 `start_frame`
  是 multipart 字段名 → 前端语义标签 `frame`）
- 后端 `handler/gateway_canvas_model_caps.go` — `GET /v1/sub2api/canvas/model-caps`，
  API Key 鉴权、`sync.Once` 缓存 body 与 ETag、支持 `If-None-Match` → 304
- 前端 `lib/canvas-model-caps-sync.ts` — 拿到第一个可用 Key 后拉取一次
  （`hydrateCanvasModelCapsOnce`，并发合并 + 单次守卫）
- 前端 `lib/canvas-model-caps.ts` — 三张 `bundled*` 表降级为**冷启动兜底**，
  registry 改为 `let` 可变 + `hydrateModelCaps()` 整表替换 + 订阅通知
- 前端 `lib/use-model-caps.ts` — `useSyncExternalStore` hook，让参数面板在
  caps 到达后重渲染

**为什么不做构建期代码生成**：`infinite-canvas` 是**独立 git 仓库**
（remote `basketikun/infinite-canvas`，嵌在本仓库内但非 submodule），
构建时拿不到 sub2api 的 Go 源码，生成步骤会让前端无法独立构建。

**为什么不完全删掉前端内置表**：`guessCapability()` 在 zustand persist 的
`merge` 回调里被**同步**调用（决定持久化 channel 的模型归类），那一刻网络请求
还没回来，必须有同步可用的数据。

**兜底表不需要手工维护**：数值允许过时，hydrate 时整表覆盖。后端加模型前端自动就有。

> ⚠️ **改 wire 格式必须同时递增两处版本号**：
> `handler/gateway_canvas_model_caps.go` 的 `canvasModelCapsSchemaVersion` 与
> `lib/canvas-model-caps-sync.ts` 的 `SUPPORTED_SCHEMA_VERSION`。
> 版本不匹配时前端整体拒绝该 payload 并保留兜底表 —— 这是有意的：
> 按旧字段解析未知结构会把非法约束写进面板，比继续用兜底表更危险。

**已知缺口**：
- `POST /v1/videos/estimate`、`POST /v1/images/estimate`（成本预估）未接
- `GET/POST /v1/tasks/{id}[/cancel]`（通用任务端点）未接，图像异步任务查不了
- **multipart 请求的提示词审计失效**：`ExtractContentModerationInput` 与
  `ExtractPromptSnapshot` 都只认 JSON body。`off`/`async` 模式下提示词扫不到
  （审计留痕缺失）；**`blocking` 模式下 `json.Unmarshal` 失败会升级成
  `GuardError{ErrorCodeInvalidResponse}` → 503，参考素材视频直接不可用**。
  `/images/edits` 有同样问题。

---

## 四、上游 Sub2API 管理 + 定时优化（功能块 C）

管理端管理 `provider_type=sub2api` 的上游实例，并定时优化账号的倍率区间。**几乎全新增文件，冲突面最低。**

### 4.1 后端新增文件

- Ent schema：`ent/schema/sub2api_provider.go`、`sub2api_optimize_schedule.go`、`sub2api_optimize_log.go`（+ 生成的 `ent/sub2api*`）
- domain：`domain/sub2api_provider.go`（`provider_type` 常量）
- handler：`handler/admin/sub2api_provider_handler.go`、`sub2api_optimize_schedule_handler.go`、`handler/dto/sub2api_provider.go`
- service：`sub2api_provider_service.go`、`sub2api_optimize_{account,runner,runner_service,schedule_service}.go`
- repository：`sub2api_provider_repository.go`、`sub2api_optimize_schedule_repository.go`
- pkg：`pkg/sub2api/{client,path_detector,token_cache}.go`
- 迁移：`migrations/030,158~164`

### 4.2 wire 依赖注入（关键，易漏）

DI 用 google/wire，新增 provider **必须**在对应 wire.go 注册，否则 `make generate` 失败：
- `service/wire.go` — `sub2api.NewTokenCache`、`ProvideSub2APIOptimizeRunnerService`
- `handler/wire.go` — `admin.NewSub2APIOptimizeScheduleHandler`
- `repository/wire.go` — `NewSub2APIOptimizeScheduleRepository`

> 曾踩坑：`wire_gen.go` 被手工编辑过导致上述 provider 未注册，同步后 `make generate` 报 "no provider" —— 已在各 wire.go 补全。

### 4.3 前端新增文件

- `views/admin/Sub2APIProvidersView.vue`（上游实例管理主视图）
- `components/admin/Sub2APIProviderActionMenu.vue`、`Sub2APIOptimizeScheduleModal.vue`
- `api/admin/sub2apiProviders.ts`、`utils/sub2apiValidation.ts`
- 路由 `router/index.ts`、侧边栏 `AppSidebar.vue` 各加一项

---

## 五、部署改动

### 5.1 自建镜像

`deploy/docker-compose.yml`：

```yaml
# 上游：weishaw/sub2api:latest
# fork：blsnt8586/sub2api:latest
image: blsnt8586/sub2api:latest
```

### 5.2 构建命令

前后端一体打包（推荐）：

```bash
bash deploy/build_image.sh          # 产出 sub2api:latest（前端内嵌进 Go 二进制）
docker tag sub2api:latest blsnt8586/sub2api:latest
docker push blsnt8586/sub2api:latest
```

仅后端二进制：

```bash
cd backend && make build            # 产出 backend/bin/server
```

### 5.3 开发辅助脚本（新增）

- `start-backend.sh` — 本地快速启动后端（`CONFIG_FILE=config.dev.yaml AUTO_SETUP=true`）
- `start-frontend.sh` — `cd frontend && pnpm dev`

---

## 五之二、OpenAI/Codex 全局 system prompt 注入（功能块 E）

管理端可配置一段全局 system prompt，注入到所有 OpenAI/Codex 请求。用于统一下发平台级规则（如禁止讨论竞品、固定角色设定等）。

### E.1 注入原理

- **目标字段**：顶层 `instructions`（string）。Codex（`/backend-api/codex/responses`）与 chat/completions 进网关后都转成 Responses 格式，系统指令统一走 `instructions`——ChatGPT 内部 Codex 端点不接受 `role:"system"`。
- **合并策略**：前置合并 `全局prompt + "\n\n" + 客户端原instructions`。保留客户端自己的指令，全局规则在前。客户端无 instructions 时直接设为全局 prompt。
- **幂等**：`instructions` 已以全局 prompt 开头则跳过，防止重试/多次调用重复注入。
- **缓存影响**：全局 prompt 固定不变→前缀稳定→同对话第二轮起正常命中上游 prompt cache，不破坏缓存。
- **默认关闭**（opt-in）：`enable_openai_system_prompt_injection` 默认 false，不改变既有行为。

### E.2 新增文件（低冲突，直接保留）

- `backend/internal/service/openai_system_prompt_inject.go` — 核心：`GetOpenAISystemPromptInjection`（60s TTL atomic 缓存 + singleflight）、`injectOpenAIGlobalInstructions`（gjson/sjson 合并）、`storeOpenAISystemPromptInjectCache` / `InvalidateOpenAISystemPromptInjectionCache`（保存后热更新）。只引用自身符号 + stdlib/gjson/sjson，零上游依赖。
- `backend/internal/service/openai_system_prompt_inject_test.go` — 6 用例（空值/合并/幂等）。

### E.3 上游钩子（2 处，纯新增，[§7 MEDIUM](#七冲突面分级high-conflict-必查) 已登记）

- `openai_gateway_forward.go`：`Forward` body 定稿后（image billing / WS / HTTP 转发之前）注入，覆盖 responses HTTP+WS 两条子路径。注入后重置 `requestView`/`reqBody` 让下游重新读取。`compatMessagesBridge`（messages 格式）跳过。
- `openai_gateway_chat_completions.go`：`ForwardAsChatCompletions` 的 `responsesBody = updatedBody` 后注入，覆盖 chat/completions。

> 未覆盖路径：passthrough 透传（设计即原样转发）、APIKey 不支持 Responses 的 raw chat completions 回退（messages 格式）。需要时另补。

### E.4 设置存储链路（8 文件，各 2~16 行纯新增，照抄 Claude OAuth 简版模式）

`domain_constants.go`（2 常量）、`settings_view.go` + `dto/settings.go`（各 2 字段）、`setting_parse.go` / `setting_update.go`（解析+写入+缓存热更新）、`setting_handler.go` / `setting_handler_update.go` / `setting_handler_audit.go`（DTO 映射+合并+审计）。

### E.5 前端（3 文件）

- `api/admin/settings.ts` — `SystemSettings` + `UpdateSettingsRequest` 各加 2 字段。
- `views/admin/SettingsView.vue` — 网关服务 tab 加 Toggle + textarea（form 默认值 + save payload）。
- `i18n/locales/{zh,en}/admin/settings.ts` — 各 5 条文案。

---

## 五之三、Codex 雷达（第三方数据代理，功能块 F）

用户端 + 管理端共用一个页面，展示第三方社区站点 codexradar.com 的 Codex 观测数据（额度重置窗口、24/48h 预测、降智分、漫画摘要图）。本平台仅做代理缓存 + 署名转载，页面醒目标注「数据非本站提供、来源第三方」并提供跳转原站的详情链接。**默认关闭（opt-in）**，第三方数据来源需管理员显式启用。

### F.1 设计要点

- **数据源**：图用稳定别名 `https://codexradar.com/assets/radar-high-readout-comic.png`（无时间戳，日更两次，CDN 4h 缓存）；摘要用公开 `https://codexradar.com/current.json`（`CORS:*`）。完整 API `/api/v1/current` 需授权（401），不使用。
- **后端代理缓存**：不让终端用户浏览器直连第三方（省对方带宽、不受其抖动影响、图走本平台域名）。进程内 `atomic.Value` 缓存，30 分钟 TTL。
- **图片后端优化（治大图跨境下载慢）**：源站漫画图约 2.2MB，跨境链路下载可达十余秒。拉取时在后端一次性 `optimizeCodexRadarImage`：若最长边超过 1080px 则等比缩放（绝不放大）+ 在 JPEG(q82)/PNG 里取更小的一份，通常压到几百 KB。优化只在拉取时做（不在请求热路径），任何解码/编码失败都原样透传原图（绝不因优化丢图）。`ETag` 改由**优化后字节内容**派生（`weakETag`，`W/"<sha256前16位>"`），内容不变则值稳定、支持 304。用 `x/image/draw`（CatmullRom，纯 Go，不破坏 `CGO_ENABLED=0` 静态编译）；JPEG 编码铺白底避免源图透明通道变黑。
- **懒加载 + stale-while-revalidate**：请求命中时按需刷新；缓存过期时先返回旧数据、后台异步刷新，不阻塞请求；失败按 30s 节流，绝不打爆对方。
- **定时预热（治冷启动加载失败）**：cron `0 7-15 * * *`（时区取 `cfg.Timezone`，默认 Asia/Shanghai），07:00–15:00 每小时整点各拉一次（共 9 次），整点覆盖对方两段日更期（上午 7–9 / 下午 13–15）；另在进程启动后延迟 5s 做一次启动预热，治「重启后缓存空、首个用户吃 30s 同步阻塞导致前端超时/404」。预热走 `forceRefresh`（绕 TTL，仍受 singleflight + 30s 节流保护），且**仅在功能开关开启时**才拉取第三方（关闭时 cron 触发即跳过，不打对方）。与上游解耦：只注入「功能是否开启」只读回调 + 时区，不持有 `SettingService` 引用。
- **鉴权**：图片接口需 JWT（Bearer 头），故前端用 axios 拉 blob → `URL.createObjectURL`，而非 `<img src>`（后者带不了 Authorization 头）。
- **合规**：对方 `current.json` 要求署名「数据来自 Codex 雷达 codexradar.com」，接口 + 页面均已带；「二次开发使用需授权」一节由用户自行决定是否走对方授权流程，代码侧仅做免责标注 + 署名。

### F.2 新增文件（零上游依赖，直接保留）

- `backend/internal/service/codexradar_service.go` — 核心：`NewCodexRadarService`、`EnsureFresh`（懒加载+SWR）、`ImageSnapshot`/`SummarySnapshot`（只读快照）、`ConfigureScheduler`/`Start`/`Stop`/`warmup`/`forceRefresh`（定时预热）、`optimizeCodexRadarImage`/`weakETag`（图片压缩 + 内容派生 ETag）。仅引用 stdlib + `singleflight` + `robfig/cron` + `x/image/draw`，零上游符号；定时预热只依赖一个「功能是否开启」的只读回调，与 `SettingService` 解耦。
- `backend/internal/service/codexradar_service_test.go` — 10 用例（拉取缓存/空态/SWR/上游报错保留旧数据 + 预热开关关闭跳过/开启拉取/forceRefresh 绕过 TTL/Start-Stop 幂等 + 图片缩放变小/非图透传）。
- `backend/internal/handler/codexradar_handler.go` — `Image`（ETag 协商缓存 + 私有 1h 缓存，源站日更两次）、`Summary`（原始 JSON + source/attribution/fetched_at 元信息）。开关关闭返回 403。
- `frontend/src/api/codexradar.ts` — `getCodexRadarSummary` + `fetchCodexRadarImageObjectURL`。
- `frontend/src/views/user/CodexRadarView.vue` — 页面：头部 + 免责说明块（跳转原站）+ 关键指标卡片 + 漫画图 + 底部署名。

### F.3 上游钩子

**零网关钩子。** 仅在以下上游文件做纯追加式接线（无逻辑侵入）：

- `service/wire.go`：`ProvideCodexRadarService`（原 `NewCodexRadarService`）加入 ProviderSet——该 provider 注入功能开关回调 + 时区（取 `cfg.Timezone`，默认 Asia/Shanghai）并 `Start()` 定时预热。
- `handler/wire.go` + `handler/handler.go`：`CodexRadarHandler` 加入 `Handlers` 结构体与 `ProvideHandlers`。
- `cmd/server/wire_gen.go`：手动补 `codexRadarService`（改用 `ProvideCodexRadarService(settingService, configConfig)`）/`codexRadarHandler` 两行构造 + `ProvideHandlers` 传参（wire 工具有类型检查 bug，手改）。**仅此一行替换**，不碰 `provideCleanup`——为保持上游足迹最小，定时预热不接入优雅关闭链路（缓存加热器，进程退出时 goroutine 随进程终止即可，无资源泄漏）。`cmd/server/wire.go`（wireinject）保持零改动。
- `server/routes/user.go`：已认证组下新增 `/codexradar/{image,summary}` 两条 GET（用户+管理员共用）。

### F.4 功能开关（opt-in 全链路，照抄 available_channels 模式）

`domain_constants.go`（`SettingKeyCodexRadarEnabled`）、`setting_public.go`（`IsCodexRadarEnabled` 运行时读取 + 公开注入 payload 字段）、`settings_view.go` + `dto/settings.go`（`SystemSettings`/`PublicSettings` 各加字段）、`setting_parse.go`（默认 false + 解析）、`setting_update.go`（写入）、`setting_handler.go` / `setting_handler_update.go` / `setting_handler_audit.go`（DTO 映射 + 合并 + 审计）、`api_contract_test.go`（快照补 `codex_radar_enabled`）。

前端功能开关：`utils/featureFlags.ts`（注册 `codexRadar` opt-in flag）、`types/index.ts` + `api/admin/settings.ts`（类型）、`stores/app.ts`（默认值）、`components/layout/AppSidebar.vue`（`buildSelfNavItems` 加入口，用户主菜单 + 管理员"我的账户"子菜单共享）、`router/index.ts`（`/codex-radar` 路由）、`views/admin/SettingsView.vue`（Toggle + form 默认值 + save payload）、`i18n/locales/{zh,en}`（`nav.codexRadar` + `codexRadar.*` 页面文案 + `admin.settings.features.codexRadar.*`）。

### F.5 恢复要点（换机/重装）

全部为新增文件 + opt-in 开关，上游同步几乎不冲突。若 `wire_gen.go` 被 `make generate` 覆盖，只需重新补 `codexRadarService`（用 `ProvideCodexRadarService(settingService, configConfig)`）/`codexRadarHandler` 两行构造 + `ProvideHandlers` 传参（见 F.3）——不涉及 `provideCleanup`。功能默认关闭，启用入口：系统设置 > 功能开关 > Codex 雷达。定时预热仅在开关开启时拉取第三方，07:00–15:00 每小时整点各一次（时区取 `config.timezone`，默认 Asia/Shanghai）。

---

## 六、解耦策略：降低上游同步冲突

fork 初期散落的平台分支（jimeng/grok 等）遍布 41 个文件，每次同步上游都与其文件拆分/重构冲突。
**Phase 1（已完成）**：把前端平台分支收敛到**单一权威文件** `frontend/src/utils/platformColors.ts`。

### 6.1 前端 platformColors.ts 注册表模式

**单一权威来源**：新增平台只需：
1. `Platform` 类型追加新值（如 `jimeng`）
2. `ALL_PLATFORMS` 数组追加
3. 各 `Record<Platform, string>` 映射补齐条目（`BADGE`、`TEXT`、`TAG` 等 10+ 个 Record）
4. `platformLabel()` 追加中文映射

**消费端统一调用**：组件不再硬编码 `switch`/`if`，改调 `platformTagClass(platform)`、`platformLabel(platform)` 等 accessor。

**覆盖率**：
- 已收敛（Wave 1~3）：
  - `AccountTableFilters.vue`、`OpsDashboardHeader.vue` — `platformSelectOptions()`
  - `UserPlatformQuotaModal.vue` — `ALL_PLATFORMS`
  - `GroupRateMultipliersModal.vue`、`GroupRPMOverridesModal.vue` — `platformStrongTextClass()`
  - `channel/types.ts` — `getPlatformTagClass()`、`getPlatformTextClass()` 转调 registry
  - `PlatformTypeBadge.vue` — `platformLabel()`、`platformTagClass()`、`platformTagSoftClass()`
- **待收敛（Phase 2）**：
  - `GroupBadge.vue` — subscription/standard 双变体徽章色（需新增 `platformBadgeSubscription()`/`platformBadgeStandard()` accessors）

### 6.2 后端 domain_constants.go 平台权威列表

`backend/internal/service/domain_constants.go` 的 `AllowedQuotaPlatforms` 是**后端单一权威来源**，
新增平台只需在此数组追加（目前含 anthropic/openai/gemini/antigravity/grok/**jimeng** 六项）。

### 6.3 Phase 2~3 规划（未启动）

- **Phase 2**（后端）：`platform_capabilities.go` 抽象平台能力语义（代替散落各处的 `platform == "jimeng"` 判断）
- **Phase 3**（后端）：视频计费逻辑提取到独立文件（`billing_video.go`），降低 `billing_service.go` 改动面

---

## 七、冲突面分级：HIGH-CONFLICT 必查

上游同步后，按优先级核对：

### 🔴 HIGH — 必查（上游常改、fork 也改）

| 文件 | fork 改动 | 同步后检查项 |
|------|----------|-------------|
| `api_key_auth.go` | `skipBilling` 含 `/v1/models` 与 `/v1/sub2api/canvas/model-caps` | 上游只有 `/v1/usage`，需补回这两项。model-caps 是静态元数据端点，若被计费拦住，前端会静默退回内置兜底表 |
| `routes/gateway.go` | 注册 `GET /v1/sub2api/canvas/model-caps`（在 `compositeTarget`/`requireGroupAnthropic` **之前**） | 位置不能挪到两个中间件之后 —— canvas 分组不是 Anthropic 平台，会被 `requireGroupAnthropic` 拒 |
| `gateway_video.go` / `gateway_audio.go` routes | jimeng 在 `videoGenerationHandler`/`videoStatusHandler`/`videoCancelHandler` 的分流 + 音频三端点 | 若上游改 `/v1/videos/*` 路由需重新插桩。**注意 `POST /v1/videos` 已被本 fork 移除**，上游若重新引入需确认不与 `/videos/generations` 冲突 |
| `openai_gateway_usage.go` | video 分支**置于 image 之前** | 上游若重构计费顺序，需保持 video > image 优先级 |
| `billing_service.go` | `CalculateVideoCost` 函数 | 上游若拆分文件，需迁移该函数 |
| `admin_group.go` | video 字段映射 | 上游若改 CreateGroup DTO，需同步 video 字段 |
| `HomeView.vue` | **全量重写**（功能块 G：深空风格首页，与上游完全分叉） | 同步冲突时整文件保留 fork 版本（`git checkout --ours`）；再人工核对上游是否新增了 `appStore`/公共设置字段需要接入 |
| `landing.ts`（zh/en） | 功能块 G 新增 compat/faq/heroTagline/footer.disclaimer 等 key | 冲突时保留双方 key 合并；fork 新增 key 不可丢 |

### 🟡 MEDIUM — 关注（上游可能重构）

| 文件 | fork 改动 | 同步后检查项 |
|------|----------|-------------|
| `domain_constants.go` | `PlatformJimeng` + `AllowedQuotaPlatforms` | 上游若改平台枚举，需补回 jimeng |
| `channel.go` | `BillingModeVideo*` 常量 | 上游若改计费模式枚举，需补回 video 模式 |
| `group.go` Ent schema | `video_price_*` 字段 | 上游若改 Group schema，冲突时保留 video 字段 |
| `platformColors.ts` | `jimeng` 条目 + Phase 1 收敛 | 上游若改颜色 token，需合并 jimeng 并保留 accessor 函数 |
| `CustomPageView.vue` / `RiskControlView.vue` | iframe `allow` 属性 | 上游若改 iframe 结构，需补回剪贴板授权（HomeView 的 iframe 授权已随功能块 G 整文件保留） |
| `openai_gateway_forward.go` | `Forward` body 定稿后的 `[CUSTOM]` 注入钩子（15 行，覆盖 responses HTTP+WS） | 上游高频重构 OpenAI 网关；若改 `if bodyModified` 块或重命名 `requestView`/`reqBody`/`compatMessagesBridge`，需重新贴钩子并核对变量名 |
| `openai_gateway_chat_completions.go` | `ForwardAsChatCompletions` 的 `[CUSTOM]` 注入钩子（7 行，锚点 `responsesBody = updatedBody` 后） | 上游若重构 chat→responses 转换流程，需重新定位注入点 |
| **`openai_gateway_scheduling.go`** | `normalizeOpenAICompatiblePlatform` 新增 jimeng 分支（7 行，`[CUSTOM]` 注释标记） | 上游若重构该函数（如改名/拆分/内联），需补回 `if platform == PlatformJimeng { return PlatformJimeng }` 分支；与 `account.go` 的 `IsOpenAICompatible` 逻辑绑定，两处必须同时维护 |
| **`handler/admin/group_handler.go`** | 新增 `import "github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"` + `GetCanvasPricingModels` handler（约 10 行，[CUSTOM]）；`CreateGroupRequest` / `UpdateGroupRequest` 各加 `ModelPricing *service.ModelPricingConfig` 字段（2 行） | 上游高频改 group handler DTO；同步冲突时需手动保留 `avi2api` import、`ModelPricing` 字段和 `GetCanvasPricingModels` 函数 |

### 🟢 LOW — 一般无冲突（fork 新增文件为主）

- Sub2API 管理全套文件（`sub2api_provider*`、`sub2api_optimize*`）— 上游未涉及，直接保留
- AVI2API 客户端（`pkg/avi2api/`、`jimeng_video_*.go`、`jimeng_audio_service.go`、`avi2api_image_service.go`）— 新增文件，直接保留
- **OpenAI/Codex 全局 system prompt 注入**（功能块 E）：核心逻辑全在新增文件 `openai_system_prompt_inject.go`(+test)，只引用自身符号 + stdlib/gjson/sjson，零上游依赖，直接保留
- 设置存储链路（功能块 E 的 setting 接入）：`domain_constants.go` / `settings_view.go` / `setting_parse.go` / `setting_update.go` / `dto/settings.go` / `setting_handler{,_update,_audit}.go` 各 2~16 行**纯新增**，与 Claude OAuth/Codex UA 等既有设置同模式；追加式改动，冲突概率低，冲突时照抄相邻设置的写法即可
- 开发脚本（`start-*.sh`）— 新增文件

---

## 八、换机 / 升级复原清单

上游新版本同步后，按此清单复原：

### 8.1 编译前检查（避免编译失败）

1. **Ent 生成代码**：若 `ent/schema/` 冲突，优先解决 schema，再 `go generate ./ent`，最后提交全部生成文件
2. **wire 依赖注入**：检查 `service/wire.go`、`handler/wire.go`、`repository/wire.go` 是否含 Sub2API optimize 相关 provider（[§4.2](#42-wire-依赖注入关键易漏)）
3. **迁移文件**：fork 新增的 `migrations/165*.sql` 保留，编号若与上游冲突需重命名
4. **前端类型**：若 `types/index.ts` 的 `AccountPlatform` / `GroupPlatform` 冲突，保留 `jimeng`

### 8.2 逐项复原（对照 [§7 冲突面](#七冲突面分级high-conflict-必查)）

按 HIGH → MEDIUM 优先级核对：
- ✅ `api_key_auth.go` 的 `skipBilling` 含 `/v1/models`
- ✅ `routes/gateway.go` 的 jimeng videoStatusHandler 分流
- ✅ `openai_gateway_usage.go` 的 video 分支优先级（image 之前）
- ✅ `billing_service.go` 的 `CalculateVideoCost` 函数
- ✅ `admin_group.go` 的 video 字段映射
- ✅ `domain_constants.go` / `channel.go` / `group.go` 的 jimeng/video 相关常量/字段
- ✅ iframe View 的 `allow` 属性
- ✅ `platformColors.ts` 的 jimeng 条目 + accessor 函数
- ✅ `openai_gateway_forward.go` / `openai_gateway_chat_completions.go` 的 `[CUSTOM]` system prompt 注入钩子（搜 `injectOpenAIGlobalInstructions`）
- ✅ 设置链路 8 文件的 `OpenAISystemPrompt` / `EnableOpenAISystemPromptInjection` 字段（搜 `OpenAISystemPrompt`）

### 8.3 编译 + 测试

```bash
# 后端
cd backend
go generate ./ent                   # 若改了 schema
make generate                       # 重新生成 wire
go test -tags=unit ./...
go test -tags=integration ./...
golangci-lint run ./...

# 前端
cd frontend
pnpm typecheck
pnpm test:run                       # 或 make test-frontend-critical（只跑关键路径）

# 一体构建
bash deploy/build_image.sh
```

### 8.4 Git 策略

fork 提交历史清晰记录了各功能块的合入点：
- `08edc867` — Nova iframe 对接（功能块 A）
- `f01b5f1d` — 即梦视频 + 计费（功能块 B）
- `48da50eb` / `1a7d895b` — Sub2API 管理（功能块 C）
- **Phase 1 前端收敛**（本批次）— 平台分支解耦（功能块 D）
- **OpenAI/Codex 全局 system prompt 注入**（本批次）— 功能块 E

若需回退某功能块，可 `git revert <commit-hash>`。复原时可 `git cherry-pick` 对应提交（注意冲突）。

---

## 九、附录：关键数据

- fork 领先上游：**13 commits**（截至 2026-07-08）
- 代码量增量：`+31909 / -149 lines`（净增 ~3.2 万行，主要是 Sub2API 管理 + ent 生成代码）
- 新增平台：**jimeng**（视频生成）
- 新增计费模式：`video` / `video_per_second`
- 新增上游类型：`sub2api` provider（Ent 实体 + 完整 CRUD + 定时优化 runner）
- 前端解耦进度：Phase 1 完成（platformColors 收敛），Phase 2~3 待启动
- 功能块 E：OpenAI/Codex 全局 system prompt 注入（合并到顶层 `instructions`，前置合并策略，默认 opt-in 关闭）

---

**最后更新**：2026-07-15（合并上游至 v0.1.156）

同步 v0.1.156 时的两处冲突/收敛决议（供下次同步参考）：

- **Grok base_url 路由收敛到上游**：`openai_gateway_chat_completions_raw.go` 的 `rawChatCompletionsURL` 原有二开（OAuth 走 xAI 官方白名单、api_key 放行第三方）与上游新增的 `grok_upstream_url.go`（`buildGrokChatCompletionsURL` + `grokBaseURLValidator`）冲突。上游实现是我方二开的**超集**（OAuth 官方恒信任 + 自定义域名按运营策略校验、api_key 走策略校验放行第三方，且带 OAuth token 防泄漏守卫），故**取上游版**，删除我方分支。延续 `29b0d5257`/`f181ab378` 的 Grok 向上游收敛趋势。
- **删除本地 `min` helper**：`sub2api_provider_service.go` 里 fork 自加的 `func min(a, b int) int` 遮蔽了 Go 1.21+ 内置泛型 `min`，导致上游新代码 `min(time.Duration, ...)` 编译失败。已删除，所有旧 `min(int,int)` 调用无缝落到内置 min。

**功能块 E（system prompt 注入，即 Grok「fufu」人设注入的载体）验证存活**：核心文件 `openai_system_prompt_inject.go` + 两处 `[CUSTOM]` 钩子（`openai_gateway_forward.go` / `openai_gateway_chat_completions.go`）+ 8 文件设置链路，合并后全部完好，锚点语义正确，单测通过。Grok 走 `OpenAIGatewayService`，注入照常生效。
