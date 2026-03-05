---
name: task-delegation
description: 跨 Agent 任务委派 — 将子任务分发给 Codex/Claude/Gemini CLI 执行。
triggers:
  intent_patterns:
    - "delegate|委派|分发|子任务|subtask|dispatch|后台执行"
  context_signals:
    keywords: ["delegate", "委派", "codex", "claude", "dispatch"]
  confidence_threshold: 0.7
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 120
---

# task-delegation

跨 Agent 任务分发：将子任务委派给外部 CLI agent 执行。

## 推荐方式（CLI 编排）

优先使用 `alex team` 命令（可通过 `shell_exec` 调用）：

```bash
alex team run --file tasks/review.yaml --wait
alex team run --template execute_and_report --goal "review auth module" --wait --mode auto
alex team templates

alex team reply --task-id task-123 --request-id req-456 --decision approve
alex team inject --task-id task-123 --message "continue"
```

### 参数要点
- `alex team run`
  - `--file` 与 `--template` 二选一。
  - `--template` 模式下需提供 `--goal`。
  - 常用：`--wait`、`--wait-timeout-seconds`、`--mode auto|team|swarm`、`--only-task`、`--role-prompt role=text`。
- `alex team templates`
  - 列出可用模板；不再使用 `--template list` 哨兵参数。
- `alex team reply`
  - 仅用于请求回复，必须提供 `--request-id`。
  - 审批用 `--decision approve|reject`；也可附加 `--option-id` 与 `--message`。
- `alex team inject`
  - 仅用于向运行中的任务注入自由文本，需 `--task-id` + `--message`。

## 兼容方式（skill 脚本）

```bash
python3 skills/task-delegation/run.py '{"action":"dispatch","agent":"codex","task":"fix the bug in main.go"}'
python3 skills/task-delegation/run.py '{"action":"list"}'
```
