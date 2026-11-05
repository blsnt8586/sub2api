# Sub2API 自定义改动说明

> 本文档记录在上游 [Wei-Shaw/sub2api](https://github.com/Wei-Shaw/sub2api) 基础上的全部二次开发（fork）改动。
> 用途：换机部署 / 升级上游后快速定位与复原自定义代码。
>
> **维护原则**：本 fork 长期紧跟上游。为降低同步痛点，二开代码尽量**新增文件**、
> 并把散落的平台分支**收敛到单一权威文件**（见 [§6 解耦策略](#六解耦策略降低上游同步冲突)）。
> 每次同步上游后，对照 [§7 冲突面分级](#七冲突面分级high-conflict-必查) 逐项核对。

---

## 2026-11-05 工具下载

- 新增登录用户与管理员均可访问的 `/tool-downloads` 页面及侧栏「工具下载」入口。工具目录在 `tool_downloads_catalog.go` 统一登记，目前包含 `BigPizzaV3/CodexPlusPlus` 和 `farion1231/cc-switch`，每个工具独立维护版本、ETag、磁盘目录与更新任务，单一工具同步失败不影响其他工具。
- 页面使用可复用 `ToolDownloadCard.vue`，桌面双列、宽屏三列、手机单列；支持工具名称 / 用途搜索、Windows / macOS / Linux 筛选、卡片内系统与 x64 / ARM64 架构切换，安装版优先、便携版次之。元数据刷新失败仍保留下载，首次同步自动轮询到完成；支持浅色、深色与减少动态效果。
- 卡片使用等高网格，统一简介、下载区和底部信息的对齐，切换系统时保留下载区高度。Codex++ 和 CC Switch 使用各自官方 Logo 的本站 WebP 副本，来源、原始 blob 和许可证保存在 `frontend/public/tools/README.md`；移除头部重复的下载说明提示。
- Codex++ 提供 Windows x64 EXE / ZIP 和 macOS Universal DMG；CC Switch 提供 Windows x64 / ARM64 MSI / ZIP、macOS Universal DMG，以及 Linux x64 / ARM64 AppImage / DEB。仅接受各自登记的官方仓库附件，过滤签名附件和更新元数据。
- 后端独立 `ToolDownloadsService` 启动时检查官方 GitHub latest Release，成功后每 24 小时检查一次；singleflight 与 ETag 避免访客重复请求。服务器预先下载全部支持的安装包，校验长度与官方 SHA256，全部完成后原子发布磁盘 manifest；同步期间页面立即返回缓存，不等待大文件。失败继续提供上次完整版本，1 小时后重试。
- 安装包保存在 `TOOL_DOWNLOADS_DIR`；未配置时使用 `DATA_DIR/tool-downloads` 或 `pricing.data_dir/tool-downloads`（默认 `./data/tool-downloads`）。启动时校验持久化文件，保留当前和前一版本，成功更新后清理更旧版本。数据目录需要持久化和写权限，不嵌入发布二进制。
- 安装包按钮走本站 `GET/HEAD /api/v1/tool-downloads/files/:release/:name`，只允许 manifest 中的文件，支持 Range / HEAD / ETag；原生浏览器下载无需面板 bearer token，因此开源安装包端点公开，列表 `GET /api/v1/tool-downloads` 仍需登录。用户下载期间不请求或重定向 GitHub；项目主页和更新说明仍为可选官方链接。仅服务器同步需访问 GitHub 与其官方 Release CDN。
- 限制正式版、官方来源、附件数量、大小与文件名，使用临时目录和原子发布避免暴露半包。后台任务接入现有 Wire 清理链路，无数据库迁移、计费或网关调度改动。
- 每个工具最多并行同步 3 个文件，单文件失败最多重试 3 次；已完成且校验正确的暂存文件可在下次批次 / 重启后复用，未完成的 `.part` 不对外开放。整批完成后再发布新版本并清理旧暂存目录。

## 2026-10-25 上游增量同步（0.2.18）

- 从本地 `0bf4d6209` 合并上游 `ba81c7fd758e425d67f856d601147ef0c5263d3f` 的 7 个新增提交；上游版本号仍为 `0.2.18`。同步前回退分支为 `backup/pre-upstream-followup-20261025-0bf4d6209`，原有二开与未提交文件继续保留。
- 接入多图按实际出图张数计费、专用图片端点限制、按次渠道上下文分档及管理端开关。合并时保留 Canvas 媒体定价、分组模型定价及已有网关扩展；认证快照版本升至 `28`，拒绝旧版 `24`–`27` 并回源，新增兼容回归覆盖。
- 从合并后的 Ent schema 重新生成代码并运行 Wire；上游使用 `248_group_image_count_billing.sql`，New API 迁移顺延到 `257_newapi_provider.sql`，SQL 内容保持不变。未运行生产数据库迁移。
- 对齐前端 pnpm `9.15.9`、工作区声明与锁文件，锁定安装通过；前端 418 个测试文件、3,260 项测试及前端构建通过，lint 无错误（8 个现有警告）。Studio 导航测试与既有隐藏入口保持一致。
- 后端图片计费、认证快照、Provider/New API、心跳及故障切换相关目标测试和完整 migrations 包通过。完整 unit/integration 仍有既有失败，已在同步前 `0bf4d6209` 加原有未提交改动的隔离副本逐项复现，未发现本次新增失败；失败日志保存在 `.git/sync-20261024/logs/`。
- 本次仅本地同步与验证，不推送、不构建发布二进制、不上传、不部署、不重启生产服务。

---

## 2026-10-22 TOKEN 魔塔

- `/arcade/tower` 统一为「TOKEN 魔塔」五十层完整战役，只有一个游戏入口，不再通过 `campaign` 查询参数切换旧版。五十张固定 14×14 地图、五个区域、40 组敌人数值配置、五个强制首领封印，含三色门锁、跨层宝库 / 补给、装备成长、区域商店报价、先攻 / 连击 / 魔法能力与完整最终救援；数据由开发时脚本生成，不运行地图服务。
- 像素素材与基础规则移植自 MIT 开源 `Armour/Magic-Tower-Qt` revision `a32ee738485fa210e3a2b32cf57f506d51446c13`，五十层地图为本项目新内容。来源、许可证与历史参考见 `frontend/public/games/token-tower/README.md`。旧十层 / 初版源码保留作回归参考，产品没有旧版切换入口。
- 默认站点内嵌，开始 / 继续不自动沉浸；紧凑左侧状态栏、中央方形地图、右侧战况 / 目标 / 商店，间距 8px，手机上下适配。战斗包含精灵、血条、交替攻击、斩击、受击、暴击 / 闪避及跳过，先攻与双击伤害完整呈现，动画不重复结算。研究见 `docs/token-tower-layout-research.md`。
- 五十层独立浏览器存档 `token-tower-50-v1:<userId>`，旧存档留存且不会误载入。逻辑、战斗、存档在浏览器执行，无游戏 API、数据库、轮询或额度接入。服务器只承担正常鉴权 / 静态资源传输。两角色 × 三种子共六条合法完整通关回放覆盖 50 层 / 首领封印 / 跨层补给 / 逐层存读档；设计与边界见 `docs/token-tower-50-floor-campaign.md`。

## 2026-10-21 TOKEN 大鱼吃小鱼

- 游戏库第三张卡片替换为 `/arcade/ocean`「TOKEN 大鱼吃小鱼」，矿工、2048 与主机式游戏库继续可用；原垂钓路由及其会话成绩兼容保留。
- 使用原生 Canvas 2D：连续方向操作、鱼群追逃、六阶段吞食成长、三格生命、冲刺保护、黄金鱼、连吃倍率及狂暴。复用原创海底与鱼类 WebP，无新增引擎依赖。支持鼠标、触屏、键盘和标准手柄，含浅色 / 暗色外壳及沉浸模式。
- 自动暂停、输入清除、资源失败重试、暂停时尺寸重绘、账号切换隔离及局 ID 去重有对应验证。纯前端练习分数不产生 API 额度，玩法和美术来源见 `frontend/public/games/token-ocean/README.md`。

- 大鱼吃小鱼新增可选「猎手挑战」：限时黄金鱼潮 / 整群奖励与回血、带锁定路线的猎手冲锋 / 疲惫咬尾、成长满后独立三次咬尾决战及三星评价；保留轻松模式，重开保留玩法选择。金鱼会重新聚群，决战移除普通捕食鱼，避免不可读的围攻；补齐横屏摄像机比例。来源说明在 `docs/token-ocean-gameplay-research.md`。

- 大鱼吃小鱼精修：移除鱼体 / 鱼群 / 护盾等描边圈及常驻可吞食标签，保留轻量方向和弱点提示；加入成群游动 / 冲刺近距离连吞、缩入回弹、合并分数字样、4.5 秒狂暴反吃普通大鱼和可选合成音效。音效默认静音，按手势创建并在退出时释放；无新运行时依赖。

## 2026-10-20 TOKEN 乐园：游戏库、主机模式与三款游戏

- 新增登录后的 `/arcade` 游戏库，用户及管理员个人导航共用“TOKEN 乐园”入口（简易模式隐藏）；游戏单独路由为 `/arcade/miner`、`/arcade/2048` 与 `/arcade/fishing`。目录定义在 `frontend/src/games/catalog.ts`，新增游戏可继续扩展，封面文字、分类和最佳成绩均读取目录。
- 游戏库提供精选大封面、游戏卡片、动作 / 益智 / 休闲 / 最近游玩筛选、最佳成绩与本次会话统计。按 F 或点击“沉浸模式”切换主机大屏界面，Esc 返回普通模式；支持方向键、标准映射手柄的方向键 / 左摇杆、A 确认与菜单键切换模式。手柄支持基于浏览器 Gamepad API，页面失焦时暂停输入，切换游戏时阻止按住的 A 误触。
- `frontend/src/games/token-miner/engine.ts` 独立实现确定性矿场生成、摆钩、碰撞、重量回收、炸药、倒计时和三关结算；`renderer.ts` 用 Canvas 合成实际生成的矿洞、机器人、矿物、抓钩与炸药 WebP，绳索及抓取保持动态；图片源自用户授权的 `gpt-image-2`，运行时不使用生图凭据或外部服务。素材及完整提示词在 `frontend/public/games/token-miner/`。
- `frontend/src/views/user/TokenMinerView.vue` 提供键盘 / 点击 / 手柄操作、手机布局、浅色 / 深色主题、可选音效、标签页隐藏自动暂停、屏幕阅读器状态提示和减少动态效果支持。关卡目标为 650 / 1000 / 1350，均限时 60 秒，每关两枚炸药。
- `frontend/src/games/token-2048/engine.ts` 实现四方向移动、单次合并、计分、随机生成方块、胜负判断及达到 2048 后继续挑战；`Token2048View.vue` 提供键盘 / 手机滑动及方向按钮 / 手柄操作、撤销一步、重新开始、暂停与自动暂停。2048 封面由用户授权的 `gpt-image-2` 生成，游戏库素材来源和提示词见 `frontend/public/games/library/README.md`。
- `frontend/src/games/token-fishing/engine.ts` 实现 TOKEN 垂钓 2D 海湾探索：可见鱼群、瞄准 / 抛竿 / 鱼钩转向、六种鱼的追饵与捕食行为、小鱼 → 中鱼 → 大鱼食物链、出售或留饵、鱼线深度 / 鱼竿承重升级、图鉴及自动奖励目标，无倒计时。Canvas 使用原创生成的海湾、橘猫钓手和透明鱼精灵，绘制摆尾 / 追逐 / 捕食 / 弧线抛竿 / 溅水 / 动态鱼线，支持鼠标、触屏、键盘、手柄基本操作和沉浸模式。按账号隔离的浏览器存档保存金币、装备、图鉴、任务与岸边待处理渔获，存档经过版本 / 数值 / 鱼类校验；新进入时只把新增收益写入大厅会话记录。页面隐藏、失焦或离屏自动暂停，加载失败可重试；已移除 Three.js 依赖。素材通过用户授权的 `gpt-image-2` CLI 生成，来源、提示词及存档语义见 `frontend/public/games/token-fishing/README.md`。
- 垂钓扩展出航玩法：寻宝磁钩可独立打捞宝箱、移动水母与三格耐久、E / 手柄 X 冲刺闪避和吸附、三条海域的海流 / 危险 / 收益差异、21 个唯一宝箱收藏、三种港口委托、连钓售价加成及每三竿限时黄金鱼；宝箱回港才结算，断线丢货，允许原位重试。存档升级 version 2 并迁移旧币 / 装备 / 待售鱼；新保存海域、宝箱 ID、最高连钓与委托状态。动画、输入、旧存档恢复及奖励唯一性有对应测试，细节见垂钓 README。
- 修复垂钓默认无法咬钩：装备 `rig` 与港口 `contract` 解耦，默认鱼饵钩，旧存档保留任务进度并恢复可钓鱼状态；空钩允许水下切换，已上鱼 / 待售时禁止切换。
- 当前为纯前端练习模式：`frontend/src/stores/arcade.ts` 按登录用户隔离本次会话记录，按局 ID 去重，记录最高得分和通关状态；离开游戏后可在大厅查看，刷新清空，不产生 API 额度或余额变更。若接入真实奖励，必须另行实现服务端验证、唯一结算流水、事务入账及发放上限，不能把本地分数当作计费依据。
- 美术加载完成后才能开始游戏；失败可重试。开始/结算卡不模糊整个矿场，移动端保持完整显示。
- 修改入口包括 router、AppSidebar、共享 Icon 和中英文 locale；新增内容均按页面懒加载。引擎与页面交互测试位于各自 `__tests__` 目录。

---

## 2026-10-10 上游同步记录（0.2.15）

- 合并 `upstream/main` 至 `3a6fd1c9d`，同步前提交 `89ec44820`；回退分支 `backup/pre-upstream-20261010-89ec44820`。
- 吸收上游平台目录、Command Code/Cline、按模型协议路由和运维测试更新；保留 Canvas、Provider、智能分组、媒体计费和全局提示词注入。
- 平台目录合并时补回 fork 的 Canvas 登记，使动态前端平台选择器与后端平台清单保持一致；保留 Canvas 账号测试、OpenCode 兼容默认端点和 quota 行为。
- 验证：前端 `pnpm typecheck` 通过；后端目标 unit 包、server routes 与 migrations 测试通过。未执行推送、生产部署或生产数据库迁移。

---

## 2026-10-03 上游同步记录（0.2.13）

- 合并 `upstream/main` 至 `b8dece900`，同步前提交 `40d6aaaca`；回退分支 `backup/pre-upstream-20261003-40d6aaaca`。
- 上游新增 TypeSafe / Jev System One、充值优惠阶梯、API Key 删除后异步计费修复等。合并平台枚举、配额表单、账号默认 URL 和路由时，同时保留 fork 的 Canvas 与智能分组行为。
- 修正上游 `241_add_typesafe_platform.sql` 的配额约束，使 TypeSafe 与已有 Canvas 配额均可写入；补全前端 TypeSafe 配色。Ent 与 Wire 已重新生成。
- 核对 Canvas model-caps 路由、OpenAI 全局提示词钩子和 Provider Wire 装配仍在。未执行远程推送、生产部署或生产数据库迁移。
- 验证：后端全量 unit/integration 测试通过；前端类型检查及 353 文件 / 2677 项测试通过；`./build.sh` 完整 embed 构建通过，版本 0.2.13。

---

## 2026-09-30 上游同步记录（0.2.11）

- 合并 `upstream/main` 至 `42bc7f6cf`，新增 21 个上游提交；同步前提交 `9b66eac52`，回退分支 `backup/pre-upstream-0211-9b66eac52`。
- API Key 创建限制与智能分组常量同时保留；Wire 重新生成，Claude 额度重置接入幂等协调器和 Redis 锁，保留 Provider、Codex Radar 和智能分组依赖。Ent 重新生成无差异，无新增数据库迁移。
- 新默认配置：`billing.inflight_reservation.enabled=true`；`api_key_create.max_active_per_user=200`、`max_per_user_per_hour=60`。余额预留随异步计费任务释放；Canvas 继续走异步任务完成后的幂等计费，不为结果轮询增加预留。
- 合入 GPT-6.1 Sol、Astra Ultrafast 模型能力/定价、远程 Codex 模型目录及 Claude 手动额度重置。保留 Canvas、Provider、全局提示词注入等二开。
- 前端 351 文件 / 2651 项测试通过；完整 embed 构建通过，版本为 0.2.11；后端增量 golangci-lint 为 0 问题。前端 ESLint 仍是既有 7 错误、8 警告。
- 首轮后端测试中阿里云验证码本地 HTTP 测试受当前代理环境影响；去除大小写 HTTP_PROXY/HTTPS_PROXY/ALL_PROXY 后对应 repository 单元和集成测试均通过。
- 隔离代理后的 integration 全包命令通过；unit 各包已分轮通过，但隔离环境的全包复跑有一项既有 `TestValidateCreateParams_CheckModeMatrix` 因 `api.kimi.com` DNS 解析超时失败，随后该测试单独复核通过。未把分轮结果记为一次全包全绿。
- 本次未执行远程推送、生产部署或生产数据库操作。

---

## 2026-09-30 上游同步记录（0.2.10）

- 合并 `upstream/main` 至 `a0f41f95a`，包含 0.2.10 版本提交之后的 Claude Code 降级分组修复；同步前提交 `f2f439041`，回退分支为 `backup/pre-upstream-20260930-f2f439041`。
- 本次合入 105 个上游提交，涉及 215 个源码/配置文件。Wire 重新生成，接入 Claude 重置额度查询和复合分组 WebSocket 路由解析，并保留 Provider、智能分组和清理链路。Ent 重新生成无差异，无新增数据库迁移。
- Redis 使用上游 exec 列表启动形式，保留本地 `127.0.0.1:6381:6379` 映射；本地及完整栈 Compose 配置校验通过，未启动或重建容器。
- 已核对首页、Canvas 元数据免计费路由/媒体计费、Provider 扩展和全局提示词注入保留。Astra 紧凑探针兼容仍保留。
- 验证：后端 unit/integration 全包测试通过；前端 351 文件 / 2608 测试通过；`./build.sh`（i18n、类型检查、Vite、Go embed）通过，产物版本 `0.2.10`。
- golangci-lint v2.13.0 相对同步前为 0 新问题；前端 ESLint 仍有 7 项既有错误、8 项既有警告，已核对不由本次改动引入。
- 本次仅本地同步与构建，未执行生产部署或生产数据库操作。

---

## 2026-09-25 上游同步记录（0.2.8）

- 合并 `upstream/main` 至 `a3eb7ef30`，同步前提交 `a7a3aef68`；回退分支为 `backup/pre-upstream-20260925-a7a3aef68`。
- 账号批量更新同时保留 OpenCode Go 用量身份清理与 fork 的 `ReleaseProbeOwnershipOnNormalStop`；代理过期切换保留 Provider 绑定迁移，并采用上游的并发状态复核。
- 配额编辑器采用上游 `PLATFORM_QUOTA_PLATFORMS`，追加 Canvas，与后端 11 个允许平台对齐；回归覆盖 Canvas 和上游新增平台的限额保存。
- Wire 重新生成，保留 Provider/智能分组服务及清理链路，同时接入 Claude Code 版本同步与 OpenCode Go 用量服务。Ent 重新生成无差异。
- 首页、Canvas 路由/计费、Provider 扩展与全局提示词注入保留；Astra UA 已采用上游实现，紧凑探针的 token 上限兼容仍需保留。
- 新增上游迁移 `239_channel_reasoning_effort_multipliers.sql`、`240_affiliate_ledger_operation_id.sql`。本次未启动生产服务或执行生产迁移。
- 验证：后端 unit/integration 全包测试通过；前端 349 文件 / 2574 测试通过；`./build.sh`（i18n、类型检查、Vite、Go embed）通过。
- golangci-lint v2.13.0（Go 1.27.0 构建）相对同步前检查为 0 新问题；全量仍有 22 项既有问题。前端 ESLint 有 7 项既有错误、8 项既有警告，本次未扩大清理范围。

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
| H. 分组动态定价（已移除） | 历史 fork 功能；当前使用上游原生静态 `rate_multiplier` | 已退役（保留兼容迁移） |
| I. Codex Astra 临时兼容补丁 | 无效自定义 UA 回退时保留运行时同步版本；Astra OAuth 账号探测省略上游拒绝的 `max_output_tokens` | **临时、低（上游等价修复后删除并采用上游）** |

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
> `platform=canvas` 现在唯一对应 AVI2API 上游。原 vendor 名 `leonardo`
> 在代码中全部改为 `avi2api`。DB 里存量账号的 `credentials.vendor` 字段被忽略，
> **无需迁移**。

**设计目标**：接入 AVI2API 图像/视频/音频生成网关。历史平台标识 `jimeng`
已由迁移 `193_rename_canvas_platform.sql` 在 accounts/groups/user_platform_quotas
中统一重命名为 `canvas`，代码标识、管理端文案与对外契约均使用 Canvas/AVI2API。
迁移 `192a_user_platform_quotas_allow_canvas_rename.sql` 在重命名前临时兼容两个标识，
迁移 `223_user_platform_quotas_add_canvas.sql` 再把用户平台配额 CHECK 约束收敛到
`domain.AllPlatforms`，迁移 `224_user_platform_quotas_finalize_canvas.sql` 为已提前执行
193/223 的环境重复保证最终约束，确保 `canvas` 可以正常写入且不再接受 `jimeng`。

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
| `POST /v1/images/generations` | 同名（JSON 文生图或 multipart 图生图，返回异步 Task） |
| `GET /v1/images/{id}` | 同名 |
| `POST /v1/images/{id}/cancel` | 同名 |
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
AVI2API 对所有 POST 创建类接口（图像统一为 `/images/generations`，以及
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
- 图像：异步任务终态成功后按 `result.data` 实际产物数与实际 `width/height` 计费；
  分组使用 `canvas_image_price_per_count`（缺省时回退模型价格/旧档位逻辑）
- 视频：`CalculateJimengVideoCost`（按次 `video_price_per_count` / 按秒 `video_price_per_second`）
- 音频：按次 `VideoCount=1`；`music-v1` 的 `duration_minutes` × 60 记入 `VideoSeconds`
  （`dialogue-v3`/`sound-effects-v2` 的 `duration` 单位是秒，直接记入）
- Canvas 异步生图完成记录将 `/v1/images/{task_id}` 归一化为 `/v1/images`，与视频
  `/v1/videos/{task_id}` → `/v1/videos` 的统计口径一致；创建接口
  `/v1/images/generations` 单独保留。任务 ID 继续保存在稳定的 `request_id` 中，
  用于幂等扣费。旧 `/v1/tasks/*` 仅作为 Canvas worker 兼容别名保留。
- **无新增 schema 字段、无新增迁移**

**音频选号坑**：`runJimengAudioForwardLoop` 必须用
`SelectAccountWithSchedulerForCapability(..., service.PlatformJimeng)`。
通用的 `SelectAccountForModelWithExclusions` 内部硬编码 `PlatformOpenAI`
（`openai_gateway_scheduling.go:190`），用它会一个 jimeng 账号都命中不了。

**AVI2API 上游模型清单（v2.0.0）**：
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
- 视频 multipart 请求的提示词审计仍待归一化；图像 JSON/multipart 已在
  `CanvasAsyncImageModerationBody` 中统一提取 `prompt` 后进入 OpenAI Images 审计协议。

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

### 4.4 篡改猴浏览器凭据导入

- `frontend/public/sub2api-credential-exporter.user.js`：在已登录的现有 Sub2API 页面读取标准
  `auth_token` / `refresh_token` / `auth_user` localStorage，并在用户点击确认后导出版本化 JSON。
  脚本同时捕获当前域可读取的 Cookie；`GM_cookie.list` 不可用时回退 `document.cookie`，不发起
  跨域请求，也不把凭据上传到任何服务。
- 导出后打开站内凭据面板：明确显示 AT/RT 状态，最多展示 5 条 Cookie 名称、域名、路径和安全
  标记（不展示值），由用户点击按钮复制完整 JSON。剪贴板权限失败时可展开 JSON 手动复制。
- Sub2API 普通登录态主要保存在 localStorage Token 对中；导出结果为 0 个 Cookie 是合法状态，
  不影响 Token 对导入。Tampermonkey 稳定版也可能无法读取 HttpOnly Cookie。
- `frontend/src/utils/sub2apiCredentialBundle.ts`：严格校验格式版本、来源 Origin、包大小和 Token
  完整性。解析结果只暴露 Token 对、来源和 Cookie 数量，不暴露 Cookie 值。
- `frontend/src/components/admin/Sub2APICredentialBundleImport.vue`：上游表单中的安装/导入入口。
  导入后回填现有 `token_pair` 认证字段；原始 JSON 立即清空，Cookie 不进入保存请求或数据库。
- Refresh Token 会轮换。应使用专用或临时浏览器会话导出并在导入后关闭，避免浏览器和 Provider
  管理端并发刷新同一枚 RT。此能力不是新的上游授权协议，上游无需增加回调或私有 API。
- Cloudflare Cookie 通常绑定浏览器 IP、User-Agent 和挑战状态。复制 Cookie 不能保证生产服务器
  绕过 Cloudflare；Token 对只能免去交互式登录，后续 API 路径仍须允许服务器出口访问。
- 脚本 `1.2.0` 增加 New API：使用公开 `/api/status` 的成功响应与 `quota_per_unit` 识别站点；
  在用户点击后填写管理访问令牌及用户 ID（旧版 `localStorage.user.id` 仅用于预填）。以
  `credentials: omit`、Bearer 和 `New-Api-User` 请求当前站点 `/api/user/self`，核验返回 ID，
  不依靠浏览器 Cookie 通过验证，不创建或修改上游令牌。验证超时可重试，关闭时取消验证。
- New API 生成独立的 `newapi-browser-credentials` v1 JSON 包，只包含来源、管理令牌和用户 ID，
  不抓取 Cookie。New API 管理令牌表单可直接导入并回填 `access_token` 和用户 ID；解析器校验
  上游类型、来源、正整数 ID 和大小。模型调用的 `sk-` 令牌不能代替管理访问令牌；新版 New API
  的登录 Token 在内存中，不假定浏览器 localStorage 有可直接复用的管理凭据。

### 4.5 上游账户余额与远程分组倍率

吸收 `worryzyy/upstream-hub` 的余额/倍率监控思路，但复用本项目已有的 Provider token pair、
Refresh Token 轮换、Cloudflare 错误分类和管理面板，不引入它的 GORM channel/session 模型。
详细源码对比和分阶段方案见 `docs/UPSTREAM_HUB_INTEGRATION_ANALYSIS.md`。

- 后端 `GET /api/v1/admin/sub2api-providers/:id/remote-overview`：管理员主动实时读取并刷新 Redis；
  `GET /api/v1/admin/sub2api-providers/remote-overviews?ids=...` 批量读取本地快照，不访问上游。
- `pkg/sub2api/client.go`：通过 `/api/v1/auth/me` 读取上游登录账户余额，通过
  `/api/v1/groups/rates` 读取用户专属倍率。
- `service/sub2api_provider_service.go`：把专属倍率与已探测的 groups path 返回结果合并；
  专属倍率接口不可用时保留余额和分组，并回退分组默认倍率。
- 平台连接探针复用同一个已认证 Client 和 Groups 自动采集资产；Redis 仅保存最新成功快照与
  最近一次采集结果，TTL 24 小时。采集失败保留旧快照，且不影响平台健康状态。
- `Sub2APIProviderCard.vue`：卡片只显示余额、可见分组数和倍率范围；完整目录在独立对话框。
- 该数据是 Provider 登录账户的商业信息，不是账号真实模型可用性的证据，不能参与健康状态判断
  或自动触发分组切换。
- 第一阶段无 Ent schema、无 SQL migration、无独立定时任务，只复用平台连接探针周期，不影响
  现有生产数据；后续余额历史、倍率变化和通知必须另建独立表并先做旧库迁移演练。

### 4.6 OpenAI 账号文本探测使用独立会话（2026-10-27）

- `service/account_test_session.go` 为每次 OpenAI 文本测试生成新的 UUID，并将
  `session-id`、`session_id`、`conversation_id`、`X-Codex-Window-ID` 设为同一值。
  连字符形式用于兼容丢弃下划线请求头的反向代理。
- `service/account_test_service.go` 在 Responses 和 Chat Completions 测试的最终请求头
  处理处调用该 helper；手动测试、后台定时探测和分组候选测试共享这条路径。
  helper 在账号请求头覆写后运行，防止固定会话覆写重新引入跨探测粘性。
- 仅随机 `X-Codex-Window-ID` 不足以隔离探测：Sub2API 的会话调度不读取它，缺少显式
  会话标识时会退回内容哈希，重复问候或算术题会复用已有绑定。
- 不改变测试题、答案校验、鉴权 Key、分组、网关用户会话和现有故障恢复阈值。
  新会话只隔离已有会话绑定，不保证上游一定选择不同账号，也不证明历史超时的原因。
- 同步上游时保留 helper、两处测试路径调用及 `account_test_session_test.go` 回归测试；
  图片、原生 compact 探测及其他平台沿用各自协议的会话处理。

### 4.7 OpenAI API Key 故障有界切换与短时冷却（2026-10-28）

- `handler/failover_loop.go` 为 OpenAI API Key 原地重试增加硬上限：首次请求加两次
  重试，同账号最多三次；第三次仍失败后切换其他账号；显式禁用重试仍有效，
  OAuth 429 的专用窗口保持原有策略。Responses、Messages、Chat、Images 和 WS 的账号
  重试入口共用判断。
- OpenAI 池模式 API Key 健康熔断默认启用：2 分钟滚动窗口内累计 3 次符合条件的
  服务故障，临时冷却 1 分钟。使用既有 Redis 原子计数、DB 冷却字段和调度缓存通知；
  既有显式设置（包括 `enabled=false`）优先，不迁移或覆写生产设置。
- `service/openai_stream_health_error.go` 保留已输出后的流内失败、HTTP/2 读取中断的
  健康归属，但不包装成可重放错误。参数/策略拒绝、客户端取消、请求级容量降载及已有
  专用恢复策略的限流不计为这类账号故障。
- `service/openai_stream_leading_whitespace.go` 仅将开头纯空白文本及其 message/content
  事件留在缓冲中；可重试失败前尚无有效输出时可切号，成功时原样输出空白。工具事件、
  拒绝、推理和未知事件保持原有重放边界。透传首输出前缓冲增加总大小和行数限制。
- 回归覆盖真实 handler 调用顺序 `[A,A,A,B]`、空白占位后失败、HTTP/2 中断、成功保留
  缩进、有效输出后停止重放，以及显式关闭熔断。同步上游时保留这些测试及窄调用钩子。

---

## 四之二、分组动态定价（已移除）

此前 fork 曾增加按分组账号上游倍率自动调整 `groups.rate_multiplier` 的动态定价功能。
该功能现已停用，分组恢复使用上游原生的静态 `rate_multiplier`；用户专属分组倍率和利润控制
仍按各自原有逻辑生效。历史迁移 `238_group_dynamic_pricing.sql` 保留用于旧库兼容，
`242_remove_group_dynamic_pricing.sql` 会停止相关触发器并把动态分组恢复到其静态备用倍率。

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

用户端 + 管理端共用一个页面，展示第三方社区站点 codexradar.com 的 RadarBench 150 分制评分数据。本平台仅做代理缓存 + 署名转载，页面醒目标注「数据非本站提供、来源第三方」并提供跳转原站的详情链接。**默认关闭（opt-in）**，第三方数据来源需管理员显式启用。

### F.1 设计要点

- **数据源（2026-10 RadarBench）**：先读 `/data/radar-bench-binding.json`，再按 `model_efforts` 和绑定中的 `transport_origin` / `endpoint_path` 请求评分（当前 `https://api.codexradar.com/api/v1/radar-bench-score?model=…&effort=…&view=summary`；仅允许此已知传输地址），请求头为 `X-Radar-Bench-Selection: user-selected-v1`。旧三个接口已不用于默认抓取。最多 100 个档位、并发 3、总超时 90 秒；校验 benchmark/catalog/task digest 和 150 分制版本。部分失败可复用同题集 24 小时内的档位快照，保留读取时间并标记过期；全部失败保留缓存。
- **展示口径**：直接使用原站 RadarBench 150 分制总分与软件工程、视觉空间、终端操作、科学研究四个分项，不合成 IQ。显示判分题次、题目覆盖率和暂定状态；平均费用为官方 API 等价美元费用，平均耗时以已知样本计算，分项不提供独立费用。浏览器每分钟读取本平台缓存，后台每小时同步；超过一小时的观测显示缓存待更新。
- **后端代理缓存**：不让终端用户浏览器直连第三方；进程内 `atomic.Value` 缓存，1 小时 TTL。
- **懒加载 + stale-while-revalidate**：请求命中时按需刷新；缓存过期时先返回旧数据、后台异步刷新，不阻塞请求；失败按 30s 节流，绝不打爆对方。
- **定时预热（治冷启动加载失败）**：cron `0 * * * *`（时区取 `cfg.Timezone`，默认 Asia/Shanghai）每小时整点拉取；另在进程启动后延迟 5s 做一次启动预热。预热仅在功能开关开启时执行，与上游解耦。
- **合规**：接口和页面均带署名「数据来自 Codex 雷达 codexradar.com」及原站链接；不采集用户私密数据。

### F.2 新增文件（零上游依赖，直接保留）

- `backend/internal/service/codexradar_service.go` — 核心：`NewCodexRadarService`、`EnsureFresh`（懒加载+SWR）、`DataSnapshot`（RadarBench 快照通过 intelligence 字段返回）、`ConfigureScheduler`/`Start`/`Stop`/`warmup`/`forceRefresh`（每小时预热）。
- `backend/internal/service/codexradar_service_test.go` — 10 用例（拉取缓存/空态/SWR/上游报错保留旧数据 + 预热开关关闭跳过/开启拉取/forceRefresh 绕过 TTL/Start-Stop 幂等 + 图片缩放变小/非图透传）。
- `backend/internal/handler/codexradar_handler.go` — `Image`（ETag 协商缓存 + 私有 1h 缓存，源站日更两次）、`Summary`（原始 JSON + source/attribution/fetched_at 元信息）。开关关闭返回 403。
- `backend/internal/service/codexradar_bench.go` — 公开题集绑定、限并发抓取、版本校验和逐档位快照回退。
- `frontend/src/utils/radarBench.ts` — RadarBench 数据类型与响应校验。
- `frontend/src/api/codexradar.ts` — `getCodexRadarSummary`，冷启动请求超时 100 秒。
- `frontend/src/views/user/CodexRadarView.vue` — 页面：RadarBench 模型 × 推理档位矩阵、五个维度切换、定期读取缓存、加载/失败/空数据状态和来源署名。

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

全部为新增文件 + opt-in 开关，上游同步几乎不冲突。若 `wire_gen.go` 被 `make generate` 覆盖，只需重新补 `codexRadarService`（用 `ProvideCodexRadarService(settingService, configConfig)`）/`codexRadarHandler` 两行构造 + `ProvideHandlers` 传参（见 F.3）——不涉及 `provideCleanup`。功能默认关闭，启用入口：系统设置 > 功能开关 > Codex 雷达。定时预热仅在开关开启时拉取第三方，每小时整点一次（时区取 `config.timezone`，默认 Asia/Shanghai），缓存 TTL 同为 1 小时。

---

## 五之四、Codex Astra 临时兼容补丁（功能块 I）

> **临时补丁，优先收敛回上游。** 2026-09-06 在生产账号上验证：
> `gpt-6-astra` 使用 `version=0.146.0` 会返回“requires a newer version of Codex”，
> 使用 `0.153.4` 可正常返回 200；同时 ChatGPT Codex Astra 会拒绝账号探针携带的
> `max_output_tokens`。上游实现以下等价行为后，删除本节补丁并采用上游代码。

### I.1 无效 UA 不得拖低自动同步版本

2026-09-20：上游 `2d37088bd` 已提供等价修复。本地临时分支已移除，采用上游身份校验和运行时版本回退；保留运行时版本回归测试。I.2 探针兼容仍保留。

- 文件：`backend/internal/service/setting_gateway_runtime.go`
- 锚点：`[CUSTOM][TEMP-UPSTREAM-COMPAT]`
- 行为：管理员填写无法解析的完整 UA（例如仅填写 `codex`）时，丢弃无效 UA 外形，
  但继续用“面板版本 → 自动同步版本 → 编译期版本”的生效版本构造标准 Codex UA。
- 原问题：无效 UA 会让后续身份收口把 `User-Agent` 和 `version` 一起退回编译期
  `0.146.0`，即使面板与自动同步均已是 `0.153.4`，Astra 仍会误报客户端过旧。
- 回归测试：`TestGetOpenAICodexCanonicalUserAgentRebuildsPanelUAVersion/非_Codex_形态使用运行时版本构造标准身份`。

### I.2 Astra OAuth 账号探测省略不兼容 token 上限

- 文件：`backend/internal/service/account_test_probe_challenge.go`
- 锚点：`[CUSTOM][TEMP-UPSTREAM-COMPAT]`
- 行为：仅对 `isOAuth && gpt-6-astra` 的紧凑账号探测省略 `max_output_tokens`；
  其他模型和 API Key 请求仍保留探针 token 上限，不改变真实网关转发。
- 原问题：Astra 的 ChatGPT Codex `/responses` 当前返回
  `Unsupported parameter: max_output_tokens`；真实网关已有 rejected-field retry，
  账号测试路径尚未接入该兼容循环。
- 回归测试：`TestAccountProbeChallengeOmitsRejectedAstraOAuthTokenLimit`。

### I.3 上游同步时的替换条件

每次合并 `upstream/main` 时检查上游是否同时满足：

1. 自定义 UA 无效时仍使用当前运行时/自动同步 Codex 版本，而不是编译期旧版本；
2. OpenAI OAuth 账号测试对 Astra 不发送 `max_output_tokens`，或收到明确字段拒绝后安全重试。

若两项均已覆盖，删除上述两个 `[CUSTOM][TEMP-UPSTREAM-COMPAT]` 分支及对应本地测试，
保留上游测试与实现；若只覆盖一项，仅删除对应项。不要长期维护与上游重复的分支。

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
| `domain_constants.go` | `PlatformCanvas` + `AllowedQuotaPlatforms` | 上游若改平台枚举，需补回 canvas，并确保 `user_platform_quotas` CHECK 约束同步更新 |
| `channel.go` | `BillingModeVideo*` 常量 | 上游若改计费模式枚举，需补回 video 模式 |
| `group.go` Ent schema | `video_price_*` 字段 | 上游若改 Group schema，冲突时保留 video 字段 |
| `platformColors.ts` | `canvas` 条目 + Phase 1 收敛 | 上游若改颜色 token，需合并 canvas 并保留 accessor 函数 |
| `CustomPageView.vue` / `RiskControlView.vue` | iframe `allow` 属性 | 上游若改 iframe 结构，需补回剪贴板授权（HomeView 的 iframe 授权已随功能块 G 整文件保留） |
| `openai_gateway_forward.go` | `Forward` body 定稿后的 `[CUSTOM]` 注入钩子（15 行，覆盖 responses HTTP+WS） | 上游高频重构 OpenAI 网关；若改 `if bodyModified` 块或重命名 `requestView`/`reqBody`/`compatMessagesBridge`，需重新贴钩子并核对变量名 |
| `openai_gateway_chat_completions.go` | `ForwardAsChatCompletions` 的 `[CUSTOM]` 注入钩子（7 行，锚点 `responsesBody = updatedBody` 后） | 上游若重构 chat→responses 转换流程，需重新定位注入点 |
| **`openai_gateway_scheduling.go`** | `normalizeOpenAICompatiblePlatform` 新增 jimeng 分支（7 行，`[CUSTOM]` 注释标记） | 上游若重构该函数（如改名/拆分/内联），需补回 `if platform == PlatformJimeng { return PlatformJimeng }` 分支；与 `account.go` 的 `IsOpenAICompatible` 逻辑绑定，两处必须同时维护 |
| **`handler/admin/group_handler.go`** | 新增 `import "github.com/Wei-Shaw/sub2api/internal/pkg/avi2api"` + `GetCanvasPricingModels` handler（约 10 行，[CUSTOM]）；`CreateGroupRequest` / `UpdateGroupRequest` 各加 `CanvasModelPricing *service.ModelPricingConfig` 字段 | 上游高频改 group handler DTO；同步冲突时必须同时保留上游 `ModelPricing []service.ChannelModelPricing` 与 fork `CanvasModelPricing`，两者不能再合并为同一 JSON 字段 |
| `groups.model_pricing` / `groups.canvas_model_pricing` | 上游 `model_pricing` 是 `[]ChannelModelPricing` JSON 数组；fork 的 Canvas 图片/视频专属价是 `ModelPricingConfig` JSON 对象，固定存 `canvas_model_pricing` | 迁移 `222_canvas_model_pricing_split.sql` 用 `jsonb_typeof(model_pricing) = 'object'` 搬迁旧 Canvas 数据，数组原位保留。以后同步不得把 Canvas DTO、认证快照或前端编辑器重新绑定到 `model_pricing` |

### 🟢 LOW — 一般无冲突（fork 新增文件为主）

- Sub2API 管理全套文件（`sub2api_provider*`、`sub2api_optimize*`）— 上游未涉及，直接保留
- 上游管理日志强制保留 3 天：`Sub2APIProviderProbeRunner` 启动后及每小时通过独立 leader lock 分批物理删除 `sub2api_provider_probe_runs`、`sub2api_provider_probe_target_runs`、`sub2api_optimize_logs` 中 `created_at < now-72h` 的记录；同步时须保留 `DeleteUpstreamLogsBefore` 及其 runner 调用
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
3. **迁移文件**：fork 新增迁移（含 `165*.sql`、`192a_user_platform_quotas_allow_canvas_rename.sql`、`222_canvas_model_pricing_split.sql`、`223_user_platform_quotas_add_canvas.sql`、`224_user_platform_quotas_finalize_canvas.sql`）保留，编号若与上游冲突需重命名；不要修改已上线迁移内容
4. **前端类型**：若 `types/index.ts` 的 `AccountPlatform` / `GroupPlatform` 冲突，保留 `canvas`

### 8.2 逐项复原（对照 [§7 冲突面](#七冲突面分级high-conflict-必查)）

按 HIGH → MEDIUM 优先级核对：
- ✅ `api_key_auth.go` 的 `skipBilling` 含 `/v1/models`
- ✅ `routes/gateway.go` 的 canvas videoStatusHandler 分流
- ✅ `openai_gateway_usage.go` 的 video 分支优先级（image 之前）
- ✅ `billing_service.go` 的 `CalculateVideoCost` 函数
- ✅ `admin_group.go` 的 video 字段映射
- ✅ `domain_constants.go` / `channel.go` / `group.go` 的 canvas/video 相关常量/字段
- ✅ iframe View 的 `allow` 属性
- ✅ `platformColors.ts` 的 canvas 条目 + accessor 函数
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

### New API 上游管理适配（2026-10-20）

- `provider_type=newapi` 接入 QuantumNous/new-api 控制面；保留原 Sub2API 接口、账号关联、计费和探针共享逻辑。协议适配集中在 `backend/internal/pkg/sub2api/newapi_client.go`，前端在上游类型中选择 **New API**。
- 支持用户名密码登录（旧 Cookie / 新 Bearer 会话）及管理访问令牌。令牌模式填写所属用户的数字 ID，使用现有 `security.provider_token_key` 加密保存；不会把模型调用令牌当作管理凭据。密码模式需交互验证码、2FA 或加密密码挑战时，使用管理令牌。
- 对接 `/api/status`、`/api/user/self`、`/api/user/self/groups`（不回退公开 `/api/user/groups`，该接口无法可靠取得用户有效倍率）、`/api/token/` 分页与候选密钥读取 `/api/token/:id/key`。常规探测不读取完整密钥；明确关联账号时核验候选完整密钥，避免仅凭前后各 4 位掩码误关联。
- 余额和累计消耗分别取 `quota` / `used_quota`，先除以官方 `quota_per_unit`，再应用上游管理金额换算除数。官方 `logger.FormatQuota` 将人民币等作为显示转换，不应重复按站点充值单价换算。组倍率已经是用户有效倍率，不重复相乘；字符串组名映射为稳定且 JS 可精确表示的 ID。
- 动态 `auto` 分组不当作固定零倍率。暂不支持 New API 自动切组（缺少分组协议标识且令牌更新会覆盖其他配置）、充值拆分、缓存指标和按密钥周期利润，界面禁用相关切组入口，缺失统计显示不可用；不伪造充值和利润。
- 新增 `257_newapi_provider.sql` 放宽上游类型约束并限制认证方式组合；无新 Ent 字段、无共享网关计费逻辑修改。新增协议、身份、分页、会话、精确关联、金额和 UI 回归测试。
- 实站需要 New API 账号/管理令牌做认证验收；本次不写入生产配置、不推送、不部署，也不构建发布二进制。

New API 本次验证：客户端全包竞态测试通过；120 项上游管理顶层回归（含子测试共 187 项）竞态测试通过；17 项前端组件测试、i18n 完整性、类型检查、前端生产构建、变更文件 ESLint、相关包 go vet 通过；迁移在临时表事务中验证后回滚。全量单测 30 项与历史失败一致，另 1 项验证码传输测试单独重跑通过；集成测试的 1 项平台桶失败与历史一致。golangci-lint 因本机工具构建 Go 版本低于项目 Go 1.27.2 无法启动。

### OpenAI 流内错误切换与等待心跳（2026-10-21）

- 统一记录并发槽位/消息队列等待阶段的 SSE 心跳字节，OpenAI 首输出与 handler 换号判断同时扣除这些字节。修复只有等待心跳时被误判为已输出、普通流内 upstream_error 无法换号的问题；真实模型输出后的禁止重放策略保留。
- native 与 passthrough SSE 在可重试错误被输出状态拦截时，记录请求关联日志、写出字节、扣除心跳后的字节和输出标记，不记录正文或凭据。API Key 裸 error 事件在输出后也保留 ops 上游错误归属。
- 回归覆盖两条 SSE 路径、等待心跳有无、真实输出前后，以及 handler 到 service 的心跳计数衔接。45 服务器截图中的三次历史错误缺少输出门判定日志，不能仅凭复现认定都是等待心跳造成；本次没有修改生产配置或部署。


### TOKEN Ocean 生物扩充与递增难度（2026-10-21）

- 新增 8 种原创手绘鱼与河豚鼓起变体，海域总计 14 种生物；素材、提示词和来源筛选见 `frontend/public/games/token-ocean/README.md` / `docs/token-ocean-gameplay-research.md`。
- `species.ts` 分离鱼种与六阶段玩家成长；`ecology.ts` 实现河豚收刺、狮子鱼毒鳍、梭鱼预警突袭、沙丁鱼体力奖励与三档警戒，场内给出近距离规则提示，游戏下方提供折叠图鉴。
- 调整鱼群密度、补充 / 远距回收、捕食者比例和猎手节奏；决战清除特殊危害且保留读招与反击窗口。移除的玩家描边圈、速度线与残影不再引入。仍为本地练习分，不接入余额 / 计费。

### TOKEN Ocean 深海远征（2026-10-21）

- 新增「深海远征」模式，保留猎手挑战与轻松开吃。远征可在最终进化后继续游玩，按日光浅礁、暮光水域、幽蓝深渊推进，并在 HUD 显示深度、水层、发现数和遗物进度。
- 增加 9 种深海生物，图鉴总数扩展到 23；深海猎食者按水层限制出现。皇带鱼、蝠鲼、鲸鲨、蓝鲸作为不会攻击且不可吞食的温和巨兽，触发限时六光晶伴游路线。
- 增加 3 枚深海遗物和一次性收集奖励；遗物、光晶、生物发现均有唯一性保护，远征状态不会污染挑战模式重置。深海场景、鱼类和巨兽使用本地原创位图素材。
- 页面模式选择改为稳定三列网格，补齐窄手机字体、间距和横屏规则；新增探索引擎、页面模式切换与完整图鉴回归测试，移动布局通过浏览器检查。仍为本地练习分，不接入余额、API 配额或计费。


### TOKEN 乐园长局与随机玩法（2026-10-23）

- TOKEN 矿工从三关扩展为八关，增加随机矿况、浮动矿值、星标幸运矿及关间三选一装备；三项装备分别改善回收速度、后续时间和晶体价值。手机结算与升级沿页面展开。成绩按完整探险 ID 累计，失败重试保留此前成绩和装备。
- TOKEN 大鱼吃小鱼的猎手挑战增加浅礁 / 暮光 / 深渊三段随机狩猎目标，全部完成且满成长才进入决战；挑战和无尽远征新增随机鱼潮与顺流窗口，保留轻松模式原有通关规则。
- 继续使用原有本地位图素材和浏览器模拟；未改 API 计费或增加真实额度、全站排行榜。魔塔仍只提供一个 50 层游戏，其静态关卡是基于参考项目的十层素材和机制扩展，参考项目本身不是五十层。
- 规则、随机范围与资源说明见 `frontend/public/games/token-miner/README.md` 与 `frontend/public/games/token-ocean/README.md`。


### TOKEN 乐园百关矿工与六十关海洋（2026-10-23）

- 替代先前矿工八关：100 关 / 10 矿区、五种布局、实际晶体回收指标，每十关升级装备；固定本关种子重试，完整百关才计胜利。侧栏仅展示当前十关，避免百行撑高布局。
- 替代海洋多模式：唯一 60 关 / 10 轮挑战，三水层、三类任务、每六关猎手战，时限及后期冲锋压力递增；删除轻松 / 无尽选项和 mode 分支。任务、成长与首领各自校验，阶段过关不同于全程胜利。
- 新增按账号隔离的本地关卡检查点，刷新从当前关起点续玩；合法范围校验、存储不可用容错，不增加游戏服务端或真实额度接口。
- 核实百层魔塔候选 `HaoDaYiGuoFan/Mota100Floors` 确有 1–100 全部 JSON；WPF 且未发现 LICENSE，仅作为研究来源，未拷贝实现和素材。现有魔塔保持当前 50 层，具体核查见 `docs/token-park-long-campaign-research.md`。


### TOKEN 魔塔百层扩展（2026-10-23）

- 同一 `/arcade/tower` 扩展为 100 层 / 10 区 / 10 位首领 / 80 组怪物配置，后半新增沙海、风暴、冥河、星界、永夜。50 层只通往下一层，100 层救援需全部十道封印。
- 前 50 层与前 40 组怪物配置保留，50 层终点由救援变为楼梯；后半程逐区成长 / 补给 / 首领曲线经六条完整合法路线验证。星级阈值按百层调整，UI与图鉴补齐中英文，封印两排展示。
- `token-tower-100-v1:<userId>` 首次缺失时严格验证并迁移旧五十层存档，保留旧键；旧胜利档可继续后半程并移除旧最终积分，清空旧撤销历史。新档优先，不重复迁移。
- 数据与 Python 作者工具在前端游戏目录，计算 / 动画 / 存档由浏览器执行，无新游戏后端或额度发放。来源与设计说明见 `docs/token-tower-100-floor-campaign.md`。


### 上游管理利润按本地请求估算（2026-11-03）

- 今日、本月、累计利润只统计本地 `usage_logs`，不参与上游充值、钱包余额或远端 API Key 消费的对账。普通倍率计费时等价于请求基础费用 ×（本地有效售价倍率 − 上游分组倍率）；通用公式为 `actual_cost - COALESCE(account_stats_cost, total_cost) * accounts.remote_group_multiplier`，保留自定义成本定价与负利润。
- 上游分组倍率使用账号最近同步值，不套用可能独立配置的 `account_rate_multiplier`，也不再次应用币值除数；未记录历史上游分组倍率的旧请求按当前同步倍率估算，缺少上游倍率且有请求时不显示利润。累计范围为本地仍留存的全部请求，本月按应用时区自然月计算。
- `/remote-overviews` 轮询以本地查询重新投影利润，远端钱包快照失效或缺失时仍可显示本地利润；不改共享计费和余额，不为计算利润主动请求上游。

### 上游利润详情独立刷新（2026-11-03）

- `Sub2APIProviderRemoteDetails.vue` 独立承载利润概览、账号今日 / 累计明细、可展开搜索的上游资产和分组倍率；金额固定两位小数，账号数字居中，窄屏改用账号卡片，无分组内部滚动容器。
- 打开详情及“刷新利润”仅调用 `/remote-overviews` 读取本地请求统计与缓存；即使没有余额快照，也不自动调用上游。“同步上游资产”是独立的显式操作，调用原实时资产接口；利润与资产各显示自己的更新时间。
- 余额读取失败不阻断本地利润；成本未知时明确标记缺少倍率，不以零成本展示利润。统计口径折叠说明本地记录保留范围及当前上游倍率用于历史成本估算的边界。

### 上游账号改用用户请求监控（2026-11-03）

- Provider 后台周期只执行平台登录 / 余额 / 分组等控制面检查和本地被动健康评估；账号主动模型探测从调度入口及目标执行条件两层禁止。旧目标启用、间隔和健康降频配置不再产生自动模型请求，也不再触发依赖主动探测结果的自动选组测试。显式手动账号测试接口仍可供管理员使用。
- 账号状态、最近 24 小时最多 60 个时间条及请求历史均来自本地 `usage_logs` / `ops_error_logs`；不混用旧主动探测记录，不制造健康样本。无近期样本显示未知；旧主动探针关闭不影响被动观察，媒体账号也可由实际请求提供样本。
- 汇总与逐条历史共享错误归属及去重查询：同账号成功覆盖中间错误，换号成功不清除原账号失败，排除用户取消 / 参数及计费等非上游故障；分组变更前及未来记录不参与当前路由判断。流式成功使用首字时间，非流式使用耗时；缺少流式首字时间时不以整段生成时间推断过慢。
- 账号配置面板替换为只读请求监控，保留 Provider 级被动阈值和原有执行开关。启用临时停调后按真实失败 / P95 证据写入临时冷却及调度 outbox，冷却到期由后续实际请求验证恢复，不轮询延长旧故障，不修改管理员状态。
- 验证覆盖禁用自动探测、真实 PostgreSQL 中的归属 / 重试去重 / 换号 / 首字延迟 / 分组边界 / 条数限制、只读前端和时间条兼容；数据库测试使用独立连接的临时表，不修改本地导入的账号或请求。

### 每账号被动监控与主动恢复结合（2026-11-03，替代上一节纯被动方案）

- 新迁移 `262_provider_account_hybrid_health.sql` 保存每账号独立策略及持久恢复状态，Provider 统一阈值界面被各账号展开区中的配置替代；默认启用，连续失败 2 次、首响应 60 秒、恢复探测间隔 60 秒、探测超时 60 秒、连续成功 2 次恢复，均可逐账号修改。
- 正常时仅统计用户请求；观察窗口内最新连续上游错误达到该账号阈值，或一次首响应超时，下一次后台检查（约 15 秒）原子写入账号隔离与 scheduler outbox，随后仅对隔离账号发起独立挑战应答恢复测试。请求参数 / 取消等归属过滤和流式首字统计继续保留，不在此模块强行截断已经转发的用户请求。
- 恢复状态不会因冷却到期自动放行：连续测试成功才清除本状态机拥有的隔离，失败则清零成功次数；恢复完成后停止主动测试并从新的请求证据开始判断，避免历史故障再次触发。测试模型来自账号设置；未配置或禁止媒体测试时保留隔离并提示原因。
- 保存配置、改组 / 换 Key、人工停用、重新绑定与在途恢复结果通过账号锁、策略版本、隔离唯一标识及管理员状态版本保护；旧结果不能覆盖新事故或手动操作。只恢复运行时 / 探测器标记的错误，不接管任意手动错误、配额和外部停调；恢复更新调度 outbox 并失效临时停调缓存。
- 各账号展示用户请求与恢复探测两种来源、连续恢复进度、下次探测时间和异常原因。新策略从初始化 / 策略变更 / 成功恢复后的证据开始，历史主动探针配置仍存储兼容，但不会给正常账号自动发模型请求。主动恢复会产生正常上游测试费用。
- PostgreSQL 临时表验证隔离不随时间过期、恢复失败清零、连续成功恢复、停止探测、账号配置互不影响、策略保存 / 手动停用 / 换组与重复完成结果防护；模型调用用 stub，不发送真实收费测试。
