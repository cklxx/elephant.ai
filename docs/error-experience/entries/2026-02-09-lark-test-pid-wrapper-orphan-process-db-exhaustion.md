# 2026-02-09 - Lark test PID 记录到包装进程，导致孤儿进程堆积并打满 auth DB

## Error
- `alex dev lark` 长期 `degraded`，`test restart failed` 持续出现。
- `setup_local_auth_db.sh` 在 migration 阶段报 `FATAL: sorry, too many clients already`。

## Impact
- test/main 反复重启失败，Lark supervisor 无法恢复健康。
- auth DB 自动初始化失效，开发链路不稳定。

## Root Cause
- `scripts/lark/test.sh` 的启动语句把 PID 写成了 `bash ... test.sh restart` 包装进程，而不是实际 `alex-server lark` 子进程。
- stop/restart 只会杀掉包装进程，真实子进程残留为孤儿；长期累计后占满 Postgres 连接。

## Remediation
- 修正 test 启动 PID 捕获方式，确保写入真实后台 `alex-server` PID。
- 新增 `scripts/lark/cleanup_orphan_agents.sh`，按 main/test 作用域自动清理未受管 `alex-server lark` 进程。
- 在 `main.sh` / `test.sh` 启动与重启前自动触发孤儿清理。
- 在 `setup_local_auth_db.sh` 针对 `too many clients already` 增加清理 + 重试机制。

## Status
- fixed
