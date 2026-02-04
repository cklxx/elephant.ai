# Plan: Lark Proactive UX Fixes (Injection ACK + Group Support + Final Review Visibility)

Owner: cklxx (requested), implemented by Codex
Date: 2026-02-04
Branch: `eli/lark-proactive-fixes`

## Summary
Fix “看起来没实现”的 Lark 主动性体验问题，聚焦三件事：
1) 任务执行中收到新消息走“插入”路径时立刻做可见 ACK（reaction）。
2) 群聊（含话题群 `topic_group`）消息能被正确识别/执行，且 task 结束后的 drain/reprocess 不丢群聊消息。
3) Final Answer Review 的额外迭代在 Lark 侧可见（reaction），并补齐权限/订阅差异文档，避免“功能没生效”的误判。

## Checklist
- [x] Read current gateway/runtime implementation and reproduce with tests
- [x] Fix chat_type classification (`group` + `topic_group`)
- [x] Fix drain/reprocess to preserve chat_type for group chats
- [x] Add/adjust tests for group + drain/reprocess
- [x] Update docs: required Lark permissions for reactions + group delivery notes
- [ ] Run `go test ./...` and repo lint, then merge back to `main`

## Acceptance
- 在群聊/话题群中，消息不被错误当成 direct 丢弃，且 `isGroup` 标记正确（用于上下文/自动拉取群历史等）。
- task 运行期间插入消息成功入队后，对该消息加 reaction（默认 `THINKING`）。
- task 结束 drain/reprocess 的消息保留原 chat_type，群聊消息不会因为 `p2p` 伪造而被过滤。
- Final Answer Review 触发时对原消息加 reaction（默认 `GLANCE`）；若权限不足，文档可指导快速定位原因。
