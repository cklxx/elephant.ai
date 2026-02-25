# 2026-02-06 - Lark 卡片回调因 verification token 缺失被整体禁用

## Error
- 线上日志出现 `Lark card callback disabled: verification token missing`，导致 `/api/lark/card/callback` 路由未注册，卡片点击直接报错。

## Impact
- `/model` 等交互卡片可以展示，但按钮点击无法生效。
- 用户侧感知为“点击卡片报错/无响应”。

## Root Cause
- `NewCardCallbackHandler` 在 `card_callback_verification_token` 为空时直接返回 `nil`，导致回调链路完全关闭。
- 同时 `channels.lark` 配置此前未参与 `${ENV}` 展开；按文档写法 `card_callback_verification_token: "${LARK_VERIFICATION_TOKEN}"` 无法可靠生效。

## Remediation
- 为 `channels.lark` 补齐环境变量展开（包含 callback token/encrypt key）。
- 在 server bootstrap 增加 callback token/encrypt key 环境变量兜底加载。
- 缺 token 时不再直接禁用回调路由，保留 action 回调处理并告警 challenge 风险。
- 补充回归测试覆盖配置展开、env 兜底与 handler 可用性。

## Status
- fixed
