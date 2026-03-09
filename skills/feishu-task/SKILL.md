---
name: feishu-task
description: |
  飞书任务管理工具。支持任务的创建、查询、更新、删除，子任务管理和任务清单。

  **当以下情况时使用此 Skill**：
  (1) 需要创建、查询、更新、完成任务
  (2) 需要管理子任务或任务清单
  (3) 用户提到"任务"、"待办"、"task"、"todo"
triggers:
  intent_patterns:
    - "任务|待办|task|todo"
    - "创建任务|完成任务|任务清单|子任务"
  context_signals:
    keywords: ["task", "任务", "待办", "todo", "subtask", "tasklist"]
  confidence_threshold: 0.5
priority: 9
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# Feishu Task Skill

## 快速索引

| 用户意图 | module | tool_action | 必填参数 |
|---------|--------|-------------|---------|
| 列出任务 | task | list | - |
| 创建任务 | task | create | summary |
| 更新任务 | task | update | task_id + 字段 |
| 删除任务 | task | delete | task_id |
| 列出子任务 | task | list_subtasks | task_id |
| 创建子任务 | task | create_subtask | task_id, summary |

## 调用示例

### 创建任务

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "task",
  "tool_action": "create",
  "summary": "完成 Q1 OKR 回顾",
  "due": "2026-03-15 18:00",
  "description": "回顾 Q1 各项 KR 的完成情况",
  "members": ["ou_xxx"]
}'
```

### 列出任务

```bash
python3 skills/feishu-cli/run.py '{"action":"tool","module":"task","tool_action":"list"}'
```

### 完成任务

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "task",
  "tool_action": "update",
  "task_id": "t_xxx",
  "status": "completed"
}'
```

### 创建子任务

```bash
python3 skills/feishu-cli/run.py '{
  "action": "tool",
  "module": "task",
  "tool_action": "create_subtask",
  "task_id": "t_xxx",
  "summary": "整理 KR1 数据"
}'
```

## 核心约束

- **due 格式**：支持 `"2026-03-15 18:00"` 或 ISO 8601
- **members**: 使用 `open_id`（`ou_` 开头），数组格式
- **任务状态**: `"needs_action"` (待完成), `"completed"` (已完成)
- **只能操作有权限的任务**：作为成员的任务
- **重复规则**: 只有设置了截止时间的任务才能设置重复规则
