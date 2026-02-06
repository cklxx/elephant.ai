# Plan: 修复 Lark 卡片回调 verification token 配置失效

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- 修复 Lark 卡片点击回调因 `verification token` 缺失/未展开导致不可用的问题。
- 保证按文档写法（`${LARK_VERIFICATION_TOKEN}`）可正确生效。
- 增加环境变量兜底，避免 YAML 未配置时回调彻底失效。

## Non-goals
- 不变更 Lark 事件签名/加密协议。
- 不改动 `/model use` 注入业务逻辑。

## Approach
1. 在 `internal/shared/config/file_loader.go` 增加 `channels.lark` 的环境变量展开逻辑。
2. 在 `internal/delivery/server/bootstrap/config.go` 增加 callback token/encrypt key 的环境变量兜底加载。
3. 在 `internal/delivery/channels/lark/card_callback.go` 调整缺 token 时的行为：不直接禁用回调路由，保留回调处理并记录警告。
4. 补充回归测试覆盖上述路径。
5. 运行全量 lint + test，更新文档与计划进度。

## Milestones
- [x] 测试先行（TDD）
- [x] 配置展开与环境兜底实现
- [x] 回调处理器行为修复
- [x] lint + 全量测试
- [x] 文档更新

## Progress Log
- 2026-02-06 16:57: 已确认根因：日志显示 `Lark card callback disabled: verification token missing`；且 `channels.lark` 未参与 `${ENV}` 展开。
- 2026-02-06 17:00: 新增失败测试覆盖：`channels.lark` 环境变量展开、Lark callback token 环境兜底、缺 token 时 callback handler 可用性。
- 2026-02-06 17:01: 完成实现：补齐 `channels.lark` env 展开；新增 callback token/encrypt key 环境变量兜底；缺 token 不再禁用回调路由。
- 2026-02-06 17:03: 完成回归：`./dev.sh lint` 与 `./dev.sh test` 全量通过。
