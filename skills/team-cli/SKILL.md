---
name: team-cli
description: Team runtime CLI 技能：读取 team 运行时产物与状态，供 LLM 直接调用 `alex team status`。
triggers:
  intent_patterns:
    - "team status|team runtime|agent team|团队运行态|tmux pane|_team_runtime"
  context_signals:
    keywords: ["team", "runtime", "status", "tmux", "_team_runtime", "team_id", "session_id"]
  confidence_threshold: 0.55
priority: 8
requires_tools: [bash]
max_tokens: 240
cooldown: 15
capabilities: [team_runtime_status]
governance_level: medium
activation_mode: auto
---

# team-cli

用于查询 Team Runtime 的真实运行状态与产物，不依赖 Lark channel。

## 调用

```bash
alex team status --json
alex team status --all --tail 50 --json
alex team status --runtime-root .elephant/tasks/_team_runtime --session-id sess-123 --json
alex team status --team-id team-executor --json

# 若本机未安装 alex 二进制，直接在仓库内运行
go run ./cmd/alex team status --json
```

## 参数（映射到 `alex team status`）
| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| --runtime-root | string | 否 | 显式 team runtime 根目录 |
| --session-id | string | 否 | 按 session 过滤 |
| --team-id | string | 否 | 按 team 过滤 |
| --all | bool | 否 | 返回全部匹配（默认仅最新） |
| --tail | int | 否 | 每个 team 返回的 events tail 条数，默认 20 |
| --json | bool | 建议 | 返回结构化 JSON 结果 |

## 产物语义

- 直接读取 `team` CLI 的真实输出（优先 `alex`，否则 `go run ./cmd/alex`）。
- Team 运行时产物通常位于：
  - `.elephant/tasks/_team_runtime`
  - `.worktrees/**/_team_runtime`
