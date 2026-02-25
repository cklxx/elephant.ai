# File-Based Orchestration Migration Plan

**Created:** 2026-02-25
**Status:** draft
**Owner:** ckl

## Problem

当前有 **10 个编排工具**（`bg_dispatch`, `bg_plan`, `bg_status`, `bg_graph`, `bg_collect`, `subagent`, `explore`, `team_dispatch`, `ext_merge`, `ext_reply`），全部注入 system prompt。LLM 需要在 10 个 tool schema 之间做路由决策，认知负担过高，且工具间职责重叠（`bg_dispatch` vs `bg_plan` vs `team_dispatch` 都在"分配任务"）。

## Goal

**10 个编排工具 → 1 个执行工具 + 已有的 `read_file` / `write_file`**。

- 规划（plan）= agent 写 YAML 文件（`write_file`，已有）
- 查状态/收结果 = agent 读 YAML 文件（`read_file`，已有）
- 执行 = 单一 `run_tasks` 工具，读 YAML → 拓扑排序 → 调度执行 → 回写状态
- IPC 回复 = `ext_reply` 保留（无法 file-based 替代进程间 stdin 通信）

净减少 **8 个 tool schema**（bg_dispatch, bg_plan, bg_status, bg_graph, bg_collect, subagent, explore, team_dispatch, ext_merge 全部删除）。

## Design

### TaskFile YAML Spec

位置：`.elephant/tasks/{plan-id}.yaml`（已 gitignore）

```yaml
# .elephant/tasks/plan-auth-impl.yaml
id: plan-auth-impl
goal: "实现用户认证模块"
mode: async                     # async (default) | sync
created_at: 2026-02-25T10:00:00Z

defaults:
  agent_type: codex             # internal | codex | claude_code | kimi
  execution_mode: execute       # execute | plan
  autonomy_level: full          # controlled | semi | full
  workspace_mode: worktree      # shared | branch | worktree
  inherit_context: true
  config: {}

tasks:
  - id: design
    description: "设计认证接口"
    prompt: "设计 JWT 认证接口，输出到 docs/design/auth.md..."
    agent_type: internal        # override defaults
    execution_mode: plan
    workspace_mode: shared

  - id: impl-auth
    description: "实现认证逻辑"
    prompt: "按照 design 任务的输出，实现..."
    depends_on: [design]
    inherit_context: true       # 注入 design 的结果到 prompt
    file_scope: [internal/auth/]
    merge_on_success: true

  - id: impl-tests
    description: "写认证测试"
    prompt: "..."
    depends_on: [design]
    file_scope: [internal/auth/auth_test.go]

  - id: review
    description: "Review 所有变更"
    prompt: "..."
    depends_on: [impl-auth, impl-tests]
    agent_type: internal

# === 以下由 run_tasks 执行器回写，agent 不写 ===
status: running                 # pending | running | completed | partial | failed
started_at: 2026-02-25T10:01:00Z
completed_at: null

results:
  design:
    status: completed
    started_at: 2026-02-25T10:01:00Z
    completed_at: 2026-02-25T10:03:30Z
    answer: "接口设计已输出到 docs/design/auth.md ..."
    tokens_used: 4200
    iterations: 5
  impl-auth:
    status: running
    started_at: 2026-02-25T10:03:31Z
    progress:
      iteration: 3
      current_tool: shell_exec
      files_touched: [internal/auth/handler.go, internal/auth/jwt.go]
  impl-tests:
    status: blocked             # depends_on design → design completed → now running
  review:
    status: blocked
```

### `run_tasks` 工具

**唯一新增的编排工具**。

```
参数:
  file:    string (required)  — TaskFile YAML 路径
  action:  string (default: "run")
           "run"      — 执行全部 pending/blocked 任务
           "run_one"  — 只执行指定 task_id
           "cancel"   — 取消指定 task_id
           "resume"   — 从断点恢复（读 YAML 中已有 results，跳过 completed）
  task_id: string (optional)  — 用于 run_one / cancel
  wait:    bool (default: false) — true 时阻塞直到所有任务完成（sync 语义）
  timeout: int (optional)     — wait=true 时的超时秒数，默认 300
```

**内部执行流程：**

1. 读 YAML → 解析 TaskFile
2. 验证：依赖存在性、无环（Kahn 拓扑排序，复用现有 `topologicalOrder`）
3. 合并 defaults 到每个 task
4. 跳过 `results` 中已 `completed` 的任务（resume 语义）
5. 按拓扑序启动任务：
   - 无依赖 → 立即启动
   - 有依赖 → 等待依赖完成后启动
   - 并发度由 `MaxConcurrent` 信号量控制（复用现有逻辑）
6. 每个任务完成/进度更新 → 回写 YAML `results` 段
7. `wait=true` 时阻塞直到全部完成，返回汇总
8. `wait=false` 时立即返回，agent 后续用 `read_file` 轮询

### Sync 模式（替代 subagent/explore）

当 `mode: sync` 或 `run_tasks(wait=true)` 时：

```yaml
id: quick-research
mode: sync
goal: "调研认证方案"
tasks:
  - id: research-jwt
    prompt: "调研 JWT 最佳实践..."
    agent_type: internal
  - id: research-oauth
    prompt: "调研 OAuth2 方案..."
    agent_type: internal
# 无 depends_on → 并行执行，wait=true → 全部完成后返回
```

等价于当前的 `subagent(tasks=[...], mode="parallel")`，但 plan 可审计、可修改。

### Team Templates（替代 team_dispatch）

现有 `TeamDefinition` 配置转换为 YAML 模板文件：

```yaml
# .elephant/teams/code-review-team.yaml
name: code-review-team
description: "代码审查团队"
defaults:
  workspace_mode: worktree
  inherit_context: true

tasks:
  - id: analyzer
    description: "分析代码变更"
    prompt: "{GOAL}\n\n你的角色是 analyzer..."
    agent_type: internal
    execution_mode: plan

  - id: reviewer
    description: "执行代码审查"
    prompt: "{GOAL}\n\n参考 analyzer 的分析..."
    agent_type: codex
    depends_on: [analyzer]

  - id: reporter
    description: "生成审查报告"
    prompt: "{GOAL}\n\n汇总审查结果..."
    agent_type: internal
    depends_on: [reviewer]
```

Agent 使用流程：
1. `read_file .elephant/teams/code-review-team.yaml`（了解模板）
2. 复制为 `.elephant/tasks/review-{ksuid}.yaml`，替换 `{GOAL}` 占位符
3. `run_tasks(file=".elephant/tasks/review-{ksuid}.yaml")`

### ext_merge 替代

不需要独立工具。在 TaskFile 中声明 `merge_on_success: true`，执行器在任务成功后自动 merge（复用现有 `tryAutoMerge` 逻辑）。

### ext_reply 保留

`ext_reply` 是进程间 IPC（写 stdin 到运行中的子进程），无法用文件替代。**保留为第二个编排工具**，但可以简化重命名为 `reply_agent`。

### 进度更新策略

问题：频繁写 YAML 可能有竞争。

方案：**双文件**
- `.elephant/tasks/{plan-id}.yaml` — 主文件，任务定义 + 最终结果（低频写入，任务完成时写）
- `.elephant/tasks/{plan-id}.status` — 进度 sidecar（高频写入，每 5s 或每个 iteration）

```
# .elephant/tasks/plan-auth-impl.status (纯文本，简洁)
design        completed  2m30s   tokens=4200
impl-auth     running    1m05s   iter=3 tool=shell_exec files=[handler.go,jwt.go]
impl-tests    running    0m42s   iter=2 tool=read_file
review        blocked    -       waiting=[impl-auth,impl-tests]
```

Agent 查实时进度：`read_file .elephant/tasks/plan-auth-impl.status`
Agent 查完整结果：`read_file .elephant/tasks/plan-auth-impl.yaml`

### 完成通知（Async 模式）

复用现有的 `injectBackgroundNotifications` 机制。当任务完成时：
1. 回写 YAML `results` 段
2. 通过 `signalCompletion` 通知 ReactEngine
3. ReactEngine 注入 system message：

```
[Tasks Completed] file=".elephant/tasks/plan-auth-impl.yaml" tasks=[design]
Use read_file to check results.
```

注意：通知消息**不再指向 bg_collect**，改为指向 `read_file`。

## Migration Phases

### Phase 0: TaskFile 核心（无外部影响）

**目标**：实现 TaskFile 读写 + 执行引擎，不改动现有工具。

新增文件：
```
internal/infra/orchestration/
├── taskfile.go          # TaskFile YAML 解析/序列化/验证
├── taskfile_test.go
├── executor.go          # TaskFile 执行引擎（拓扑排序、并发调度、状态回写）
├── executor_test.go
├── status_writer.go     # .status sidecar 高频写入
└── status_writer_test.go
```

核心逻辑：
- `TaskFile` struct + `ParseTaskFile()` / `WriteTaskFile()`
- `TaskFileExecutor` struct：
  - 接收 `TaskExecutor`（coordinator.ExecuteTask）和 `ExternalAgentExecutor`（bridge 执行）
  - 复用 `BackgroundTaskManager` 中的：`validateDependencies`（环检测）、`awaitDependencies`（依赖等待）、`buildContextEnrichedPrompt`（上下文继承）
  - 不复制代码——提取公共逻辑到 `internal/domain/agent/orchestration/` 包

依赖提取（从 `background.go` 中抽出）：
```
internal/domain/agent/orchestration/
├── deps.go              # ValidateDependencies(), TopologicalSort() — 从 background.go + bg_plan.go 提取
├── context_enrichment.go # BuildContextEnrichedPrompt() — 从 background.go 提取
└── workspace.go         # WorkspaceManager 接口不变，直接复用
```

**验证**：单元测试覆盖 YAML 解析、环检测、拓扑排序、状态回写。

### Phase 1: `run_tasks` 工具 + 注册

**目标**：新增 `run_tasks` 工具，与现有工具**并存**。

新增文件：
```
internal/infra/tools/builtin/orchestration/run_tasks.go
internal/infra/tools/builtin/orchestration/run_tasks_test.go
```

修改文件：
```
internal/app/toolregistry/registry.go   # RegisterSubAgent() 中注册 run_tasks
```

工具实现：
- 读 YAML → `TaskFileExecutor.Execute()` → 回写结果
- `wait=true` 时阻塞返回，`wait=false` 时立即返回
- 完成通知接入现有 `signalCompletion` 通道

**验证**：集成测试——写 YAML、调用 run_tasks、读回结果。

### Phase 2: 通知链路适配

**目标**：`injectBackgroundNotifications` 兼容新路径。

修改文件：
```
internal/domain/agent/react/runtime_background.go  # 通知消息改为指向 read_file
```

当检测到完成的任务来自 TaskFile 执行器时：
```
[Tasks Completed] file=".elephant/tasks/{plan-id}.yaml" tasks=[{task_ids}]
Use read_file to check results.
```

**验证**：端到端测试——async 模式下 agent 收到正确的通知消息。

### Phase 3: Team Templates

**目标**：将现有 `TeamDefinition` 配置导出为 YAML 模板。

新增：
```
.elephant/teams/                    # 模板目录
internal/infra/orchestration/
└── team_template.go               # 模板渲染：读模板 + 替换占位符 + 写 TaskFile
```

`run_tasks` 增加 `template` 参数：
```
run_tasks(file=".elephant/teams/code-review-team.yaml", template_vars={"GOAL": "review auth module"})
```

内部：读模板 → 替换变量 → 生成 TaskFile → 执行。

**验证**：现有 team 配置转换为 YAML 模板，执行结果一致。

### Phase 4: 删除旧工具

**目标**：一次性删除 8 个旧工具。

删除文件：
```
internal/infra/tools/builtin/orchestration/
├── bg_dispatch.go      (486 行)
├── bg_collect.go       (139 行)
├── bg_status.go        (255 行)
├── bg_graph.go         (100 行)
├── bg_plan.go          (477 行)
├── subagent.go         (327 行)
├── subagent_lifecycle.go (363 行)
├── subagent_prompt.go  (110 行)
├── explore.go          (217 行)
├── team_dispatch.go    (487 行)
└── ext_merge.go        (131 行)
```

修改文件：
```
internal/app/toolregistry/registry.go
  # RegisterSubAgent() 只注册 run_tasks + ext_reply（改名 reply_agent）
  # WithoutSubagent() 只排除 run_tasks + reply_agent

internal/domain/agent/react/runtime_background.go
  # 删除 bg_collect 引导消息
```

保留（重构）：
```
internal/infra/tools/builtin/orchestration/
├── run_tasks.go          # 新工具
├── reply_agent.go        # ext_reply 改名
├── args.go               # 共享参数解析（保留有用的部分）
```

**净删除**：~3,092 行

**验证**：全量 lint + test + 确认 tool registry 只暴露 2 个编排工具。

### Phase 5: BackgroundTaskManager 瘦身

**目标**：`background.go`（1,226 行）中被 TaskFile 执行器替代的逻辑可以精简。

`BackgroundTaskManager` 的职责缩小为：
- 持有运行中任务的 goroutine 生命周期
- 完成通知（三层防御保留）
- 取消任务

以下逻辑移至 `internal/domain/agent/orchestration/`：
- `validateDependencies` → `orchestration.ValidateDeps`
- `topologicalOrder` → `orchestration.TopoSort`（已在 Phase 0 提取）
- `buildContextEnrichedPrompt` → `orchestration.EnrichPrompt`
- `awaitDependencies` → `orchestration.AwaitDeps`

`BackgroundTaskManager` 中删除重复代码，改为调用 orchestration 包。

**验证**：现有 background_test.go 全部通过。

### Phase 6: 提示词 & 文档更新

修改：
```
CLAUDE.md                          # Codex worker protocol 中增加 TaskFile 用法
internal/app/agent/kernel/         # kernelFounderDirective 如有引用旧工具名则更新
docs/guides/                       # 新增 orchestration-guide.md
```

## Risk & Mitigation

| 风险 | 缓解 |
|------|------|
| YAML 回写竞争（多 goroutine 同时写） | 主文件用 `flock` 或 `sync.Mutex` 保护；高频进度走 `.status` sidecar |
| Agent 不会自动用新工具 | Phase 1-3 新旧并存，可逐步引导；Phase 4 一刀切 |
| 文件系统延迟影响 async 通知 | 通知仍走内存 channel（`signalCompletion`），文件只是持久化层 |
| Sync 模式长时间阻塞 tool call | 强制 timeout 上限（默认 300s），超时返回已完成的部分结果 |
| ext_reply 仍需独立工具 | 可接受——IPC 是硬约束，且使用频率低（仅 interactive claude_code） |

## Success Metrics

- Tool schema 数量：10 → 2（`run_tasks` + `reply_agent`）
- System prompt token 减少：约 3,000-4,000 tokens（10 个 schema → 2 个）
- 代码行数净减少：~2,500+ 行
- 所有编排操作可通过 `read_file` 审计 YAML 文件
- 崩溃恢复：`run_tasks(action="resume")` 读 YAML 断点续跑
