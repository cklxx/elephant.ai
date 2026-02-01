# Track 2: 系统交互层 — OKR-First ROADMAP

> **Parent:** `docs/roadmap/roadmap-lark-native-proactive-assistant.md`
> **Owner:** cklxx
> **Created:** 2026-02-01
> **Last Updated:** 2026-02-01

---

## Objective & Key Results (O2)

**Objective (O2):** 工具链稳定、可度量、可路由，支撑“日程+任务”闭环与可选 Coding 能力。

**Key Results:**
- **KR2.1** 核心工具 SLA 基线 + 路由/降级策略可用
- **KR2.2** Coding Agent Gateway 可接入 Codex/Claude Code（可选）
- **KR2.3** Scheduler 支撑提醒/跟进闭环

---

## Roadmap (OKR-driven)

### M0: 工具与执行基线（支撑 KR2.1 / KR2.3）
- 工具注册/权限体系稳定，SLA 采集基线
- Sandbox/执行环境稳定
- Scheduler 基础可用

### M1: 工具治理 + 智能路由（支撑 KR2.1 / KR2.3）
- Tool allow/deny policy (D1)
- SLA 画像 + 动态路由/降级
- Scheduler Job 持久化/冷却/并发 (D4)

### M2: Coding Gateway 全链路（支撑 KR2.2）
- Codex/Claude Code/Kimi 多 adapter
- 修复循环 + 并行编码
- 云端隔离环境

### M3: 工具自治
- 历史效果路由 + 自动优化
- 工具链可配置扩展

---

## Baseline & Gaps (Current State)

**关键路径：** `internal/tools/` · `internal/coding/` · `internal/scheduler/` · `skills/`

### 1. 工具引擎

> `internal/tools/` · `internal/tools/builtin/`

**现状**
- 69+ 个内置工具，7 层分类（L0 编排 → L7 媒体）
- 带 schema 的工具注册 + 元数据
- 权限预设（Full/ReadOnly/Safe/Sandbox/Architect 五档）
- 并发执行 + 结果去重 + 超时控制（重试/限流部分实现）
- 工具重叠矩阵已梳理（file ops 双路径、shell 本地 vs 沙箱、web 静态 vs 交互）

**Milestones (initiatives)**

#### M0: 引擎稳固

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 工具注册与元数据 | 69+ 工具带 schema | ✅ 已实现 | `tools/registry.go` |
| 权限预设 | Full/ReadOnly/Safe/Sandbox/Architect 五档 | ✅ 已实现 | `internal/agent/domain/presets/` |
| 并发执行 | 批量工具调用 + 去重 | ✅ 已实现 | `react/tool_batch.go` |
| 超时/重试/限流 | 超时控制已实现；通用重试/限流部分实现 | ⚙️ 部分 | `internal/errors/retry.go` |
| SLA 基线采集 | 每个工具的延迟/成本/可靠性/成功率指标采集 | ❌ 待实现 | `tools/sla.go` |

#### M1: 智能工具路由 + 工具治理

| 项目 | 描述 | 状态 | 路径 | OpenClaw Delta |
|------|------|------|------|------|
| **Tool allow/deny policy** | `ToolPolicy` deny-first 语义，支持按 group/tool 粒度的 allow/deny 规则 | ❌ 待实现 | `toolregistry/policy.go` | **D1** |
| **policyAwareRegistry** | 仿照 `filteredRegistry` 模式，在 `List()`/`Get()` 中加 policy 过滤层 | ❌ 待实现 | `toolregistry/registry.go` | **D1** |
| **Group tags 补充** | 所有 builtin tools 的 `Metadata().Tags` 补充 `group:` 前缀标签 | ❌ 待实现 | 各 `builtin/*/` tool 文件 | **D1** |
| **Profile-based policy** | `PolicyProfile` 按 channel/role/session 维度选择策略；默认 default/lark_user/web_sandbox 三档 | ❌ 待实现 | `toolregistry/policy.go` + `configs/tools/policy.yaml` | **D1** |
| 工具 SLA 画像 | 从采集数据构建每个工具的性能画像 | ❌ 待实现 | `tools/sla.go` | |
| 动态路由 | 基于 SLA 画像 + 当前任务需求自动选择工具链 | ❌ 待实现 | `tools/router.go` | |
| 自动降级链 | 缓存命中 → 弱工具 → 提示用户，按链路依次尝试 | ❌ 待实现 | `tools/fallback.go` | |
| 结果缓存 | 工具结果缓存 + 语义去重（相同查询不重复执行） | ❌ 待实现 | `tools/cache.go` | |
| 工具热加载 | 运行时注册新工具，无需重启 | ❌ 待实现 | `tools/registry.go` | |

#### M2: 工具自治

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 历史效果路由 | 基于历史成功率/延迟/成本的 multi-armed bandit | ❌ 待实现 | `tools/router_adaptive.go` |
| 跨会话热点缓存 | 高频工具结果跨会话缓存 | ❌ 待实现 | `tools/cache.go` |
| 工具组合推荐 | 基于任务自动推荐工具组合（而非单工具） | ❌ 待实现 | `tools/recommender.go` |

---

### 2. 沙箱与执行环境

> `internal/tools/builtin/execution/` · `internal/tools/builtin/sandbox/`

**现状**
- 代码执行沙箱（隔离环境）
- Shell 执行（本地 + 沙箱）
- 浏览器自动化（Headless browser）
- 文件操作（本地 + 沙箱双路径）

**Milestones (initiatives)**

#### M0: 沙箱稳固

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 代码执行沙箱 | 隔离环境执行代码 | ✅ 已实现 | `builtin/execution/` |
| Shell 执行 | 本地/沙箱 shell | ✅ 已实现 | `builtin/execution/` |
| 浏览器自动化 | Headless browser | ✅ 已实现 | `builtin/sandbox/` |
| 文件操作 | 本地/沙箱读写 | ✅ 已实现 | `builtin/fileops/` |

#### M1: 增强执行

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 精细化资源隔离 | 文件/网络/CPU/内存 per-task 配额 | ❌ 待实现 | `builtin/execution/` |
| 多步浏览器流程 | 表单填写、多页导航、动态内容等待 | ❌ 待实现 | `builtin/sandbox/` |
| 执行环境快照 | 保存/恢复沙箱状态 | ❌ 待实现 | `builtin/execution/` |
| 执行结果可视化 | 截图/录屏/HTML 快照 输出给用户 | ❌ 待实现 | `builtin/sandbox/` |

#### M2: 云端隔离环境

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 独立容器 | 每个 Agent 独立 Docker 容器 | ❌ 待实现 | 新增 `internal/environment/vm/` |
| 容器编排 | K8s Job 管理容器生命周期 | ❌ 待实现 | `internal/environment/orchestrator.go` |
| 容器快照与恢复 | CRIU / Docker checkpoint 断点恢复 | ❌ 待实现 | `internal/environment/snapshot.go` |
| 资源配额管理 | 按用户/任务分配 CPU/内存/存储/网络 | ❌ 待实现 | `internal/environment/quota.go` |

---

### 3. Coding Agent Gateway

> 新增 `internal/coding/`

**架构定位：** elephant.ai 作为 meta-agent，编排外部 coding agent CLI（Codex、Claude Code、Kimi K2 等）完成软件工程任务。**Coding Gateway 是可选增强能力**，不作为“日程+任务”闭环的前置。

**Milestones (initiatives)**

#### M0: Gateway 基础

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Gateway 抽象接口 | 统一接口：Submit / Stream / Cancel / Status | ❌ 待实现 | `coding/gateway.go` |
| 首个 adapter | Claude Code CLI 或 Codex CLI 跑通端到端 | ❌ 待实现 | `coding/adapters/` |
| 本地 CLI 自动探测 | 启动时检测本地已安装的 coding agent CLI（`which codex`/`which claude`），有则自动注册为可用 adapter | ❌ 待实现 | `coding/adapters/detect.go` |
| 任务翻译 | 用户自然语言 → coding agent 结构化指令 | ❌ 待实现 | `coding/task.go` |
| 工作目录管理 | 隔离工作目录 / git worktree 管理 | ❌ 待实现 | `coding/workspace.go` |
| 构建验证 | Agent 产出后自动 build 确保编译通过 | ❌ 待实现 | `coding/verify_build.go` |

#### M1: 全链路编码

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Claude Code adapter | 通过 `claude` CLI 交互，流式输出，项目上下文传递 | ❌ 待实现 | `coding/adapters/claude_code.go` |
| Codex adapter | 通过 `codex` CLI headless 模式交互 | ❌ 待实现 | `coding/adapters/codex.go` |
| Kimi K2 adapter | 对接 Kimi K2 编码能力 | ❌ 待实现 | `coding/adapters/kimi.go` |
| 通用 adapter 框架 | 可插拔注册机制，新增 agent 只需实现接口 | ❌ 待实现 | `coding/adapters/registry.go` |
| 上下文组装 | 自动收集项目结构、相关文件、git diff、测试状态 | ❌ 待实现 | `coding/context.go` |
| 任务拆解引擎 | 复杂需求拆解为原子子任务序列 | ❌ 待实现 | `coding/decomposer.go` |
| Agent 能力画像 | 各 agent 的语言/框架/任务类型能力矩阵 | ❌ 待实现 | `coding/profiles.go` |
| 规则路由 | 语言 + 任务类型 → 首选 agent，带降级链 | ❌ 待实现 | `coding/router.go` |
| 测试验收 | 自动运行项目测试 + lint + 解析结果 | ❌ 待实现 | `coding/verify_test.go` |
| Diff 审查 | 自动 review agent 产出的 diff，检测异常变更 | ❌ 待实现 | `coding/verify_diff.go` |
| Git 状态追踪 | 跟踪 agent 产生的文件变更、staged/unstaged | ❌ 待实现 | `coding/git_tracker.go` |
| 变更快照与回滚 | 每步可回滚，不满意一键撤回 | ❌ 待实现 | `coding/snapshot.go` |
| 自动 commit + PR | 验收通过后自动 commit + 创建 PR + 生成描述 | ❌ 待实现 | `coding/deliver.go` |
| 人工确认门禁 | 关键变更暂停等人工 review | ❌ 待实现 | `coding/approval.go` |

#### M2: 高级编码

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 多 Agent 并行编排 | 独立子任务分发给不同 agent 并行执行 | ❌ 待实现 | `coding/parallel.go` |
| 依赖感知调度 | 子任务间有依赖时按拓扑序串行 | ❌ 待实现 | `coding/scheduler.go` |
| 历史效果路由 | bandit 自适应，基于历史成功率/速度/成本 | ❌ 待实现 | `coding/router_adaptive.go` |
| 修复循环 | 验证失败 → 错误注入 → agent 修复 → 再验证，多轮 | ❌ 待实现 | `coding/fix_loop.go` |
| MCP 协议适配 | 通过 Model Context Protocol 与支持 MCP 的 agent 交互 | ❌ 待实现 | `coding/adapters/mcp.go` |
| 云端工作区 | 工作区运行在云端容器中 | ❌ 待实现 | `coding/workspace_cloud.go` |
| 跨轮次上下文保持 | 多次对话中保持同一项目编码上下文 | ❌ 待实现 | `coding/session.go` |

---

### 4. 数据与文件处理

> `internal/tools/builtin/fileops/` · `internal/tools/builtin/media/`

**现状**
- 文本文件读写（Markdown/JSON/YAML）
- 图像生成（text-to-image / image-to-image）
- PPT 生成（从图像/模板）
- 视频生成（text-to-video）

**Milestones (initiatives)**

#### M0: 已达成

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 文本文件读写 | Markdown/纯文本/JSON/YAML | ✅ 已实现 | `builtin/fileops/` |
| 图像生成 | text-to-image / image-to-image | ✅ 已实现 | `builtin/media/` |
| PPT 生成 | 从图像/模板 | ✅ 已实现 | `builtin/media/` |
| 视频生成 | text-to-video | ✅ 已实现 | `builtin/media/` |

#### M1: 多模态处理

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| PDF 解析 | 文本/表格/图像提取 | ❌ 待实现 | `builtin/fileops/` |
| Excel/CSV 处理 | 读取/写入/分析表格数据 | ❌ 待实现 | `builtin/fileops/` |
| 音频转录 | 语音 → 文字 | ❌ 待实现 | `builtin/media/` |
| 数据分析与可视化 | 统计分析 + 图表生成 | ❌ 待实现 | 新增 `builtin/data/` |
| 文档格式转换 | Markdown ↔ PDF ↔ DOCX | ❌ 待实现 | `builtin/fileops/` |

---

### 5. 技能系统

> `skills/` · `internal/skills/`

**现状**
- 12 个 Markdown 驱动技能（深度研究、会议纪要、邮件、PPT、视频等）
- 技能注册与 LLM 暴露
- 技能目录自动生成（web 消费）

**Milestones (initiatives)**

#### M0: 已达成

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| Markdown 驱动技能 | 12 个内置技能 | ✅ 已实现 | `skills/` |
| 技能注册与暴露 | LLM 可发现、可调用 | ✅ 已实现 | `internal/tools/builtin/session/` |

#### M1: 技能增强

| 项目 | 描述 | 状态 | 路径 |
|------|------|------|------|
| 多源研究引擎 | 网页 + 论文 + 文档 + 数据库聚合 | ❌ 待实现 | `skills/deep-research/` |
| 结构化报告 | 带引用、带置信度 | ❌ 待实现 | `skills/research-briefing/` |
| 用户自定义技能 | 用户通过 Markdown 定义自己的技能 | ❌ 待实现 | `internal/skills/custom.go` |
| 技能组合 | 多个技能串联执行（研究 → 报告 → PPT） | ❌ 待实现 | `internal/skills/compose.go` |

---

### 6. Scheduler 增强 (OpenClaw D4)

> `internal/scheduler/` · `internal/tools/builtin/session/`

**现状**
- robfig/cron v3，静态 trigger + OKR 动态 trigger + 5min 自动同步
- `AgentCoordinator.ExecuteTask` + Lark/Moltbook 通知
- 无运行时动态添加 job、无执行状态持久化、无冷却/并发控制

**Milestones (initiatives)**

#### M1: Job 持久化 + 执行状态

| 项目 | 描述 | 状态 | 路径 | OpenClaw Delta |
|------|------|------|------|------|
| **JobStore** | 文件化 Job 持久化（`~/.alex/scheduler/jobs/<id>.yaml`），YAML 原子写入 | ❌ 待实现 | `scheduler/job_store.go` | **D4** |
| **执行状态跟踪** | 每次触发更新 `last_run_at`/`last_run_status`/`consec_failures` | ❌ 待实现 | `scheduler/executor.go` | **D4** |
| **冷却控制** | `cooldown_seconds` 防止同一 job 在冷却期内重复触发 | ❌ 待实现 | `scheduler/executor.go` | **D4** |
| **并发控制** | `max_concurrency` + tryAcquire/release mutex 防重叠执行 | ❌ 待实现 | `scheduler/executor.go` | **D4** |
| **连续失败自动暂停** | consec_failures ≥ 阈值 → status=error 自动暂停 | ❌ 待实现 | `scheduler/executor.go` | **D4** |
| **动态 Job 创建工具** | `scheduler_create/list/delete/pause` 对话式工具，Agent 可在对话中创建定时任务 | ❌ 待实现 | `builtin/session/scheduler_tool.go` | **D4** |
| **isolated 模式** | `mode: isolated` job 创建独立 session 执行，不绑定已有会话 | ❌ 待实现 | `scheduler/scheduler.go` | **D4** |
| **启动恢复** | 重启时从 JobStore 加载持久化 jobs，自动恢复 cron 注册 | ❌ 待实现 | `scheduler/scheduler.go` | **D4** |

---

## 跨 Track 边界说明

### Lark 工具注册 (→ Track 3)

`internal/tools/builtin/larktools/` 中的 `lark_doc_*` / `lark_sheet_*` / `lark_wiki_*` / `lark_calendar_*` / `lark_task_*` / `lark_approval_*` 工具的**实现逻辑属于 Track 3**（Lark 领域），但通过 Track 2 的工具引擎注册机制暴露给 Agent。

### 验证逻辑 (→ Track 4 消费)

`coding/verify` 中的构建/测试/Lint/Diff 审查/修复循环是**规范实现位置**。Track 4 Shadow Agent 作为消费者调用此接口，仅编排重试策略与决策逻辑。

---

## 进度追踪

| 日期 | 模块 | 更新 |
|------|------|------|
| 2026-02-01 | All | Track 2 详细 ROADMAP 创建。工具引擎和沙箱基础完备，核心缺口在 Coding Agent Gateway 和智能工具路由。 |
| 2026-02-01 | Gateway/Boundary | Review 优化：新增跨 Track 边界说明（larktools 端口/适配器约定、verify 消费关系）。 |
| 2026-02-01 | All | 实现审计修正：工具数 83→69+；权限预设 三档→五档（Full/ReadOnly/Safe/Sandbox/Architect）；技能数 13→12；超时/重试/限流 ✅→⚙️。 |
| 2026-02-01 | 工具引擎 | OpenClaw D1 集成：§1 M1 新增 Tool allow/deny policy 四项（ToolPolicy + policyAwareRegistry + group tags + profile config）。 |
| 2026-02-01 | Scheduler | OpenClaw D4 集成：新增 §6 Scheduler 增强章节（JobStore + 状态跟踪 + 冷却/并发 + 动态 Job 工具 + isolated 模式）。 |
| 2026-02-01 | All | Roadmap 重构为 OKR-First，围绕 O2/KR2.* 重新组织章节。 |
