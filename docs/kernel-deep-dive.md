# Kernel 子系统完整深度梳理

Created: 2026-02-15

---

## 目录

1. [设计理念：Always-On AI Employee](#一设计理念always-on-ai-employee)
2. [架构分层](#二架构分层)
3. [核心类型与接口](#三核心类型与接口)
4. [Engine.RunCycle() 完整代码走读](#四engineruncycle-完整代码走读)
5. [Executor 验证与重试的实现细节](#五executor-验证与重试的实现细节)
6. [FileStore 持久化的并发设计](#六filestore-持久化的并发设计)
7. [Prompt 工程深度分析](#七prompt-工程深度分析)
8. [LLMPlanner 设计方案讨论](#八llmplanner-设计方案讨论)
9. [Kernel 与 Lark 集成的完整链路](#九kernel-与-lark-集成的完整链路)
10. [演进脉络](#十演进脉络)
11. [总结](#十一总结)

---

## 一、设计理念：Always-On AI Employee

### 核心问题

传统 Agent 是 **请求-响应** 模式 — 用户提问，AI 回答，结束。但 elephant.ai 想要的是一个 **永不下班的 AI 员工** — 它定期醒来、审视状态、决定做什么、执行、更新状态、休眠，循环往复。

### 解题思路：OODA 循环

借鉴军事决策理论 OODA（Observe-Orient-Decide-Act），kernel 被设计为一个 **cron 驱动的自主循环**：

```
┌─────────────┐
│   Observe   │ ← 读取 STATE.md（agent 的工作记忆）
├─────────────┤
│   Orient    │ ← 恢复中断的 dispatch、查看各 agent 上轮状态
├─────────────┤
│   Decide    │ ← Planner 决定本轮派发哪些 agent
├─────────────┤
│    Act      │ ← Executor 并发执行 agent 任务
├─────────────┤
│   Update    │ ← 更新 STATE.md、通知 Lark
└─────────────┘
        │
        ↓ (等待 cron 下一次触发)
```

### 三个关键设计选择

| 选择 | 决策 | 理由 |
|------|------|------|
| **状态存储** | 文档驱动（STATE.md） | Agent 自己决定状态结构，系统只做传递，不耦合 schema |
| **系统职责** | 仅调度 + 队列 + 进程管理 | Kernel 不理解任务内容，只保证"agent 被调用了" |
| **行为范式** | Founder Mindset | 永不询问、永不等待、永不阻塞 — 一切自主决策 |

---

## 二、架构分层

```
internal/
├── domain/kernel/           # 领域层：纯业务类型，零外部依赖
│   ├── types.go             # Dispatch, DispatchSpec, CycleResult
│   └── store.go             # Store 接口（dispatch 队列的端口）
│
├── app/agent/kernel/        # 应用层：编排逻辑
│   ├── config.go            # KernelConfig, AgentConfig, 种子状态
│   ├── engine.go            # Engine — RunCycle() + Run() 主循环 (681行)
│   ├── planner.go           # Planner 接口 + StaticPlanner
│   ├── executor.go          # Executor 接口 + CoordinatorExecutor (347行)
│   ├── state_file.go        # STATE.md 原子读写 + 版本化存储
│   ├── notifier.go          # Lark 周期通知格式化
│   ├── bootstrap_docs.go    # INIT.md / SYSTEM_PROMPT.md 渲染
│   ├── fallback.go          # 沙箱受限时的降级持久化
│   └── sandbox.go           # 沙箱路径限制检测
│
├── infra/kernel/            # 基础设施层：持久化适配器
│   └── file_store.go        # FileStore — JSON dispatch 队列 (328行)
│
└── delivery/server/bootstrap/
    ├── kernel.go            # KernelStage — 生命周期管理
    └── kernel_once.go       # kernel-once 单次执行命令
```

**总代码量**：~4060 行（含测试），其中测试约 1823 行。

---

## 三、核心类型与接口

### 3.1 领域类型（`domain/kernel/types.go`）

```
DispatchSpec ──(Planner产出)──> Dispatch ──(Store持久化)──> CycleResult
     │                              │                           │
   AgentID                     DispatchID                    CycleID
   Prompt                      Status (状态机)               Dispatched/Succeeded/Failed
   Priority                    TaskID                        AgentSummary[]
   Metadata                    LeaseOwner/Until              Duration
```

**Dispatch 状态机**：

```
pending ──→ running ──→ done
                │
                └──→ failed
```

### 3.2 三大接口

| 接口 | 职责 | 实现 |
|------|------|------|
| **Store** | Dispatch 队列 CRUD | `infra/kernel/FileStore`（JSON 文件） |
| **Planner** | 决定本轮派发哪些 agent | `StaticPlanner`（静态过滤 + {STATE} 替换） |
| **Executor** | 执行单个 agent 任务 | `CoordinatorExecutor`（包装 AgentCoordinator） |

---

## 四、Engine.RunCycle() 完整代码走读

### 4.1 入口与生命周期

`engine.go:92-167` — `RunCycle` 是整个 kernel 的心跳，每次 cron 触发执行一次。

**关键设计**：用 `defer` 确保 **无论成功还是失败** 都写入 runtime state：

```go
// engine.go:95-98
defer func() {
    e.persistCycleRuntimeState(result, err)   // 更新 STATE.md runtime 区块
    e.persistSystemPromptSnapshot()           // 刷新 SYSTEM_PROMPT.md
}()
```

这意味着即使 cycle 中途 panic（被 recover 后），runtime 区块也能记录 `latest_status: error`。

### 4.2 Phase 1: PERCEIVE — 读取状态

```go
// engine.go:100-123
stateContent, err := e.stateFile.Read()
```

**三重降级策略**：

```
Read STATE.md
  │
  ├─ 成功 → stateContent = 文件内容
  │
  ├─ 沙箱权限错误 → markStateWritesRestricted(err)
  │     └─ stateContent = e.config.SeedState (内存种子)
  │
  └─ 内容为空
        ├─ 已标记受限 → stateContent = SeedState
        │
        └─ 未受限 → Seed(SeedState)
              ├─ Seed 成功 → stateContent = SeedState
              ├─ Seed 沙箱错误 → markStateWritesRestricted, 用 SeedState
              └─ Seed 其他错误 → 返回 error，cycle 中止
```

`markStateWritesRestricted` 使用 `atomic.Bool` + `CompareAndSwap`，只设置一次，之后所有写操作都跳转到 fallback 路径。

### 4.3 Phase 2: ORIENT — 恢复与查询

```go
// engine.go:125-140
if recoverer, ok := e.store.(staleDispatchRecoverer); ok {
    recovered, _ := recoverer.RecoverStaleRunning(ctx, e.config.KernelID)
}
recentByAgent, _ := e.store.ListRecentByAgent(ctx, e.config.KernelID)
```

`staleDispatchRecoverer` 是一个**可选接口**（类型断言），FileStore 当前没有实现它。这是一个向前兼容的扩展点 — 如果未来 FileStore 添加了 RecoverStaleRunning，这里自动生效。

`ListRecentByAgent` 的错误被**降级处理**（warn + empty map），不阻断 cycle。

### 4.4 Phase 3: DECIDE — 计划派发

```go
// engine.go:142-154
specs, err := e.planner.Plan(ctx, stateContent, recentByAgent)
if len(specs) == 0 {
    return &CycleResult{Status: CycleSuccess, ...}
}
```

空 plan 不是错误 — 返回 `CycleSuccess` 和零 dispatch。如果所有 agent 都在 running，本轮静默跳过。

### 4.5 Phase 4: ACT (Enqueue)

```go
// engine.go:156-160
dispatches, err := e.store.EnqueueDispatches(ctx, e.config.KernelID, cycleID, specs)
```

入队失败是**硬错误** — 直接返回 error，cycle 中止。如果连队列都写不进去，执行是无意义的。

### 4.6 Phase 5: ACT (Execute) — 并发调度核心

`engine.go:504-598` — `executeDispatches` 是 kernel 最复杂的方法。

**并发模型**：信号量（buffered channel）限制最大并发数：

```go
sem := make(chan struct{}, maxConcurrent)

for _, d := range dispatches {
    go func(d Dispatch) {
        sem <- struct{}{}       // 获取信号量（阻塞等待）
        defer func() { <-sem }() // 释放信号量

        // 1. 标记 running
        e.store.MarkDispatchRunning(ctx, d.DispatchID)

        // 2. 复制 metadata（防止并发 mutation）
        meta := make(map[string]string, len(d.Metadata)+2)
        for k, v := range d.Metadata { meta[k] = v }
        meta["user_id"] = e.config.UserID
        meta["channel"] = e.config.Channel
        meta["chat_id"] = e.config.ChatID

        // 3. 执行
        execResult, execErr := e.executor.Execute(ctx, d.AgentID, d.Prompt, meta)

        // 4. 聚合结果（需要 mu 互斥）
        mu.Lock()
        defer mu.Unlock()
        if execErr != nil {
            result.Failed++
            result.FailedAgents = append(result.FailedAgents, d.AgentID)
            e.store.MarkDispatchFailed(ctx, d.DispatchID, execErr.Error())
        } else {
            result.Succeeded++
            e.store.MarkDispatchDone(ctx, d.DispatchID, execResult.TaskID)
        }
    }(d)
}
wg.Wait()
```

**聚合后处理**：

```go
// 确定性排序（按 AgentID + Status）
sort.Slice(result.AgentSummary, ...)

// 三态结果
switch {
case result.Failed == 0:      → CycleSuccess
case result.Succeeded > 0:    → CyclePartialSuccess
default:                      → CycleFailed
}
```

### 4.7 Phase 6: UPDATE — 状态持久化

`engine.go:169-247` — `persistCycleRuntimeState` 是最长的私有方法（79 行），处理大量边界情况。

**完整流程**：

```
1. Pre-cycle git commit（"pre-cycle run-xxx"）
     │
2. 读取当前 STATE.md
     │ (空/错误 → SeedState)
     │
3. 解析已有 cycle_history 表
     │
4. 新增当前 cycle entry → 前置插入
     │
5. 裁剪到 MaxCycleHistory 行
     │
6. 渲染 runtime block
     │
7. Upsert 到 STATE.md
     │
     ├─ 沙箱受限 → WriteKernelStateFallback()
     │
     └─ 正常 → stateFile.Write(updated)
           │
           └─ 写失败且是沙箱错误 → fallback
     │
8. Post-cycle git commit（"post-cycle run-xxx"）
```

`upsertKernelRuntimeBlock` 是一个精巧的字符串手术：

```go
// engine.go:472-501
找到 <!-- KERNEL_RUNTIME:START --> 和 <!-- KERNEL_RUNTIME:END -->
  ├─ 找到 → 精确替换该区间
  └─ 未找到 → 追加到文件末尾
```

### 4.8 Run() — Cron 主循环

`engine.go:622-666` — 简洁的无限循环：

```go
sched, _ := cronParser.Parse(e.config.Schedule)

for {
    nextRun := sched.Next(time.Now())
    timer := time.NewTimer(time.Until(nextRun))

    select {
    case <-ctx.Done():    → 退出
    case <-e.stopped:     → 退出
    case <-timer.C:       → RunCycle()
    }
}
```

RunCycle 在 `func(){}()` 中同步执行（不是 goroutine），但 `wg.Add(1)` + `defer wg.Done()` 确保 Drain 能等待它完成。一个 cycle 没执行完，下一个 cron 触发不会叠加 — timer.C 会在 cycle 执行完后才重新设置。

### 4.9 优雅关闭

```go
func (e *Engine) Drain(_ context.Context) error {
    e.Stop()      // 关闭 stopped channel
    e.wg.Wait()   // 等待 in-flight RunCycle
    return nil
}
```

`stopOnce` 确保 `close(e.stopped)` 只执行一次，多次调用 Stop 不 panic。

---

## 五、Executor 验证与重试的实现细节

### 5.1 Execute() 主流程

`executor.go:101-163` — CoordinatorExecutor 的核心方法。

**Context 构建链**（8 个步骤）：

```go
execCtx = id.WithRunID(ctx, runID)                      // 1. 运行 ID
execCtx = id.WithSessionID(execCtx, sessionID)          // 2. 会话 ID（"kernel-{agent}-{run}"）
execCtx = id.WithUserID(execCtx, meta["user_id"])       // 3. 用户 ID
execCtx = appcontext.WithChannel(execCtx, channel)      // 4. 渠道
execCtx = appcontext.WithChatID(execCtx, chatID)        // 5. 聊天 ID
execCtx = appcontext.WithLLMSelection(execCtx, sel)     // 6. LLM 提供商/模型选择
execCtx = toolshared.WithAutoApprove(execCtx, true)     // 7. 自动审批
execCtx = context.WithTimeout(execCtx, timeout)         // 8. 超时控制
```

### 5.2 验证逻辑

`executor.go:286-294` — `validateKernelDispatchResult` 执行两个检查：

**检查 1：确认请求检测**

```go
func dispatchStillAwaitsUserConfirmation(result) bool {
    // 层 1：结构化 stop reason
    if result.StopReason == "await_user_input" → true

    // 层 2：结构化消息提取
    if ExtractAwaitUserInputPrompt(result.Messages) → true

    // 层 3：文本启发式匹配（最后手段）
    return answerContainsUserConfirmationPrompt(result.Answer)
}
```

`answerContainsUserConfirmationPrompt` 的双语匹配表：

| 英文模式 | 中文模式 |
|---------|---------|
| `"do you want me"` | `"你要我" + "吗"` |
| `"my understanding is" + "?"` | `"我的理解是" + ("对吗" \| "是否")` |
| `"please confirm"` | `"请确认"` |
| `"please choose"` | `"请选择"` |
| `"option a" + "option b"` | `"可选" + ("a)" \| "b)")` |
| — | `"请回复"` |

**检查 2：真实工具执行检测**

`executor.go:240-284` — `containsSuccessfulRealToolExecution` 有三层逻辑：

```
层 1：构建 ToolCall ID → Name 映射表
     │
层 2：遍历 ToolResults
     │
     ├─ 有 name + 无 error + 非编排工具 → return true
     ├─ 无 name + 无 error → 标记 sawSuccessfulUnknown
     └─ 全部遍历完
           │
           ├─ hasToolResults && sawSuccessfulUnknown → return true
           │   （宽容模式：provider 没返回 name 也算）
           │
           └─ fallback: 遍历 ToolCalls（无 result 对应）
                 └─ 有非编排工具 → return true
```

**编排工具黑名单**：

```go
func isOrchestrationTool(name string) bool {
    switch: "plan", "clearify", "clarify", "todo_read",
            "todo_update", "attention", "context_checkpoint",
            "request_user"
}
```

注意 `"clearify"` 是个 typo 兼容 — 说明曾经有 agent 拼错过这个工具名。

### 5.3 重试机制

```go
// executor.go:143-155
if validateErr := validateKernelDispatchResult(result); validateErr != nil {
    // 分类错误类型
    recoveredFrom = classifyKernelValidationError(validateErr)

    // 构建重试 prompt = 原始 prompt + 重试指令 + 上次执行摘要
    retryPrompt := appendKernelRetryInstruction(taskPrompt, result)

    // 用同一个 sessionID 再执行一次（continuation）
    retryResult, retryErr := e.coordinator.ExecuteTask(execCtx, retryPrompt, sessionID, nil)

    attempts++

    // 如果重试也失败验证 → 放弃，返回错误
    if retryValidateErr := validateKernelDispatchResult(retryResult); retryValidateErr != nil {
        return ExecutionResult{}, retryValidateErr
    }
    result = retryResult
}
```

**重试 prompt 结构**：

```
[原始 prompt（含 founder directive + agent prompt + summary instruction）]

Kernel retry requirement:
- Your previous attempt was not autonomously complete.
- Do NOT ask questions, confirmations, or A/B choices.
- Execute at least one concrete real tool action now.
- Write files directly to ~/.alex/kernel/default/ (this path is authorized and writable).
- Return a factual "## 执行总结" with concrete actions and artifact paths.

Previous attempt summary:
[上次执行的 ## 执行总结 内容]
```

重试使用**同一个 sessionID** — ReAct 循环的上下文是连续的，LLM 能看到之前的对话历史。

### 5.4 执行摘要提取

```go
func extractKernelExecutionSummary(result) string {
    answer := result.Answer
    if idx := strings.Index(answer, "## 执行总结"); idx >= 0 {
        answer = answer[idx:]  // 只保留 ## 执行总结 之后的内容
    }
    return compactSummary(answer, 500)  // 截断到 500 字符
}
```

### 5.5 自主性信号装饰

```go
func decorateAutonomySummary(result ExecutionResult) string {
    // 默认: "[autonomy=actionable]"
    // 重试成功: "[autonomy=actionable, attempts=2, recovered_from=no_real_action]"
    parts := []string{fmt.Sprintf("autonomy=%s", autonomy)}
    if result.Attempts > 1 {
        parts = append(parts, fmt.Sprintf("attempts=%d", result.Attempts))
    }
    if result.RecoveredFrom != "" {
        parts = append(parts, fmt.Sprintf("recovered_from=%s", recovered))
    }
    return "[" + strings.Join(parts, ", ") + "]" + " " + summary
}
```

---

## 六、FileStore 持久化的并发设计

### 6.1 双层架构

`file_store.go` 使用经典的 **内存缓存 + 磁盘持久化** 双层架构：

```
┌──────────────────────────────────────────┐
│          内存层（map + RWMutex）          │
│                                          │
│  dispatches map[string]kernel.Dispatch   │
│  ┌─────────────────────────────────┐     │
│  │ "uuid-1" → Dispatch{pending}   │     │
│  │ "uuid-2" → Dispatch{running}   │     │
│  │ "uuid-3" → Dispatch{done}      │     │
│  └─────────────────────────────────┘     │
├──────────────────────────────────────────┤
│          磁盘层（dispatches.json）        │
│                                          │
│  AtomicWrite (tmp + rename)              │
│  确定性排序（created_at ASC）            │
└──────────────────────────────────────────┘
```

### 6.2 锁策略

| 操作 | 锁类型 | 理由 |
|------|--------|------|
| `EnqueueDispatches` | `mu.Lock()` 写锁 | 修改 map + persist |
| `ClaimDispatches` | `mu.Lock()` 写锁 | 修改 lease 字段 + persist |
| `MarkDispatchRunning/Done/Failed` | `mu.Lock()` 写锁 | 修改 status + persist |
| `ListActiveDispatches` | `mu.RLock()` 读锁 | 只遍历 map |
| `ListRecentByAgent` | `mu.RLock()` 读锁 | 只遍历 map |
| `load` | `mu.Lock()` 写锁 | 初始化 map |

每次写操作都触发 `persistLocked()` — 即 **每次状态变更都刷盘**。这牺牲了性能但保证了崩溃一致性。对于 kernel 的使用场景（每 10-30 分钟一次 cycle，每次 1-3 个 dispatch），这是正确的权衡。

### 6.3 事务语义

`EnqueueDispatches` 有**手动回滚**：

```go
// file_store.go:87-98
s.mu.Lock()
for _, d := range created {
    s.dispatches[d.DispatchID] = d  // 写入内存
}
if err := s.persistLocked(); err != nil {
    // 持久化失败 → 回滚内存
    for _, d := range created {
        delete(s.dispatches, d.DispatchID)
    }
    return nil, err
}
```

其他写操作（Mark*）没有回滚 — 因为它们是**幂等**的：如果 persist 失败但内存已更新，下次 persist 会把正确的状态写入磁盘。

### 6.4 ClaimDispatches 的优先级排序

```go
// file_store.go:132-138
sort.Slice(candidates, func(i, j int) bool {
    if candidates[i].Priority != candidates[j].Priority {
        return candidates[i].Priority > candidates[j].Priority  // 高优先级在前
    }
    return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)  // 同优先级按时间排
})
```

Claim 还检查 **lease 过期**：

```go
if d.LeaseOwner != "" && d.LeaseUntil != nil && d.LeaseUntil.After(now) {
    continue  // 仍在 lease 期内，跳过
}
```

一个 worker 崩溃后，其 lease 到期后，其他 worker 可以重新 claim 这个 dispatch。

### 6.5 持久化格式

```go
func (s *FileStore) persistLocked() error {
    // 1. 收集所有 dispatch
    doc := fileStoreDoc{Dispatches: ...}

    // 2. 确定性排序（按 CreatedAt ASC）
    sort.Slice(doc.Dispatches, ...)

    // 3. 格式化 JSON（缩进，便于调试）
    data, _ := jsonx.MarshalIndent(doc, "", "  ")
    data = append(data, '\n')

    // 4. 原子写入（tmp + rename）
    filestore.AtomicWrite(s.filePath, data, 0o600)
}
```

**确定性排序**保证了相同状态的两次 persist 生成相同的文件内容 — 这对 diff/审计很有价值。

### 6.6 注入时间（可测试性）

```go
type FileStore struct {
    now func() time.Time // injectable for tests
}
```

所有时间戳操作都通过 `s.now()` 而不是 `time.Now()` — 测试可以注入固定时间，使断言精确。

### 6.7 当前不足

FileStore **没有实现** `RecoverStaleRunning`。Engine 里的类型断言 `e.store.(staleDispatchRecoverer)` 会失败，跳过恢复逻辑。这意味着如果进程在 dispatch 执行中被杀死，该 dispatch 会永远保持 `running` 状态。

**缓解方式**：`StaticPlanner` 会跳过 `status=running` 的 agent，但它只看最近一条。如果最近一条不是 running 的，旧的 stale running dispatch 就不会被清理。实际影响较小，因为 dispatch 记录会越来越多，最终被新记录掩盖。

---

## 七、Prompt 工程深度分析

### 7.1 三层 Prompt 包装

`wrapKernelPrompt` (`executor.go:176-190`) 将 agent 的原始 prompt 包装成三段结构：

```
┌─────────────────────────────────────────────────────┐
│ Layer 1: kernelFounderDirective (创始人心态)         │
│                                                     │
│ 你是 elephant.ai 的 kernel 自主代理，以创始人心态运作。 │
│ - 永不询问                                          │
│ - 永不等待                                          │
│ - 只做四件事                                        │
│ - 创始人心态                                        │
│ - 每个 cycle 必须产出可观测的进展                     │
├─────────────────────────────────────────────────────┤
│ Layer 2: Agent Prompt (来自 YAML config)             │
│                                                     │
│ 当前系统状态:                                        │
│ {STATE} ← 已被替换为 STATE.md 实际内容               │
│ 请执行...                                           │
├─────────────────────────────────────────────────────┤
│ Layer 3: kernelDefaultSummaryInstruction (执行要求)   │
│ (仅当 agent prompt 不含 "## 执行总结" 时追加)        │
│                                                     │
│ - MUST complete at least one real tool action        │
│ - Do NOT claim completion without tool evidence      │
│ - Do NOT use request_user, clarify                  │
│ - Write files to ~/.alex/kernel/default/            │
│ - Include "## 执行总结"                             │
└─────────────────────────────────────────────────────┘
```

### 7.2 Prompt 流转路径

```
YAML config (agent.prompt 模板)
    │
    ├── ${ENV_VAR} 展开 ← 启动阶段（一次性）
    │
    ▼
StaticPlanner.Plan()
    │
    ├── {STATE} → STATE.md 内容 ← 每个 cycle 动态替换
    │
    ▼
wrapKernelPrompt()
    │
    ├── 前置: kernelFounderDirective（创始人心态准则）
    ├── 中间: agent 自身 prompt（已含 STATE）
    └── 后置: kernelDefaultSummaryInstruction（执行要求）
    │
    ▼
AgentCoordinator.ExecuteTask()
    │
    └── ReAct 循环（Think → Act → Observe）
```

### 7.3 为什么用中文写 Founder Directive？

Founder Directive 使用中文，而 Summary Instruction 使用英文 — 这不是随意的：

- **Founder Directive** 是行为范式指导，用中文更容易被 LLM 理解为"角色设定"而非"任务指令"
- **Summary Instruction** 是格式化要求，用英文更精确（避免"必须"的歧义程度）
- Agent Prompt 由用户在 YAML 中自定义，可以是任何语言

### 7.4 防重复追加

```go
if !strings.Contains(trimmed, "## 执行总结") {
    b.WriteString(kernelDefaultSummaryInstruction)
}
```

如果 agent prompt 自己已经包含了 `## 执行总结` 要求，不会再追加 summary instruction — 避免指令冗余导致 LLM 困惑。

### 7.5 SeedState 的 Prompt 工程

```
# Kernel State
## identity
elephant.ai autonomous kernel — founder mindset.
永不询问、永不等待、只派发任务、记录状态、做总结、思考规划。
## recent_actions
(none yet)
```

种子状态在 `{STATE}` 替换后被注入到 agent prompt 中。它的 `## identity` 部分是一个**自我强化循环**：

1. Agent 读到自己的 identity 是"永不询问、永不等待"
2. Founder Directive 也说"永不询问、永不等待"
3. Summary Instruction 也说"Do NOT use request_user"
4. 三重强化 → 高可靠性的自主行为

### 7.6 PATH 硬编码的原因

```
Write files directly to ~/.alex/kernel/default/
(this path is authorized and writable).
```

为什么在 prompt 里硬编码路径？因为 kernel 调用的外部 agent（如 Claude Code bridge）可能有沙箱限制。明确告诉 LLM 这个路径是被授权的，避免 LLM 因为"不确定是否有写权限"而放弃写文件。

---

## 八、LLMPlanner 设计方案讨论

### 8.1 当前 StaticPlanner 的局限

`planner.go:32-51` — StaticPlanner 只做三件事：

1. 过滤 `Enabled=false` 的 agent
2. 跳过 `status=running` 的 agent
3. 字符串替换 `{STATE}`

它 **完全不理解状态内容**。这意味着：

- 即使 STATE 中记录了"任务已完成"，agent 仍然会被派发
- 即使 STATE 中记录了"API 限流中"，agent 仍然会尝试调用 API
- 无法根据上一轮结果调整优先级

### 8.2 LLMPlanner 设计草案

```go
type LLMPlanner struct {
    llm        llm.Client       // LLM 客户端
    agents     []AgentConfig     // 可用 agent 列表
    kernelID   string
}

func (p *LLMPlanner) Plan(ctx context.Context, stateContent string, recentByAgent map[string]Dispatch) ([]DispatchSpec, error) {
    // 1. 构建 planning prompt
    prompt := buildPlanningPrompt(stateContent, recentByAgent, p.agents)

    // 2. 调用 LLM（低成本模型即可）
    response, err := p.llm.Complete(ctx, prompt)

    // 3. 解析 LLM 输出（结构化 JSON）
    decisions := parsePlanningDecisions(response)

    // 4. 转换为 DispatchSpec
    return toDispatchSpecs(decisions, stateContent, p.agents)
}
```

**Planning Prompt 设计**：

```markdown
你是 kernel 调度器。根据当前状态和最近执行记录，决定本轮应该派发哪些 agent。

## 当前状态
{stateContent}

## 可用 Agent
| agent_id | priority | 上次状态 | 上次时间 |
|----------|----------|---------|---------|
| research | 10 | done | 5min ago |
| report   | 5  | failed | 15min ago |

## 决策规则
1. 跳过最近 2 轮内成功且无新任务的 agent
2. 失败的 agent 降低优先级（除非状态中标记"urgent"）
3. 输出 JSON:
   [{"agent_id": "research", "dispatch": true, "reason": "新任务"}, ...]
```

### 8.3 Hybrid Planner（推荐方案）

```go
type HybridPlanner struct {
    static *StaticPlanner
    llm    *LLMPlanner
    mode   string  // "static" | "llm" | "hybrid"
}

func (p *HybridPlanner) Plan(ctx, state, recent) ([]DispatchSpec, error) {
    // Static 做基本过滤
    staticSpecs, _ := p.static.Plan(ctx, state, recent)

    if p.mode == "static" || len(staticSpecs) <= 1 {
        return staticSpecs, nil  // 只有 0-1 个 agent 时不需要 LLM 决策
    }

    // LLM 做智能裁剪/重排
    return p.llm.Refine(ctx, state, recent, staticSpecs)
}
```

### 8.4 扩展占位符系统

当前只有 `{STATE}`，未来可以扩展：

| 占位符 | 替换时机 | 内容 |
|--------|---------|------|
| `{STATE}` | Plan 阶段 | STATE.md 全文 |
| `{RECENT_RESULTS}` | Plan 阶段 | 上一轮各 agent 的执行摘要表 |
| `{TIME}` | Plan 阶段 | 当前时间（ISO 8601） |
| `{MEMORY}` | Execute 阶段 | Memory engine 检索的相关上下文 |
| `{CYCLE_HISTORY}` | Plan 阶段 | 最近 N 轮的历史表 |

实现方式保持简单 — 仍然是 `strings.ReplaceAll`。

### 8.5 实现代价评估

| 方案 | 改动量 | 风险 | 收益 |
|------|--------|------|------|
| 扩展占位符 | ~50 行 | 低 | 中（更多上下文但不智能） |
| LLMPlanner | ~200 行 | 中（LLM 可能做出错误决策） | 高（按需调度） |
| HybridPlanner | ~250 行 | 低（static 兜底） | 高 |

---

## 九、Kernel 与 Lark 集成的完整链路

### 9.1 启动阶段 — DI 构建

`builder_hooks.go:222-328` — `buildKernelEngine`：

```
YAML Config
    │
    ├─ kernel.enabled = true?
    │     └─ No → 跳过
    │
    ├─ ValidateSchedule(cfg.Schedule)  ← 编译期校验 cron
    │
    ├─ NewFileStore(dir, leaseDuration)  ← Dispatch 队列
    │     └─ EnsureSchema() ← 创建目录 + 加载已有数据
    │
    ├─ VersionedStore(stateDir)  ← Git 版本化
    │     └─ Init() ← git init（失败则降级到普通文件）
    │
    ├─ NewStateFile/NewVersionedStateFile  ← STATE.md 管理器
    │     ├─ SeedInit(INIT.md)  ← 只写一次
    │     └─ WriteSystemPrompt(SYSTEM_PROMPT.md)  ← 每次启动刷新
    │
    ├─ NewStaticPlanner(kernelID, agents)
    │
    ├─ NewCoordinatorExecutor(coordinator, timeout)
    │     └─ SetSelectionResolver(buildKernelSelectionResolver())
    │
    └─ NewEngine(config, stateFile, store, planner, executor, logger)
          └─ SetSystemPromptProvider(coordinator.GetSystemPrompt)
```

### 9.2 LLM Selection 解析链

`builder_hooks.go:330-370` — `buildKernelSelectionResolver`：

```
kernel config 的 channel/chatID/userID
            │
            ▼
    构建 fallback scope 链：
    [
      {channel=lark, chatID=oc_xxx},              // chat 级别
      {channel=lark, chatID=oc_xxx, userID=cklxx}, // user 级别
      {channel=lark},                              // channel 全局级别
    ]
            │
            ▼
    store.GetWithFallback(scopes...)
    按顺序查找第一个匹配的 Selection
            │
            ▼
    resolver.Resolve(selection)
    解析 provider/model → API key + base URL
            │
            ▼
    appcontext.WithLLMSelection(ctx, resolved)
    注入到执行上下文
```

如果你在 Lark 群里切换了模型，kernel 也会跟着切换。

### 9.3 Bootstrap 阶段 — 启动引擎

`bootstrap/kernel.go:15-62` — `KernelStage`：

```
Foundation.KernelStage()
    │
    ├─ kernel.Enabled && KernelEngine != nil?
    │     └─ No → 静默跳过
    │
    ├─ 如果 LarkGateway != nil:
    │     │
    │     └─ 注册 CycleNotifier:
    │           │
    │           ├─ loader := gw.NoticeLoader()
    │           │   └─ 从 /notice 绑定获取目标 chatID
    │           │
    │           └─ notifier = func(ctx, result, err) {
    │                 chatID, ok, _ := loader()
    │                 if !ok → 跳过（/notice 未绑定）
    │                 text := FormatCycleNotification(kernelID, result, err)
    │                 gw.SendNotification(ctx, chatID, text)
    │               }
    │
    └─ SubsystemManager.Start:
          │
          ├─ async.Go("kernel-engine", engine.Run(ctx))
          │     └─ 独立 goroutine 运行 cron 循环
          │
          ├─ 注册为 Drainable
          │     └─ 优雅关闭时 Drain() → Stop() + wg.Wait()
          │
          └─ 返回 engine.Stop 作为关闭回调
```

### 9.4 通知格式

`notifier.go:17-59` — `FormatCycleNotification`：

```
Kernel[default] 周期完成总结
- cycle_id: run-abc123
- 状态: success
- 任务总数: 2
- 已完成: 2
- 失败: 0
- 完成率: 100.0%
- 失败任务: (none)
- 主动性: actionable=2/2, auto_recovered=1, blocked_awaiting_input=0, blocked_no_action=0
- 执行总结:
  - [research|done] [autonomy=actionable, attempts=2, recovered_from=no_real_action] ## 执行总结 ...
  - [report|done] [autonomy=actionable] ## 执行总结 ...
- 耗时: 15.4s
```

### 9.5 主动性信号统计

`notifier.go:101-125` — `summarizeAutonomySignals`：

四个信号提供了 kernel 运行质量的**定量指标**：

| 信号 | 含义 | 诊断价值 |
|------|------|---------|
| **Actionable** | 完全自主完成的 agent 数 | 健康指标 |
| **AutoRecovered** | 第一次失败但重试成功的 agent 数 | 说明 prompt 需要优化 |
| **BlockedAwaitingInput** | 坚持要求人类确认的 agent 数 | 说明 founder directive 不够强 |
| **BlockedNoAction** | 没有做任何实际工作的 agent 数 | 说明 prompt 目标不清晰 |

### 9.6 /notice 绑定机制

`gw.NoticeLoader()` 返回一个 **延迟加载闭包** — 不是启动时就读取 chatID，而是每次 cycle 通知时动态读取。可以在运行时通过 `/notice` 命令重新绑定通知目标，无需重启。

```
Lark 群里输入 "/notice"
    │
    ▼
LarkGateway 记录 chatID
    │
    ▼
下次 kernel cycle 完成
    │
    ▼
notifier 调用 loader() → 获取最新 chatID
    │
    ▼
SendNotification(chatID, text)
    │
    ▼
Lark 群里收到周期通知
```

### 9.7 完整端到端时序

```
t=0    Server 启动
         │
t=1    buildKernelEngine()
         ├─ 创建 FileStore, StateFile, Planner, Executor
         ├─ Seed INIT.md（仅首次）
         └─ 写 SYSTEM_PROMPT.md
         │
t=2    KernelStage.Init()
         ├─ 注册 CycleNotifier（如果有 LarkGateway）
         └─ async.Go → engine.Run(ctx)
         │
t=3    engine.Run() 开始 cron 循环
         └─ 等待第一个 cron 时间点
         │
t=10m  cron 触发 → RunCycle()
         │
t=10m  PERCEIVE: StateFile.Read()
         │
t=10m  ORIENT: ListRecentByAgent()
         │
t=10m  DECIDE: StaticPlanner.Plan()
         ├─ 过滤 disabled/running agents
         └─ 替换 {STATE}
         │
t=10m  ACT: EnqueueDispatches() → FileStore
         │
t=10m  ACT: executeDispatches()
         │     ├─ goroutine 1: Execute(agentA, prompt, meta)
         │     │   ├─ Context 构建（8 步）
         │     │   ├─ wrapKernelPrompt(prompt)
         │     │   ├─ coordinator.ExecuteTask() → ReAct 循环
         │     │   │   └─ LLM 调用 → 工具执行 → 观察 → 下一步
         │     │   ├─ validateKernelDispatchResult()
         │     │   │   ├─ 检查确认请求
         │     │   │   └─ 检查真实工具执行
         │     │   ├─ [如果验证失败] → 追加重试指令 → 再次 ExecuteTask
         │     │   └─ MarkDispatchDone/Failed
         │     │
         │     └─ goroutine 2: Execute(agentB, prompt, meta)
         │         └─ ...
         │
t+Δ    wg.Wait() → 所有 dispatch 完成
         │
t+Δ    聚合 CycleResult
         │
t+Δ    persistCycleRuntimeState()
         │   ├─ pre-cycle git commit
         │   ├─ upsert kernel_runtime 区块到 STATE.md
         │   ├─ 更新 cycle_history 表
         │   └─ post-cycle git commit
         │
t+Δ    persistSystemPromptSnapshot()
         │   └─ SYSTEM_PROMPT.md 刷新
         │
t+Δ    notifier(ctx, result, nil)
         │   ├─ loader() → chatID
         │   ├─ FormatCycleNotification()
         │   └─ gw.SendNotification(chatID, text)
         │
t+Δ    日志: Kernel[default] cycle run-xxx: success (dispatched=2 ok=2 fail=0 15.4s)
         │
t=20m  等待下一个 cron 触发...
```

---

## 十、演进脉络

Kernel 从 2026-02-11 到 2026-02-15，经历了 **5 个版本迭代**（30 个 commit），每个版本都在解决上一版本暴露的实际问题。

### V1.0 — 骨架搭建 (02-11)

```
e429db2c feat(kernel): add domain types for dispatch queue and cycle result
a0a62948 feat(kernel): add StateFile and Postgres dispatch store
0b86ad89 feat(kernel): add Engine, StaticPlanner, and CoordinatorExecutor
71b5d112 fix(kernel): address P0/P1 code review findings
```

**做了什么**：完整的 OODA 循环、Postgres 持久化、STATE.md 原子读写、cron 调度。

**暴露的问题**：
- MockExecutor 并发数据竞争（P0）
- Postgres 入队没有事务保护（P0）
- 没有优雅关闭、没有 cron 校验（P1）

### V1.1 — 运行时状态 + 通知 (02-11 ~ 02-12)

```
f067944f feat(kernel): persist init/system prompt docs and runtime state block
116fbc8a feat(kernel): notify /notice-bound Lark group after non-empty cycles
3a98dd69 fix(kernel): recover stale running dispatches before planning
8ceba0ff fix(kernel): require successful real tool execution
```

**做了什么**：
- INIT.md/SYSTEM_PROMPT.md 快照持久化
- STATE.md 中 `kernel_runtime` 区块（cycle 执行指标）
- Lark 周期通知
- 中断 dispatch 恢复（stale running → pending）
- **Real Action Guard**：要求至少一次真实工具执行

**暴露的问题**：Agent 执行完但没做任何实际事情（只是输出文字），需要强制要求工具证据。

### V1.2 — 可观测性 + 历史 (02-12 ~ 02-13)

```
4bea3613 feat(kernel): add rolling cycle history to STATE.md runtime block
e0d5f491 feat(kernel): integrate StateFile with VersionedStore
2977863b kernel: expose autonomy metadata and cycle notification signals
```

**做了什么**：
- 滚动 cycle 历史表（STATE.md 内最近 5 轮记录）
- Git 版本化存储（pre/post cycle commit）
- Agent 自主性信号分类（actionable / auto_recovered / blocked_*）

### V1.3 — 自主性保障 (02-13 ~ 02-14)

```
d881b0b4 kernel: add fallback state persistence
59a1c1d0 kernel: enforce autonomous completion in cycle dispatch
3d18bae3 kernel: retry autonomously when dispatch result is non-final
5dec3dd3 kernel: auto-approve all tool executions in kernel dispatch context
```

**做了什么**：
- 沙箱降级路径（`~/.alex/kernel/` 被限制时回退到 `artifacts/`）
- **自主完成验证**：检测"请确认"等等待人类回应的模式
- **自动重试**：验证失败时追加重试指令再执行一次
- **Auto-Approve**：kernel context 设置 `WithAutoApprove(true)` 跳过审批门禁

**暴露的问题**：Agent 虽然不再卡住，但仍然会"客气地请求确认"。

### V1.4 — 创始人心态 + 文件化 (02-14 ~ 02-15)

```
71f78730 feat(kernel): add founder mindset directive to kernel dispatch prompts
b93fe796 refactor(kernel): adopt filestore primitives in dispatch store
81467efa refactor: remove all Postgres stores, replace with file-based alternatives
```

**做了什么**：
- **Founder Directive**：每个 dispatch prompt 前置"创始人心态"指令
- Postgres → FileStore 迁移（轻量化部署）
- 采用 filestore 统一持久化原语

### 演进模式

每个版本的演进都遵循同一模式：

```
实际运行 → 暴露问题 → 记录 plan doc → 修复 → 验证
```

15 个 plan 文件记录了每一个设计决策的上下文和理由。这不是过度设计，而是 **compounding engineering** — 每个失败都转化为了制度化的防线。

---

## 十一、总结

### 设计哲学

| 哲学 | 体现 |
|------|------|
| **State as Opaque Document** | Engine 不解析 STATE.md 的业务内容，只做传递和 runtime 区块 upsert |
| **Separation of Concerns** | 系统层管"何时/谁"，Agent 层管"做什么/怎么做" |
| **Autonomous by Design** | 三层代码保障（权限层 + 验证层 + 恢复层），而非依赖 prompt |

### 技术精髓

| 维度 | 精髓 |
|------|------|
| **状态管理** | STATE.md 不透明设计 — 系统只传递不解析，Agent 拥有 schema 自由度 |
| **并发控制** | 信号量 + metadata 复制 + 互斥聚合 — 三层并发安全 |
| **自主保障** | Prompt 三层包装 + 结构化验证 + 自动重试 — 代码级防线而非 prompt 祈祷 |
| **可观测性** | Runtime 区块 + cycle history + autonomy 信号 + Lark 通知 — 全链路可追踪 |
| **优雅降级** | 沙箱 fallback + 版本化 fallback + stale dispatch 恢复 — 任何单点故障不中断循环 |
| **可测试性** | 时间注入 + 接口化依赖 + 确定性输出 — 1823 行测试代码 |

### 当前局限与未来方向

| 局限 | 影响 | 演进方向 |
|------|------|---------|
| **StaticPlanner** 无条件分发 | 所有 enabled agent 每轮都执行 | → **LLMPlanner** / **HybridPlanner** |
| **单进程** | FileStore 无分布式锁 | → Redis/etcd 分布式协调 |
| **`{STATE}` 是唯一占位符** | 无法注入时间、memory 等上下文 | → `{RECENT_RESULTS}`, `{TIME}`, `{MEMORY}` |
| **重试只有一次** | 两次都失败就放弃 | → 指数退避 + 多策略重试 |
| **Prompt 注入风险** | STATE.md 内容直接字符串替换进 prompt | → 结构化 state + 清洗/引用 |
| **FileStore 无 RecoverStaleRunning** | 被杀进程的 running dispatch 永远 stale | → 实现基于时间的 lease 过期恢复 |

### 代码规模

- 30 个 commit
- 15 个 plan 文档
- 4060 行代码（含 1823 行测试）
- 5 天从零到 production-ready V1.4
