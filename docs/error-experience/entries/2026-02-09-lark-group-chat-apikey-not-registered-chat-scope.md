# 2026-02-09 - 群聊偶发 `apikey not registered`（chat 作用域实现与语义不一致）

## Error
- Lark 群聊链路仍会出现上游认证错误（例如 `apikey not registered`），单聊链路正常。

## Impact
- 群聊中模型选择命中不稳定，容易回落到未配置或不匹配凭证的 provider。
- 同一群内不同发言人触发时行为不一致，问题难以复现和定位。

## Root Cause
- `/model use --chat` 文案语义是“当前会话级”，实现却按 `chat_id + user_id` 存储，群聊换发言人即可能命不中。
- sender 身份提取仅读取 `open_id`，部分事件场景下为空，会进一步放大作用域匹配失败概率。

## Remediation
- `SelectionScope` 支持真正 chat 级 key（`channel + chat_id`），并保留 legacy `chat+user` 兼容读取。
- Lark 选择查询顺序改为：`chat` -> `legacy chat+user` -> `channel`。
- `--chat` 写入 chat 级作用域；清理时同时清理 legacy chat+user 作用域。
- sender 提取补充 `user_id` / `union_id` 回退。
- 补充测试覆盖 chat 级共享与 legacy 兼容、sender id 回退路径。

## Status
- fixed
