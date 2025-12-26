# elephant.ai / ALEX Roadmap

This roadmap is a guided reading order for the codebase. Follow it when you onboard, or when you need to trace how an event trave
ls from the CLI/web into the agent core and back to the user.

## 1) Orientation

Start with the "why" and the top-level mechanics:

- `README.md` — product overview, quickstart (`./dev.sh`), and high-level architecture.
- `docs/README.md` — table of contents for deeper docs.
- `docs/AGENT.md` — the Think → Act → Observe loop, event lifecycle, and orchestration semantics.

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
- Deployment helpers: `deploy.sh`, `k8s/`, and Docker images built via `Makefile` targets.

## 7) Quality Gates

Run these to validate changes end-to-end:

- Go lint + tests: `./scripts/run-golangci-lint.sh run --timeout=10m ./...` and `make test`.
- Web lint + unit tests: `npm --prefix web run lint` and `npm --prefix web test`.
- End-to-end + evaluations: `npm --prefix web run e2e` and `./dev.sh test` for the orchestrated suite.

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
