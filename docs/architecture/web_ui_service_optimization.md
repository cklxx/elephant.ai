# Web UI 与服务架构优化调研与实施计划（精简版）

> 目标：在不显著提升技术复杂度的前提下，逐步改善单体式应用的可用性与可维护性。全部改动以现有技术栈（Next.js、Go、Redis、Postgres）为基础，优先复用现有组件与脚本。

## 1. 当前基础功能梳理

### 1.1 用户视角
- **会话驱动任务执行**：通过单列聊天界面与 Agent 交互，支持文本与结构化指令。
- **工具调用反馈**：浏览器、终端、代码编辑等工具的输出以消息形式回传。
- **文件上传与引用**：可上传文件供 Agent 读取、修改并回写。
- **执行状态提示**：以消息 Loading 与文本提示反馈进度。
- **历史记录**：会话维度存储历史，支持重新打开。

### 1.2 运维视角
- **单体式 Web 应用**：Next.js + Node 服务统一部署，依靠 WebSocket 与后端调度通信。
- **任务调度流程**：API 服务直接调用执行器，内部 RPC 完成工具操作。
- **事件与日志**：操作写入应用日志，通过 ELK 收集。
- **监控能力**：Prometheus 提供基础指标，Grafana 告警有限。
- **权限模型**：以用户级 Token 控制访问，缺少组织/角色粒度。

### 1.3 已识别约束
- **界面信息密度有限**：单列对话难承载多源信息。
- **执行耦合度高**：调度、通知、工具调用集中在单体内。
- **异步体验薄弱**：长任务缺乏主动通知。

## 2. 竞品功能调研（摘要）
- **OpenAI o1 Workspace**：多面板协作、Plan 审批、执行状态可视化。
- **Claude Team Workspace**：事件时间线、文档侧边栏、回放模式。
- **Cursor IDE**：代码上下文注入、操作历史回滚。
- **Perplexity Labs**：结构化引用、任务编排图。
- **Notion Q&A Agents**：知识库整合、异步通知、多租户控制。

## 3. 当前痛点与差距分析
1. **界面信息密度不足**：缺少结构化的计划、执行与文件视图。
2. **会话与执行脱节**：计划与工具输出没有统一呈现。
3. **缺乏任务编排视角**：难以快速定位执行上下文。
4. **服务架构耦合**：扩展通知、审批等能力需要改动核心链路。
5. **缺少异步通知**：长任务完成后无法第一时间感知。
6. **缺乏多租户能力**：无法支撑团队场景。

## 4. Web UI 架构优化计划

### 4.1 布局与基础交互
- **双栏栅格布局（保持单页模式）**  
  - **代码执行计划**：在 `web/app/conversation/page.tsx` 中以 CSS Grid 调整布局（使用现有 `Container` 组件）；在 `web/components/layout/ConversationScaffold.tsx` 新建栅格封装组件；更新 `web/app/globals.css` 添加 `grid-template-columns` 与间距变量；在 `web/components/input/ComposerBar.tsx` 调整宽度自适应逻辑。  
  - **验收方案**：运行 `npm run test:unit -- ConversationScaffold` 覆盖布局渲染；通过 `npm run lint` 确认样式无冲突；在 Chrome DevTools 手动验证桌面与移动断点，截图存档于 `docs/design/review/conversation-layout.md`。

- **折叠式辅助面板（复用现有 Accordion）**  
  - **代码执行计划**：在 `web/components/workspace/WorkspaceAccordion.tsx` 基于 `@/components/ui/accordion` 输出工具、文件、日志面板；在 `web/hooks/useWorkspaceState.ts` 增加展开状态持久化（localStorage）；在 `web/app/conversation/page.tsx` 引入该组件并通过 props 控制可见性。  
  - **验收方案**：新建 `web/components/workspace/__tests__/WorkspaceAccordion.test.tsx` 覆盖展开/收起与状态恢复；产品走查录屏保存在 `docs/review/workspace-accordion.mp4`；灰度开启 5% 用户并收集反馈，验证错误率无提升。

- **轻量响应式适配**  
  - **代码执行计划**：在 `web/tailwind.config.ts` 使用现有 `md`、`lg` 断点定义栅格列数；在 `web/app/globals.css` 新增 `.layout--compact` 类用于移动端单列；在 `web/components/layout/ConversationScaffold.tsx` 根据 `useBreakpoint` 切换布局。  
  - **验收方案**：Playwright 脚本 `web/e2e/conversation-responsive.spec.ts` 覆盖桌面/平板/移动布局；手动执行 `npm run e2e -- conversation-responsive` 并上传截图；设计确认断点展示后记录在审阅文档。

### 4.2 状态展示与任务跟踪
- **计划概览卡片（卡片式摘要）**  
  - **代码执行计划**：在 `web/components/plan/PlanSummaryCard.tsx` 渲染 Plan 标题、阶段、进度条；扩展 `web/lib/types.ts` 的 `SessionPlan` 结构以包含简要统计；在 `web/app/conversation/page.tsx` 通过 SWR 获取 `/api/session/{id}/plan/summary` 数据。  
  - **验收方案**：单测 `web/components/plan/__tests__/PlanSummaryCard.test.tsx` 覆盖不同阶段渲染；在 Staging 触发一次真实审批流程并记录截图；后端契约测试 `tests/integration/session_plan_summary_test.go` 需通过。

- **执行日志列表（分页）**  
  - **代码执行计划**：在 `web/components/events/ExecutionLog.tsx` 以现有 `List` 组件渲染日志，支持关键字过滤；更新 `web/lib/api.ts` 添加 `/api/session/{id}/logs?cursor=` 请求；在 `web/hooks/useExecutionLogs.ts` 处理分页和轮询。  
  - **验收方案**：Vitest 用例覆盖过滤与分页；Playwright 用例 `web/e2e/execution-log.spec.ts` 验证长任务滚动加载；运维确认日志请求对后端 QPS 影响可控（监控截图附于审阅文档）。

- **文件 diff 只读查看（延后编辑）**  
  - **代码执行计划**：在 `web/components/files/DiffPreview.tsx` 复用现有 `CodeViewer` 组件（Monaco 只读模式）；扩展 `web/components/files/FileTabs.tsx` 支持 `diff` 标签；记录技术决策于 `docs/architecture/adr/adr-002-diff-viewer.md`。  
  - **验收方案**：单测覆盖亮/暗主题渲染；QA 在 Staging 使用实际 diff 验证性能；文档更新后经设计确认配色无冲突。

- **通知提醒（Toast + Inbox 列表）**  
  - **代码执行计划**：复用 `@/components/ui/toast` 在 `web/hooks/useNotifications.ts` 中统一调度；新增 `web/components/notification/InboxDrawer.tsx` 展示历史通知；在 `web/app/providers.tsx` 注入 Provider；后端沿用现有 WebSocket 推送。  
  - **验收方案**：单测覆盖已读标记和批量清除；在 Staging 触发长任务确认通知顺序；QA 检查无障碍（axe 扫描）并记录在 `docs/qa/accessibility/notification.md`。

### 4.3 设计系统与可访问性
- **主题 Token 梳理（CSS 变量）**  
  - **代码执行计划**：在 `web/app/globals.css` 定义 `--color-surface`、`--color-border` 等基础变量；更新 `web/components/ui/theme-provider.tsx` 支持浅/深色切换；在 `web/tailwind.config.ts` 映射变量到主题颜色。  
  - **验收方案**：运行 `npm run test:unit -- theme-provider` 验证持久化；设计确认对比度并在 `docs/design/token-review.md` 记录；`npm run lint:css` 通过。

- **组件复用清单**  
  - **代码执行计划**：整理 `web/components/ui` 目录，新增 `Panel.tsx`、`CardHeader.tsx` 等轻量封装；更新 `web/components/ui/index.ts` 导出；在 Storybook (`npm run storybook`) 中补充示例。  
  - **验收方案**：单测覆盖组件导出；Storybook Review 记录在 `docs/design/storybook-review.md`；`npm run lint` 与 `npm run test:unit` 全绿。

- **关键流程无障碍补强**  
  - **代码执行计划**：在 `web/components/agent/AgentChat.tsx`、`web/components/session/SessionList.tsx` 添加 aria 标签与键盘导航；更新 `web/lib/i18n.ts` 支持多语言占位符；补充 `web/locales/en/conversation.json` 文案。  
  - **验收方案**：运行 `npm run lint:i18n` 与 `npm run test:a11y`；QA 使用 axe DevTools 手动检查；记录结果于 `docs/qa/a11y-checklist.md`。

### 4.4 体验优化（循序渐进）
- **最近任务快捷入口**  
  - **代码执行计划**：在 `web/components/history/RecentSessions.tsx` 展示最近 5 次任务；更新 `web/lib/api.ts` 复用现有 `/api/sessions/recent`；在 `web/app/conversation/page.tsx` 通过 props 控制显隐。  
  - **验收方案**：单测覆盖空态与正常态；埋点验证点击率（数据录入 `docs/analytics/recent-sessions.md`）；灰度上线 1 周监控点击数据。

- **离线提示与重连**  
  - **代码执行计划**：扩展 `web/hooks/useConnectionStatus.ts` 监听 `navigator.onLine`；在 `web/components/system/OfflineBanner.tsx` 显示提示；在 `web/app/layout.tsx` 挂载 Banner。  
  - **验收方案**：单测覆盖在线/离线状态切换；手动断网测试并录屏；监控验证心跳失败告警未增加。

- **基础协作批注（备注文本 + @ 提醒）**  
  - **代码执行计划**：在 `web/components/notes/SessionNotes.tsx` 新增备注列表，支持 Markdown；后端复用现有 `/api/notes` 接口；在 `web/hooks/useSessionNotes.ts` 处理创建与 @ 通知；通知与上文 Inbox 复用。  
  - **验收方案**：单测覆盖创建/删除/编辑；在 Staging 两名测试者联调确认 @ 推送；记录使用指引于 `docs/guide/session-notes.md`。

## 5. 服务架构优化计划（保持单体内渐进改造）

### 5.1 单体内模块化
- **领域分层梳理**  
  - **代码执行计划**：在 `internal/session`、`internal/workflow`、`internal/notification` 之间引入接口定义（例如 `internal/session/service.go` 提供 `SessionService` 接口）；在 `cmd/alex-server/main.go` 注入依赖；将 http handler 拆分到 `internal/server/http/session`、`internal/server/http/notification`。  
  - **验收方案**：运行 `go test ./internal/... ./tests/server/...`；架构师 Review 依赖图（生成脚本 `scripts/dev/print-deps.sh` 输出结果）；在 `docs/architecture/adr/adr-003-service-boundary.md` 更新记录。

- **应用内事件通知（同步 -> 异步封装）**  
  - **代码执行计划**：在 `internal/events/bus.go` 定义轻量接口，提供 `internal/events/inmemory` 实现；在 `internal/notification/service.go` 中改为订阅事件；将现有同步调用改为事件发布。  
  - **验收方案**：`go test ./internal/events/...`；Staging 运行一周监控延迟；在 `docs/ops/event-bus-runbook.md` 记录使用方式。

- **HTTP 层与任务执行解耦**  
  - **代码执行计划**：引入 `internal/workflow/queue/simple_queue.go`（基于 channel + goroutine）实现基础异步执行；在 `cmd/alex-server/main.go` 将耗时任务投递到队列；保留 Redis/Asynq 作为后续选项并记录在 ADR。  
  - **验收方案**：`go test ./internal/workflow/...`；压测脚本 `tests/load/simple_queue_benchmark_test.go` 达到 100 jobs/min；运维确认监控无异常。

### 5.2 渐进式技术增强
- **API 层整理**  
  - **代码执行计划**：统一 `internal/server/http/router.go` 路由注册，生成 `api/openapi.yaml`（使用 `oapi-codegen` 当前流程）；新增 `tests/api/openapi_validation_test.go`。  
  - **验收方案**：`go test ./tests/api/...`；CI 中运行 `scripts/ci/validate-openapi.sh`；产品确认接口文档并在 `docs/api/changelog.md` 留存。

- **实时通信统一**  
  - **代码执行计划**：在 `internal/realtime/manager.go` 整合 SSE/WebSocket 逻辑，提供自动降级；前端 `web/lib/api.ts` 引入降级判定；复用现有心跳机制。  
  - **验收方案**：`go test ./internal/realtime/...`；Playwright 用例 `web/e2e/realtime-fallback.spec.ts` 验证断网重连；QA 在弱网环境手测并记录。

- **可观测性补强（不引入新集群）**  
  - **代码执行计划**：扩展 `internal/observability/metrics.go` 增加任务耗时与通知失败指标；在 `deployments/docker-compose.yml` 添加 Prometheus Job；更新 `docs/ops/observability.md` 提供 Dashboard 配置。  
  - **验收方案**：`go test ./internal/observability/...`；Grafana Dashboard 截图记录；SRE 确认告警阈值。

### 5.3 数据与权限（轻量方案）
- **租户标识扩展**  
  - **代码执行计划**：在 `internal/session/model.go` 添加 `TenantID` 字段及索引；编写迁移 `internal/session/migrations/2024XXXX_add_tenant.sql`；更新 `internal/session/repository.go` 的查询条件；前端在 `web/lib/api.ts` 通过 header 传递 `X-Tenant-ID`。  
  - **验收方案**：执行 `make migrate-up` 于影子环境并完成回滚；`go test ./tests/session/...` 覆盖跨租户访问；灰度租户验证无越权。

- **审计日志增强（沿用数据库存储）**  
  - **代码执行计划**：在 `internal/audit/logger.go` 定义统一写入接口；在现有 handler 中调用；新增导出 CLI `cmd/audit-exporter/main.go`；配置 `deployments/cronjobs/audit-export.yaml` 定期导出。  
  - **验收方案**：`go test ./internal/audit/...`；演练一次导出流程并记录到 `docs/ops/audit-export.md`；安全团队确认日志留存要求。

- **缓存策略优化**  
  - **代码执行计划**：复用 Redis，引入 `internal/cache/redis/client.go` 封装；在 `internal/session/service.go` 为热点数据添加缓存并使用 `singleflight`；在 `deployments/docker-compose.yml` 中补充 Redis 健康检查。  
  - **验收方案**：`go test ./internal/cache/...`；压测报告记录命中率；运维确认 Redis 监控指标稳定。

### 5.4 DevOps 与交付
- **部署脚本整合**  
  - **代码执行计划**：整理 `deploy.sh`、`scripts/cd/*`，统一入口为 `make deploy ENV=staging`; 在 `docs/ops/deploy-playbook.md` 更新流程；CI 中新增 `scripts/ci/check-deploy.sh` 做干跑。  
  - **验收方案**：CI 执行干跑脚本并通过；运维按新脚本完成一次 Staging 部署并记录反馈。

- **回滚策略标准化**  
  - **代码执行计划**：在 `docs/ops/rollback-checklist.md` 明确回滚步骤；在 `deployments/docker-compose.yml` 和 `k8s/overlays/staging/` 增加旧版本镜像回滚配置；更新 `scripts/cd/rollback.sh` 触发自动健康检查。  
  - **验收方案**：演练一次回滚并在日志中记录耗时；SRE 确认健康检查脚本输出；复盘纪要归档。

- **基础测试矩阵（逐项落地）**  
  - **代码执行计划**：在 `Makefile` 增加 `test:frontend`、`test:backend`、`test:e2e` 目标；更新 `scripts/ci/run-tests.sh` 串联执行；在 `web/e2e` 目录按模块补充脚本，后端维持现有集成测试。  
  - **验收方案**：CI Pipeline 必须在主干执行全部测试目标；记录测试覆盖率于 `docs/qa/test-matrix.md`；QA 每月复盘一次执行结果。

