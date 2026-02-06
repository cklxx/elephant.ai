# Plan: Lark 卡片回调加密缺签名头兼容

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- 修复 Lark 卡片回调在「加密 payload 但缺少签名头」场景下的点击报错。
- 保持现有明文回调、加密+签名回调能力不回退。

## Non-goals
- 不修改卡片动作语义映射与注入流程。
- 不改动全局 Lark 事件订阅逻辑，仅限 `/api/lark/card/callback`。

## Approach
1. 在 card callback handler 中保留现有 dispatcher：
   - 明文 dispatcher
   - 加密+验签 dispatcher
2. 新增加密跳过验签 dispatcher（仅用于缺签名头时的 fallback）。
3. 路由策略：
   - 明文 payload -> 明文 dispatcher
   - 加密 payload 且存在签名头 -> 加密+验签 dispatcher
   - 加密 payload 且缺签名头 -> 加密跳过验签 dispatcher
4. TDD：新增回调测试，覆盖“加密 event_callback 无签名头也可正常返回”。

## Milestones
- [x] 增加失败测试（加密无签名头）
- [x] 实现 fallback dispatcher 路由
- [x] 本包测试通过
- [x] lint + 全量测试通过

## Progress Log
- 2026-02-06 17:46: 本地 probe 证实当前进程明文/加密+签名回调可通过；仍存在“加密无签名头会 500”的潜在路径。
- 2026-02-06 17:48: 新增 `TestCardCallbackHandlerEncryptedEventWithoutSignatureHeaders` 并复现失败（500 signature verification failed）。
- 2026-02-06 17:50: 完成 fallback 修复：加密回调缺签名头时自动改用 skip-sign dispatcher；目标测试和本包测试通过。
- 2026-02-06 17:52: 完成 `./dev.sh lint` 与 `./dev.sh test` 全量回归。
