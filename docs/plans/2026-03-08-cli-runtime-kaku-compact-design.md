# CLI Runtime with Kaku — Compact Design

Date: 2026-03-08
Status: proposed
Updated: 2026-03-09 (实测验证)

## 1. One-line definition

Build a **multi-session CLI agent runtime**:
- **Kaku** = shared terminal runtime/panel
- **Codex / Claude Code / Kimi / AnyGen / Colab ...** = members using the same terminal path as a human
- **Hooks / monitors** = async completion + health signals
- **Agent scheduler** = dispatch next steps based on member status/artifacts

## 2. Core product model

### Runtime
Responsible for:
- create session
- attach/inject/capture terminal
- persist tape/state
- emit hooks
- resume/stop/cancel

### Session
A long-running task unit.
Fields:
- session_id
- member_type
- goal
- cwd
- status
- terminal_ref
- tape_ref
- artifact_refs
- created_at / updated_at

### Member
A pluggable CLI worker.
Examples:
- codex
- claude_code
- kimi
- anygen
- colab

Contract:
- start command
- input strategy
- output capture strategy
- completion detection
- hook adapter

### Panel
One universal terminal panel.
Same UI for all members:
- live output
- inject input
- stop/resume
- status
- artifacts

## 3. Key principle

**All coding/research members should work through the same human-like terminal path.**
No special Agent API required.
Difference is only:
- startup wrapper
- hooks
- output parsing
- lifecycle monitoring

## 4. Why this is the right cut

1. No need to rebuild agentapi
2. Long tasks can hang in independent sessions
3. Multiple task contexts coexist without mixing
4. New members extend by adapter, not architecture rewrite
5. Terminal becomes the product surface, not hidden implementation

## 5. Architecture

### L0 Member kernels
- codex cli
- claude code cli
- kimi cli
- anygen agent
- colab / notebook worker

### L1 Kaku runtime
- open panel
- send input
- send enter / control commands
- capture output
- attach/detach

### L2 Runtime manager
- session registry
- tape store
- scheduler
- state machine
- artifact index
- hook dispatcher

### L3 Skill facade
Expose stable skills to agent:
- runtime_start_session
- runtime_list_sessions
- runtime_session_status
- runtime_inject
- runtime_resume
- runtime_stop
- runtime_read_artifacts

### L4 Orchestration
- dependency graph
- wait for hook/event
- trigger next member
- merge results

## 6. Status model

- starting
- running
- quiet_running
- needs_input
- stalled
- completed
- failed
- cancelled

## 7. Hook model

Hooks provide async signals back to the scheduler:
- started
- heartbeat
- output_updated
- artifact_created
- needs_input
- completed
- failed
- stalled

Rule:
- scheduler should react to hooks, not poll blindly
- polling remains fallback only

## 8. Extensibility

New members should plug in by adapter only.
Required adapter fields:
- member name
- launch command
- cwd/env builder
- terminal prompt heuristic
- completion heuristic
- hook integration
- artifact collector

So adding `anygen` / `colab` is additive, not architectural.

## 9. Product experience

User sees:
- one task board
- multiple sessions
- one universal panel
- who is running / stalled / done
- outputs and artifacts

User does not need to care whether backend member is codex, cc, kimi, anygen, or colab.

## 10. Implementation order

### Phase 1
Kaku-backed universal session runtime:
- create/inject/capture/resume/stop
- tape persistence
- hooks

### Phase 2
Member adapters:
- codex
- claude code
- kimi

### Phase 3
Scheduler:
- dependency-based auto dispatch
- hook-driven continuation
- async completion handling

### Phase 4
More members:
- anygen
- colab
- other domain workers

## 11. Design guardrails

- one universal panel, not per-member UI
- member-specific logic lives in adapters only
- scheduler uses hook events first
- session/tape is the unit of persistence and recovery
- chat UX stays compact: progress checkpoints, no silent long gaps

## 12. Recommended product phrasing

> Kaku is the CLI runtime.
> Codex / Claude Code / Kimi / AnyGen / Colab are pluggable members.
> Agent schedules multiple sessions, watches hooks, and continues work automatically.


---

## 13. 实测验证结论（2026-03-09）

### Kaku CLI 控制能力

Kaku 是基于 WezTerm 的 GUI terminal emulator，暴露了完整的 `kaku cli` 控制接口：

```bash
# 查看当前所有 pane
kaku cli list

# 拆分 pane（右侧/底部）
kaku cli split-pane --pane-id <ID> --right --percent 30 --cwd <DIR> -- bash -l
kaku cli split-pane --pane-id <ID> --bottom --percent 60 --cwd <DIR> -- bash -l

# 向 pane 发送命令（相当于 tmux send-keys）
kaku cli send-text --pane-id <ID> "command\n"

# 读取 pane 当前输出
kaku cli get-text --pane-id <ID>

# 激活/聚焦 pane
kaku cli activate-pane --pane-id <ID>

# 设置 tab 标题
kaku cli set-tab-title --tab-id <ID> "Kaku Runtime"
```

### 全链路验证结果

在 Kaku pane 内启动 CC（claude_code）并完成任务：

✅ 可以通过 `kaku cli split-pane` 创建新 pane  
✅ 可以通过 `kaku cli send-text` 向 pane 注入命令  
✅ 可以通过 `kaku cli get-text` 读取 pane 输出  
✅ CC 在独立 pane 中完整执行并返回结果  
✅ 用户在 Kaku 界面中实时可见整个执行过程  

### 关键约束：CLAUDECODE 环境变量

CC 检测到 `CLAUDECODE` 环境变量时会拒绝启动（防嵌套保护）：

```
Error: Claude Code cannot be launched inside another Claude Code session.
```

**处理方式：** Kaku adapter 在启动 CC 时必须 unset 该环境变量：

```bash
unset CLAUDECODE && claude --dangerously-skip-permissions -p "..."
```

或在 MemberAdapter.StartSession() 中通过 `Env` 字段显式清除：

```go
proc.Env["CLAUDECODE"] = ""  // 或从 env 中删除
```

### 对 Runtime 设计的影响

- **Kaku CLI 就是 Kaku runtime 的控制面** — `kaku cli` 替代了原始 tmux 操控
- `split-pane` = 创建 session 的执行容器
- `send-text` = InjectInput
- `get-text` = CaptureOutput（轮询模式）
- pane 进程状态 = session 生命周期
- 用户直接在 Kaku GUI 看到所有 session 的实时输出，无需额外 UI

这意味着 **P0 Runtime Skeleton 可以直接基于 `kaku cli` 构建**，无需自己实现 pty/terminal 管理。

完整操作手册见：[docs/guides/kaku-runtime-guide.md](../guides/kaku-runtime-guide.md)
