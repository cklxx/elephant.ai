# Kernel 触发流程与设计方案

## 一、整体定位

Kernel 是 elephant.ai 的**自治调度核心** — 一个 cron 驱动的 agent 编排引擎，以 "founder mindset" 运行：**永不询问、永不等待**，只做四件事：思考/计划 → 派发任务 → 记录状态 → 汇总。

## 二、架构分层

```
┌─────────────────────────────────────────────────────────────┐
│ Delivery: CLI / Server                                       │
│  cmd/alex-server/main.go                                     │
│    ├── kernel-daemon  → RunKernelDaemon() → Engine.Run()     │
│    └── kernel-once    → RunKernelOnce()   → Engine.RunCycle()│
├─────────────────────────────────────────────────────────────┤
│ Bootstrap: kernel.go / kernel_daemon.go / kernel_once.go     │
│  BootstrapFoundation → DI.BuildKernelEngine → KernelStage   │
├─────────────────────────────────────────────────────────────┤
│ Application: internal/app/agent/kernel/                      │
│  ┌─────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │ Engine   │→│ Planner      │  │ CoordinatorExecutor    │ │
│  │ (调度循环)│  │ Static/LLM/  │  │ (prompt包装 + 重试     │ │
│  │          │  │ Hybrid       │  │  + 验证 + 自治保障)     │ │
│  └─────────┘  └──────────────┘  └────────────────────────┘ │
├─────────────────────────────────────────────────────────────┤
│ Domain: internal/domain/kernel/                              │
│  Store, Dispatch, DispatchSpec, CycleResult 等领域模型        │
├─────────────────────────────────────────────────────────────┤
│ Infrastructure:                                              │
│  StateFile (STATE.md, INIT.md, SYSTEM_PROMPT.md)             │
│  FileStore (dispatch 持久化, JSON file / Postgres)            │
│  AgentCoordinator.ExecuteTask → ReAct Loop → LLM + Tools    │
└─────────────────────────────────────────────────────────────┘
```

## 三、触发方式（三种入口）

### 1. Cron 定时触发（主模式）

`engine.go:638-673` — `Engine.Run(ctx)`

```
Schedule: "8,38 * * * *" → 每小时的 :08 和 :38 触发
                ↓
    robfig/cron 解析 → timer 等待下次触发
                ↓
    <-timer.C → runCycleWithLogging()
```

- 定时器到期时会 drain 已有的 triggerCh 信号，避免重复执行

### 2. 外部即时触发

`engine.go:625-633` — `Engine.TriggerNow()`

```
Lark 消息到达 → Gateway /notice handler → engine.TriggerNow()
                                             ↓
                            triggerCh <- struct{}{} (buffered 1, 非阻塞)
                                             ↓
                            Engine.Run 的 select 分支 <-e.triggerCh
                                             ↓
                            runCycleWithLogging()
```

- 幂等：已有 pending trigger 时直接返回 false

### 3. 单次执行（调试/CLI）

`kernel_once.go:18` — `RunKernelOnce()`

```
alex-server kernel-once
    → BootstrapFoundation()
    → engine.RunCycle(ctx)  // 执行一次后退出
    → 打印结果, os.Exit
```

## 四、RunCycle — OODA 循环

`engine.go:93-168` 实现了 **PERCEIVE → ORIENT → DECIDE → ACT → UPDATE** 五阶段：

```
                    RunCycle(ctx)
                         │
    ┌────────────────────┼────────────────────┐
    │                    ↓                    │
    │  1. PERCEIVE: 读 STATE.md              │
    │     - sandbox 降级: 用 seed state       │
    │     - 空文件: 自动 seed                  │
    │                    ↓                    │
    │  2. RECOVER: 回收 stale dispatches      │
    │     - 上次 crash 遗留的 running 状态     │
    │                    ↓                    │
    │  3. ORIENT: 查询各 agent 最近 dispatch   │
    │     - store.ListRecentByAgent()         │
    │                    ↓                    │
    │  4. DECIDE: planner.Plan()             │
    │     - Static / LLM / Hybrid            │
    │     - 返回 []DispatchSpec              │
    │     - 空 → 返回 CycleSuccess           │
    │                    ↓                    │
    │  5. ENQUEUE: store.EnqueueDispatches() │
    │                    ↓                    │
    │  6. ACT: executeDispatches()           │
    │     - 并发执行 (sem = MaxConcurrent=3)  │
    │     - 每个 dispatch → Executor.Execute  │
    │                    ↓                    │
    │  7. UPDATE (defer):                    │
    │     - persistCycleRuntimeState()       │
    │     - persistSystemPromptSnapshot()    │
    │     - 写入 cycle history 到 STATE.md   │
    │     - git commit (pre/post cycle)      │
    └─────────────────────────────────────────┘
```

## 五、Planner 三级决策体系

### StaticPlanner (`planner.go`)

- 最简单：遍历 `[]AgentConfig`，跳过 disabled 和 running 的，替换 `{STATE}` 占位符
- 默认唯一 agent: `founder-operator` (priority=10)

### LLMPlanner (`llm_planner.go`)

- 读取 STATE.md + GOAL.md + 各 agent 近期 dispatch history
- 拼接 planning prompt → 调用小模型 (temperature=0.3, max_tokens=8192)
- 系统提示核心原则: **Action over research** — 优先执行任务，而非继续调研
- 输出: JSON 数组 `[{agent_id, dispatch, priority, prompt, reason}]`
- 支持**动态创建 agent** — 不限于静态配置的 agent，可创建 `website-builder`、`api-applicant` 等

### HybridPlanner (`llm_planner.go:292`)

- LLM → 成功则用 LLM 结果
- LLM 失败/空 → 降级到 Static

## 六、CoordinatorExecutor — 自治执行保障

`executor.go:100-164` 是 kernel 最关键的执行层，确保每次 dispatch 都**产生真实行动**:

```
Execute(ctx, agentID, prompt, meta)
        │
        ├─ 构建 context:
        │   - RunID / SessionID
        │   - UserID / Channel / ChatID
        │   - LLM Selection (via SelectionResolver)
        │   - AutoApprove = true       ← 跳过审批门
        │   - MarkUnattendedContext()   ← 标记无人值守
        │   - Timeout (default 900s)
        │
        ├─ wrapKernelPrompt(prompt):
        │   ┌─────────────────────────────┐
        │   │ kernelFounderDirective      │ ← 永不询问/永不等待
        │   │ + 原始 prompt                │
        │   │ + kernelDefaultSummaryInstr  │ ← 要求 ## Execution Summary
        │   └─────────────────────────────┘
        │
        ├─ 第1次执行: coordinator.ExecuteTask(wrappedPrompt)
        │        ↓
        │   validateKernelDispatchResult(result):
        │     ├─ dispatchStillAwaitsUserConfirmation?
        │     │   - StopReason == "await_user_input"
        │     │   - 答案含 "do you want me"/"请确认"/"请选择" 等
        │     │
        │     └─ containsSuccessfulRealToolExecution?
        │         - 遍历 ToolResults, 过滤 orchestration tools
        │         - orchestration: plan/clarify/todo_read/request_user 等
        │         - 必须有 ≥1 个非 orchestration 工具成功执行
        │
        ├─ 验证失败 → 第2次执行 (retry):
        │   prompt += kernelRetryInstruction + 上次摘要
        │   coordinator.ExecuteTask(retryPrompt)
        │   再次验证 → 仍失败则返回 error
        │
        └─ 返回 ExecutionResult:
            - TaskID: "kernel-{agentID}-{runID}"
            - Summary: 提取 "## Execution Summary" 之后内容 (≤500 chars)
            - Attempts: 1 or 2
            - Autonomy: "actionable" / "awaiting_input" / "no_real_action"
```

## 七、状态管理

### STATE.md — 唯一持久状态文件

```markdown
# Kernel State
## identity
elephant.ai autonomous kernel — founder mindset.

## recent_actions
(agent 自行维护)

<!-- KERNEL_RUNTIME:START -->
## kernel_runtime
- updated_at: 2026-02-24T10:38:00Z
- latest_cycle_id: abc123
- latest_status: success
- latest_dispatched: 1
- latest_succeeded: 1
- ...

### cycle_history
| cycle_id | status | dispatched | succeeded | failed | summary | updated_at |
|----------|--------|------------|-----------|--------|---------|------------|
| abc123   | success| 1          | 1         | 0      | ...     | ...        |
<!-- KERNEL_RUNTIME:END -->
```

核心设计原则:

- **Kernel 只写 `kernel_runtime` bounded block**，其余内容完全由 agent 自行维护（opaque）
- Rolling history: 最多保留 5 条 cycle 记录
- 每个 cycle 前后各做一次 git commit (versioned store)
- Sandbox 降级: 写入受限时 fallback 到 `./artifacts/kernel_state.md`

### INIT.md — 启动快照（只写一次，不可变）

### SYSTEM_PROMPT.md — 每 cycle 刷新，纯 observability

## 八、关键设计决策总结

| 设计点 | 决策 | 原因 |
|--------|------|------|
| 审批门 | `AutoApprove=true` + `MarkUnattendedContext` | Kernel 无人值守，不能阻塞在 approval gate |
| 重试策略 | 最多 2 次，附带上次摘要 | 兼顾可靠性和资源消耗 |
| 工具验证 | 必须有≥1非 orchestration 工具成功 | 确保每个 cycle 产出真实行动 |
| 反"询问用户"检测 | 中英双语关键词匹配 | 防止 LLM 退化为交互模式 |
| Planner 降级 | LLM → Static fallback | 即使 LLM 规划失败也保证执行 |
| State 降级 | filesystem → sandbox fallback → in-memory seed | 三级弹性，任何环境都能运行 |
| Dispatch 租约 | LeaseSeconds=1800s，stale recovery | 防止 crash 后 dispatch 永久卡在 running |
| 并发控制 | `sem = MaxConcurrent(3)` | 有界并行，避免 LLM API 过载 |

## 九、数据流总图

```
     ┌──────────┐   cron / TriggerNow
     │  Engine   │◄──────────────────── Lark Gateway
     └────┬─────┘
          │ RunCycle
          ▼
    ┌──────────┐      STATE.md + GOAL.md + history
    │  Planner  │────────────────────────────────── LLM (small model)
    │ (Hybrid)  │
    └────┬─────┘
         │ []DispatchSpec
         ▼
    ┌──────────┐      Store (enqueue)
    │  Engine   │──────────────────── FileStore / Postgres
    └────┬─────┘
         │ parallel dispatch (max 3)
         ▼
    ┌────────────────┐     kernelFounderDirective
    │ Coordinator    │     + prompt + summaryInstr
    │ Executor       │────────────────────────────── AgentCoordinator
    └────┬───────────┘                                    │
         │ validate                                       ▼
         │  ├─ real tool?                            ReAct Loop
         │  └─ awaiting user?                        (LLM + Tools)
         │
         │ retry if needed (max 2)
         ▼
    ┌──────────┐
    │  Result   │──── CycleResult → STATE.md runtime block
    └────┬─────┘                  → Lark notification
         │                        → git commit
         ▼
    下一次 cron tick...
```

## 十、源码索引

| 文件 | 职责 |
|------|------|
| `internal/app/agent/kernel/engine.go` | Engine 主循环、RunCycle、executeDispatches、状态持久化 |
| `internal/app/agent/kernel/executor.go` | CoordinatorExecutor、founder directive、重试验证 |
| `internal/app/agent/kernel/llm_planner.go` | LLMPlanner、HybridPlanner、planning system prompt |
| `internal/app/agent/kernel/planner.go` | Planner 接口、StaticPlanner |
| `internal/app/agent/kernel/config.go` | RuntimeSettings、AgentConfig、默认值 |
| `internal/app/agent/kernel/state_file.go` | StateFile 原子读写、versioned store 代理 |
| `internal/app/agent/kernel/notifier.go` | CycleNotifier、autonomy signal 汇总 |
| `internal/app/agent/kernel/bootstrap_docs.go` | INIT.md / SYSTEM_PROMPT.md 渲染 |
| `internal/app/agent/kernel/fallback.go` | Sandbox fallback 路径 |
| `internal/app/agent/kernel/sandbox.go` | Sandbox 路径限制检测 |
| `internal/delivery/server/bootstrap/kernel.go` | KernelStage、Lark notifier 绑定 |
| `internal/delivery/server/bootstrap/kernel_daemon.go` | RunKernelDaemon 入口 |
| `internal/delivery/server/bootstrap/kernel_once.go` | RunKernelOnce 入口 |
| `internal/domain/kernel/` | Store、Dispatch、CycleResult 领域模型 |
