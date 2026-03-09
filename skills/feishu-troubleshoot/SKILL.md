---
name: feishu-troubleshoot
description: |
  飞书插件问题排查工具。包含常见问题 FAQ 和授权诊断。

  常见问题可随时查阅。用于排查复杂问题（多次授权仍失败、权限不足等）。
triggers:
  intent_patterns:
    - "飞书问题|feishu error|授权失败|权限不足"
    - "feishu troubleshoot|飞书排查|飞书诊断"
  context_signals:
    keywords: ["troubleshoot", "排查", "诊断", "权限不足", "授权失败"]
  confidence_threshold: 0.5
priority: 8
requires_tools: [bash]
max_tokens: 300
cooldown: 15
---

# 飞书问题排查

## 常见问题（FAQ）

### 授权相关

**Token 过期**
- 现象：API 调用返回 `99991668` 或 `99991672`
- 解决：重新执行 OAuth 授权流程

**权限不足**
- 现象：API 返回 `99991400` 或 `99991403`
- 解决：检查应用是否开通了所需的 API 权限，参考下方权限对照表

**Refresh Token 过期**
- 现象：刷新 token 时返回错误
- 解决：需要重新走完整 OAuth 流程

### 权限对照表

| 功能模块 | 所需权限 scope |
|---------|---------------|
| 日历读写 | `calendar:calendar`, `calendar:calendar:readonly` |
| 文档读写 | `docx:document`, `docx:document:readonly` |
| 多维表格 | `bitable:app`, `bitable:app:readonly` |
| 任务管理 | `task:task`, `task:task:readonly` |
| 通讯录 | `contact:user.base:readonly`, `contact:department.base:readonly` |
| 消息读写 | `im:message`, `im:message:readonly` |
| 云盘 | `drive:drive`, `drive:drive:readonly` |
| Wiki | `wiki:wiki`, `wiki:wiki:readonly` |
| 邮件 | `mail:mailgroup`, `mail:mailgroup:readonly` |

### API 调用问题

**频率限制**
- 飞书 API 有频率限制，通常 100 次/秒
- 批量操作建议串行 + 延迟 0.5-1 秒

**字段值格式错误**
- 参考 `skills/feishu-bitable/references/record-values.md`

## 诊断命令

```bash
# 查看授权状态
python3 skills/feishu-cli/run.py '{"action":"auth","subcommand":"status"}'

# 查看可用模块
python3 skills/feishu-cli/run.py '{"action":"help","topic":"modules"}'
```
