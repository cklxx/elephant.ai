---
name: anygen
description: 封装 AnyGenIO/anygen-skills 为统一 CLI（help/task），支持渐进式披露与任务执行（task-manager）。
triggers:
  intent_patterns:
    - "anygen|ppt|slides|生成文档|docx|storyboard|website|data analysis|smart_draw|diagram"
  context_signals:
    keywords: ["anygen", "slide", "ppt", "doc", "website", "data_analysis", "smart_draw", "storyboard"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 260
cooldown: 20
output:
  format: markdown
  artifacts: true
  artifact_type: file
---

# AnyGen (CLI + Progressive Skills)

将上游仓库 `https://github.com/AnyGenIO/anygen-skills` 封装成 elephant.ai 可直接调用的统一入口：

- `help`：渐进式披露（overview → modules → module → action）
- `task`：执行 `task-manager` 的动作（`create/status/poll/download/run`）

## 快速前置

- 需要环境变量：`ANYGEN_API_KEY=sk-xxx`
- 入口命令：`python3 skills/anygen/run.py '{...}'`

## 渐进式披露（推荐顺序）

```bash
# 1) 顶层总览
python3 skills/anygen/run.py '{"action":"help"}'

# 2) 模块清单
python3 skills/anygen/run.py '{"action":"help","topic":"modules"}'

# 3) 查看模块说明
python3 skills/anygen/run.py '{"action":"help","topic":"module","module":"task-manager"}'

# 4) 查看具体动作参数与示例
python3 skills/anygen/run.py '{"action":"help","topic":"action","module":"task-manager","action_name":"create"}'
```

## task-manager 执行动作

支持 operation：`chat|slide|doc|storybook|data_analysis|website|smart_draw`

```bash
# 创建任务
python3 skills/anygen/run.py '{"action":"task","task_action":"create","operation":"slide","prompt":"Q2 roadmap deck","style":"business"}'

# 查询状态（单次）
python3 skills/anygen/run.py '{"action":"task","task_action":"status","task_id":"task_xxx"}'

# 轮询直到结束（可自动下载）
python3 skills/anygen/run.py '{"action":"task","task_action":"poll","task_id":"task_xxx","output":"./output"}'

# 直接下载完成任务文件
python3 skills/anygen/run.py '{"action":"task","task_action":"download","task_id":"task_xxx","output":"./output"}'

# 一步式 create + poll (+可下载)
python3 skills/anygen/run.py '{"action":"task","task_action":"run","operation":"doc","prompt":"Technical design for notification service","output":"./output"}'
```

## 模块边界

- `task-manager`：本封装可执行。
- `finance-report`：当前提供引导说明（help 可见），不在此 skill 内直接执行。

## 兼容说明

- 输入支持 `command` 或 `action` 作为顶层命令名。
- 当 `action=create/status/poll/download/run` 时，会自动路由到 `task` 命令。
