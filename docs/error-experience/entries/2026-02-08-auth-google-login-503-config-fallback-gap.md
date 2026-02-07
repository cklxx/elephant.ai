# 2026-02-08 - /api/auth/google/login 返回 503（auth 配置回退缺失）

## Error
- 访问 `GET /api/auth/google/login` 返回 `503 Service Unavailable`，响应为 `Authentication module not configured`。

## Impact
- OAuth 登录入口不可用。
- 依赖 auth middleware 的链路在本地环境表现为“全坏”。

## Root Cause
- server bootstrap 的 `AuthConfig` 仅读取 YAML，不读取环境变量回退；本地常用 `AUTH_JWT_SECRET` 无法生效。
- 当 `auth.database_url` 不可达时，`BuildAuthService` 直接返回错误并禁用整套 auth 路由。

## Remediation
- 在 `LoadConfig` 中增加 `applyAuthEnvFallback`，为 auth 字段补齐 env fallback。
- 在 `BuildAuthService` 中对 development/dev/internal/evaluation 环境启用安全降级：
  - 缺失 JWT secret 时使用开发默认 secret 并告警。
  - auth DB 不可达时回退到 memory stores 并告警，而不是关闭 auth 模块。
- 增加服务端 e2e 测试，覆盖 `/api/auth/google/login` 与 `/api/dev/logs/index` 的可达性。

## Status
- fixed
