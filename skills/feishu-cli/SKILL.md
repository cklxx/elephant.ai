---
name: feishu-cli
description: 统一飞书 CLI（auth/tool/api/help），支持渐进式帮助，供 agent 本地直接调用。
triggers:
  intent_patterns:
    - "feishu cli|飞书cli|飞书工具|lark api|飞书授权|oauth"
    - "日历|日程|calendar|会议|meeting|视频会议"
    - "通讯录|contact|用户查询|部门"
    - "云文档|document|docx|创建文档|读取文档"
    - "云盘|drive|文件管理|文件夹"
    - "邮件|email|mail|邮件组"
    - "多维表格|bitable|表格记录"
    - "OKR|目标管理|okr"
    - "电子表格|spreadsheet|sheets"
    - "知识库|wiki|知识空间"
  context_signals:
    keywords: ["feishu", "lark", "oauth", "calendar", "docx", "wiki", "drive", "bitable", "contact", "mail", "okr", "sheets", "meeting", "spreadsheet"]
  confidence_threshold: 0.4
priority: 8
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# feishu-cli

统一的飞书 CLI skill。通过一个运行时处理：
- `help` 渐进式帮助（适配 LLM 自发现）
- `auth` 授权与 token 管理
- `tool` 业务动作（calendar/contact/doc/wiki/drive/sheets/mail/meeting/okr/bitable）
- `api` 原始 Open API 调用

## 渐进式 help（推荐顺序）

```bash
# 1) 顶层命令总览
python3 skills/feishu-cli/run.py '{"action":"help"}'

# 2) 认证帮助
python3 skills/feishu-cli/run.py '{"action":"help","topic":"auth"}'

# 3) 查看全部模块
python3 skills/feishu-cli/run.py '{"action":"help","topic":"modules"}'

# 4) 查看某模块动作
python3 skills/feishu-cli/run.py '{"action":"help","topic":"module","module":"calendar"}'

# 5) 查看某动作参数与示例
python3 skills/feishu-cli/run.py '{"action":"help","topic":"action","module":"calendar","action_name":"create"}'
```

## auth 示例

```bash
# 查看授权状态
python3 skills/feishu-cli/run.py '{"action":"auth","subcommand":"status"}'

# 获取租户 token（复用现有 lark_auth）
python3 skills/feishu-cli/run.py '{"action":"auth","subcommand":"tenant_token"}'

# 生成 OAuth URL（先做 redirect allowlist 预检）
python3 skills/feishu-cli/run.py '{"action":"auth","subcommand":"oauth_url","redirect_uri":"https://example.com/callback","scopes":["contact:user.base:readonly"]}'
```

## tool 示例

```bash
# 创建日程
python3 skills/feishu-cli/run.py '{"action":"tool","module":"calendar","tool_action":"create","title":"周会","start":"2026-03-06 10:00","duration":"60m"}'

# 读取文档
python3 skills/feishu-cli/run.py '{"action":"tool","module":"doc","tool_action":"read","document_id":"doccnxxxx"}'

# 查询通讯录用户
python3 skills/feishu-cli/run.py '{"action":"tool","module":"contact","tool_action":"get_user","user_id":"ou_xxx"}'
```

## api 示例

```bash
# 原始 Open API 调用（tenant）
python3 skills/feishu-cli/run.py '{"action":"api","method":"GET","path":"/contact/v3/scopes"}'
```

## 各模块动作速查

| module | 支持的 tool_action |
|--------|-------------------|
| calendar | create, query, update, delete, list_calendars, freebusy |
| contact | get_user, search_user, list_departments |
| doc | create, read, read_content, write_markdown, list_blocks, update_block_text |
| wiki | list_spaces, list_nodes, create_node, get_node |
| drive | list_files, upload, download |
| **sheets** | **create, get, list_sheets** |
| mail | list_mailgroups, list_members |
| meeting | create, list, get, update, delete |
| **okr** | **list_periods, batch_get, list_user_okrs** |
| bitable | list_tables, list_fields, list_records, create_record, update_record, delete_record |
| message | send_message, history, upload_file |
| task | list, create, update, delete, complete, uncomplete, list_subtasks, create_subtask |

> **别名兼容**：旧名 `create_spreadsheet`→`create`，`get_spreadsheet`→`get`，`list_okr_periods`→`list_periods`，`batch_get_okrs`→`batch_get` 均已配置别名，可正常调用但文档以新名为准。

## 统一环境变量（建议）

- `LARK_APP_ID`
- `LARK_APP_SECRET`
- `LARK_TENANT_TOKEN`（可选，优先直用）
- `LARK_OPEN_BASE_URL`（可选）
- `LARK_OAUTH_REDIRECT_URI`（OAuth）
- `LARK_OAUTH_REDIRECT_ALLOWLIST`（OAuth 预检）
- `LARK_OAUTH_SCOPES`（OAuth 默认 scope）
