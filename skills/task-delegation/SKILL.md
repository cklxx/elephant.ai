---
name: task-delegation
description: When a subtask should run on a different agent (Codex/Claude/Gemini) → delegate and track cross-agent execution.
triggers:
  intent_patterns:
    - "delegate|委派|分发|子任务|subtask|dispatch|后台执行"
    - "交给.*codex|send.*to.*codex|让.*claude.*做|ask.*claude"
    - "后台.*跑|run.*background|异步.*执行|async.*execute"
    - "分配.*给|assign.*to|外包.*出去|outsource"
    - "用.*另一个.*agent|use.*another.*agent|换.*模型.*做|switch.*model"
  context_signals:
    keywords: ["delegate", "委派", "codex", "claude", "dispatch", "后台", "异步", "分配", "agent", "gemini"]
  confidence_threshold: 0.65
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 120
---

# task-delegation

跨 Agent 任务分发：将子任务委派给外部 CLI agent 执行。

## 调用

```bash
python3 skills/task-delegation/run.py dispatch --agent codex --task 'fix the bug in main.go'
python3 skills/task-delegation/run.py list
```
