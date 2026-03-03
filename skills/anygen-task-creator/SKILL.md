---
name: anygen-task-creator
description: "Generate content via AnyGen AI: slides (PPT), documents, storybooks, data analysis, websites, and chat."
triggers:
  intent_patterns:
    - "PPT|ppt|slide|幻灯片|演示文稿|presentation|文档生成|generate doc|storybook|故事板|data.?analysis|数据分析|生成网站|generate website|anygen"
  context_signals:
    keywords: ["PPT", "slide", "幻灯片", "演示文稿", "文档生成", "storybook", "故事板", "数据分析", "生成网站", "anygen"]
  confidence_threshold: 0.6
priority: 6
requires_tools: [bash]
max_tokens: 200
cooldown: 30
output:
  format: markdown
  artifacts: true
  artifact_type: file
---

# AnyGen Content Generator

Create AI generation tasks using AnyGen OpenAPI, supporting multiple content generation modes.

## Required Env

- `ANYGEN_API_KEY` (required) — AnyGen API Key (format: `sk-xxx`)

## Supported Operations

| Operation | Description | Downloadable |
|-----------|-------------|-------------|
| `slide` | PPT / Slides | Yes |
| `doc` | Document (DOCX/PDF) | Yes |
| `chat` | General AI conversation | No (URL only) |
| `storybook` | Storyboard creation | No (URL only) |
| `data_analysis` | Data analysis report | No (URL only) |
| `website` | Website generation | No (URL only) |

## Actions

### `run` (default) — Full workflow: create + poll + download

```bash
python3 skills/anygen-task-creator/run.py '{"action":"run","operation":"slide","prompt":"AI history presentation","style":"business formal","output":"./output/"}'
```

### `create` — Create a task only

```bash
python3 skills/anygen-task-creator/run.py '{"action":"create","operation":"slide","prompt":"AI history presentation"}'
```

### `poll` — Poll task status

```bash
python3 skills/anygen-task-creator/run.py '{"action":"poll","task_id":"task_abc123"}'
```

### `download` — Download completed file

```bash
python3 skills/anygen-task-creator/run.py '{"action":"download","task_id":"task_abc123","output":"./output/"}'
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| operation | slide / doc / chat / storybook / data_analysis / website | Yes |
| prompt | Content description | Yes |
| language | zh-CN (default) or en-US | No |
| slide_count | Number of PPT pages | No |
| template | PPT template style | No |
| ratio | 16:9 (default) or 4:3 | No |
| doc_format | docx (default) or pdf | No |
| style | Style preference (e.g., "business formal", "minimalist modern") | No |
| files | List of file paths for reference attachments (max 10MB each) | No |
| output | Output directory for download | No |

## Notes

- Maximum task execution time: 10 minutes
- Download link valid for 24 hours
- Single attachment max 10MB (Base64 encoded)
- Poll interval: 3 seconds

## Files

```
anygen-task-creator/
  SKILL.md
  run.py
  scripts/
    anygen.py
```
