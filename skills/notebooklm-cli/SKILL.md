---
name: notebooklm-cli
description: NotebookLM 独立 CLI skill：本地直接调 `nlm`，统一 `command/op`，返回结构化结果。
triggers:
  intent_patterns:
    - "notebooklm|notebook lm|nlm|音频概览|podcast|research notebook"
  context_signals:
    keywords: ["notebooklm", "nlm", "notebook", "source", "podcast", "report"]
  confidence_threshold: 0.6
priority: 7
requires_tools: [bash]
max_tokens: 200
cooldown: 20
---

# notebooklm-cli

独立本地运行，不依赖任何 channel 注册。
统一入口：

```bash
python3 skills/notebooklm-cli/run.py <command> [op] [--flag value ...]
```

## 渐进式 help（给 LLM 先学契约）

- `overview`：入口、环境变量、命令总览
- `schema`：全部命令契约（推荐机器读取）
- `<command>`：单命令契约
- `progressive`：overview + 各命令契约链路

```bash
python3 skills/notebooklm-cli/run.py help --topic overview
python3 skills/notebooklm-cli/run.py help --topic schema
python3 skills/notebooklm-cli/run.py help --topic progressive
```

如果需要底层原生命令帮助，可加：

```bash
python3 skills/notebooklm-cli/run.py help --topic source --include_cli true
```

## 输入契约（统一）

- `command`: `help | auth | notebook | source | query | report | studio | raw`
- `op`: 命令内操作；不填走默认 op（见 `help/schema`）
- 兼容旧别名：`action`, `*_action`，新代码统一用 `command/op`
- 统一返回字段：`success, command, exit_code, stdout, stderr, hints, error?`

## 命令总览（精简）

- `auth`: 登录与 profile 管理（`profile_delete` 强制 `confirm=true`）
- `notebook`: list/create/get/describe/rename/query/delete（delete 强制确认）
- `source`: list/add_*/get/describe/content/rename/delete（delete 强制确认）
- `query`: notebook query 快捷入口
- `report`: create（默认），强制确认，支持 format/prompt/language/source_ids
- `studio`: status/rename/delete（delete 强制确认）
- `raw`: 透传 argv；禁止 `nlm chat start`；delete 需 `confirm=true`

## 最小 E2E 示例

```bash
python3 skills/notebooklm-cli/run.py auth check
python3 skills/notebooklm-cli/run.py notebook create --title 'NLM E2E'
python3 skills/notebooklm-cli/run.py source add_url --notebook_id '<nb-id>' --url https://example.com/article
python3 skills/notebooklm-cli/run.py query --notebook_id '<nb-id>' --question '总结 3 个关键结论'
python3 skills/notebooklm-cli/run.py report --notebook_id '<nb-id>' --confirm true
python3 skills/notebooklm-cli/run.py studio status --notebook_id '<nb-id>'
```

## 规则

- 删除前必须显式确认：`confirm=true`。
- 禁止交互式 `nlm chat start`。
- 鉴权失败先执行 `auth/login` 再重试业务命令。
