# Web UI 与服务架构优化调研与实施计划

## 1. 当前基础功能梳理

### 1.1 用户视角
- **会话驱动任务执行**: 通过单列聊天界面向 Agent 提交需求, 支持自然语言与结构化指令混合输入。
- **工具调用反馈**: Agent 可调用浏览器、终端、代码编辑等工具, 并将输出以串行消息形式回传。
- **文件上传与引用**: 支持上传本地文件, 在对话中引用, Agent 可读取内容进行推理或编辑。
- **执行状态提示**: 当前以消息 Loading 与文本提示方式反馈执行进度, 缺乏可视化细节。
- **历史记录**: 支持按会话维度存储历史, 用户可重新进入查看过往任务。

### 1.2 运维视角
- **单体式 Web 应用**: Next.js 前端与 Node 后端耦合部署, 通过 WebSocket 与后端调度通信。
- **任务调度流程**: API 服务接收请求后直接调用执行器, 依赖内部 RPC 完成工具操作。
- **事件与日志**: 关键操作写入应用日志, 通过 ELK 收集, 缺少统一事件总线。
- **监控能力**: 已接入基础的 Prometheus 指标 (请求量、错误率), Grafana 告警规则较少。
- **权限模型**: 目前以用户级 Token 控制访问, 未提供组织/角色粒度的权限配置。

### 1.3 已识别约束
- **界面信息密度有限**: 单列对话结构难以承载复杂任务的上下文、文件和工具输出。
- **执行耦合度高**: 调度、工具调用、通知在同一服务内完成, 难以横向扩展。
- **异步体验薄弱**: 长任务仅能通过轮询了解进度, 未与通知、时间线等能力结合。

## 2. 竞品功能调研

### 2.1 OpenAI o1 Preview Workspace
- **多面板协作**: 左侧任务流 + 右侧工具输出, 支持代码、终端、浏览器分栏。
- **任务计划与审批**: 提供显式 Plan 审批机制, 允许用户对自动化步骤进行确认或修改。
- **工作状态可视化**: 实时显示每个子任务的执行进度、耗时与结果状态。
- **协同编辑**: 支持多人同时查看与编辑工作区, 提供时间线回溯。

### 2.2 Anthropic Claude Team Workspace
- **会话时间线**: 展示每次 Agent 行动、工具调用与输出, 支持过滤与跳转。
- **文档/文件侧边栏**: 将引用的文件、研究资料集中管理, 支持预览与快速插入。
- **回放模式**: 可以按步骤重播, 方便复盘与审计。
- **自动总结**: 长对话结束时自动生成摘要与下一步建议。

### 2.3 Cursor AI IDE
- **代码上下文感知**: 会话区支持快速注入当前打开文件上下文。
- **操作历史堆栈**: 记录每次代码修改的 diff, 支持一键回滚。
- **本地/云端混合执行**: 结合本地 IDE 状态与云端 Agent 执行记录。

### 2.4 Perplexity Pages / Labs
- **研究模式**: 结构化展示检索源、引用与推理链条。
- **可视化引用**: 每条结论附带来源标注, 支持一键复制完整报告。
- **任务编排**: 通过卡片式流程图呈现多步研究流程。

### 2.5 Notion Q&A Agents
- **知识库即服务**: 与企业知识库深度整合, 支持权限控制与数据隔离。
- **离线/异步任务**: 任务可在后台持续执行, 完成后推送通知。

## 3. 当前痛点与差距分析

1. **界面信息密度不足**: 缺少实时执行状态、文件 diff、工具输出等多维信息面板。
2. **会话与执行脱节**: 任务输入输出集中在单列, 不利于复杂任务的跟踪与回放。
3. **缺乏任务编排视角**: 没有可视化的计划、依赖关系或进度展示。
4. **服务架构耦合**: WebSocket、任务调度、工具执行耦合度高, 扩展成本大。
5. **缺少异步通知**: 长任务完成后无法主动通知用户。
6. **缺乏多租户/权限模型**: 难以支持团队场景与资源隔离。

## 4. Web UI 架构优化计划

### 4.1 布局重构
- **AppShell 双栏主布局 + 底部输入区**: 左侧承载会话/计划, 右侧聚合工作区 (计算机 / 时间线 / 文件标签)。
  - **代码执行计划**: 在 `web/app/layout.tsx` 引入 `AppShell` 容器并接入现有 `ThemeProvider`; 在 `web/components/layout/AppShell.tsx` 构建左右栏布局 (使用 `ResizablePanel`), 并为右侧内容预留插槽; 通过 `web/app/conversation/page.tsx` 切换到新的 `AppShell` 并挂接 Feature Flag (配置于 `web/lib/feature-flags.ts`); 更新 `web/components/input/ComposerBar.tsx` 调整底部固定定位与宽度同步。
  - **验收方案**: 通过 `web/e2e/app-shell.responsive.spec.ts` 在桌面/平板/移动视口下比对截图; 确认 Feature Flag 默认为灰度状态且在 `web/app/conversation/page.tsx` 中可开关; 由设计评审最新交互稿, 并在手动探索中验证输入框与左右栏缩放逻辑无布局抖动。
- **渐进式可停靠面板系统**: 先以受控折叠面板起步, 待验证后再开放拖拽停靠。
  - **代码执行计划**: 在 `web/components/workspace/WorkspacePanel.tsx` 实现受控折叠组件 (复用 `shadcn/ui` 的 `Collapsible`); 在 `web/hooks/useWorkspaceLayout.ts` 管理用户偏好 (localStorage) 并暴露恢复默认函数; 于 `web/app/conversation/page.tsx` 中通过 Feature Flag 切换 Dock Beta, 结合 `web/components/workspace/__tests__/WorkspacePanel.test.tsx` 覆盖折叠逻辑。
  - **验收方案**: 单元测试 `WorkspacePanel.test.tsx` 需覆盖展开/收起及偏好持久化, 并在 Storybook QA 中确认交互; 通过手动测试验证在刷新后恢复用户偏好, 且 Feature Flag 关闭时回退旧体验。
- **多终端响应式适配**: 保持桌面端三栏、平板双栏、移动单列, 同时复用现有 Tailwind token。
  - **代码执行计划**: 更新 `web/app/globals.css` 统一栅格变量, 在 `web/tailwind.config.ts` 定义 `2xl`、`md` 特定断点; 在 `web/components/layout/AppShell.tsx` 和 `web/components/workspace/WorkspacePanel.tsx` 添加断点逻辑; 使用 `web/e2e/app-shell.responsive.spec.ts` (Playwright) 记录快照并在 `package.json` 的 `test:e2e` 流水线中串联。
  - **验收方案**: Playwright 基线截图需经设计确认后固化, 并在 CI 中保持稳定; 手动在 Chrome DevTools 设备模式核对断点布局; Lighthouse Performance ≥ 85 且无布局偏移告警。

### 4.2 状态展示与交互
- **任务计划模块**: 在会话区顶部呈现当前 Plan, 支持阶段性完成标记与人工审批, 并与现有 `SessionCard` 数据结构对齐。
  - **代码执行计划**: 新建 `web/components/plan/PlanOverview.tsx` 读取 `SessionPlan` 数据 (类型定义在 `web/lib/types.ts`); 在 `web/app/conversation/page.tsx` 的 loader 中并发拉取会话与 Plan 数据; 在 `web/hooks/usePlanApproval.ts` 对接后端审批 API 并加入错误兜底; 在 `web/components/plan/__tests__/PlanOverview.test.tsx` 覆盖不同状态展示。
  - **验收方案**: `PlanOverview.test.tsx` 需覆盖待审批/已通过/被驳回状态; Storybook 中通过产品验收 Plan 显示与交互; 在 Staging 通过实际审批流程验证与后端契约 (使用 `tests/integration/session_plan_test.go`) 一致。
- **实时事件流**: 工作区提供可过滤事件流 (工具调用、日志、错误), 复用现有 WebSocket 信道, 并提供断网恢复机制。
  - **代码执行计划**: 在 `web/components/events/EventStream.tsx` 构建虚拟滚动列表 (使用 `react-virtual`); 更新 `web/lib/api.ts` 抽象 SSE/WebSocket 客户端并对接 `useConnectionStatus`; 在 `web/components/events/__tests__/EventStream.test.tsx` 添加过滤与断线重连用例。
  - **验收方案**: Vitest 覆盖需确保在模拟断网时触发重连; E2E 脚本 `web/e2e/event-stream.reconnect.spec.ts` 模拟长任务并验证事件顺序; QA 手动断开网络确认 Offline Banner 与事件恢复时间 < 5s。
- **文件 diff 与编辑**: 将 diff 查看与编辑能力分阶段推出 (先 diff, 后编辑)。
  - **代码执行计划**: 在 `web/components/files/DiffViewer.tsx` 集成 `@monaco-editor/react` 只读模式, 并提供 Theme 支持; 扩展 `web/components/files/FileTabs.tsx` 以识别 `diff` 标签类型; 在 `web/components/files/__tests__/DiffViewer.test.tsx` 使用真实样例覆盖亮/暗主题; 通过 `docs/architecture/adr/adr-002-diff-viewer.md` 记录技术选型。
  - **验收方案**: 组件单测需覆盖语法高亮、主题切换与大文件滚动; 在 Storybook 中由设计确认 diff 样式; 集成测试 `tests/integration/file_diff_render_test.go` 校验后端 diff 数据与前端渲染一致。
- **执行时间线**: 时间线视图展示步骤起止、耗时、状态, 可跳转到事件流或文件 diff。
  - **代码执行计划**: 新建 `web/components/timeline/TimelineView.tsx` 渲染垂直时间线 (基于 `TimelineItem` 子组件); 在 `web/lib/api.ts` 暴露步骤数据并缓存最近查询; 在 `web/components/timeline/__tests__/TimelineView.test.tsx` 校验跳转和可访问性标签。
  - **验收方案**: 单测需包含键盘可达性与 aria 标签; 通过 `web/e2e/timeline-navigation.spec.ts` 验证可从时间线跳转到事件流/文件 diff; 与产品进行可用性评审确保数据密度符合预期。
- **通知系统**: 顶部引入 Notification Center, 支持任务完成、错误、审批请求提醒, 结合浏览器推送与站内 Inbox。
  - **代码执行计划**: 在 `web/components/notification/NotificationBell.tsx` 与 `NotificationDrawer.tsx` 实现 UI; 在 `web/app/providers.tsx` 注入 `NotificationProvider`; 更新 `web/hooks/useNotifications.ts` 支持轮询 + WebSocket 双通道; 在 `web/components/notification/__tests__/NotificationDrawer.test.tsx` 覆盖筛选、已读与批量处理。
  - **验收方案**: 前端单测需验证已读状态与批量操作; 在 Staging 通过真实长任务触发通知并观测浏览器推送; 记录无障碍审计 (axe) 确保通知 Drawer 无严重问题。

### 4.3 设计系统与组件
- **主题规范化**: 以设计 Token 为核心, 统一浅/深色并暴露可配置主题。
  - **代码执行计划**: 在 `web/app/globals.css` 定义 `--surface-*`、`--text-*` Token; 扩展 `web/components/ui/theme-provider.tsx` 支持 `prefers-color-scheme` 与用户持久化; 更新 `web/tailwind.config.ts` 映射 Token 至 `theme.extend.colors`; 在 `web/components/ui/__tests__/theme-provider.test.tsx` 校验切换与持久化。
  - **验收方案**: Vitest 需验证主题切换后的 DOM 属性与 localStorage 写入; 使用 Percy/Chromatic 比对深浅色截图; 设计与无障碍双评审确认对比度符合 WCAG AA。
- **组件库封装**: 在现有 `web/components/ui` 基础上梳理通用组件, 遵循 `shadcn/ui` 增量引入原则, 避免一次性大规模迁移。
  - **代码执行计划**: 新增 `web/components/ui/panel.tsx`, `timeline.tsx`, `notification-tray.tsx` 并复用原子组件; 更新 `web/components/ui/index.ts` 输出; 在 `web/scripts/sync-shadcn.ts` 补全自动更新脚本; 为每个组件在 `web/components/ui/__tests__/` 下添加可访问性与快照测试。
  - **验收方案**: 单测需覆盖组件导出与无障碍属性; 在实际消费方 (Timeline/Notification) 中通过 TypeScript 编译与 Storybook 链路验证; 运行 `npm run lint` 确保无未使用导出。
- **无障碍与国际化**: 以会话主流程为优先, 完善 aria 标签与 i18n。
  - **代码执行计划**: 扩展 `web/lib/i18n.tsx` 支持命名空间拆分; 在 `web/locales/{en,zh}/conversation.json` 补充核心词条; 更新 `web/components/agent/AgentChat.tsx`、`web/components/session/SessionList.tsx` 添加 aria 描述; 在 `web/lib/__tests__/i18n-provider.test.tsx` 与 `web/components/session/__tests__/SessionList.a11y.test.tsx` 验证。
  - **验收方案**: Lint 阶段运行 `npm run lint:i18n` 检查缺失词条; 使用 axe 自动化扫描确保新增 aria 标签通过; 与翻译团队确认中英文对照并通过 Beta 用户可用性反馈。

### 4.4 体验优化
- **任务上下文记忆**: 会话侧支持引用历史任务并显示摘要, 依赖后端已存在的会话列表接口, 先实现快速访问能力。
  - **代码执行计划**: 在 `web/components/history/TaskContextSidebar.tsx` 构建历史摘要列表组件; 扩展 `web/lib/api.ts` 新增 `/sessions/{id}/context` 请求; 在 `web/components/history/__tests__/TaskContextSidebar.test.tsx` 覆盖筛选与跳转行为; 通过 `docs/design/OUTPUT_DESIGN.md` 与设计确认信息密度。
  - **验收方案**: 单测需验证分页/筛选/跳转正确; 通过可用性测试收集 3 位目标用户反馈; 在 Staging 上导入真实历史数据确保 API 响应 < 200ms。
- **批注与协作**: 对任一步骤添加批注, 并在时间线与事件流同步显示, 同时记录作者身份。
  - **代码执行计划**: 在 `web/components/timeline/TimelineView.tsx` 增加批注入口; 新建 `web/components/timeline/CommentDrawer.tsx` 展示详情; 在 `web/hooks/useTimelineComments.ts` 管理乐观更新和错误回滚; 在 `web/components/timeline/__tests__/CommentDrawer.test.tsx` 覆盖多用户交互。
  - **验收方案**: 单元测试需验证乐观更新与回滚分支; 在集成测试 `tests/collaboration/comment_flow_test.go` 中模拟多人并发; 由安全团队评审批注权限校验逻辑。
- **离线模式提示**: 网络中断时提供离线提示与重连机制, 优先复用当前错误处理框架。
  - **代码执行计划**: 在 `web/hooks/useConnectionStatus.ts` 监听 `navigator.onLine` 与心跳; 在 `web/components/system/OfflineBanner.tsx` 展示提示并提供重连按钮; 更新 `web/app/layout.tsx` 注入 Banner; 在 `web/components/system/__tests__/OfflineBanner.test.tsx` 验证提示显示与重试按钮。
  - **验收方案**: 单测需覆盖在线/离线切换与按钮回调; QA 在真实弱网环境中验证 Banner 出现与自动重连; 监控侧确认相关心跳指标在 Grafana 中可观测。

## 5. 服务架构优化计划

### 5.1 服务边界重构 (分阶段)
- **阶段 1 — 单体内模块化**: 在现有 `cmd/alex-server` 服务中梳理领域模块, 提前建立清晰边界, 降低直接拆分风险。
  - **代码执行计划**: 在 `internal/server/http` 中新增 `handlers/session`、`handlers/workflow` 子包; 整理 `internal/session`、`internal/agent` 之间的耦合, 通过接口定义下沉到 `internal/session/service.go`; 利用 `tests/server/session_http_test.go` 与 `tests/workflow/orchestration_test.go` 补充回归覆盖; 编写 `docs/architecture/adr/adr-003-service-boundary.md` 记录决策。
  - **验收方案**: 通过 Go 单元与集成测试全绿验证拆分无回归; Code Review 时由架构师确认模块边界与依赖图 (使用 `go list -deps` 导出); ADR 经技术委员会签署后归档。
- **阶段 2 — 事件驱动骨架**: 在单体内引入事件总线接口, 先以内存实现验证流程, 再替换为 Kafka/NATS。
  - **代码执行计划**: 新建 `internal/events/bus.go` 定义发布/订阅接口, 在 `internal/events/inmemory` 下提供默认实现; 在 `cmd/alex-server/main.go` 注入事件总线, 将通知、审计等调用改为事件; 在 `tests/events/integration_test.go` 校验事件传播; 准备 `infrastructure/terraform/modules/kafka` 的 IaC 模板。
  - **验收方案**: 集成测试需覆盖事件发布/订阅、失败重试; 在 Staging 启动内存实现跑一周观察; 基础设施团队审阅 Kafka 模块并通过 `terraform validate`。
- **阶段 3 — 独立可部署服务**: 依据阶段 1 边界, 逐步拆出 Session API、Notification、File 处理等服务, 并通过 API Gateway 聚合。
  - **代码执行计划**: 创建 `cmd/session-api/main.go` 与 `internal/session/http` 暴露 REST 接口; 将通知能力抽出到 `cmd/notification-consumer/main.go` 订阅事件总线; 在 `deployments/docker-compose.yml` 增加上述服务并配置健康检查; 更新 `tests/integration/session_api_test.go`、`tests/integration/notification_consumer_test.go` 验证跨服务流程。
  - **验收方案**: 新服务需通过负载测试 (10rps) 与健康检查; 在预发环境通过蓝绿发布演练; Postman 契约测试通过且 API Gateway 日志无 5xx。

### 5.2 技术栈升级
- **API Gateway**: 优先采用与现有 Go 栈兼容的 REST 聚合层 (例如 `chi` + OpenAPI), 评估后再引入 GraphQL。
  - **代码执行计划**: 新建 `cmd/api-gateway/main.go` 复用 `internal/server/http/router.go`; 在 `internal/gateway/proxy.go` 聚合 Session/Workflow/Notification 服务; 生成 `api/openapi.yaml` 并通过 `scripts/ci/validate-openapi.sh` 校验; 在 `tests/gateway/proxy_integration_test.go` 编写契约测试。
  - **验收方案**: CI 中的 `scripts/ci/validate-openapi.sh` 必须通过且生成的 OpenAPI 文件经产品与后端双签; `proxy_integration_test.go` 覆盖成功/失败路由; 安全团队完成初步渗透测试并在 API Catalog 中登记流量路径。
- **实时传输**: 以 SSE + WebSocket 兼容方案为基础, 通过中间层适配, 将 WebTransport 作为实验性选项。
  - **代码执行计划**: 在 `internal/realtime/manager.go` 统一管理连接, 提供 SSE/WebSocket 双栈; 更新 `cmd/alex-server/main.go` 将实时事件路由迁移至 `realtime` 模块; 在 `web/lib/api.ts` 引入自动降级逻辑; 在 `tests/realtime/fallback_test.go` 校验弱网场景。
  - **验收方案**: 在 QA 环境模拟 WebSocket 降级到 SSE 保证事件顺序一致; 连续 14 天监控连接成功率与重连次数无异常波动; 安全审计确认 WebTransport 仅对白名单租户开放并记录审计日志。
- **任务队列**: 先对齐现有长任务执行链路, 采用 Celery/Asynq 等轻量方案验证后再评估 Temporal。
  - **代码执行计划**: 在 `internal/workflow/queue/asynq.go` 集成任务队列; 更新 `cmd/alex-server/main.go` 注册 worker; 在 `deployments/docker-compose.yml` 添加 Redis 任务队列配置; 在 `tests/workflow/retry_policy_test.go` 验证重试策略; 后续通过 ADR 评估是否迁移 Temporal。
  - **验收方案**: 压测任务吞吐达到 200 jobs/min 且失败自动重试成功率 > 95%; Grafana 队列监控面板显示延迟与堆积阈值; ADR 获得技术委员会签字并归档。
- **可观测性**: 统一指标/日志/链路追踪, 与现有 `internal/observability` 模块对齐, 并将 Dashboard 自动化。
  - **代码执行计划**: 扩展 `internal/observability/tracing.go` 支持 OpenTelemetry, 在 `cmd/alex-server/main.go` 注入; 在 `deployments/helm/observability/values.yaml` 添加 Tempo/Loki 集群配置; 编写 `scripts/infra/bootstrap-observability.sh` 自动化部署; 在 `tests/observability/tracing_integration_test.go` 与 `tests/observability/logging_test.go` 验证采集链路。
  - **验收方案**: `tests/observability/*` 全量通过并生成追踪样例; Grafana Dashboard 模板经 SRE 审核上线; 灰度环境运行 48 小时无采集丢失或告警。

### 5.3 数据与权限
- **多租户模型**: 在 Session/Task 模型中引入 Tenant + Role, 并通过数据库迁移与缓存兼容策略保障平滑上线。
  - **代码执行计划**: 更新 `internal/session/model.go` 与 `internal/session/repository.go` 增加 `TenantID` 字段; 编写 `internal/session/migrations/2024XXXX_add_tenant.sql` 与 `scripts/migrations/run.sh` 联动执行; 在 `web/lib/types.ts` 与 `web/lib/api.ts` 同步字段并更新 `Authorization` Header; 在 `tests/session/tenant_authorization_test.go` 校验访问控制与缓存一致性。
  - **验收方案**: 数据库迁移在影子环境执行并完成回滚演练; `tenant_authorization_test.go` 覆盖跨租户访问拒绝; Beta 租户试运行 1 周监控无越权告警。
- **审计日志**: 所有审批、执行动作记录审计事件, 支持合规导出。
  - **代码执行计划**: 在 `internal/audit/logger.go` 定义事件模型与写入接口; 更新 `internal/server/http/middleware/audit.go` 拦截关键请求; 在 `deployments/helm/audit/values.yaml` 配置存储 (Postgres/BigQuery); 在 `tests/audit/audit_log_export_test.go` 校验导出格式与权限。
  - **验收方案**: 审计日志需通过 SOC 合规审查并完成数据留存演练; `audit_log_export_test.go` 校验导出与权限; Staging 环境跑一周确认日志量与成本可控。
- **缓存策略**: 采用 Redis, 明确过期策略与缓存击穿保护。
  - **代码执行计划**: 新建 `internal/cache/redis/client.go` 封装连接与熔断; 在 `internal/session/service.go` 与 `internal/storage/file_store.go` 引入缓存读写与 `singleflight` 防击穿; 更新 `deployments/helm/cache/templates/statefulset.yaml` 配置 Redis Cluster; 在 `tests/cache/cache_invalidation_test.go` 验证失效策略与熔断逻辑。
  - **验收方案**: 压测中缓存命中率达到 80% 以上且无击穿; `cache_invalidation_test.go` 覆盖失效与熔断; 运维确认 Redis Cluster 主从切换演练通过。

### 5.4 DevOps 与交付
- **基础设施即代码**: Terraform/Helm 管理环境, 与现有部署脚本解耦并纳入 CI 检查。
  - **代码执行计划**: 在 `infrastructure/terraform/environments/` 拆分 `dev/staging/prod` 目录, 在 `main.tf` 引入 Helm Provider; 更新 `deployments/helm/README.md` 记录 GitOps 流程与回滚策略; 在 `scripts/ci/gitops-sync.sh` 实现与 ArgoCD 的自动同步并在 `tests/ci/gitops_pipeline_test.sh` 进行冒烟验证。
  - **验收方案**: `terraform fmt` 与 `terraform validate` 通过; `gitops_pipeline_test.sh` 在 CI 中稳定运行; 运维演练一次 GitOps 回滚并记录手册更新。
- **金丝雀发布**: 以 Ingress 权重 + 服务标签结合的方式分流, 支持按租户灰度。
  - **代码执行计划**: 在 `deployments/helm/*/values-canary.yaml` 增加 `canaryWeight` 与 `matchLabels`; 更新 `scripts/cd/deploy_canary.sh` 支持 Prometheus 指标判断回滚; 在 `docs/operations/canary_playbook.md` 撰写操作手册; 在 `tests/canary/traffic_shift_test.go` 模拟灰度流量并校验监控阈值。
  - **验收方案**: 通过 `traffic_shift_test.go` 验证 10/90→50/50 流量切换; Canary 发布演练中记录 Prometheus 指标阈值与自动回滚; 运营值班确认操作手册完整。
- **自动化测试矩阵**: 构建前端 E2E、后端契约、混沌测试, 纳入 CI/CD Gate。
  - **代码执行计划**: 在 `web/e2e` 扩充核心脚本并通过 `package.json` 添加 `test:e2e` 命令; 更新 `Makefile` 与 `scripts/ci/run-tests.sh` 将 `unit`、`integration`、`chaos` 组合执行; 在 `tests/chaos/chaos_runner_test.go` 编排基础混沌案例; 在 `docs/operations/testing_strategy.md` 记录执行规范与阈值。
  - **验收方案**: CI Pipeline 需在 PR 阶段跑通 `unit`/`integration`/`e2e`/`chaos` 流水线; 生成测试覆盖率报告并达到约定阈值; 发布前由 QA 审批测试策略文档。

## 6. 实施路线图 (季度节奏)

### Q1: 基础能力搭建
- 完成 AppShell 布局、事件流基础与通知骨架。
  - **代码执行计划**: 在 `web/app/layout.tsx`、`web/components/layout/AppShell.tsx` 实现双栏布局并通过 `web/e2e/app-shell.responsive.spec.ts` 校验; 新建 `web/components/events/EventStream.tsx` 与 `web/hooks/useConnectionStatus.ts` 打通事件流; 在 `web/components/notification/NotificationBell.tsx` 和 `NotificationDrawer.tsx` 打通前端通知骨架。
  - **验收方案**: E2E 截图基线经设计签字; 事件流在 Staging 执行长任务可完整回放; 通知骨架触发浏览器推送与站内消息无报错。
- 建立单体内模块化与内存事件总线。
  - **代码执行计划**: 在 `internal/server/http/handlers` 拆分会话与工作流路由; 新增 `internal/events/bus.go` 与 `internal/events/inmemory` 实现内存事件流; 在 `cmd/alex-server/main.go` 注入并通过 `tests/events/integration_test.go`、`tests/server/session_http_test.go` 覆盖。
  - **验收方案**: 关键后端测试全绿并覆盖事件发布链路; 通过预发演练验证事件重放与幂等; 架构评审确认模块拆分文档。
- 搭建 observability 最小集。
  - **代码执行计划**: 扩展 `internal/observability/tracing.go` 接入 OpenTelemetry SDK, 在 `cmd/alex-server/main.go`、`cmd/alex/main.go` 注入; 更新 `deployments/helm/observability/values.yaml` 启用 Prometheus/Grafana/Loki; 在 `docs/operations/observability.md` 更新监控使用手册。
  - **验收方案**: OpenTelemetry 采集在 Staging 连续 7 天无断点; 监控手册经 SRE 审核; Prometheus/Grafana 仪表盘同步生成并归档。

### Q2: 体验强化与协同
- 上线 Plan 概览、时间线回放与文件 diff。
  - **代码执行计划**: 合并 `web/components/plan/PlanOverview.tsx`、`web/components/timeline/TimelineView.tsx`、`web/components/files/DiffViewer.tsx` 至会话流程; 更新 `web/lib/types.ts`、`web/lib/api.ts` 同步字段; 在相关组件 `__tests__` 目录及 `web/components/files/__tests__/DiffViewer.test.tsx` 增加 Vitest 覆盖; 后端在 `internal/session/service.go` 与 `internal/storage/file_store.go` 提供接口并在 `tests/session/tenant_authorization_test.go`、`tests/storage/file_store_test.go` 校验。
  - **验收方案**: 前后端契约测试通过并在 Storybook 演示中获产品确认; E2E 跑通从 Plan 到文件 diff 的跨组件跳转; Beta 用户试用满意度 ≥ 80%。
- 发布多租户与权限模型。
  - **代码执行计划**: 执行 `internal/session/migrations/2024XXXX_add_tenant.sql`、更新 `internal/session/repository.go` 支持租户过滤; 在 `web/lib/api.ts` 携带租户 Header 与缓存键; 在 `tests/session/tenant_authorization_test.go` 校验权限与缓存兼容。
  - **验收方案**: 迁移脚本在灰度库执行并验证回滚; `tenant_authorization_test.go` 确认跨租户访问受限; Beta 团队验证租户切换体验无异常。
- 推出离线通知与 Slack 集成。
  - **代码执行计划**: 在 `internal/notification/provider/slack.go` 实现渠道, 新建 `cmd/notification-consumer/main.go` 订阅事件; 前端扩展 `web/hooks/useNotifications.ts`、`web/components/system/OfflineBanner.tsx`; 在 `tests/notification/provider_test.go` 与 `web/components/system/__tests__/OfflineBanner.test.tsx` 补充。
  - **验收方案**: Slack 渠道在 Sandbox 工作区完成消息验证; `provider_test.go` 与 Offline Banner 单测全绿; 实际断网演练下通知可回放且用户收到补发提醒。
- 完成设计系统与 i18n 增量落地。
  - **代码执行计划**: 在 `web/app/globals.css` 注入设计 Token, 更新 `web/components/ui/theme-provider.tsx` 与 `web/lib/i18n.tsx`; 扩展 `web/locales/{en,zh}/conversation.json`; 在 `web/components/ui/__tests__/theme-provider.test.tsx`、`web/lib/__tests__/i18n-provider.test.tsx` 校验。
  - **验收方案**: 设计评审确认 Token 对齐; i18n lint 与单测通过; Beta 用户在中英文环境体验无错漏文案。

### Q3: 智能化与规模化
- 引入任务自动总结与下一步推荐。
  - **代码执行计划**: 在 `internal/workflow/recommendation.go` 调用 LLM, 写入 `internal/events` 推送结果; 在 `web/components/summary/SessionSummary.tsx` 呈现; 在 `web/lib/api.ts` 定义请求; 通过 `tests/workflow/recommendation_test.go` 验证。
  - **验收方案**: `recommendation_test.go` 覆盖成功/失败分支; 人工评估 20 条推荐准确性并达成 ≥80% 满意度; 日志审计确认推荐调用链可追踪。
- 强化协作 (批注、共享、多人编辑) 与外部共享。
  - **代码执行计划**: 在 `internal/collaboration/sharing.go`、`internal/session/http/share_handler.go` 实现共享逻辑; 前端扩展 `web/components/timeline/CommentDrawer.tsx`、`web/components/session/ShareDialog.tsx`; 在 `tests/collaboration/share_session_test.go`、`web/components/timeline/__tests__/CommentDrawer.test.tsx` 校验。
  - **验收方案**: 共享链接权限通过渗透测试; 协作 E2E 用例验证多人批注同步; 法务确认对外分享条款并更新文档。
- 将事件总线升级为 Kafka 并拆分通知服务。
  - **代码执行计划**: 在 `internal/events/kafka` 实现生产/消费, 更新 `cmd/alex-server/main.go` 与 `cmd/notification-consumer/main.go` 使用 Kafka; 调整 `deployments/docker-compose.yml`、`infrastructure/terraform/modules/kafka`; 在 `tests/events/kafka_integration_test.go` 验证。
  - **验收方案**: Kafka 集群在预发环境跑稳定性测试 (24h); `kafka_integration_test.go` 覆盖断线重连; 监控确认生产/消费延迟符合 SLO。
- 落地金丝雀 + 混沌体系并提升扩展性。
  - **代码执行计划**: 完成 `scripts/cd/deploy_canary.sh`、`tests/canary/traffic_shift_test.go`、`tests/chaos/chaos_runner_test.go` 的灰度/混沌自动化; 在 `deployments/helm/*/values.yaml` 配置 HPA 与资源配额; 利用 `tests/load/load_test.go` 验证 10k 并发。
  - **验收方案**: 灰度脚本在预发完成一次完整演练并生成报告; Chaos/Load 测试结果达到设定阈值; 运维与业务方共同签署扩展性评估。

## 7. 成功度量指标 (KPIs)

- **Task Success Rate**: 任务成功率提升 ≥ 20%。
- **Mean Time To Insight**: 用户获取关键输出时间缩短 30%。
- **System Availability**: 服务可用性 ≥ 99.9%。
- **Front-End Performance**: 首屏渲染 < 2.5s, 交互延迟 < 100ms。
- **User Engagement**: 每日活跃团队数提升 2 倍, Plan 审批使用率 ≥ 60%。

## 8. 依赖与风险

- **数据安全**: 多租户与日志审计增加合规复杂度, 需投入安全团队。
- **技术复杂度**: 引入 Kafka、Asynq、OpenTelemetry 等组件需额外运维与治理能力。
- **UI 重构风险**: 大规模布局调整需阶段灰度, 避免影响现有用户。
- **团队协作**: 需要跨前端、后端、产品、设计协同, 建议设立专项 Tiger Team。

