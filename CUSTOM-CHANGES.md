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
| B. 即梦（jimeng）视频平台 | 新增视频生成平台，按次/按秒计费，走 OpenAI 网关链路 | 中（新增为主，少量上游文件插桩） |
| C. 上游 Sub2API 管理 + 定时优化 | 管理端管理 `provider_type=sub2api` 上游实例，定时优化账号倍率区间 | 低（几乎全新增文件） |
| D. 平台分支解耦重构 | 把散落各处的平台 `switch/if` 收敛到 `platformColors.ts` / `domain_constants.go` | 降低未来冲突 |

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
- `backend/internal/pkg/jimeng/` — 平台协议客户端（`models.go` / `url.go` + 测试）
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
| `api_key_auth.go` | `skipBilling` 含 `/v1/models` | 上游只有 `/v1/usage`，需补回 `/v1/models` |
| `gateway.go` routes | jimeng `videoStatusHandler` 分流 | 若上游改 `/v1/videos/*` 路由需重新插桩 |
| `openai_gateway_usage.go` | video 分支**置于 image 之前** | 上游若重构计费顺序，需保持 video > image 优先级 |
| `billing_service.go` | `CalculateVideoCost` 函数 | 上游若拆分文件，需迁移该函数 |
| `admin_group.go` | video 字段映射 | 上游若改 CreateGroup DTO，需同步 video 字段 |

### 🟡 MEDIUM — 关注（上游可能重构）

| 文件 | fork 改动 | 同步后检查项 |
|------|----------|-------------|
| `domain_constants.go` | `PlatformJimeng` + `AllowedQuotaPlatforms` | 上游若改平台枚举，需补回 jimeng |
| `channel.go` | `BillingModeVideo*` 常量 | 上游若改计费模式枚举，需补回 video 模式 |
| `group.go` Ent schema | `video_price_*` 字段 | 上游若改 Group schema，冲突时保留 video 字段 |
| `platformColors.ts` | `jimeng` 条目 + Phase 1 收敛 | 上游若改颜色 token，需合并 jimeng 并保留 accessor 函数 |
| `HomeView.vue` / `CustomPageView.vue` / `RiskControlView.vue` | iframe `allow` 属性 | 上游若改 iframe 结构，需补回剪贴板授权 |

### 🟢 LOW — 一般无冲突（fork 新增文件为主）

- Sub2API 管理全套文件（`sub2api_provider*`、`sub2api_optimize*`）— 上游未涉及，直接保留
- jimeng 平台客户端（`pkg/jimeng/`、`jimeng_video_*.go`）— 新增文件，直接保留
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

若需回退某功能块，可 `git revert <commit-hash>`。复原时可 `git cherry-pick` 对应提交（注意冲突）。

---

## 九、附录：关键数据

- fork 领先上游：**13 commits**（截至 2026-07-08）
- 代码量增量：`+31909 / -149 lines`（净增 ~3.2 万行，主要是 Sub2API 管理 + ent 生成代码）
- 新增平台：**jimeng**（视频生成）
- 新增计费模式：`video` / `video_per_second`
- 新增上游类型：`sub2api` provider（Ent 实体 + 完整 CRUD + 定时优化 runner）
- 前端解耦进度：Phase 1 完成（platformColors 收敛），Phase 2~3 待启动

---

**最后更新**：2026-07-08（合并上游 144 commits + Phase 1 前端收敛）
