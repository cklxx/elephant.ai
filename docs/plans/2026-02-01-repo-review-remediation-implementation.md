# 仓库 review 修复方案执行计划

Updated: 2026-02-01 16:00

## 目标
- 落地 `docs/plans/2026-02-01-repo-review-remediation.md` 的 Phase 1-5。
- 新增必要测试并保持兼容性。
- 完成全量 lint/test 与分步提交。

## 计划
- [x] Phase 1：统一外部 HTTP 响应上限（配置 + 工具/服务端）
- [x] Phase 2：AsyncEventHistoryStore 失败保留 + 重试
- [x] Phase 3：Scheduler 并发策略 + sessionID 生成 + 超时
- [x] Phase 4：EventBroadcaster 缺失 session 处理
- [x] Phase 5：SSE LRU / FileStore ctx / Attachment ctx
- [x] 测试：新增/更新单测，跑 `./dev.sh lint` 与 `./dev.sh test`
- [x] 提交：按 phase 拆分提交
