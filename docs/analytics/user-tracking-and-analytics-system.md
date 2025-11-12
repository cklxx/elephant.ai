# MVP 用户埋点与分析系统落地方案（免费方案）

## 0. 进度追踪（持续更新）
| 阶段 | 关键事项 | 状态 | 说明 |
| --- | --- | --- | --- |
| Day 0 | 创建 PostHog Cloud Free 项目并获取 Project API Key | ✅ 已完成 | 项目已创建并提供 `Project API Key`，用于本地/生产配置 |
| Day 1-2 | 梳理指标与 `tracking-plan.yaml` | ✅ 已完成 | `docs/analytics/tracking-plan.yaml` 已合并且覆盖核心事件 |
| Day 3-4 | Web SDK 集成与事件上报 | ✅ 已完成 | `web` 前端已集成 PostHog SDK，并在关键交互中调用 `captureEvent` |
| Day 3-4（可选） | 移动端 SDK 接入 | ⏳ 未开始 | 如上线移动端客户端需按官方 SDK 指引接入 |
| Day 5 | 服务端 SDK 集成与任务生命周期埋点 | ✅ 已完成 | `internal/server/app` 已在任务状态变化时发送事件 |
| Day 6 | `TrackingPlanMatches` 校验测试 | ✅ 已完成 | `go test ./internal/analytics -run TrackingPlanMatches` 已纳入测试流程 |
| Day 6 | PostHog Alerts 告警配置 | ⏳ 未开始 | 需在 PostHog 控制台手动创建事件异常告警 |
| Day 7-8 | Dashboard（漏斗、留存、趋势）建设 | ⏳ 未开始 | 待产品/运营在 PostHog 仪表板中创建并分享 |
| Day 9-10（可选） | Feature Flag / GrowthBook 实验 | ⏳ 未开始 | 视业务需求启用，可复用 PostHog flag 或部署 GrowthBook |

## 1. 项目目标
- 在 10 天内交付一个可用于 MVP 产品的埋点与行为分析系统。
- 全程使用 **PostHog Cloud Free**（事件采集、分析、Feature Flag）+ **GrowthBook OSS**（实验统计，可选）。
- 支持 Web、iOS/Android、服务器端关键事件上报，并为产品/运营提供基础漏斗、留存、看板能力。

## 2. 角色与分工
| 角色 | 主要职责 |
| --- | --- |
| 产品经理 | 梳理关键指标、确认 Tracking Plan、验收报表 |
| 前端工程师 | 集成 Web/PostHog SDK、配置自动采集与自定义事件 |
| 后端工程师 | 集成服务端 SDK、保障关键事件准确上报 |
| 移动端工程师（如有） | 集成 iOS/Android SDK，完成离线缓存设置 |
| 数据/运营 | 配置漏斗、留存报表，搭建看板并解读数据 |

## 3. 工具账号与环境准备（Day 0）
1. 在 [https://posthog.com/signup](https://posthog.com/signup) 注册 PostHog 账号，选择 Cloud Free 计划。
2. 新建项目并记录 Project API Key 与 Host（默认为 `https://app.posthog.com`）。
3. 创建开发、生产两个项目，分别用于测试与线上数据，避免污染。
4. 若需要 A/B 实验，按文档部署 GrowthBook OSS：
   - 使用 Render 免费套餐：Fork 官方仓库 `growthbook/growthbook`，按指南部署 Web 与 API 服务。
   - 或在公司服务器执行 `docker compose up -d` 启动。
5. 在本仓库中更新环境变量（开发、预发、生产环境保持一致）：
   - Web 前端：`NEXT_PUBLIC_POSTHOG_KEY`（必填）、`NEXT_PUBLIC_POSTHOG_HOST`（自托管时修改）。
   - 服务端：`POSTHOG_API_KEY`（与前端共用 Project API Key）、`POSTHOG_HOST`（默认 `https://app.posthog.com`）。
6. `docs/analytics/tracking-plan.yaml` 已内置核心事件定义，可作为 Tracking Plan 基线继续扩展。

## 4. 实施步骤与时间表
### Day 1-2：梳理指标与埋点设计（✅ 已完成）
- 产品经理输出核心用户旅程（任务输入→执行→终态），明确成功指标：提交次数、取消率、任务成功率等。
- `docs/analytics/tracking-plan.yaml` 已收录以下事件，可直接复用并迭代：
  - `task_submitted`：Web 端提交任务（记录是否附带文件、是否使用模拟 SSE）。
  - `task_cancel_requested` / `task_cancel_failed`：取消按钮点击与失败场景（前端、服务端都会上报并通过 `source` 区分）。
  - `task_execution_started` / `task_execution_completed` / `task_execution_failed` / `task_execution_cancelled`：后端任务生命周期节点（带迭代次数、耗时、终止原因）。
  - `session_created` / `session_selected` / `session_deleted`：控制台会话切换与清理。
  - `sidebar_toggled`、`timeline_viewed`：UI 交互洞察。
- 建立评审流程：每次埋点新增需提交 PR，并更新 `tracking-plan.yaml` 以保持文档与实现同步。

### Day 3-4：前端 SDK 集成（✅ 已完成）
1. `web/lib/analytics/posthog.ts` 已封装初始化与事件缓冲逻辑，只需在 `.env.local` 写入 `NEXT_PUBLIC_POSTHOG_KEY` 即可启用；`app/providers.tsx` 在应用加载时自动调用 `initAnalytics()`。
2. `app/conversation/ConversationPageContent.tsx` 已对任务提交、取消、会话切换、侧栏/时间线操作等关键行为调用 `captureEvent`。如需新增埋点，直接引用 `AnalyticsEvent` 常量即可避免拼写错误。
3. 执行 `npm test` 验证 `lib/__tests__/analytics.posthog.test.ts` 通过，确保初始化与事件排队行为稳定。
4. 本地启动前端后，打开 PostHog `Data > Live events` 检查事件是否携带 `source=web_app` 属性；若未看到事件，确认 `.env.local` 是否被加载。

### Day 3-4（并行）：移动端 SDK（可选，⏳ 未开始）
- 根据平台引入官方 SDK：
  - iOS（Swift Package Manager）：`https://github.com/PostHog/posthog-ios`。
  - Android（Gradle）：`implementation "com.posthog:posthog-android:3.5.1"`。
- 在初始化时开启离线缓存：`posthog.setFlushInterval(60000)`。

### Day 5：服务端关键事件上报（✅ 已完成）
1. `internal/analytics` 新增 PostHog 客户端实现（封装 `github.com/posthog/posthog-go`），`cmd/alex-server/main.go` 会在启动时读取 `POSTHOG_API_KEY` / `POSTHOG_HOST` 自动创建客户端。
2. `internal/server/app/server_coordinator.go` 会在任务生命周期内发送事件：
   - `task_execution_started`：队列任务时包含附件数量、会话层级。
   - `task_execution_completed`：记录耗时、迭代次数、StopReason、所用预设。
   - `task_execution_failed` / `task_execution_cancelled`：写入错误原因或终止类型。
   - `task_cancel_requested`：记录后端是否找到活跃任务以及请求来源。
3. 运行 `go test ./internal/server/app -run Analytics` 可验证新增单测 `TestServerCoordinatorAnalyticsCapture`。
4. 部署后在 PostHog `Data > Live events` 过滤 `source=server` 可区分前后端数据。

### Day 6：配置数据验证与告警（部分完成）
- ✅ 在 CI 流程中加入 Schema 校验：执行 `go test ./internal/analytics -run TrackingPlanMatches`，确保 `tracking-plan.yaml` 与前后端事件常量保持一致。
- ⏳ 在 PostHog 中设置关键事件的 Alerts（`Tools > Alerts`），监控事件量突增/下跌。

### Day 7-8：分析看板落地（⏳ 未开始）
1. 创建以下报表并分享给团队（全部基于当前埋点事件，可直接在 PostHog 配置）：
   - 漏斗：`task_submitted -> task_execution_started -> task_execution_completed`，用于衡量提交请求到成功响应的整体转化率。
   - 留存：以 `task_submitted` 作为 Cohort 的 Day 1/7/14 留存，对比交互后是否持续触发 `task_submitted`/`session_selected`。
   - 事件趋势：按日跟踪 `task_submitted`、`task_cancel_requested`、`task_execution_failed`，作为产品健康度指标。
2. 使用 Dashboard 组合上述报表，并设定每周邮件推送。
3. 运营根据报表撰写周报，沉淀在 Confluence 或团队文档中。

### Day 9-10：A/B 实验（可选，⏳ 未开始）
1. 在 PostHog 启用 Feature Flag（无需 GrowthBook 时）：
   - 进入 `Feature Flags`，创建 flag（如 `new_onboarding_flow`），设定实验组 50%。
   - 前端读取 flag：
     ```ts
     const isNewFlow = posthog.isFeatureEnabled('new_onboarding_flow');
     ```
2. 若使用 GrowthBook：
   - 将 PostHog 作为数据源，配置 API key。
   - 在前端集成 GrowthBook SDK，并根据 flag 控制实验分流。
   - 在 GrowthBook 中设置目标指标为 `task_execution_completed` 转化率，至少运行 7 天后查看显著性结论。

## 5. 交付物清单
| 类别 | 交付内容 | 完成标志 | 当前状态 |
| --- | --- | --- | --- |
| 规范 | `tracking-plan.yaml` | 文件合并至主分支并获双人评审 | ✅ 已完成 |
| SDK 集成 | Web/移动端/后端埋点代码 | 上线后在 PostHog Live events 可实时看到数据 | ✅ Web/Server 已上线；移动端待接入 |
| 报表 | 漏斗、留存、事件趋势 Dashboard | 团队成员可访问并订阅邮件推送 | ⏳ 未开始 |
| 告警 | PostHog Alerts | 当事件量异常时自动发 Slack/邮件 | ⏳ 未开始 |
| 实验（可选） | Feature Flag 或 GrowthBook 实验报告 | 报告包含实验目标、结果、结论 | ⏳ 未开始 |

## 6. 维护与迭代
- 每周例会复盘 Dashboard，收集新增指标需求。
- 新功能上线前至少提前 2 天在 `tracking-plan.yaml` 提交埋点变更。
- 每月导出历史事件到对象存储（PostHog 提供 CSV 导出），防止超过免费额度后数据被裁剪。
- 若月事件量接近 1M，评估升级 PostHog 付费层或迁移至自托管版本。

## 7. 风险与应对
| 风险 | 影响 | 预防/缓解措施 |
| --- | --- | --- |
| SDK 未正确初始化 | 数据缺失 | 在前端加入 `posthog.on('loaded', ...)` 回调，记录初始化失败日志 |
| 埋点方案失真 | 指标无法复现 | 实施埋点走查，QA 使用 PostHog Debug 工具录屏核对 |
| 免费额度超限 | 事件被截断 | 每周查看 Usage 报告，提前清理无用事件或导出备份 |
| GrowthBook 服务中断 | 实验无法生效 | 生产环境实验使用 PostHog 原生 Feature Flag 作为备份 |

## 8. 已完成工作的验证记录（2024-12-02）
- ✅ `npm --prefix web test -- --run`：前端 Vitest 套件全部通过，包含 `lib/__tests__/analytics.posthog.test.ts` 对 PostHog SDK 初始化/事件排队逻辑的覆盖。
- ✅ `go test ./...`：服务端所有单元测试和 `TrackingPlanMatches` 校验通过，覆盖 `internal/analytics` 与 `internal/server/app` 的埋点实现。
- ✅ 手工核对 `docs/analytics/tracking-plan.yaml` 与 `web/lib/analytics/events.ts` / `internal/analytics/events.go`，确认已实现事件与文档保持一致、无孤儿事件。
- ✅ 使用提供的 Project API Key 向 PostHog `/capture/` 接口发送 `task_submitted` 测试事件并收到 `200/Ok` 响应，确认数据通道可用；在本地设置 `NEXT_PUBLIC_POSTHOG_KEY`/`POSTHOG_API_KEY` 后，可在 PostHog Live Events 检查事件并确保带有 `source` 属性区分前后端。

---

该方案聚焦在可在两周内完成的最小化实现，所有步骤均可由中小团队直接落地，无需建设复杂大数据平台。
