# Kernel Prompt & State File Flow

Created: 2026-02-11

---

## Overview

Kernel Engine 是一个 cron 驱动的 OODA 循环（Observe-Orient-Decide-Act），每 `*/10` 分钟执行一次。核心数据流有两条：

1. **State 流** — `STATE.md` 文件，在每个 cycle 开头被读取，注入到 agent prompt 中
2. **Prompt 流** — 从 YAML 配置的模板 → `{STATE}` 占位符替换 → Postgres dispatch queue → AgentCoordinator 执行

```
                    ┌─────────────────────────────────────────────┐
                    │              Kernel Engine                    │
                    │                                               │
  ┌──────────┐     │  ┌───────┐   ┌─────────┐   ┌──────────────┐ │
  │ STATE.md │────>│  │ Read  │──>│ Planner │──>│ Dispatch Queue│ │
  │ (file)   │     │  └───────┘   │         │   │  (Postgres)   │ │
  └──────────┘     │              │{STATE}  │   └──────┬───────┘ │
                    │              │replace  │          │         │
  ┌──────────┐     │              └─────────┘          v         │
  │ Agent    │     │                            ┌─────────────┐  │
  │ Configs  │─────────────────────────────────>│  Executor   │  │
  │ (YAML)   │     │                            │ (Coordinator│  │
  └──────────┘     │                            │  .Execute)  │  │
                    │                            └─────────────┘  │
                    └─────────────────────────────────────────────┘
```

---

## 1. State File（STATE.md）

### 1.1 文件位置

```
~/.alex/kernel/{kernel_id}/STATE.md
```

路径由内建默认值 `DefaultStateRootDir="~/.alex/kernel"` 解析，实际为 `~/.alex/kernel/default/STATE.md`（不再通过 runtime YAML 暴露 `state_dir` 配置）。

同目录下还会在 kernel 构建阶段刷新写入：

- `INIT.md`：kernel 运行配置快照（schedule、路由、agent prompt 模板、seed state）
- `SYSTEM_PROMPT.md`：当前 `AgentCoordinator.GetSystemPrompt()` 的快照

### 1.2 生命周期

```
                       首次 cycle
                       STATE.md 不存在？
                            │
                    ┌───────┴───────┐
                    │ Yes           │ No
                    v               v
              Seed(DefaultSeedStateContent)   Read() → content
              Write seed to     返回文件内容
              STATE.md
                    │
                    v
              content = DefaultSeedStateContent
```

| 阶段 | 操作 | 代码位置 |
|------|------|---------|
| **首次启动** | `StateFile.Seed(DefaultSeedStateContent)` — 仅当文件不存在时写入 | `engine.go` + `container_builder.go` |
| **kernel 构建时** | `StateFile.WriteInit(...)` / `StateFile.WriteSystemPrompt(...)` | `container_builder.go` |
| **每个 cycle 开头** | `StateFile.Read()` — 读取当前内容 | `engine.go:70` |
| **cycle 执行中/结束后** | Engine upsert `kernel_runtime` 区块（cycle id/status/error 等） | `engine.go` |

### 1.3 Seed State（种子状态）

来自内建默认常量 `internal/app/agent/kernel/config.go::DefaultSeedStateContent`：

```md
# Kernel State
## identity
elephant.ai autonomous kernel
## recent_actions
(none yet)
```

**关键设计：Seed 只写一次。** `StateFile.Seed()` 内部先 `os.Stat` 检查文件是否存在，已存在则直接返回 nil，不覆盖。这意味着：

- 第一次 cycle：写入 seed → 读回 seed 内容
- 后续 cycle：跳过 seed → 直接读取文件

### 1.4 State 更新机制（当前）

Engine 在每次 cycle 结束后会把执行结果 upsert 到 `STATE.md` 的 runtime 区块：

```md
<!-- KERNEL_RUNTIME:START -->
## kernel_runtime
- updated_at: ...
- cycle_id: ...
- status: success|partial_success|failed|error
- dispatched: ...
- succeeded: ...
- failed: ...
- failed_agents: ...
- duration_ms: ...
- error: ...
<!-- KERNEL_RUNTIME:END -->
```

该区块用于可观测性，固定为单块替换（不会无限追加）。除该区块外，其余内容仍视为 agent 维护的业务状态。

---

## 2. Prompt 流转

### 2.1 总览：从配置到执行

```
   YAML config                StaticPlanner              Postgres               Executor
   ┌──────────┐              ┌──────────────┐          ┌──────────┐          ┌────────────┐
   │ agents:  │              │              │          │ dispatch │          │            │
   │  - prompt│──AgentConfig─>│  {STATE} →   │─DispatchSpec─>│  queue   │─Dispatch──>│ Coordinator│
   │    "{STATE}             │  实际内容替换 │          │ (pending)│          │ .ExecuteTask│
   │    Do X."│              │              │          │          │          │            │
   └──────────┘              └──────────────┘          └──────────┘          └────────────┘
```

### 2.2 阶段 1：Agent 配置（静态模板）

```yaml
kernel:
  agents:
    - agent_id: "daily-report"
      prompt: |
        当前系统状态:
        {STATE}

        请根据上述状态生成今日工作简报。
      priority: 10
      enabled: true
```

`{STATE}` 是唯一支持的占位符。prompt 字段支持 `${ENV_VAR}` 展开（在 YAML 加载阶段由 `expandProactiveFileConfigEnv` 处理，早于 `{STATE}` 替换）。

**两种展开的时机不同：**

| 占位符 | 展开时机 | 展开方 |
|--------|---------|--------|
| `${ENV_VAR}` | 配置加载时（启动阶段） | `expandProactiveFileConfigEnv()` |
| `{STATE}` | 每次 cycle 的 Plan 阶段 | `StaticPlanner.Plan()` |

### 2.3 阶段 2：Planner（{STATE} 替换）

`StaticPlanner.Plan()` (`planner.go:32-51`) 做三件事：

1. **过滤** — 跳过 `Enabled=false` 的 agent
2. **去重** — 跳过上一轮 dispatch 仍在 `running` 状态的 agent（防止重复执行）
3. **替换** — `strings.ReplaceAll(a.Prompt, "{STATE}", stateContent)`

```go
// planner.go:42
prompt := strings.ReplaceAll(a.Prompt, "{STATE}", stateContent)
```

替换后的完整 prompt 被封装为 `DispatchSpec`：

```go
DispatchSpec{
    AgentID:  "daily-report",
    Prompt:   "当前系统状态:\n# Kernel State\n## identity\n...\n\n请根据上述状态生成今日工作简报。",
    Priority: 10,
    Metadata: map[string]string{...},
}
```

### 2.4 阶段 3：Dispatch Queue（Postgres 持久化）

`Engine.RunCycle()` 将 specs 写入 Postgres：

```
engine.go:103  →  store.EnqueueDispatches(ctx, kernelID, cycleID, specs)
```

写入的行：

| 列 | 值 |
|----|----|
| `dispatch_id` | 唯一 ID（`id.NewRunID()`） |
| `kernel_id` | `"default"` |
| `cycle_id` | 当次 cycle 的唯一 ID |
| `agent_id` | `"daily-report"` |
| `prompt` | **完整的、已替换 {STATE} 的 prompt** |
| `priority` | `10` |
| `status` | `"pending"` |
| `metadata` | agent 配置中的 metadata（JSONB） |

**Dispatch 状态机：**

```
  pending ──────> running ──────> done
                    │
                    └─────────> failed
```

状态转换由 Engine 调用 Store 方法完成：

```
pending → running  :  MarkDispatchRunning()   (engine.go:140)
running → done     :  MarkDispatchDone()      (engine.go:170)
running → failed   :  MarkDispatchFailed()    (engine.go:164)
```

### 2.5 阶段 4：Executor（AgentCoordinator 执行）

`CoordinatorExecutor.Execute()` (`executor.go:32-51`)：

```go
sessionID := fmt.Sprintf("kernel-%s-%s", agentID, runID)
// → "kernel-daily-report-abc123"

coordinator.ExecuteTask(execCtx, prompt, sessionID, nil)
```

此时 prompt 进入标准的 ReAct 循环（Think → Act → Observe），与用户在 Lark/Web 发送的消息走完全相同的执行路径。Agent 拥有全套 tools（搜索、文件读写、shell、浏览器等）。

**Metadata 注入：** Engine 在执行前向 dispatch metadata 追加两个字段：

```go
// engine.go:149-154
meta["user_id"] = e.config.UserID    // "cklxx"
meta["channel"] = e.config.Channel   // "lark"
```

这些 metadata 用于 coordinator 确定输出目标（如 Lark 消息发送的 user_id）。

---

## 3. 完整 Cycle 时序

```
t=0    cron 触发 RunCycle
       │
t=1    ┌─ PERCEIVE ─────────────────────────────────────────┐
       │  StateFile.Read() → stateContent                    │
       │  (首次: Seed → Write default seed → stateContent)   │
       └─────────────────────────────────────────────────────┘
       │
t=2    ┌─ ORIENT ───────────────────────────────────────────┐
       │  store.ListRecentByAgent(kernelID)                  │
       │  → map[agentID]Dispatch (最近一次 dispatch 状态)     │
       └─────────────────────────────────────────────────────┘
       │
t=3    ┌─ DECIDE ───────────────────────────────────────────┐
       │  planner.Plan(stateContent, recentByAgent)          │
       │  对每个 enabled agent:                               │
       │    - 跳过 status=running 的 agent                   │
       │    - strings.ReplaceAll(prompt, "{STATE}", state)   │
       │  → []DispatchSpec                                    │
       └─────────────────────────────────────────────────────┘
       │
t=4    ┌─ ACT (enqueue) ────────────────────────────────────┐
       │  store.EnqueueDispatches(kernelID, cycleID, specs)  │
       │  → []Dispatch (status=pending, 写入 Postgres)       │
       └─────────────────────────────────────────────────────┘
       │
t=5    ┌─ ACT (execute, 并发, 受 MaxConcurrent 限制) ───────┐
       │  for each dispatch (goroutine):                      │
       │    MarkDispatchRunning(dispatchID)                   │
       │    meta += {user_id, channel}                        │
       │    executor.Execute(agentID, prompt, meta)           │
       │      → coordinator.ExecuteTask(prompt, sessionID)    │
       │      → ReAct loop (LLM + tools)                     │
       │    if ok:  MarkDispatchDone(dispatchID, sessionID)   │
       │    if err: MarkDispatchFailed(dispatchID, err)       │
       └─────────────────────────────────────────────────────┘
       │
t=6    汇总 CycleResult{Dispatched, Succeeded, Failed}
       写日志到 alex-kernel.log
```

---

## 4. 数据存储分布

| 数据 | 存储位置 | 持久性 |
|------|---------|--------|
| STATE.md | 本地文件 `~/.alex/kernel/{id}/STATE.md` | 持久，跨 cycle 保留 |
| Agent 模板 prompt | YAML 配置 → 内存 `AgentConfig.Prompt` | 随进程生命周期 |
| 展开后的 prompt | Postgres `kernel_dispatch_tasks.prompt` | 持久，可审计 |
| Dispatch 状态 | Postgres `kernel_dispatch_tasks.status` | 持久 |
| 执行结果 | Session store（Postgres/文件） | 持久 |

---

## 5. V1 局限与演进方向

### 5.1 STATE.md 的可观测性区块

当前 Engine 会自动回写 `kernel_runtime` 区块，确保即使任务失败（例如 LLM 限流）也能在 `STATE.md` 中看到“本轮是否执行、失败原因是什么”。

仍可进一步演进为更强的 state reducer（规则/LLM），将业务层摘要（而非仅执行指标）融合回主状态。

### 5.2 StaticPlanner 无条件分发

当前 planner 只做简单过滤（enabled + 非 running），不会根据 STATE 内容做智能决策。

**演进路径：** 引入 `LLMPlanner`，读取 STATE 后让 LLM 决定本轮应该执行哪些 agent、优先级如何调整。

### 5.3 Prompt 模板只有 {STATE} 一个占位符

未来可扩展为：
- `{RECENT_RESULTS}` — 上一轮各 agent 的执行摘要
- `{TIME}` — 当前时间
- `{MEMORY}` — 从 memory engine 检索的相关上下文
