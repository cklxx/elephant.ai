# elephant.ai / ALEX Roadmap

This roadmap is a guided reading order for the codebase. Follow it when you onboard, or when you need to trace how an event trave
ls from the CLI/web into the agent core and back to the user.

## 1) Orientation

Start with the "why" and the top-level mechanics:

- `README.md` — product overview, quickstart (`./dev.sh`), and high-level architecture.
- `docs/README.md` — table of contents for deeper docs.
- `docs/reference/ARCHITECTURE_AGENT_FLOW.md` — the Think → Act → Observe loop, event lifecycle, and orchestration semantics.

## 2) Entry Surfaces

Trace how requests arrive and responses are streamed back:

- CLI/TUI: `cmd/alex`, wired through `internal/cli` for prompts, history, and output formatting.
- HTTP + SSE server: `cmd/alex-server`, `internal/server`, `internal/http` for routing and streaming.
- Web dashboard: `web/` (see `web/README.md`) for Next.js app structure and dev tasks.

## 3) Agent Runtime (Go)

Core execution path and orchestration hooks:

- Application services: `internal/agent/app` coordinates conversations and tool calls.
- Domain model + ports: `internal/agent/domain`, `internal/agent/ports` define aggregates, events, and boundaries.
- Dependency wiring: `internal/di` assembles adapters (LLM, vector stores, tools, storage).

## 4) Context, Tools, and Skills

How the agent gathers context and safely executes actions:

- Context builder + prompt injection: `internal/context`, `internal/context/manager.go`.
- Tool registry and built-ins: `internal/toolregistry`, `internal/tools`.
- Skills (Markdown playbooks): `skills/` with LLM-exposed wrappers in `internal/tools/builtin/skills.go`.
- Skill catalog generation: `internal/skills/index.go` produces metadata consumed by the web app.

## 5) Data Plane and Events (Web)

Follow the event stream that powers the dashboard:

- SSE ingestion and dedupe: `web/hooks/useSSE.ts`.
- Attachment hydration + renderers: `web/lib/events/attachmentRegistry.ts`.
- Conversation stream UI: `web/components/agent/ConversationEventStream.tsx`.
- Right-hand resources (skills, attachments): `web/app/conversation/ConversationPageContent.tsx`.
- Catalog generation bridge: `web/scripts/generate-skills-catalog.js` → `web/lib/generated/skillsCatalog.json`.

## 6) Persistence and Ops

Supporting infrastructure and migrations:

- Database migrations: `migrations/` and `deploy/` manifests for cluster installs.
- Configuration: `configs/` and service defaults under `internal/config`.
- Deployment helpers: `deploy.sh` and Docker images built via `Makefile` targets.

## 7) Quality Gates

Run these to validate changes end-to-end:

- Go lint + tests: `./scripts/run-golangci-lint.sh run --timeout=10m ./...` and `make test`.
- Web lint + unit tests: `npm --prefix web run lint` and `npm --prefix web test`.
- End-to-end + evaluations: `npm --prefix web run e2e` and `./dev.sh test` for the orchestrated suite.

## 8) Contribution entrypoints (MVP slices)

Use these when turning roadmap items into issues. Each slice is sized for a focused PR and maps to concrete paths.

- [ ] Cross-process orchestration (MVP: resume from persisted state on server restart) — `internal/session/state_store`, `internal/agent/app`.
- [ ] Planner replan (MVP: replan after tool failure + emit a replan event) — `internal/agent/domain/react_runtime.go`, `internal/agent/domain/events.go`.
- [ ] Tool SLA profiles (MVP: record per-tool latency/cost in registry + surface via event stream) — `internal/toolregistry`, `internal/agent/domain/events.go`, `web/lib/events`.
- [x] Eval gate automation (MVP: optional `workflow_dispatch` + `eval/` tag quick eval via `scripts/eval-quick.sh`) — `.github/workflows/eval.yml`, `scripts/eval-quick.sh`.

Suggested labels: `good first issue`, `help wanted`, `mvp-slice`.

## Agent System Robustness TODOs

每个能力拆成可落地的 feature 清单，并根据代码库现状标记是否已具备（`[x]` 已实现，`[ ]` 待补齐）。路径提示用于快速定位实现。

### 底座
- 任务/环境契约（Task & Env Contract）
  - [x] 输入/输出 schema 与任务状态枚举（`internal/server/http`, `internal/agent/types`）
  - [x] 执行终止条件与安全目录约束（`internal/workflow`, `internal/session/filestore`）
  - [ ] 自动化需求澄清与契约化验收门禁
- 状态机与运行时（Agent Runtime）
  - [x] 可序列化节点状态机（`internal/workflow`, `internal/agent/app`）
  - [x] 断点快照/重放（`internal/session/state_store`, `internal/context`）
  - [ ] 跨进程/重启级别的自动续跑 orchestration
- 消息与数据结构（Typed I/O）
  - [x] 工具/事件强类型定义（`internal/mcp`, `internal/toolregistry`, `internal/agent/types`）
  - [x] JSON RPC 与 HTTP schema 校验（`internal/server/http`）
  - [ ] 计划/证据 AST 级约束与静态验证
- 上下文工程（Context Engineering）
  - [x] system/policy/task/memory 分层拼装（`internal/context`）
  - [x] 动态摘要与上下文压缩（`internal/context/compress.go`）
  - [ ] 成本/Token 预算器驱动的上下文裁剪

### 能力
- 计划器（Planner）
  - [x] 分阶段执行计划（prepare/execute/summarize/persist 节点，`internal/workflow`）
  - [ ] 子目标/子计划显式建模与迭代 replan
- 工具抽象层（Tool Interface Layer）
  - [x] 带 schema 的工具注册与元数据（`internal/toolregistry`, `internal/tools`）
  - [x] 权限/模式预设（full/read-only/safe，`internal/agent/presets`）
  - [ ] 工具 SLA/成本画像驱动的动态路由
- 工具选择策略（Tool Policy）
  - [x] 预设驱动的开关/阻断策略（`internal/agent/presets`, `internal/server/http` 中鉴权）
  - [ ] 历史效果/延迟感知的多臂 bandit 选择
  - [ ] 自动降级链（缓存命中→弱工具→提示用户）
- 执行器（Executor）
  - [x] 并发/批处理的工具执行与去重（`internal/workflow`, `internal/tools/builtin` 并发用例）
  - [x] 超时/重试/限流封装（`internal/errors/retry.go`, `internal/httpclient`）
  - [ ] 结果级别缓存与幂等锁
- 观测与归因（Observation & Attribution）
  - [x] 事件/附件可追溯（`internal/analytics`, `internal/server/app`）
  - [ ] 结论必须绑定证据的置信度控制
- 检索与知识层（RAG / KB）
  - [x] 向量/切片化嵌入与检索管线（`internal/rag`）
  - [ ] 检索冲突消解与多源合并策略
- 记忆系统（Memory）
  - [x] 用户/对话情景记忆服务（`internal/memory`）
  - [ ] 记忆纠错/遗忘策略与质量度量
- 验证器（Verifier / Checker）
  - [x] 工具/输入规则校验与丰富单测覆盖（`internal/server/http`, `internal/tools` 测试）
  - [ ] 关键节点强制 verify gate（再执行/反例搜寻）
- 自我一致性与不确定性管理
  - [ ] 多样化采样+投票的执行路径
  - [ ] 置信度建模与“未知/澄清”策略下沉到运行时

### 鲁棒
- 错误恢复与鲁棒策略（Recovery）
  - [x] 错误分类与可重试/永久错误分层（`internal/errors`）
  - [x] 任务取消/回滚能力（`internal/workflow`, `internal/server/app`）
  - [ ] 跨工具的补偿事务与自动修复 playbook
- 安全与权限（AuthZ / Policy）
  - [x] 工具安全预设与模式限制（safe/read-only，`internal/agent/presets`）
  - [x] OAuth/Token 保护与路由鉴权（`internal/auth`, `internal/server/http`）
  - [ ] 数据脱敏与敏感操作二次确认
- 沙箱与隔离（Sandboxing）
  - [x] 代码执行沙箱/资源限制（`internal/tools/builtin/code_execute` 测试）
  - [ ] 精细化文件/网络隔离与资源配额
- 多智能体协作（Multi-agent）
  - [x] 子代理/委派执行（subagent 事件流，`web` 与 `internal/agent` 测试）
  - [ ] 仲裁器/冲突合并策略
- 学习与自我改进（Online Learning Loop）
  - [ ] 反馈采集→策略更新闭环
  - [ ] 轨迹案例库与版本化策略灰度

### 评分 / 性能
- 评测体系（Eval Harness）
  - [x] SWE-bench/agent_eval 套件与打分（`evaluation/`）
  - [ ] 自动回归门禁与线上 A/B
- 观测性（Tracing/Telemetry）
  - [x] 事件流、日志与指标采集（`internal/observability`, `internal/analytics`）
  - [ ] 全链路可重放与成本分析看板
- 缓存与去重（Caching/Dedup）
  - [ ] 工具/检索结果缓存与语义去重
  - [ ] 跨会话热点缓存
- 模型与推理策略（Model Routing）
  - [x] 预设驱动的模型/工具能力开关（`internal/agent/presets`, `internal/config`）
  - [ ] 动态路由与温度/预算调优

### 工程化
- 提示与策略版本管理（PromptOps）
  - [x] Prompt 配置与预设版本（`internal/context`, `internal/agent/presets`）
  - [ ] 版本化/灰度与回滚流水线
- 数据治理（Data Governance）
  - [ ] 数据分级/保留/删除流程与日志隔离
  - [ ] PII 脱敏与合规审计
- 人机交互层（UX for Agents）
  - [x] 过程可视化、SSE 流、可打断（stop/cancel，`web`, `internal/server/http`）
  - [x] 敏感动作提示/订阅管理（`internal/auth/app`, web 订阅 UI）
  - [ ] 用户纠错/改目标的回传与重试入口

---

## 与顶级项目差距分析（对标 Manus）

基于对当前项目功能与 Manus（自主 AI 代理系统）的对比分析，识别出的关键差距和改进方向。Manus 是由 Butterfly Effect Pte. Ltd. 开发的自主人工智能代理，采用多智能体架构，在独立云端虚拟环境中运行，能够无需持续人类指导即可独立执行复杂的现实世界任务。

### 多智能体架构与执行环境

- **独立云端虚拟环境**
  - [x] 基础子代理委派（`internal/tools/builtin/subagent.go`）
  - [ ] 独立执行环境隔离（每个智能体独立沙箱/容器）
  - [ ] 云端虚拟环境管理（Docker/K8s 容器编排）
  - [ ] 环境资源配额与限制（CPU、内存、存储）
  - [ ] 环境生命周期管理（创建、销毁、快照、恢复）
  - 路径：扩展 `internal/tools/builtin/subagent.go`，新增 `internal/environment/vm/` 模块

- **非对等多智能体并行协作 agent as tool**
  - [x] 基础并行任务执行（subagent 支持）
  - [ ] 智能体间通信与协调机制
  - [ ] 任务分配与负载均衡
  - [ ] 智能体状态同步与一致性
  - 路径：扩展 `internal/agent/app/coordinator.go`，新增 `internal/agent/orchestration/` 模块

- **自主任务规划**
  - [x] 基础任务分解（`internal/workflow`）
  - [ ] 高级自主规划（无需人类干预的任务分解与执行）
  - [ ] 动态计划调整（基于执行结果 replan）
  - [ ] 多步骤流程自动化（端到端任务执行）
  - [ ] 目标导向的长期规划
  - 路径：扩展 `internal/agent/domain/react_runtime.go`，新增 `internal/agent/planner/` 模块

### 多模态处理能力

- **多模态输入处理**
  - [x] 图像分析（`internal/tools/builtin/seedream.go` vision_analyze）
  - [x] 基础附件支持（`internal/attachments`）
  - [ ] PDF 文档解析与提取
  - [ ] Excel/CSV 表格处理与分析
  - [ ] 视频内容理解与摘要
  - [ ] 音频转录与分析
  - [ ] 多模态内容融合理解
  - 路径：扩展 `internal/attachments`，新增 `internal/multimodal/` 模块

- **多模态输出生成**
  - [x] 图像生成（seedream image-to-image）
  - [x] 视频生成（seedream video）
  - [x] PPT 生成（`internal/tools/builtin/pptx_from_images.go`）
  - [ ] 交互式仪表板生成
  - [ ] 数据可视化图表生成
  - [ ] 多格式报告生成（Markdown、PDF、HTML）
  - [ ] 富媒体内容组合
  - 路径：扩展现有工具，新增 `internal/multimodal/generation/` 模块

### 研究与分析能力

- **多源研究**
  - [x] 基础网页搜索（`internal/tools/builtin/web_search.go`）
  - [x] 浏览器自动化（`internal/tools/builtin/browser.go`）
  - [ ] 多源信息聚合（网页、论文、文档、数据库）
  - [ ] 信息可信度评估与来源追踪
  - [ ] 结构化研究报告生成（带引用）
  - [ ] 研究深度与广度控制
  - 路径：扩展 `internal/tools/builtin/web_search.go`，新增 `internal/research/` 模块

- **数据分析与可视化**
  - [ ] 数据集加载与预处理
  - [ ] 统计分析（描述性统计、相关性分析）
  - [ ] 数据可视化生成（图表、仪表板）
  - [ ] 交互式数据分析
  - [ ] 数据洞察提取与报告
  - 路径：新增 `internal/tools/builtin/data_analysis.go`，集成数据分析库

### 异步执行与透明界面

- **异步任务执行**
  - [x] 后台任务执行（`internal/server/app/server_coordinator.go` ExecuteTaskAsync）
  - [ ] 云端持久化执行（用户离线后继续执行）
  - [ ] 任务队列与优先级管理
  - [ ] 长时间任务支持（小时/天级别）
  - [ ] 任务完成通知（邮件、Webhook、推送）
  - [ ] 任务暂停与恢复
  - 路径：扩展 `internal/server/app`，新增 `internal/taskqueue/` 模块

- **透明执行界面（"Manus 的计算机"）**
  - [x] 基础事件流可视化（`web/components/agent/ConversationEventStream.tsx`）
  - [ ] 执行过程完整回放
  - [ ] 步骤级详细日志展示
  - [ ] 执行时间线可视化
  - [ ] 资源使用监控（CPU、内存、网络）
  - [ ] 执行决策树可视化
  - 路径：扩展 `web/components/agent/`，新增执行可视化组件

- **任务状态管理**
  - [x] 基础任务状态跟踪（`internal/server/ports/task_store.go`）
  - [ ] 任务依赖关系管理
  - [ ] 任务失败自动重试策略
  - [ ] 任务结果持久化与版本管理
  - [ ] 任务执行历史查询
  - 路径：扩展 `internal/server/ports`，增强任务存储能力

### 内容创作与文件管理

- **内容创作能力**
  - [x] PPT 生成（`internal/tools/builtin/pptx_from_images.go`）
  - [x] 视频生成（`internal/tools/builtin/seedream.go`）
  - [ ] 文章/博客自动生成
  - [ ] 营销材料生成（海报、宣传页）
  - [ ] 技术文档生成（API 文档、用户手册）
  - [ ] 多语言内容生成
  - 路径：扩展 `internal/tools/builtin/`，新增内容生成工具

- **文件格式处理**
  - [x] 基础文件读写（`internal/tools/builtin/file_*.go`）
  - [ ] PDF 解析与提取（文本、表格、图像）
  - [ ] Excel/CSV 读取与写入
  - [ ] 图像处理（裁剪、转换、标注）
  - [ ] 文档格式转换（Markdown ↔ PDF ↔ DOCX）
  - [ ] 批量文件处理
  - 路径：新增 `internal/tools/builtin/file_formats/` 模块

- **网页自动化增强**
  - [x] 基础浏览器自动化（`internal/tools/builtin/browser.go`）
  - [ ] 多步骤流程自动化（表单填写、多页面导航）
  - [ ] 动态内容等待与交互
  - [ ] 截图与录屏能力
  - [ ] 网页数据提取与结构化
  - [ ] 反爬虫策略处理
  - 路径：扩展 `internal/tools/builtin/browser.go`，增强自动化能力

### 记忆与学习系统

- **长期记忆与学习**
  - [x] 基础记忆系统（`internal/memory`）
  - [x] 向量存储与检索（`internal/rag`）
  - [ ] 用户偏好学习与个性化
  - [ ] 任务模式识别与复用
  - [ ] 错误模式学习与避免
  - [ ] 成功案例库构建
  - [ ] 记忆质量评估与优化
  - 路径：扩展 `internal/memory`，新增 `internal/learning/` 模块

- **上下文适应**
  - [x] 基础上下文管理（`internal/context`）
  - [ ] 上下文自动更新与维护
  - [ ] 跨会话上下文共享
  - [ ] 上下文版本管理
  - [ ] 上下文压缩与优化
  - 路径：扩展 `internal/context`，增强上下文智能管理

### 优先级建议

**P0（核心差距，对标 Manus 核心能力）**
1. 独立云端虚拟环境（每个智能体独立执行环境）
2. 多智能体并行协作（智能体间通信与协调）
3. 自主任务规划（无需人类干预的端到端执行）
4. 异步任务执行（云端持久化，用户离线继续）

**P1（重要功能，提升自主性）**
1. 多模态处理增强（PDF、Excel、视频、音频）
2. 研究与分析能力（多源信息聚合、结构化报告）
3. 透明执行界面（完整执行过程可视化）
4. 数据分析与可视化（统计、图表、仪表板）

**P2（增强功能，差异化优势）**
1. 内容创作能力（文章、营销材料、技术文档）
2. 文件格式处理（PDF、Excel、图像处理）
3. 网页自动化增强（多步骤流程、数据提取）
4. 长期记忆与学习（用户偏好、任务模式学习）

**P3（长期优化）**
1. 高级学习系统（反馈循环、模型微调）
2. 数据治理完善
3. 部署扩展性（K8s、高可用）
4. 性能测试套件
