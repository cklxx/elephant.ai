# CLI Runtime + Kaku 实施计划（2026-03-08）

状态：Implementation Plan v2
更新：2026-03-09
目标：把 Kaku 收敛为统一 CLI runtime，让 Codex / Claude Code / Kimi / AnyGen / Colab 等成员以一致方式接入，并支持多 session 并行、hooks 通知、Leader Agent 调度。

---

## 0. 先说结论

按 **4 个阶段** 做，先做 runtime 骨架，再接入成员，再做调度 + Leader Agent，再做产品化。

1. **P0：Runtime Skeleton**
2. **P1：CLI Member Adapter**
3. **P2：Hooks + Scheduler + Leader Agent**
4. **P3：Panel / UX / Team Recipe**

原则：
- 先把"能稳定开多个 session"做出来
- 再把"Leader Agent 根据 hooks 做调度决策"跑通
- 最后做复杂 UI 和更多成员扩展

---

## 1. 与现有架构的关系

### Session 并存模型

Runtime session 和 Leader Agent session 是**并存关系，不是替代**。

```
┌──────────────────────────────────────────────┐
│          Leader Agent Session                │  ← Leader 自身的 agent 对话 session
│  （决策上下文、调度历史、与用户交互）            │     用于推理和调度决策
├──────────────────────────────────────────────┤
│          Runtime                             │
│  ┌────────────┬────────────┬────────────┐    │
│  │ Runtime    │ Runtime    │ Runtime    │    │
│  │ Session A  │ Session B  │ Session C  │    │  ← 管理外部 CLI 进程的生命周期
│  │ (codex)    │ (cc)       │ (kimi)     │    │     IO 记录、状态追踪
│  └─────┬──────┴─────┬──────┴─────┬──────┘    │
│        ↕            ↕            ↕           │
│    codex CLI    cc CLI      kimi CLI         │  ← 外部进程，各自有自己内部的 session
└──────────────────────────────────────────────┘
```

- **Leader Agent session**：Leader 作为 agent 的对话 session，承载调度推理上下文、与用户的交互历史
- **Runtime session**：管理一个外部 CLI 进程的生命周期（start/stop/resume）、IO 交互记录、状态机
- **Member 内部 session**：各 CLI 工具自己的内部 session（如 claude code 的对话），runtime 不侵入

三层各管各的，互不替代。

### 职责分界

| 层 | 职责 | 不管什么 |
|---|---|---|
| Leader Agent | 调度决策、任务分配、状态判断、用户交互 | 不管进程管理、不管 IO 注入 |
| Runtime | session 生命周期、进程管理、IO 注入/捕获、hooks 事件总线 | 不管调度决策、不管 member 内部推理 |
| Member Adapter | 命令模板、输出格式解析、状态识别规则 | 不管生命周期、不管调度 |
| Member CLI（外部） | 自身的 LLM 推理、tool 调用、context 管理 | 不知道自己被 runtime 管理 |

---

## 2. 控制模型：统一 CLI Wrapping

所有成员统一走 CLI wrapping（stdin/stdout/signal 进程控制），不走 SDK 集成。

### 为什么

- **一致性大于最优性**。两套控制模型（SDK + CLI）会让 adapter 层分裂，上层调度必须处理两种语义。
- **CLI 是最大公约数**。每个 AI 工具都有 CLI，不是每个都有 SDK。统一走 CLI 让接入新成员的成本恒定。
- **解耦彻底**。Kaku 不依赖任何 member 的内部实现，member 升级/替换不影响 runtime。
- **代价可控**。状态识别不完全靠输出解析——可以利用各 CLI 自身的 hooks 机制补充结构化信号。

### MemberAdapter 接口

```go
type MemberAdapter interface {
    StartSession(ctx context.Context, opts StartOpts) (Process, error)
    InjectInput(ctx context.Context, proc Process, input string) error
    CaptureOutput(ctx context.Context, proc Process) (<-chan OutputEvent, error)
    Interrupt(ctx context.Context, proc Process) error
    Resume(ctx context.Context, proc Process) error
    DetectState(ctx context.Context, proc Process) (SessionState, error)
}
```

每个 adapter 只需实现：命令模板、输出格式识别、状态判断规则。

---

## 3. 目标拆分

### P0：Runtime Skeleton（3~5 天）

交付：
- `runtime session` 对象模型 + 状态机
- session 生命周期：create / start / stop / resume / cancel / list / status
- session 元数据持久化（文件系统 JSON）
- `kaku panel` 统一命令执行面

建议模块：
- `internal/runtime/session/` — session 对象、状态机
- `internal/runtime/panel/` — 统一命令执行面
- `internal/runtime/store/` — session 元数据持久化

最小能力：
- 一个 panel 可绑定一个 session
- 一个 runtime 可同时管理多个 session
- 重启后 session 元数据可恢复

验收：
- 同时启动 3 个 session
- 可以单独 inject 输入
- 可以停止/恢复某个 session
- 重启后仍能看到 session 列表和最近状态

CLI 草案：
- `alex runtime session start`
- `alex runtime session list`
- `alex runtime session status`
- `alex runtime session inject`
- `alex runtime session stop`
- `alex runtime session resume`

---

### P1：CLI Member Adapter（4~6 天）

交付：
- `MemberAdapter` 接口定义
- 首批适配器：codex / claude_code / kimi
- 扩展预留：anygen / colab

原则：
- 成员都按"人类使用 CLI"的方式接入
- Kaku 负责注入和控制
- adapter 只补每个 CLI 的命令模板、状态识别、hooks 格式

验收：
- 3 个 coding CLI 都能通过同一 runtime 拉起
- 同一套 session 面板能切换不同 member
- adapter 不影响上层调度逻辑

---

### P2：Hooks + Scheduler + Leader Agent（4~6 天）

交付：
- hooks 事件模型
- 调度器（按依赖/状态自动推进）
- Leader Agent MVP

#### Hooks 事件

```
started / heartbeat / needs_input / completed / failed / stalled / handoff_required
```

#### Scheduler（执行引擎）

负责"怎么执行"：
- A completed → 触发 B start
- A needs_input → 进入 waiting_input
- 并行 session 状态汇总
- 超时/stalled 检测

#### Leader Agent（决策层）

负责"执行什么"：
- 看各 session 当前状态 + hooks 信号
- 决定谁继续、谁暂停、谁接棒、谁并行
- 以"最小化总完工时间"为目标

Leader Agent 有自己的 agent session，独立于 runtime session。它通过 runtime 暴露的 API 读取 session 状态、下发调度指令。

MVP 规则（第一版不做复杂优化器）：
1. 关键路径不断档
2. 有可并行任务时，优先喂给空闲 agent
3. stalled 超阈值 → 降级 / 换人 / 请求人工
4. 完成后自动扫描可启动的下游任务
5. 每轮输出 compact checkpoint

核心信号：
- session 状态：starting / running / stalled / waiting_input / completed / failed
- 最近输出时间 / hooks 心跳
- artifact 产出情况
- 任务 DAG 依赖是否解除
- 各 agent 擅长类型（coding / review / research / doc / tool-use）
- 当前成本与并发占用

关键动作：
- dispatch：给空闲 agent 分配新任务
- rebalance：某 agent 卡住时，把可拆子任务转给别人
- unblock：发现等待输入/依赖时，优先补齐缺口
- promote：关键路径任务优先
- handoff：完成后自动触发下游 session
- intervene：需要人接手时及时上报

验收：
- 两个 session 可串行依赖
- 三个 session 可并行跑
- 某个 session 卡住时能自动上报，不静默
- 完成事件可触发后续 session
- Leader Agent 能基于状态做出合理调度决策

---

### P3：Panel / UX / Team Recipe（5~7 天）

交付：
- 通用 panel
- session list / live output / compact progress checkpoint
- team recipe（team 降成 recipe，不承担 runtime 职责）

用户可见区块：
- Sessions — 谁在跑、谁卡住、谁完成
- Live Terminal — 当前 session 实时输出
- Artifacts — 各 session 产出
- Recent Events — hooks 事件流
- Intervention — 需要人介入的决策

新增能力：
- team recipe = 一组 session 模板
- 支持 coding / research / anygen / colab 混编

产品表现（用户 10 秒内能看懂）：
- 当前谁空闲
- 谁在关键路径上
- 谁卡住了
- 下一步系统准备把什么任务交给谁
- 为什么这么分配

验收：
- 用户能在一个界面看多个 session
- 10 秒内看懂全局状态
- team 只是 recipe，不再承担 runtime 本身职责

---

## 4. 架构总览

```
┌─────────────────────────────────────────────┐
│     Leader Agent（自己的 agent session）      │  ← 决策层：分配、rebalance、promote
│     读取 runtime 状态，下发调度指令             │
├─────────────────────────────────────────────┤
│              Scheduler                      │  ← 执行层：依赖编排、事件驱动推进
├─────────────────────────────────────────────┤
│           Hooks Event Bus                   │  ← 信号层：started/completed/stalled/...
├──────┬──────┬──────┬──────┬─────────────────┤
│ Sess │ Sess │ Sess │ ...  │                 │
│  A   │  B   │  C   │      │     Store       │  ← runtime session + 元数据持久化
├──────┼──────┼──────┼──────┤                 │
│Adapt │Adapt │Adapt │      │                 │
│codex │ cc   │ kimi │      │                 │  ← CLI wrapping adapter
└──────┴──────┴──────┴──────┴─────────────────┘
         ↕ stdin/stdout/signal
┌──────┐┌──────┐┌──────┐
│codex ││claude││ kimi │                        ← 外部 CLI 进程
│ CLI  ││ code ││ CLI  │
└──────┘└──────┘└──────┘
```

模块布局：
- `internal/runtime/session/` — session 对象、状态机
- `internal/runtime/panel/` — 统一命令执行面
- `internal/runtime/store/` — 元数据持久化
- `internal/runtime/adapter/` — MemberAdapter 接口 + 各 CLI adapter
- `internal/runtime/hooks/` — 事件总线
- `internal/runtime/scheduler/` — 依赖编排、事件驱动
- `internal/runtime/leader/` — Leader Agent 调度控制器

---

## 5. 风险与处理

### 风险 1：不同 CLI 的交互模式不一致
处理：adapter 层吸收差异，runtime 不感知具体 CLI 内核。

### 风险 2：长任务恢复不稳定
处理：先保证 session metadata 恢复，tape 细节设计后续迭代。

### 风险 3：hooks 噪音太多
处理：只保留 7 类核心事件（started/heartbeat/needs_input/completed/failed/stalled/handoff_required），其他归并到 event payload。

### 风险 4：team/runtime 抽象打架
处理：明确 runtime 是底座，team 是 recipe。

### 风险 5：Leader Agent 和 Scheduler 职责模糊
处理：Scheduler 只做机械执行（事件驱动推进），Leader Agent 只做决策（读状态、下指令）。Scheduler 不做判断，Leader 不直接操作进程。

---

## 6. 执行建议

先做 **P0 + P1**，证明 4 件事：
- 多 session 能开
- 能恢复
- 能 inject
- 不同成员能共用同一个 runtime

这 4 件成立，P2 的 scheduler + Leader Agent 就有坚实基础。

Tape 的详细设计（schema、粒度、存储格式）在 P0 验证完成后单独出方案，不在本轮定义。

---

## 7. Member Hooks / Notify 增强事件监控

统一 CLI wrapping 的核心挑战之一是**事件监控**——如何知道 member 完成了、卡住了、需要输入。纯靠 stdout 解析不够可靠，需要利用各 CLI 原生的 hooks/notify 机制作为结构化信号源。

### 7.1 Claude Code Hooks

Claude Code 原生支持 3 类 hook 事件，配置在 `.claude/settings.json`：

| Hook 事件 | 触发时机 | Kaku 用途 |
|---|---|---|
| `PreToolUse` | tool 执行前 | 可选：检测活跃度（heartbeat） |
| `PostToolUse` | tool 执行后 | **进度追踪**：每次 tool call 作为 heartbeat 信号 |
| `Stop` | session 终止 | **完成检测**：确定性的终止信号，含 `stop_reason` + `answer` |

#### 配置方式

```json
{
  "hooks": {
    "PostToolUse": [{
      "hooks": [{
        "type": "command",
        "command": "notify_kaku.sh",
        "async": true,
        "timeout": 5
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "command",
        "command": "notify_kaku.sh",
        "async": true,
        "timeout": 5
      }]
    }]
  }
}
```

#### Hook Payload 字段

```json
{
  "event": "PostToolUse|Stop",
  "session_id": "...",
  "tool_name": "Read|Write|Edit|Bash|...",
  "tool_input": { "file_path": "..." },
  "output": "...",
  "stop_reason": "end_turn|max_turns|error",
  "answer": "final result text"
}
```

#### Kaku 事件映射

| CC Hook | Kaku 事件 | 说明 |
|---|---|---|
| `PostToolUse` | `heartbeat` | 有 tool 活动 → session 活跃 |
| `Stop` (stop_reason=end_turn) | `completed` | 正常完成 |
| `Stop` (stop_reason=error) | `failed` | 异常终止 |
| 长时间无 PostToolUse | `stalled` | runtime 侧通过心跳超时判断 |

#### 现有基础设施

项目已有完整的 hooks 链路可复用：
- `scripts/cc_hooks/notify_lark.sh` — hook 脚本模板，读 stdin JSON → POST 到 hooks bridge
- `internal/delivery/server/hooks_bridge.go` — 接收、去重、聚合 hook 事件 → 发送 Lark 通知
- Kaku adapter 只需把 notify 目标从 Lark 换成 runtime event bus

### 7.2 Codex Hooks

Codex CLI **没有原生 hook 机制**，但通过 `codex exec --json` 提供 JSONL 事件流。

| Codex JSONL 事件 | 触发时机 | Kaku 用途 |
|---|---|---|
| `thread.started` | session 启动 | `started` |
| `item.started` (command_execution) | 开始执行命令 | `heartbeat` |
| `item.completed` (agent_message) | agent 输出消息 | 进度追踪 |
| `turn.completed` | 一个 LLM turn 结束 | `heartbeat` + token 统计 |
| 进程退出 (exit 0) | 正常结束 | `completed` |
| 进程退出 (exit ≠ 0) | 异常结束 | `failed` |

#### Kaku 事件映射

| Codex 信号 | Kaku 事件 | 说明 |
|---|---|---|
| `item.started` | `heartbeat` | 有命令执行 → session 活跃 |
| 进程退出 exit 0 + 最终 `result` | `completed` | bridge 发出 `{"type":"result"}` |
| 进程退出 exit ≠ 0 | `failed` | bridge 发出 `{"type":"error"}` |
| 长时间无 JSONL 事件 | `stalled` | runtime 侧通过心跳超时判断 |

#### 现有基础设施

- `scripts/codex_bridge/codex_bridge.py` — 已实现 Codex JSONL → 统一事件协议的翻译
- bridge 产出的 `{"type":"tool"}` 和 `{"type":"result"}` 事件可直接映射为 Kaku hooks

### 7.3 Kimi Hooks

Kimi CLI 的 hook 能力待探测。当前策略：
- 优先查看 kimi CLI 是否支持类似 claude code 的 hooks 配置
- fallback 到 stdout 解析 + 进程退出码检测
- 心跳超时检测 stalled

### 7.4 统一事件总线设计

所有 member 的 hooks/notify 信号最终汇入同一个 runtime event bus：

```
CC Hook (PostToolUse/Stop)  ──→  notify_kaku.sh  ──→ ┐
Codex JSONL (item/result)   ──→  codex_bridge.py ──→ ├→ Runtime Event Bus ──→ Scheduler / Leader Agent
Kimi (stdout parse/exit)    ──→  kimi_adapter    ──→ ┘
```

事件总线统一格式：

```yaml
event: started | heartbeat | needs_input | completed | failed | stalled | handoff_required
session_id: "..."
member_type: "claude_code | codex | kimi"
timestamp: "..."
payload:
  # member-specific details (tool_name, answer, error, etc.)
```

### 7.5 Stalled 检测策略

不依赖 member 主动上报 stalled，由 runtime 统一判断：

1. 每个 session 维护 `last_heartbeat` 时间戳
2. 任何 hook/JSONL 事件都刷新 `last_heartbeat`
3. 超过阈值（建议 120s）无 heartbeat → 标记 `stalled`
4. Leader Agent 收到 `stalled` 后决策：等待 / 重试 / 换人 / 上报

---

## 8. 待定项

- Tape 详细设计（IO 事件级粒度、replay/debug 用途、文件系统 JSON 存储 — 方向已定，schema 待设计）
- PMO Agent 是否与 Leader Agent 合并或独立（P2 MVP 先合并，后续按需拆分）
- AnyGen / Colab adapter 的具体接入时机（P3 预留入口，按需求优先级排）
