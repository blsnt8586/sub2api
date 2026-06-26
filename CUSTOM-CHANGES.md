# Sub2API 自定义改动说明

> 本文档记录在上游 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 基础上，为对接 **Nova Image Studio**（iframe 嵌入）所做的全部自定义改动。
> 用途：换机部署 / 升级上游后快速复原。

---

## 一、改动总览

| # | 类型 | 文件 | 状态 | 作用 |
|---|------|------|------|------|
| 1 | 后端 Go | `backend/internal/server/middleware/api_key_auth.go` | 已提交 `08edc867` | `/v1/models` 跳过计费，仅鉴权 |
| 2 | 前端 iframe | `HomeView.vue` / `CustomPageView.vue` / `RiskControlView.vue` | 已提交 `08edc867` | iframe 加 clipboard/fullscreen 权限 |
| 3 | 前端导航 | `frontend/src/views/user/CustomPageView.vue` | **未提交** | 监听嵌入页 postMessage 做白名单路由跳转 |
| 4 | 部署 | `deploy/docker-compose.yml` | **未提交** | 镜像改为自建 `blsnt8586/sub2api:latest` |
| 5 | 二进制 | `sub2api-custom`（仓库根，94MB） | 未跟踪 | 自定义构建的 Go 静态二进制产物 |

---

## 二、后端改动

### 2.1 `/v1/models` 跳过计费

`backend/internal/server/middleware/api_key_auth.go`：

```go
// 改动前
skipBilling := c.Request.URL.Path == "/v1/usage"
// 改动后
skipBilling := c.Request.URL.Path == "/v1/usage" || c.Request.URL.Path == "/v1/models"
```

**原因**：Nova 需要拉模型列表（`/v1/models`），此接口只做鉴权、不应计费/扣订阅额度。

---

## 三、前端改动

### 3.1 iframe 剪贴板/全屏权限（已提交）

`HomeView.vue` / `CustomPageView.vue` / `RiskControlView.vue` 三处嵌入 Nova 的 iframe 均加上：

```html
<iframe
  ... 
  allow="clipboard-write; clipboard-read; fullscreen"
  allowfullscreen
></iframe>
```

**原因**：Nova 在 iframe 内复制图片时被浏览器拦截（Clipboard API 默认禁用），父页面必须显式授权。

### 3.2 嵌入页导航联动（未提交）

`frontend/src/views/user/CustomPageView.vue` 监听 Nova 通过 `postMessage` 发来的导航请求，在 sub2api **当前页内路由跳转**（不整页刷新），体验更顺滑。

核心逻辑：

```ts
// 白名单：target → 路由路径(避免任意跳转)
const NAVIGATE_TARGETS: Record<string, string> = { keys: '/keys' }

function handleEmbeddedMessage(event: MessageEvent) {
  const data = event.data
  if (!data || typeof data !== 'object' || data.type !== 'sub2api:navigate') return
  // 安全：仅接受来自当前 iframe origin 的消息
  const expectedOrigin = new URL(embeddedUrl.value).origin
  if (event.origin !== expectedOrigin) return
  const path = NAVIGATE_TARGETS[String(data.target)]
  if (path) void router.push(path)
}

onMounted(() => window.addEventListener('message', handleEmbeddedMessage))
onUnmounted(() => window.removeEventListener('message', handleEmbeddedMessage))
```

**安全要点**：①消息类型固定 `sub2api:navigate`；②校验 `event.origin` 与嵌入页 origin 一致；③`target` 必须命中白名单（当前仅 `keys → /keys`）。Nova 侧发送示例：`parent.postMessage({ type: 'sub2api:navigate', target: 'keys' }, '*')`。

---

## 四、部署改动

### 4.1 自建镜像

`deploy/docker-compose.yml`：

```yaml
# 改动前
image: weishaw/sub2api:latest
# 改动后
image: blsnt8586/sub2api:latest
```

### 4.2 二进制 / 镜像构建

仓库根的 `sub2api-custom` 是自定义构建的 Go 静态二进制（ELF 64-bit, statically linked, stripped, ~94MB），含上述后端改动。

构建镜像（推荐，前后端一体打包）：

```bash
# 本地构建(脚本封装好了构建参数，含国内 goproxy)
bash deploy/build_image.sh          # 产出 sub2api:latest

# 打 tag 并推到自己的 registry
docker tag sub2api:latest blsnt8586/sub2api:latest
docker push blsnt8586/sub2api:latest
```

Dockerfile 流程：①Node 阶段 `pnpm run build` 编译前端 → ②Go 阶段 `CGO_ENABLED=0 go build -o /app/sub2api`（内嵌前端 dist）→ ③Alpine 运行时镜像。

仅构建后端二进制（如需单独替换）：

```bash
cd backend && make build        # 产出 backend/bin/server
```

---

## 五、换机/升级复原清单

上游有新版本想合并时，按序复原以下改动：

1. **后端**：`api_key_auth.go` 的 `skipBilling` 加回 `/v1/models`
2. **前端 iframe**：三个 View 的 `allow="clipboard-write; clipboard-read; fullscreen"`
3. **前端导航**：`CustomPageView.vue` 的 `handleEmbeddedMessage` + 白名单
4. **部署**：`docker-compose.yml` 镜像名改为自建
5. **重新构建**：`bash deploy/build_image.sh` → tag → push
6. 改动 1~3 已在提交 `08edc867`，可 `git cherry-pick 08edc867` 快速套用（注意 3 当前未提交，需另外补）


