# 2026-03-03 Agent Teams 方案全链路审查（Full Review）

Updated: 2026-03-03

## 0. 结论先行

当前 `agent teams` 方案已经具备较完整的工程闭环：
- 调度入口统一到 `run_tasks`（file/template 两种模式）；
- 执行内核由 `taskfile + BackgroundTaskManager` 承载（依赖阻塞、上下文继承、外部 agent 支持）；
- kernel 已具备 team dispatch 原生能力（`DispatchKindTeam`）；
- 可观测与审计有 sidecar + recorder + typed events；
- 测试覆盖包含 unit/integration/e2e，且对团队场景有专门回归矩阵。

总体评价：`可用且可扩展`，主执行链路已经稳定；当前唯一需要单独拉齐的是文档与脚本叙述一致性，其他项已整理为可直接实施的优化设计方案。

---

## 1. 审查范围与方法

### 1.1 审查范围

1. Kernel team dispatch 路径
- `internal/domain/kernel/types.go`
- `internal/app/agent/kernel/{config.go,llm_planner.go,executor.go,engine.go}`

2. Orchestration 工具面
- `internal/infra/tools/builtin/orchestration/{run_tasks.go,reply_agent.go,args.go}`

3. TaskFile 执行引擎
- `internal/domain/agent/taskfile/*`

4. 后台任务运行时
- `internal/domain/agent/react/{runtime.go,background.go,runtime_background.go,runtime_external_input.go}`

5. 配置加载与 DI 注入
- `internal/shared/config/{types.go,file_config.go,runtime_file_loader.go,load.go,external_agents_autodetect.go}`
- `internal/app/di/{container_builder.go,builder_hooks.go}`

6. 团队运行审计
- `internal/infra/external/teamrun/file_recorder.go`

7. 测试与脚本
- `internal/infra/integration/agent_teams_*`
- `internal/app/agent/kernel/*teams*`
- `scripts/test_agents_teams_e2e.sh`

### 1.2 审查方法

- 从配置入口（YAML）向运行时行为做正向追踪；
- 从失败与边界行为（依赖失败、等待超时、外部输入）做逆向追踪；
- 用测试矩阵反向验证“设计意图是否落地”；
- 对文档与脚本做一致性检查。

---

## 2. 方案总览（当前实现）

### 2.1 核心抽象

1. Kernel 层 team 抽象
- `DispatchKind`: `agent | team`
- `TeamDispatchSpec`: `template/goal/prompts/timeout_seconds/wait`
- cycle summary 中新增 `TeamRoleSummary`

2. Tool 层调度抽象
- `run_tasks`: 文件任务或模板任务的统一入口
- `reply_agent`: 外部 agent 交互回传入口

3. Runtime 层执行抽象
- `BackgroundTaskManager`：统一管理 internal/external task 生命周期
- `taskfile.Executor`：将 YAML/task template 编译并分发给 manager

4. 配置层团队抽象
- `runtime.external_agents.teams`: 模板化定义 `roles/stages`

### 2.2 数据与状态产物

1. Task 状态 sidecar
- 文件：`<plan>.status.yaml`
- 来源：`taskfile.StatusWriter`
- 用途：LLM/tool 可读进度与结果

2. Team run recorder
- 文件：`${session_dir}/_team_runs/*.json`
- 来源：`teamrun.FileRecorder`
- 用途：团队调度审计（模板、阶段、角色、状态）

3. Kernel STATE 反馈
- 来源：cycle summary + dispatch history
- 用途：下一轮 planner 反馈学习（失败类型、角色完成度）

---

## 3. 端到端执行链路

### 3.1 用户/代理触发 `run_tasks(template=...)`

1. 参数路由
- `run_tasks` 校验 `file/template` 互斥
- `template=list` 走模板枚举

2. 模板解析
- 从上下文读取 team definitions（`agent.WithTeamDefinitions`）
- `taskfile.TeamTemplateFromDefinition` + `RenderTaskFile`

3. 执行选择
- `mode=team|swarm|auto`
- `auto` 依据 DAG 和 `inherit_context` 自动决策

4. 执行落地
- 统一转 `BackgroundDispatchRequest`
- manager 负责 internal/external 执行、依赖阻塞、状态推进

5. 结果与审计
- `wait=true` 时同步等待并写最终 sidecar
- 模板模式下写 team run record（best-effort）

### 3.2 `run_tasks(file=...)`

1. 读 YAML
- 当前仅做 `os.ReadFile + yaml.Unmarshal`

2. 验证与编译
- `Validate`（id/prompt/依赖合法性/环检测）
- `ResolveDefaults`（含 external coding 默认值注入）

3. 调度
- team 模式：拓扑顺序派发 + manager 依赖阻塞
- swarm 模式：分层并行 + 自适应并发 + stale retry

4. 状态
- sidecar 初始化为 `pending/blocked`
- polling 每 2s 更新

### 3.3 Kernel team dispatch

1. Planner
- LLM 输出 `kind=team` 决策时，转 `DispatchKindTeam`
- 有显式约束：`TeamDispatchEnabled/MaxTeamsPerCycle/AllowedTeamTemplates`

2. Executor
- `ExecuteTeam` 构造强约束 prompt：要求只调用一次 `run_tasks` 并读取 sidecar
- 自动 `wait=true` + timeout 默认 `DefaultKernelTeamTimeoutSeconds`

3. Engine
- `executeDispatches` 内按 `Kind` 路由到 `ExecuteTeam` 或 `Execute`
- cycle summary 写入 role-level 结果

### 3.4 外部输入回路（interactive external agent）

1. manager 监听 `InputRequests()`
- 记录为 `pendingInput`
- 注入 runtime 消息：指导调用 `reply_agent`

2. `reply_agent`
- 接受 `task_id/request_id/approved/option_id/message`
- 转发给 `ExternalInputResponder.ReplyExternalInput`

3. 反馈闭环
- pendingInput 清理
- 发出 response event

---

## 4. 执行语义细节（TaskFile/Team/Swarm）

### 4.1 mode=auto 决策规则

`AnalyzeMode` 当前规则（按顺序）：
1. 任一 task `inherit_context=true` -> `team`
2. DAG 层深 > 3 -> `team`
3. 否则 -> `swarm`

这个策略是“偏保守协作 + 默认并行”的折中。

### 4.2 Team template 渲染机制

`RenderTaskFile` 关键行为：
- task id 生成：`team-<role>`
- stage 依赖：后续 stage 依赖前一 stage 全部输出
- `debate_mode=true` 时自动追加 challenger task：`team-<role>-debate`
- debate challenger 固定 `inherit_context=true` + `workspace_mode=shared`

### 4.3 coding 默认值注入

针对 external coding agents（`codex/claude_code/kimi`）：
- execute 默认：`verify=true`, `merge_on_success=true`, `retry_max_attempts=3`, `workspace=worktree`
- plan 默认：`verify=false`, `merge_on_success=false`, `retry_max_attempts=1`, `workspace=shared`
- `autonomy_level=controlled` 被提升为 `full`

### 4.4 swarm 调度增强

- 分层并行执行（topological layers）
- 自适应并发（成功率驱动 scale up/down）
- stale/non-terminal 重试（`StaleRetryMax`）
- 对层内 dispatch 清空 `DependsOn`，避免重复阻塞

---

## 5. BackgroundTaskManager 运行时语义

### 5.1 生命周期与状态

- 初始：`pending` 或 `blocked`（有依赖时）
- 运行：`running`
- 终态：`completed | failed | cancelled`

### 5.2 依赖处理

1. 派发时校验
- 依赖存在
- 不允许自依赖
- 全图 DFS 环检测

2. 运行时阻塞
- `awaitDependencies` 每 2s / dep notify 检查
- 上游失败直接级联失败

### 5.3 上下文继承

- `inherit_context=true` 时拼接 `[Collaboration Context]`
- 注入所有依赖任务的状态/结果/错误摘要

### 5.4 上下文传播与隔离

- 后台 task context 复制 run/session/correlation 等 ID
- 通过 `ContextPropagators` 传播 app-layer 值（当前已用于 LLM selection）
- subagent 运行默认过滤 orchestration 工具，防递归委派

### 5.5 外部 agent 进度与输入

- 进度回调节流发事件
- 心跳事件防长任务静默
- interactive 输入进入 pending/notify/reply 闭环

### 5.6 workspace 与 merge

- 非 shared 模式可分配 branch/worktree
- coding task 支持 auto merge（含 resolve 分支冲突子任务）

---

## 6. 配置链路梳理（YAML -> Runtime -> Context）

### 6.1 配置结构

入口：`runtime.external_agents.teams`
- team: `name/description/roles/stages`
- role: `name/agent_type/prompt_template/execution_mode/autonomy_level/workspace_mode/config/inherit_context`
- stage: `name/roles`

### 6.2 加载流程

1. `applyFile` -> `parseRuntimeConfigYAML`
2. `expandRuntimeFileConfigEnv`（支持 `${ENV}` 展开）
3. `applyExternalAgentsFileConfig` -> `convertTeamFileConfigs`
4. `normalizeRuntimeConfig`

### 6.3 DI 注入流程

1. container build 时 `convertTeamConfigs`
2. `WithTeamDefinitions` 注入 coordinator/react runtime context
3. tool registry 注册 `run_tasks/reply_agent`
4. task 执行时 `run_tasks` 从 context 读取模板定义

### 6.4 kernel 对模板的控制面

- `collectTeamTemplateNames` 从配置提取 allowlist
- LLM planner team 决策必须命中 allowlist
- 每轮 team dispatch 数量受 `MaxTeamsPerCycle` 限制

---

## 7. 可观测性与审计

### 7.1 状态 sidecar

- 初始化：任务列表 + `pending/blocked`
- 同步：dispatcher status 拉取后写回
- IO 语义：tmp + rename 原子写

### 7.2 team run recorder

- 记录字段：team/goal/causation_id/stages/roles/dispatch_state
- 文件命名：`<timestamp>-<team>-<runid>.json`
- 默认路径：`${session_dir}/_team_runs`

### 7.3 事件

- background dispatched/completed
- external input request/response
- external progress heartbeat
- kernel cycle summary 聚合

---

## 8. 测试覆盖现状

### 8.1 核心单测

1. `taskfile/*_test.go`
- validate/topo/layers/mode/resolve/swarm/status/template 全覆盖

2. `run_tasks_test.go`
- file/template/list/wait/filter/recorder 场景

3. `kernel` 相关
- planner 解析、team allowlist、maxTeams、autonomy gate、prompt compact
- executor 对 team prompt、重试自治、工具动作校验

### 8.2 集成与 e2e

1. `internal/infra/integration/agent_teams_dispatch_e2e_test.go`
- 深链路、状态轮询、取消、重复 ID、依赖失败、吞吐、cycle detection

2. `internal/infra/integration/agent_teams_lark_inject_e2e_test.go`
- Lark 注入下 happy path / multi-stage / mixed agent / partial failure / status file / recorder

3. `internal/infra/integration/p0_p2_features_e2e_test.go`
- context preamble / debate mode / stale retry

4. `internal/app/agent/kernel/engine_teams_e2e_test.go`
- team dispatch 回环、失败反馈、mixed dispatch、多 team 并发

5. 脚本回归
- `scripts/test_agents_teams_e2e.sh`（当前 claude-only）

整体结论：测试面广，且包含关键边界；但文档与脚本之间存在漂移（见问题项）。

---

## 9. 现存问题（仅保留）

### 9.1 文档与测试脚本漂移

现状：
- `docs/guides/agents-teams-testing.md` 仍以 kimi 叙述为主；
- `scripts/test_agents_teams_e2e.sh` 已切到 claude-only 测试编排。

影响：
- 执行手册与实际回归入口不一致；
- 新同学按文档执行会遇到预期偏差。

处理建议：
- 同步测试指南与 reference 文档中的模板名、超时、前置依赖；
- 在文档中明确当前默认 e2e profile（claude-only）与可选 profile（kimi/codex）切换方式。

---

## 10. 设计方案优化（已收敛）

以下为本轮整理后的优化设计目标，作为后续迭代的标准方案，不再作为“风险清单”处理。

### 10.1 配置语义闭环优化（Team Stage Contract）

目标：
- 统一 team stage 在 file/runtime/domain 三层的数据契约，保证模板语义单向透传、无语义折损。

方案：
- 为 stage 维度建立显式契约清单（如 `name/roles/debate_mode`）；
- 在 `runtime_file_loader` 与 DI `convertTeamConfigs` 形成同构映射；
- 增加“契约快照测试”：给定 YAML，断言 runtime config 与 domain definition 一致。

收益：
- 降低配置语义漂移；
- 避免“YAML 可写但运行时不生效”的隐性行为。

### 10.2 状态语义与审计优化（Status Truth Model）

目标：
- 让 sidecar 状态读取失败与任务成功严格解耦，保证 recorder 语义可审计。

方案：
- team run `dispatch_state` 引入显式枚举：`completed/failed/partial_failure/unknown`；
- 读取 sidecar 失败时记录 `unknown`，同时保存失败原因元数据；
- 保持 `wait=true` 与状态写入逻辑幂等，避免“完成但不可验证”被吞掉。

收益：
- 审计数据更可信；
- 后续统计和失败归因更准确。

### 10.3 任务过滤优化（Dependency Closure）

目标：
- `task_ids` 过滤行为默认可执行、可预期，不制造依赖断裂。

方案：
- 过滤策略默认补全依赖闭包（downstream selection 自动拉齐 upstream）；
- 保留“严格过滤”开关用于高级用户；
- 错误文案直接回显缺失依赖链，减少排查成本。

收益：
- 减少误用导致的 validate 错误；
- 提升模板复用与局部重跑体验。

### 10.4 输入契约优化（TaskFile Entry Contract）

目标：
- 强化 `run_tasks(file=...)` 输入契约，保证工具语义与用户预期一致。

方案：
- 明确 TaskFile 入口校验（YAML 扩展名、最小字段、plan_id 规范）；
- 在工具返回中提供可执行修复建议（示例路径、字段模板）。

收益：
- 降低调用歧义；
- 提高错误可操作性。

### 10.5 归一化逻辑收敛（Normalization Single Source）

目标：
- agent type 归一化规则只维护一份实现。

方案：
- 提炼共享 canonicalization 函数，orchestration/taskfile 共用；
- 将别名映射与 coding-agent 判定放入统一模块，并加回归测试。

收益：
- 降低双份规则漂移风险；
- 后续新增 agent 类型时只改一处。

---

## 11. 落地优先级（建议）

### 11.1 P0 文档一致性（立即）

1. 完成 `agents-teams-testing` 与脚本一致性更新；
2. 在 reference 中补充当前 e2e profile 声明与切换说明。

### 11.2 P1 语义闭环（近期）

1. 配置契约同构化（含 stage 字段透传与快照测试）；
2. 状态 truth model 落地（含 `unknown` 语义）。

### 11.3 P2 体验与维护性（持续）

1. `task_ids` 闭包过滤；
2. 输入契约强化；
3. 归一化逻辑单源收敛。

---

## 12. 最终评价

- 架构完整度：高
- 工程可用性：高
- 可观测性：中高
- 方案可演进性：高（优化路径清晰）
- 当前待拉齐项：文档与脚本一致性

一句话：
当前方案已经能稳定承担多代理协作主路径；除文档与脚本漂移外，其余关键点已整理为可执行的优化设计，后续按优先级落地即可。

---

## Appendix A: 关键文件索引

- Kernel
  - `internal/domain/kernel/types.go`
  - `internal/app/agent/kernel/config.go`
  - `internal/app/agent/kernel/llm_planner.go`
  - `internal/app/agent/kernel/executor.go`
  - `internal/app/agent/kernel/engine.go`
- Orchestration
  - `internal/infra/tools/builtin/orchestration/run_tasks.go`
  - `internal/infra/tools/builtin/orchestration/reply_agent.go`
- TaskFile
  - `internal/domain/agent/taskfile/taskfile.go`
  - `internal/domain/agent/taskfile/validate.go`
  - `internal/domain/agent/taskfile/resolve.go`
  - `internal/domain/agent/taskfile/mode.go`
  - `internal/domain/agent/taskfile/executor.go`
  - `internal/domain/agent/taskfile/swarm.go`
  - `internal/domain/agent/taskfile/status.go`
  - `internal/domain/agent/taskfile/template.go`
  - `internal/domain/agent/taskfile/topo.go`
- Runtime
  - `internal/domain/agent/react/runtime.go`
  - `internal/domain/agent/react/background.go`
  - `internal/domain/agent/react/runtime_background.go`
  - `internal/domain/agent/react/runtime_external_input.go`
- Config/DI
  - `internal/shared/config/types.go`
  - `internal/shared/config/file_config.go`
  - `internal/shared/config/runtime_file_loader.go`
  - `internal/shared/config/load.go`
  - `internal/app/di/container_builder.go`
  - `internal/app/di/builder_hooks.go`
- Recorder
  - `internal/infra/external/teamrun/file_recorder.go`
- Docs/Tests
  - `docs/reference/external-agents-codex-claude-code.md`
  - `docs/guides/orchestration.md`
  - `docs/guides/agents-teams-testing.md`
  - `scripts/test_agents_teams_e2e.sh`
  - `internal/infra/integration/agent_teams_*`
  - `internal/app/agent/kernel/engine_teams_e2e_test.go`
