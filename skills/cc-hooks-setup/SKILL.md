---
name: cc-hooks-setup
description: 自动配置 Claude Code hooks，将操作动向同步到 Lark 通知群。
triggers:
  intent_patterns:
    - "(?i)配置.*claude.*code.*hook"
    - "(?i)cc.*hook.*setup"
    - "(?i)claude.*hook.*配置"
  context_signals:
    keywords: ["hooks", "claude code", "notify_lark", "settings.local"]
  confidence_threshold: 0.7
priority: 8
requires_tools: [bash]
max_tokens: 200
cooldown: 60
---

# cc-hooks-setup

配置 Claude Code hooks，将操作动向自动同步到 Lark 通知群。

## 调用

```bash
python3 skills/cc-hooks-setup/run.py '{"action":"setup","server_url":"...","token":"..."}'
```

参数说明：
- `action`（必填）：`setup`（配置 hooks）或 `remove`（移除 hooks）
- `server_url`（setup 必填）：elephant.ai 服务地址，如 `http://localhost:8080`
- `token`（可选）：hooks bridge 认证 token
- `project_dir`（可选）：项目根目录，默认为当前工作目录

成功后返回配置文件路径和确认消息。
