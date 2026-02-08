# Plan: Auth Session Persistence Hardening via Auth DB Pool Cap

## Summary

`/api/auth/refresh` 在开发环境出现 `401` 的一类根因是 Auth DB 连接被多实例 `alex-server` 耗尽，认证模块在启动时降级到内存 store，导致重启后 refresh session 丢失。为降低该问题复发概率，给 auth Postgres 连接池增加可配置上限，并设置保守默认值。

## Scope

- `internal/shared/config/file_config.go`
- `internal/delivery/server/bootstrap/config.go`
- `internal/delivery/server/bootstrap/auth.go`
- `internal/delivery/server/bootstrap/config_test.go`
- `docs/reference/CONFIG.md`

Out of scope:

- 改变开发环境“DB 不可用时可降级内存 store”的既有策略
- 改造 lark/test worktree 生命周期管理

## Checklist

- [x] 创建计划并记录问题背景
- [x] 在 auth 配置中新增 `database_pool_max_conns`（YAML + env fallback）
- [x] 在 Auth 启动流程中应用连接池上限（含默认值）
- [x] 增加/更新测试覆盖配置回退行为
- [x] 更新配置文档
- [x] 运行相关测试并记录结果
- [ ] 分支提交并合并回 `main`

## Progress Notes

- 2026-02-08 20:10: 在 `AuthConfig` 中新增 `database_pool_max_conns`，并接入 `AUTH_DATABASE_POOL_MAX_CONNS` env fallback。
- 2026-02-08 20:10: `BuildAuthService` 改为 `pgxpool.ParseConfig` + `NewWithConfig`，默认 `max_conns=4`（可配置覆盖）。
- 2026-02-08 20:10: 新增配置测试：
  - `TestLoadConfig_AuthDatabasePoolMaxConnsFromEnvFallback`
  - `TestLoadConfig_AuthDatabasePoolMaxConnsYAMLOverridesEnvFallback`
- 2026-02-08 20:10: 验证结果：
  - `go test ./internal/delivery/server/bootstrap` ✅
  - `go test ./internal/delivery/server/bootstrap -run 'TestLoadConfig_AuthDatabasePoolMaxConns|TestBuildAuthService'` ✅
  - `./dev.sh lint` ❌（仓库既有 errcheck/staticcheck 问题，集中在 `internal/devops/*` 与 `cmd/alex/dev*.go`）
  - `./dev.sh test` ❌（仓库既有 `internal/shared/config` getenv guard + `internal/delivery/server/bootstrap` race 失败）

## Acceptance Criteria

- 当 `auth.database_url` 配置存在时，auth DB 连接池最大连接数可通过 `auth.database_pool_max_conns` 或 `AUTH_DATABASE_POOL_MAX_CONNS` 控制。
- 未显式配置时，auth DB 连接池使用保守默认上限，避免单进程占用过多连接。
- 现有配置加载与 auth 启动逻辑测试通过。
