---
name: team-cli
description: Team runtime CLI 技能：通过 `alex team run/status/inject` 执行与观测 team（flag-only，无 JSON）。
triggers:
  intent_patterns:
    - "team run|team status|team inject|team runtime|agent team|团队运行态|tmux pane|_team_runtime"
  context_signals:
    keywords: ["team", "runtime", "status", "inject", "tmux", "_team_runtime", "team_id", "session_id", "task_id"]
  confidence_threshold: 0.55
priority: 8
requires_tools: [bash]
max_tokens: 300
cooldown: 15
capabilities: [team_run, team_runtime_status, team_inject, team_terminal]
governance_level: medium
activation_mode: auto
---

# team-cli

通过独立 CLI 使用团队能力，不依赖 channel 工具注册。

## 强约束

- 只允许调用 `alex team ...`（或 `go run ./cmd/alex team ...` 回退）。
- 只允许 flag 传参，禁止 JSON 参数。
- 不使用 `python3 skills/team-cli/run.py`（本 skill 无 run.py）。

## CLI 总览

```bash
alex team run ...
alex team status ...
alex team inject ...
alex team terminal ...
```

若本机未安装 `alex` 二进制，在仓库内使用：

```bash
go run ./cmd/alex team run ...
go run ./cmd/alex team status ...
go run ./cmd/alex team inject ...
go run ./cmd/alex team terminal ...
```

## 1) 运行 team（run）

### 1.1 用模板执行

```bash
alex team run --template claude_research --goal "Compare Postgres logical replication vs CDC for this repo"
```

### 1.2 查看可用模板

```bash
alex team run --template list
```

### 1.3 用 taskfile 执行

```bash
alex team run --file /tmp/team-task.yaml
```

### 1.4 单 prompt 任务（必须支持）

```bash
alex team run --prompt "Audit current branch changes and list top 3 regression risks" --workspace-mode shared
```

### 1.5 常用可选参数

```bash
alex team run \
  --template claude_analysis \
  --goal "Evaluate migration strategy" \
  --mode auto \
  --timeout-seconds 900 \
  --session-id sess_manual_001 \
  --role-prompt analyst_a="Focus on correctness" \
  --role-prompt analyst_b="Focus on ops risk"
```

- `--task-id` 可重复传入，限制执行特定 task。
- `--role-prompt role=prompt` 可重复传入，覆盖角色提示词。

## 2) 查询状态（status）

```bash
alex team status --json
alex team status --all --tail 50 --json
alex team status --session-id sess_manual_001 --json
alex team status --team-id team-executor --json
alex team status --runtime-root .elephant/tasks/_team_runtime --json
```

参数（`alex team status`）：
- `--runtime-root`: 显式 team runtime 根目录
- `--session-id`: 按 session 过滤
- `--team-id`: 按 team 过滤
- `--all`: 返回全部匹配（默认仅最新）
- `--tail`: 每个 team 返回的 events tail 条数（默认 20）
- `--json`: 建议开启，便于结构化消费

## 3) 注入输入（inject）

向 team role 对应 tmux pane 注入真实输入：

```bash
alex team inject --task-id analyst_a-1 --message "Continue with stricter evidence and cite files"
alex team inject --role-id analyst_a --message "Focus on replication lag scenarios"
alex team inject --session-id sess_manual_001 --team-id claude_analysis --role-id analyst_b --message "Add operational rollback plan"
```

参数（`alex team inject`）：
- `--runtime-root`: 显式 runtime 根目录
- `--session-id`: 过滤 session
- `--team-id`: 过滤 team
- `--role-id`: 目标角色
- `--task-id`: 由 task_id 自动推导 role（可替代 `--role-id`）
- `--message`: 注入内容（必填）

## 4) 终端可视化（terminal）

直观查看 team 打开的 tmux 终端（会话或角色 pane）：

```bash
alex team terminal --mode attach
alex team terminal --mode capture --lines 200
alex team terminal --task-id team-researcher --mode capture
```

参数（`alex team terminal`）：
- `--runtime-root`: 显式 runtime 根目录
- `--session-id`: 过滤 session
- `--team-id`: 过滤 team
- `--role-id`: 指定角色 pane
- `--task-id`: 由 task_id 自动推导 role（可替代 `--role-id`）
- `--mode`: `attach|capture|stream`（默认 `stream`）
- `--lines`: 抓取/展示行数窗口（默认 120）

## 产物语义

- Team 运行时产物通常位于：
  - `.elephant/tasks/_team_runtime`
  - `.worktrees/**/_team_runtime`
- run 命令成功后，应继续使用 `status` 读取角色状态、最近事件、路径产物。
