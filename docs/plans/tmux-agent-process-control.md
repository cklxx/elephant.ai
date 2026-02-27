# tmux 作为 Agent Teams 进程控制层 — 调研分析

## 核心思路

用 tmux 替代当前的 `os/exec` + PID file + Setsid 方案，作为 agent 子进程的**守护和进程控制基础设施**。事件、任务协调、状态机等仍然由 elephant.ai 自己处理。

tmux 的角色 = **进程容器 + 可观测终端**，不是事件总线。

---

## 现状分析：当前进程管理的 3 层

| 层 | 代码 | 做了什么 | 痛点 |
|---|---|---|---|
| `subprocess.Subprocess` | `internal/infra/external/subprocess/` | `exec.Command` + Setsid/Setpgid + PID + stdout pipe/file | 手动管生命周期、手写 orphan detection、没有人类可观测窗口 |
| `bridge.Executor` | `internal/infra/external/bridge/` | Python 桥进程管理、JSONL 事件解析、detached 模式 | detached 模式要自己管输出文件 tail、done sentinel |
| `devops/process.Manager` | `internal/devops/process/` | PID file + meta file + identity match + orphan scan | 整套 PID state 管理、graceful shutdown 都是手写的 |

**共同痛点：**
1. 没有人类可观测性 — 无法 attach 看 agent 实时输出
2. Orphan detection 是自己造的轮子（PID file + signal 0 + identity match）
3. Detached 进程的输出管理（文件 tail、done sentinel）是自己实现的
4. 进程组管理（SIGTERM → wait → SIGKILL）每个地方都在重复

---

## tmux 能力评估

### 进程生命周期

```bash
# 创建 session，spawn 进程
tmux new-session -d -s agent-task-123 -x 200 -y 50 'python3 bridge.py'

# 检查是否存活
tmux has-session -t agent-task-123 2>/dev/null && echo alive

# 优雅终止
tmux send-keys -t agent-task-123 C-c   # SIGINT
tmux kill-session -t agent-task-123     # SIGHUP → 进程收到 HUP

# 强制终止
tmux kill-session -t agent-task-123
```

**优势：**
- tmux server 本身是 daemon，天然 detach — 不需要 Setsid 手动处理
- `has-session` 直接查活 — 不需要 PID file + signal 0
- `kill-session` 干掉整个 session — 不需要手动管 process group
- Session 名字就是 namespace — 不需要 PID file 系统

### 输出捕获

```bash
# 方案 A：pipe-pane（实时流到文件）
tmux pipe-pane -t agent-task-123 -o 'cat >> /tmp/agent-123.jsonl'

# 方案 B：capture-pane（快照式）
tmux capture-pane -t agent-task-123 -p -S -1000  # 最近 1000 行

# 方案 C：进程自己写文件，tmux 只管进程生命周期
# （当前 detached 模式已经这样做了，tmux 替代 Setsid 即可）
```

**推荐方案 C** — 让 bridge 进程继续用 `--output-file` 写 JSONL，tmux 只负责守护。这样：
- 零改动事件协议
- Go 侧用现有的 `OutputReader` tail 文件
- tmux 提供额外的人类可观测窗口（`tmux attach`）

### 钩子系统

```bash
# pane 进程退出时触发
tmux set-hook -t agent-task-123 pane-died 'run-shell "curl http://localhost:PORT/task/123/died"'

# session 关闭时触发
tmux set-hook -t agent-task-123 session-closed 'run-shell "..."'
```

**适合做：**
- 进程死亡通知（替代当前的 `cmd.Wait()` goroutine）
- 写 done sentinel 文件

**不适合做：**
- 事件分发（延迟不可控、无结构化数据）

### 可观测性（杀手特性）

```bash
# 人类 attach 看实时输出
tmux attach -t agent-task-123

# 列出所有 agent sessions
tmux list-sessions -F '#{session_name} #{session_activity} #{session_windows}'

# 查看所有 agent 的最新输出
for s in $(tmux list-sessions -F '#{session_name}'); do
  echo "=== $s ==="
  tmux capture-pane -t $s -p -S -5
done
```

这是**当前方案完全没有**的能力。现在要看 agent 在干嘛，只能读日志文件。

---

## 架构方案：tmux 作为进程容器层

```
┌─────────────────────────────────────────────────────┐
│                  elephant.ai Go 进程                  │
│                                                       │
│  BackgroundTaskManager                                │
│    ├── 事件系统 (typed events, JSONL)                 │
│    ├── 依赖图 (DAG, awaitDependencies)                │
│    ├── 状态机 (pending→running→completed)             │
│    └── TmuxProcessController (new)                    │
│          ├── SpawnInSession(name, cmd) → SessionID    │
│          ├── IsAlive(session) → bool                  │
│          ├── Kill(session) → error                    │
│          ├── CaptureOutput(session, lines) → string   │
│          └── ListSessions(prefix) → []SessionInfo     │
│                                                       │
│  输出读取: OutputReader tails JSONL file (unchanged)  │
└──────────────────┬────────────────────────────────────┘
                   │ tmux new-session / kill-session
                   ▼
┌──────────────────────────────────────────────────────┐
│                    tmux server                        │
│                                                       │
│  Session: elephant-task-abc123                        │
│    └── Pane 0: python3 cc_bridge/cc_bridge.py        │
│         stdout → /path/to/output.jsonl (pipe-pane)   │
│                                                       │
│  Session: elephant-task-def456                        │
│    └── Pane 0: python3 codex_bridge/codex_bridge.py  │
│                                                       │
│  Session: elephant-team-review-stage1                 │
│    ├── Window 0: role-planner (claude_code)           │
│    └── Window 1: role-coder (codex)                   │
└──────────────────────────────────────────────────────┘
```

### 映射关系

| elephant.ai 概念 | tmux 概念 | 理由 |
|---|---|---|
| Team run | Session | 一个 session 包含一个 team 的所有角色 |
| Team stage | Window group | 同一 stage 的角色在同一组 window 中 |
| Agent/Role | Window (or Pane) | 每个 agent 一个 window，独立进程 |
| 单独后台任务 | 独立 Session | 不属于 team 的单任务 |

**注意：** tmux session 内的 window/pane 共享 session 级别的 env vars，正好适合 team 级别的共享配置。

### Team 编排示例

```
# Stage 1: Plan — 只跑 planner
tmux new-session -d -s elephant-team-review \
  -n planner 'python3 cc_bridge.py'

# Stage 2: Execute — planner 完成后，开 coder 和 reviewer
tmux new-window -t elephant-team-review \
  -n coder 'python3 codex_bridge.py'
tmux new-window -t elephant-team-review \
  -n reviewer 'python3 cc_bridge.py'

# 观察全局
tmux attach -t elephant-team-review
# Ctrl-b n/p 切换 window 看不同 agent
```

---

## 对比：tmux vs 现状 vs 替代方案

| 维度 | 现状 (os/exec+PID) | tmux | Overmind | 容器 |
|---|---|---|---|---|
| Spawn 延迟 | ~5ms | ~20ms (tmux RPC) | ~30ms | ~500ms |
| 进程守护 | 手动 Setsid | tmux server 原生 | tmux-based | containerd |
| 输出捕获 | 自己管 pipe/file | pipe-pane 或自管 | 自动 | docker logs |
| 人类可观测 | ❌ 只有日志 | ✅ tmux attach | ✅ overmind connect | ✅ docker exec |
| 进程组管理 | 手动 PGID kill | session/window kill | procfile group | cgroup |
| Orphan 检测 | 手写 PID+identity | `list-sessions` | 内置 | 容器存活 |
| Go 集成 | 原生 os/exec | exec tmux CLI 或 go-tmux 库 | CLI 调用 | Docker SDK |
| 跨平台 | ✅ | macOS+Linux ✅, Windows ❌ | macOS+Linux | 全平台 |
| 资源隔离 | ❌ | ❌ | ❌ | ✅ cgroups |
| 规模 | 无限 | ~100 sessions 轻松 | ~20 进程 | ~100 容器 |
| 依赖 | 零 | tmux binary | tmux + overmind | Docker daemon |

### Overmind 单独说

[Overmind](https://github.com/DarthSim/overmind) 是 Go 写的、基于 tmux 的 Procfile manager。它：
- 自动为每个进程创建 tmux window
- 支持 `overmind connect <name>` attach 到特定进程
- 支持 restart 单个进程
- **但是**：它是为固定 Procfile 设计的，不支持动态 spawn/kill

所以 Overmind 的模式值得参考，但不能直接用 — 我们需要**动态的** session/window 管理。

---

## 实现方案

### Phase 1: TmuxProcessController（替代 subprocess.Subprocess 的 detached 模式）

```go
// internal/infra/external/tmux/controller.go

type SessionConfig struct {
    Name       string            // tmux session name, e.g. "elephant-task-abc123"
    Command    string            // full command to run
    Args       []string
    Env        map[string]string
    WorkingDir string
    OutputFile string            // JSONL output file (bridge writes here)
}

type Controller struct {
    prefix string // "elephant-" namespace prefix
    logger logging.Logger
}

func (c *Controller) Spawn(cfg SessionConfig) error {
    // tmux new-session -d -s <name> -e KEY=VAL '<command>'
}

func (c *Controller) IsAlive(sessionName string) bool {
    // tmux has-session -t <name>
}

func (c *Controller) Kill(sessionName string) error {
    // tmux send-keys -t <name> C-c; sleep 2; tmux kill-session -t <name>
}

func (c *Controller) ListSessions() ([]SessionInfo, error) {
    // tmux list-sessions -F '#{session_name}:#{session_activity}:...'
}

func (c *Controller) Capture(sessionName string, lines int) (string, error) {
    // tmux capture-pane -t <name> -p -S -<lines>
}

func (c *Controller) AddWindow(sessionName, windowName, command string) error {
    // tmux new-window -t <session> -n <name> '<command>'
}
```

### Phase 2: 接入 bridge.Executor

```go
// executeDetached 改用 tmux

func (e *Executor) executeDetached(...) (*agent.ExternalAgentResult, error) {
    // 之前：subprocess.New(Config{Detached: true, Setsid: true, OutputFile: ...})
    // 之后：tmuxCtrl.Spawn(SessionConfig{Name: taskID, Command: pythonBin, ...})

    // 输出读取不变：OutputReader tails the JSONL file
    reader := NewOutputReader(outputFile, doneFile)
    events := reader.Read(ctx)
    // ... 和现在一样处理 events
}
```

### Phase 3: Team 编排

```go
// Team run 创建一个 tmux session
// 每个 role 是 session 内的一个 window
// Stage 推进 = 新一批 windows

func (m *BackgroundTaskManager) dispatchTeam(ctx context.Context, team TeamDef, goal string) {
    sessionName := fmt.Sprintf("elephant-team-%s-%s", team.Name, runID)

    for _, stage := range team.Stages {
        for _, role := range stage.Roles {
            windowName := fmt.Sprintf("%s-%s", stage.Name, role.Name)
            tmuxCtrl.AddWindow(sessionName, windowName, buildBridgeCmd(role))
        }
        // 等 stage 内所有 window 完成
        awaitWindows(sessionName, stage.Roles)
    }
}
```

---

## 关键设计决策

### 1. 事件传递：不走 tmux

tmux 的 `pipe-pane` 和 hooks 延迟不可控。事件传递继续用：
- **Attached 模式**: stdout pipe → Go scanner → typed events
- **Detached 模式**: JSONL file → OutputReader tail → typed events

tmux 只负责进程存活，不参与数据流。

### 2. 进程死亡检测：双重保障

```
主路径: Go 侧 OutputReader 读到 EOF / done sentinel → 标记完成
备份路径: tmux hook (pane-died) → 写 done sentinel → OutputReader 感知
健康检查: 定期 tmux has-session 确认进程还在
```

### 3. 命名规范

```
elephant-task-{taskID}           # 单独后台任务
elephant-team-{teamName}-{runID} # team session
  └── window: {stageName}-{roleName}
```

### 4. tmux server 管理

- elephant.ai 启动时检查 tmux server：`tmux start-server`
- elephant.ai 退出时可选清理：`tmux kill-server` 或留着（detached 任务继续跑）
- 用 socket 隔离：`tmux -L elephant` 独立 socket，不影响用户自己的 tmux

### 5. 降级策略

tmux 不可用时（未安装、server crash）→ 自动降级到当前 `subprocess.Subprocess` 方案。

---

## 风险与 mitigations

| 风险 | 严重度 | Mitigation |
|---|---|---|
| tmux 未安装 | 中 | 启动时检测，降级到 os/exec |
| tmux server crash | 低 | 重启 server + recover sessions（tmux-resurrect 模式） |
| Session 名冲突 | 低 | prefix + UUID |
| tmux CLI 调用开销 | 低 | ~20ms/call，可以 batch，或用 tmux control mode (-CC) 长连接 |
| 编码/terminal 问题 | 中 | 对 bridge 进程设 `TERM=dumb`，避免 escape sequences |
| tmux 版本差异 | 低 | 要求 tmux ≥ 3.0（hooks 稳定版），macOS brew 和 Linux apt 都有 |

---

## 不做什么

- **不用 tmux 做事件总线** — 延迟和结构化不够，继续用 JSONL
- **不用 tmux 做 IPC** — Go channels + typed events 更可靠
- **不用 tmux 做状态持久化** — BackgroundTaskManager 的状态机 + TaskStore 继续用
- **不替换 attached 模式** — 直接 pipe 给 Go 进程的场景不需要 tmux

---

## 收益总结

| 收益 | 影响 |
|---|---|
| 🔭 **人类可观测性** | `tmux attach -t elephant-task-xxx` 直接看 agent 在干嘛 |
| 🧹 **简化 orphan 管理** | `tmux list-sessions` 替代 PID file + identity match |
| 🛡️ **进程守护** | tmux server 天然守护 detached 进程，不需要 Setsid hack |
| 🎛️ **Team 可视化** | 一个 session 多个 window，Ctrl-b n/p 切换看不同 agent |
| 🔧 **运维友好** | 人类可以直接 `tmux send-keys` 往 agent 进程发指令 |
| 📉 **减少自建代码** | 删掉 PID file 管理、orphan detection、process group 手动处理 |

---

## 下一步

1. 确认方案方向：tmux 做进程容器 + 人类可观测，事件自己管
2. 写 `TmuxProcessController` 原型
3. 先接入 detached bridge executor（最高收益点）
4. 再扩展到 team 编排
