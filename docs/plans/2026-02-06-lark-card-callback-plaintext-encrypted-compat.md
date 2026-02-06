# Plan: Lark 卡片回调明文/加密兼容修复

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- 修复 Lark 卡片点击回调在配置了 `card_callback_encrypt_key` 后仍可能报错的问题。
- 保证同一服务端在不修改业务逻辑的情况下，同时兼容明文回调与加密回调。

## Non-goals
- 不修改 Lark 卡片业务动作映射（`/model use`、审批/确认等）。
- 不改动网关消息分发、会话注入流程。

## Approach
1. 为 card callback handler 维护两套 dispatcher：
   - `verificationToken + encryptKey`（处理加密回调）
   - `verificationToken + ""`（处理明文回调）
2. `ServeHTTP` 读取请求体后，按 payload 是否包含非空顶层 `encrypt` 字段选择 dispatcher。
3. 保持现有 header/status/body 回写逻辑不变，避免行为回归。
4. 测试先行（TDD）：新增覆盖“配置 encrypt key 但收到明文 url_verification”的场景。

## Milestones
- [x] 新增失败测试（明文 + 已配置 encrypt key）
- [x] 实现 dispatcher 路由逻辑
- [x] 补充/更新计划进度记录
- [x] 通过 lint + 全量测试

## Progress Log
- 2026-02-06 17: 已确认问题根因：SDK 在配置 encrypt key 时会强制要求请求体包含 `encrypt`，导致明文事件直接报错。
- 2026-02-06 17: 建立修复计划，准备先补测试复现，再实现双 dispatcher 兼容路径。
- 2026-02-06 18: 新增测试覆盖“明文 url_verification + encrypt key 已配置”并先失败，随后实现双 dispatcher 自动路由通过测试。
- 2026-02-06 18: 完成 `./dev.sh lint` 与 `./dev.sh test` 全量回归，修复范围内无新增失败。
