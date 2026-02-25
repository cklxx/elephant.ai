# Prompt 组装逻辑与生命周期分析

> Date: 2026-02-25
> Status: Analysis / Discussion

---

## 一、System Prompt 组装的共享管道

两条路径最终都通过 `composeSystemPrompt()` (`internal/app/context/manager_prompt.go`) 组装 system prompt。这是一条 **有序的 section 管道**：

```
Identity & Persona          ← persona.Voice / SOUL.md 引用
Tooling                     ← 注册的 tool 列表
Tool Routing                ← 工具路由策略
Safety                      ← 硬约束
Habit Stewardship           ← 习惯记忆规则
Mission Objectives          ← long-term / mid-term / metrics
Guardrails & Policies       ← hard constraints / soft preferences
Knowledge & Experience      ← SOP 引用 / memory keys
Persistent Memory           ← SOUL.md + USER.md 快照
OKR Goals                   ← OKR context (optional)
Kernel Alignment            ← kernel mission context (optional, unattended only)
Skills                      ← 可用技能列表
Self-Update / Workspace / Docs / Workspace Files / Sandbox
Timezone / Reply Tags / Heartbeats / Runtime / Reasoning
Environment                 ← world / capabilities / limits / cost
Live Session State          ← plans / beliefs (dynamic)
Meta                        ← persona version
★ Unattended Override       ← **仅 kernel 路径注入**
```

**关键分叉点**: `Unattended` 标志位。Kernel executor 在 `Execute()` 中 `MarkUnattendedContext(ctx)` 后，context manager 检测到该标志，在 system prompt **末尾追加** `buildUnattendedOverrideSection()`。

---

## 二、Kernel 视角：Prompt 组装与生命周期

### 2.1 Prompt 三层叠加

Kernel dispatch 的最终 prompt 是三层叠加：

```
┌─────────────────────────────────────┐
│  Layer 1: System Prompt             │  ← composeSystemPrompt() 输出
│  (Identity + Tools + Safety + ...   │     含 Unattended Override
│   + Kernel Alignment + Memory)      │
├─────────────────────────────────────┤
│  Layer 2: Task Prompt (user msg)    │  ← wrapKernelPrompt() 输出:
│  = kernelFounderDirective           │     Founder Directive (永不询问/永不等待)
│  + agent prompt (含 {STATE} 注入)    │     + Agent-specific prompt
│  + kernelDefaultSummaryInstruction  │     + Summary 要求
├─────────────────────────────────────┤
│  Layer 3 (conditional): Retry       │  ← appendKernelRetryInstruction()
│  = original prompt                  │     仅在首次验证失败时追加
│  + kernelRetryInstruction           │
│  + previous attempt summary         │
└─────────────────────────────────────┘
```

### 2.2 生命周期：OODA Cycle

```
                    ┌──────────────┐
                    │  cron tick   │  "8,38 * * * *"
                    │  or trigger  │
                    └──────┬───────┘
                           │
              ┌────────────▼────────────┐
              │  1. PERCEIVE            │  读 STATE.md (或 seed state)
              │     RecoverStaleRunning │  回收上周期崩溃的 dispatch
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │  2. ORIENT              │  查询每个 agent 最近 dispatch 状态
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │  3. DECIDE              │  LLMPlanner / StaticPlanner
              │     注入 STATE 到 prompt │  生成 dispatch specs
              │     template ({STATE})   │  优先级排序
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │  4. ACT                 │  并行执行 (max 3)
              │     CoordinatorExecutor │  每个 dispatch:
              │     ├─ wrapKernelPrompt │    注入 founder directive
              │     ├─ coordinator      │    走 Preparation → ReAct
              │     │  .ExecuteTask()   │    auto-approve all tools
              │     ├─ validate result  │    检查 real tool execution
              │     └─ optional retry   │    最多重试 1 次
              └────────────┬────────────┘
                           │
              ┌────────────▼────────────┐
              │  5. UPDATE              │  写入 STATE.md runtime block
              │     cycle_history 滚动  │  通知 Lark (CycleNotifier)
              │     SYSTEM_PROMPT.md    │  快照当前 system prompt
              └─────────────────────────┘
```

### 2.3 Kernel 独有的 Prompt 层

| 组件 | 注入位置 | 作用 |
|------|---------|------|
| `kernelFounderDirective` | task prompt 最前 | 永不询问/永不等待/只做四件事 |
| `kernelDefaultSummaryInstruction` | task prompt 最后 | 要求 real tool + Execution Summary |
| `kernelRetryInstruction` | retry prompt 追加 | 恢复自主性 |
| `buildUnattendedOverrideSection` | system prompt 最后 | 全面覆盖：禁止 clarify/request_user |
| `buildKernelAlignmentSection` | system prompt 中段 | mission/soul/user context |
| `{STATE}` 模板替换 | agent prompt 内 | 当前状态注入 |

---

## 三、Lark Agent 视角：Prompt 组装与生命周期

### 3.1 Prompt 结构

```
┌─────────────────────────────────────┐
│  Layer 1: System Prompt             │  ← composeSystemPrompt() 输出
│  (Identity + Tools + Safety + ...   │     **无** Unattended Override
│   + Memory + Skills + OKR)          │     含 File Output Addendum
│   + Habit Stewardship               │     含 Habit Stewardship
│   + Knowledge (full SOP inline)     │     含完整 SOP 内容
├─────────────────────────────────────┤
│  Layer 2: Session History           │  ← 持久化的多轮对话历史
│  (compressed if too long)           │     含 important notes 摘要
│  + recalled user history            │     含 memory search 结果
│  + preloaded attachments            │
├─────────────────────────────────────┤
│  Layer 3: User Message              │  ← 当前用户消息
│  (after quickTriage / preAnalysis)  │     可能含图片附件
└─────────────────────────────────────┘
```

### 3.2 生命周期：消息驱动的多轮会话

```
  ┌──────────────┐
  │ Lark WS Event│  P2MessageReceiveV1
  └──────┬───────┘
         │
  ┌──────▼───────────────┐
  │ 1. RECEIVE           │  去重 (10min TTL)
  │    Parse message      │  AI Chat 多 bot 协调
  │    Resolve session    │  复用 last session / 持久绑定 / 新建
  └──────┬───────────────┘
         │
  ┌──────▼───────────────┐
  │ 2. SLOT MANAGEMENT   │  三态: idle → running → awaitingInput
  │    If running: inject │  inject user input 到正在运行的任务
  │    If awaiting: resume│  恢复 await_user_input 的会话
  │    If idle: start new │  启动新任务 goroutine
  └──────┬───────────────┘
         │
  ┌──────▼───────────────┐
  │ 3. PREPARE           │
  │  ├─ quickTriage (无LLM)│  短消息快速分类
  │  ├─ preAnalyze (async) │  异步 LLM 分类 (4s timeout)
  │  ├─ credential refresh │  长驻服务刷新凭证
  │  ├─ BuildWindow()      │  system prompt 组装
  │  ├─ history load       │  session 消息历史
  │  ├─ memory recall      │  相关记忆检索
  │  └─ build TaskState    │  最终执行状态
  └──────┬───────────────┘
         │
  ┌──────▼───────────────┐
  │ 4. EXECUTE           │
  │    ReAct loop:        │  Thought → Tool calls → Observation
  │  ├─ LLM streaming     │  边生成边推送进度
  │  ├─ tool execution    │  需 approval gate (非 auto-approve)
  │  ├─ approval bridge   │  Lark card 交互式审批
  │  ├─ background tasks  │  异步子任务
  │  └─ stop conditions   │  final_answer / await_user_input / error
  └──────┬───────────────┘
         │
  ┌──────▼───────────────┐
  │ 5. RESPOND           │
  │  ├─ format reply      │  7C 质量塑造
  │  ├─ plan review       │  如需审批: Lark card
  │  ├─ send message      │  编辑进度消息 / 新消息
  │  ├─ send attachments  │  文件上传
  │  └─ update slot state │  idle / awaitingInput
  └──────────────────────┘
```

### 3.3 Lark 独有的 Prompt 层

| 组件 | 注入位置 | 作用 |
|------|---------|------|
| Session History | messages 层 | 多轮对话记忆 |
| Memory Recall | messages 层 | 基于 memory_search 的相关历史 |
| File Output Addendum | system prompt 中段 | Lark 上传文件指引 |
| Artifacts Addendum | system prompt 中段 | Web 模式 artifact 渲染指引 |
| Habit Stewardship | system prompt 中段 | 用户习惯记忆规则 |
| quickTriage 分类 | 前处理 | 跳过 greeting/ack 的上下文加载 |
| preAnalysis 分类 | 异步 | 任务复杂度/表情/标题 |
| Credential Refresh | 前处理 | 长驻进程凭证刷新 |
| Approval Gates | 执行时 | 交互式工具审批 (非 auto-approve) |

---

## 四、核心差异对比

| 维度 | Kernel | Lark Agent |
|------|--------|------------|
| **触发** | Cron 定时 (30min) / trigger channel | 用户消息驱动 |
| **会话模型** | 每次执行全新 session，无历史 | 持久 session，跨消息复用 |
| **Prompt 额外层** | Founder Directive + Retry + Unattended Override | Session History + Memory Recall + File/Artifact Addendum |
| **交互性** | 零交互，auto-approve all | 多轮交互，approval gates，await_user_input |
| **Tool 审批** | `AutoApprove=true`, 全部自动通过 | 需要 approval bridge (Lark card) |
| **Stop 条件** | 只接受 actionable (有 real tool action) | final_answer / await_user_input 均合法 |
| **重试** | 内置 1 次重试 + validation | 无自动重试 |
| **状态持久化** | STATE.md + dispatch store + cycle_history | Session store + chat-session binding |
| **Context 特化** | `{STATE}` 模板注入 + Kernel Alignment | quickTriage + preAnalysis + credential refresh |
| **并发模型** | 多 agent 并行 (max 3) | 单 slot 串行 (每 chat 一个任务) |

---

## 五、架构是否需要拆分？

### 当前耦合点

```
Kernel ──uses──▶ AgentCoordinator.ExecuteTask()  ◀──uses── Lark Gateway
                        │
                ┌───────▼────────┐
                │  Preparation   │  共享 Prepare() 流程
                │  ReAct Runtime │  共享执行引擎
                │  Context Mgr   │  共享 prompt 组装
                └────────────────┘
```

两者共享的核心是 **coordinator → preparation → react runtime → context manager** 这条管道。分叉点是：
1. `Unattended` 标志 → 影响 system prompt 尾部
2. `AutoApprove` 标志 → 影响 tool 执行
3. `wrapKernelPrompt()` → 只在 kernel executor 层叠加

### 判断：**不需要拆分**

理由：

1. **分叉点已经干净** — 差异通过 context flags (`Unattended`, `AutoApprove`) 和 wrapper function (`wrapKernelPrompt`) 表达，没有 if-else 纠缠。
2. **共享管道是对的** — Preparation → ReAct → Context Mgr 这条路径的复用消除了大量重复。如果拆分，两套 prompt 组装逻辑会 diverge 并各自腐化。
3. **Kernel 是 Lark Agent 的特化** — Kernel = Lark Agent + (Founder Directive, Auto-Approve, Unattended Override, Validation Gate, Retry)。这是组合而非分叉。

### 可以改进的地方

| 问题 | 建议 |
|------|------|
| `wrapKernelPrompt()` 在 executor 层硬编码 prompt 常量 | 考虑将 founder directive / summary instruction 作为 `KernelConfig` 的可配置字段 |
| Kernel Alignment section 仅在 unattended 时注入，但概念上也适用于 kernel-triggered lark messages | 解耦 "kernel alignment" 和 "unattended" — alignment 是目标，unattended 是执行模式 |
| System prompt 已接近 32000 char clamp | 考虑 SOP summary-only 作为 kernel 默认模式，减少 prompt 体积 |

---

## 六、如何保持 Kernel 永远保活

### 当前机制（已有的）

```
┌─────────────────────────────────────┐
│  1. Cron Loop (robfig/cron)         │  Run() 内 select{} 永不退出
│  2. Lease Recovery                  │  RecoverStaleRunning 回收崩溃 dispatch
│  3. Graceful Drain                  │  Stop() + wg.Wait()
│  4. Docker restart: unless-stopped  │  进程级重启
└─────────────────────────────────────┘
```

### 缺失的保活层

当前架构有一个 **致命盲区**：kernel engine 本身的 goroutine panic 或 `Run()` 内部 error 退出，没有进程内恢复机制。只依赖 Docker restart。

### 建议：三层保活设计

```
Layer 1: In-Process Supervisor (新增)
├── Goroutine panic recovery: Run() 内 defer/recover
├── 自动重启: panic 后 exponential backoff 重新启动 cycle loop
├── Health signal: 周期性向 health endpoint 写 last_cycle_at
└── Metric: kernel_cycle_panic_total counter

Layer 2: Process Health Probe (增强)
├── 现有 /health endpoint 增加 kernel 探针:
│   if time.Since(lastCycleAt) > 2*scheduleInterval → NotReady
├── Docker HEALTHCHECK 改为检查 /health 而非 --help
└── Liveness vs Readiness 分离

Layer 3: External Watchdog (可选, 生产级)
├── systemd: Restart=always + WatchdogSec=120
├── 或 K8s: livenessProbe → /health/kernel
└── 或 自研: Lark 消息告警 if no cycle in 2h
```

### 具体实现方案

#### Layer 1: In-Process Supervisor

```go
// engine.go — 改造 Run()

func (e *Engine) Run(ctx context.Context) {
    for {
        err := e.runLoop(ctx)
        if ctx.Err() != nil {
            return // Context cancelled, clean exit
        }
        select {
        case <-e.stopped:
            return // Explicit stop
        default:
        }
        // Unexpected exit — log, backoff, restart
        e.logger.Error("kernel loop exited unexpectedly, restarting",
            "error", err, "backoff", e.nextBackoff())
        e.emitMetric("kernel_loop_restart_total")
        select {
        case <-time.After(e.nextBackoff()):
        case <-ctx.Done():
            return
        case <-e.stopped:
            return
        }
    }
}

func (e *Engine) runLoop(ctx context.Context) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = fmt.Errorf("kernel panic: %v\n%s", r, debug.Stack())
            e.logger.Error("kernel panic recovered", "panic", r)
            e.emitMetric("kernel_cycle_panic_total")
        }
    }()
    // 现有的 cron schedule loop...
}
```

#### Layer 2: Health Probe

```go
// 在 Engine 上增加
func (e *Engine) HealthStatus() HealthResult {
    last := e.lastCycleAt.Load()
    threshold := 2 * e.scheduleInterval()
    if time.Since(last) > threshold {
        return HealthResult{Ready: false, Reason: "no cycle in " + threshold.String()}
    }
    return HealthResult{Ready: true, LastCycle: last}
}
```

Docker HEALTHCHECK 改为：
```dockerfile
HEALTHCHECK --interval=30s --timeout=3s --start-period=60s --retries=3 \
    CMD curl -f http://localhost:9090/health || exit 1
```

#### Layer 3: Lark 自监控

```go
// 在 CycleNotifier 中增加 absence alert
func (e *Engine) checkAbsenceAlert(ctx context.Context) {
    if time.Since(e.lastSuccessfulCycle) > 2*time.Hour {
        e.notifier(ctx, nil, fmt.Errorf("kernel absence: no successful cycle for 2h"))
    }
}
```

### 保活优先级

| 层级 | 实现难度 | 覆盖场景 | 建议优先级 |
|------|---------|---------|-----------|
| Panic recovery + auto-restart | 低 (20行) | goroutine panic, 意外退出 | **P0 — 立即** |
| Health probe 增强 | 低 (30行) | 进程存活但 kernel 停转 | **P0 — 立即** |
| Docker HEALTHCHECK 改进 | 低 (1行) | 进程存活但不健康 | **P1** |
| Lark absence alert | 中 (50行) | 所有沉默故障 | **P1** |
| systemd/K8s watchdog | 取决于部署环境 | 进程崩溃 | **P2** |

---

## 七、总结

1. **Prompt 组装逻辑是同一条管道的两种配置** — 差异点清晰，通过 flags 和 wrapper 表达。
2. **架构不需要拆分** — Kernel 是 Lark Agent 的特化（组合模式），共享管道的复用价值远大于拆分后的独立演化自由度。
3. **Kernel 保活是当前最大的架构短板** — 缺少 in-process panic recovery 和 health probe。建议立即补上 Layer 1 + Layer 2，成本极低但消除了 silent death 风险。
