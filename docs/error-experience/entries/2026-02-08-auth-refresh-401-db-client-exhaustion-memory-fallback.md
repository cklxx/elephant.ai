# 2026-02-08 - /api/auth/refresh 401（Auth DB 连接耗尽触发内存降级）

## Error
- 本地环境 `POST /api/auth/refresh` 间歇性返回 `401 Unauthorized`。
- 表现为重启后端（或前端联动重启后端）后必须重新登录。

## Impact
- 登录状态无法稳定跨重启保留。
- 会话持久性行为与预期不一致，影响开发联调效率。

## Root Cause
- 多个 `alex-server` 进程并存时，auth Postgres 连接被耗尽，日志出现 `too many clients already`。
- `BuildAuthService` 在 development 环境会回退到 memory stores；refresh session 因为是内存态，重启后丢失并导致 `401`。

## Remediation
- 运行时清理残留 `alex-server` 进程，恢复 auth DB 可用。
- 代码层新增 auth DB 池上限控制（`auth.database_pool_max_conns` / `AUTH_DATABASE_POOL_MAX_CONNS`），并设置默认 `4`，降低单进程连接占用导致的级联降级风险。

## Status
- fixed
