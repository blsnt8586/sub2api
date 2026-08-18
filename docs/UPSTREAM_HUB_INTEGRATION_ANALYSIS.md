# upstream-hub 能力分析与 Sub2API 集成方案

## 1. 调研基线

- 项目：<https://github.com/worryzyy/upstream-hub>
- 调研提交：`b276f58252939313a9628f05bed382b01b74004a`
- 分支/版本：`main` / `v0.1.0`
- 调研日期：2026-08-17

本结论来自上述提交的后端 connector、monitor、scheduler、storage、notify 与 migration 实现，不只依据 README。

## 2. 项目定位

`upstream-hub` 是 NewAPI/Sub2API 站点的独立监控面板。它把一个 channel 定义为“可登录的上游站点账号”，定时读取：

1. 上游账号余额；
2. 当前账号可见的分组和倍率；
3. 余额与倍率历史；
4. 低余额、倍率变化等通知事件。

它还包含账号密码登录、Turnstile 第三方打码、多通知渠道、扫描进度 SSE 和历史数据清理。

## 3. 与现有 Sub2API 的对象边界

两个项目的核心对象不能直接等同：

| 概念 | upstream-hub | 当前 Sub2API |
| --- | --- | --- |
| 上游对象 | 一个站点登录账号 | 一个 `Sub2APIProvider` 实例 |
| 下挂对象 | 余额、全部可见分组 | 多个真实 AI 平台账号 |
| 健康判断 | 能否登录和读取站点接口 | 平台连接探针 + 每账号真实模型探针 |
| 认证 | access token、cookie 或账号密码 | 密码或加密 token pair，支持 refresh token 轮换 |
| 调度 | 周期扫描余额/倍率 | 探针周期、账号分组优化、探针联动切组 |

因此，余额和倍率应命名为“上游账户余额”和“远程分组倍率”，不能混入账号独立探针，也不能用它们证明某个模型账号可用。

## 4. 能力矩阵

| upstream-hub 能力 | 当前 Sub2API | 结论 |
| --- | --- | --- |
| Sub2API 登录与 token 复用 | 已有，且支持 refresh token 轮换和加密持久化 | 复用现有实现 |
| Cloudflare 错误识别 | 已有 challenge/access 分类和 HTML 脱敏 | 现有实现更完整 |
| 每账号真实业务探针 | 已有 | 不重复实现 |
| 上游账户余额 | 原来没有 | 高价值，优先接入 |
| 全部远程分组倍率 | 只有已绑定账号的当前分组倍率 | 高价值，独立接入 |
| 余额快照/趋势 | 没有 Provider 级历史 | 第二阶段 |
| 倍率变化历史 | 有切组日志，但没有远程倍率目录变化历史 | 第二阶段 |
| 低余额/倍率变化通知 | 没有完整 Provider 事件通知 | 第三阶段，先复用邮件/系统通知 |
| Telegram/钉钉/飞书/Bark 等 | 没有统一外部通知 dispatcher | 后续按需求建设，不直接复制 |
| Turnstile 自动打码 | 当前采用浏览器登录后导入 token/cookie 路线 | 默认不接入，存在安全、合规和封号风险 |
| 扫描进度 SSE | 当前探针已有动态轮询和局部更新 | 暂无必要新增 |

## 5. 第一阶段实现

第一阶段采用无数据库迁移的实时读取与 Redis 最新快照方案：

- `GET /api/v1/admin/sub2api-providers/:id/remote-overview`
- `GET /api/v1/admin/sub2api-providers/remote-overviews?ids=...`
- 使用 Provider 现有密码/token pair 认证；
- 调用远程 `/api/v1/auth/me` 读取余额；
- 调用已探测的 groups path（默认 `/api/v1/groups/available`）读取分组；
- 调用 `/api/v1/groups/rates` 合并当前登录用户的专属倍率；
- 老版本上游没有 `/groups/rates` 时，降级显示分组默认倍率；
- Provider 卡片只展示余额、分组数和倍率范围；
- 完整分组明细在独立对话框显示；
- 平台连接探针复用同一个已认证 Client 和已读取的 Groups 自动采集，不增加独立定时器；
- 管理页面的 15 秒轮询只通过批量接口读取本地 Redis，不产生 N+1 上游请求；
- Redis 仅保留最新成功快照和最近一次采集结果，TTL 为 24 小时；失败保留上次成功数据；
- 手动读取和平台探针并发时按采集时间条件更新，旧结果不会覆盖新结果；
- 资产失败不改变 Provider 的健康状态，也不改变账号或调度状态；
- 快照不落数据库，无 Ent schema 和 SQL migration。

这一阶段的目的，是先验证不同现场版本的响应兼容性，避免尚未稳定时就引入生产表结构和定时扫描。

## 6. 推荐后续阶段

### 第二阶段：历史与变化检测

建议新增独立表，而不是把历史塞进探针详情 JSON：

- `sub2api_provider_balance_snapshots`
  - `provider_id`
  - `balance`
  - `sampled_at`
- `sub2api_provider_group_rate_snapshots`
  - `provider_id`
  - `remote_group_id`
  - `group_name`
  - `default_multiplier`
  - `effective_multiplier`
  - `first_seen_at`
  - `last_seen_at`
- `sub2api_provider_group_rate_changes`
  - 旧倍率、新倍率、变化时间和来源

周期扫描应使用独立的 leader lock 和随机抖动，不能和账号探针或优化切组共用同一个任务状态。余额建议 15 至 30 分钟，倍率建议 30 至 60 分钟。

### 第三阶段：事件与通知

先接入现有邮件和站内通知：

- 余额低于阈值；
- 远程分组倍率变化超过阈值；
- 连续多次读取失败；
- token 需要重新导入。

必须持久化 cooldown，避免多实例部署重复发送。Webhook/Telegram/企业微信/钉钉/飞书/Bark 应在统一通知抽象成熟后再增加。

## 7. 生产升级边界

当前第一阶段：

- 不修改 Ent schema；
- 不新增 SQL migration；
- 不修改已有数据；
- 不自动创建周期任务；
- 不改变健康状态和分组调度结果；
- 删除前端入口和后端路由即可完整回退。

第二阶段若实施表结构，必须使用新增表的 expand-only migration，并在 PostgreSQL 旧库副本上演练升级、回滚应用版本和历史清理，再进入生产。

## 8. 不直接复制的实现

不建议把 `upstream-hub` 源码整体搬进当前仓库：

1. 它的 channel 语义与 `Sub2APIProvider` 不同；
2. 它的登录/session 模型弱于当前 token pair 轮换；
3. 第三方打码引入额外凭据、费用和风险；
4. 独立 GORM schema 与当前 Ent/migration 体系不兼容；
5. 直接复制通知系统会一次性扩大数据库、设置页和安全审计范围。

正确的复用方式是吸收“余额/倍率作为独立监控域”的产品设计，继续使用当前 Sub2API 已有的认证、错误分类、Provider、探针与管理界面基础设施。
