# 用户登录系统设计
> Last updated: 2025-11-18


## 目标

- 支持三种登录方式：用户名/密码、Google 登录、微信登录。
- 提供统一的用户身份体系，便于服务端授权与审计。
- 将认证逻辑模块化，便于在 CLI、HTTP API、Web Dashboard 等不同交付面复用。
- 确保凭据安全存储、令牌安全传输，并满足最小权限原则。

## 架构概览

```
┌─────────────────────┐      ┌───────────────────────┐
│ Next.js Web 客户端  │◄────►│ internal/server/http  │
│ (app/, components/) │      │  REST + Callback      │
└─────────────────────┘      └─────────┬─────────────┘
                                        │
                                        ▼
                             ┌──────────────────────┐
                             │ internal/auth/app    │
                             │  (认证应用服务层)    │
                             ├──────────────────────┤
                             │ internal/auth/domain │
                             │  Users, Identity     │
                             ├──────────────────────┤
                             │ internal/auth/adapt  │
                             │  Google, WeChat, DB  │
                             └─────────┬────────────┘
                                        │
                     ┌──────────────────┴──────────────────┐
                     ▼                                     ▼
          internal/storage (用户数据)            external IdP API
```

- 新增 `internal/auth` 模块，包含 `domain`、`app`、`adapters`、`ports` 等子包，遵循现有分层结构。
- 服务端暴露 `/api/auth/*` REST 接口；OAuth 回调由同一模块处理。
- Next.js 前端通过新的 `auth` 服务调用 REST 接口，并将登录状态写入统一的 session store。

## 数据模型

### 数据表

| 表名 | 主要字段 | 说明 |
| --- | --- | --- |
| `users` | `id`, `email`, `display_name`, `password_hash`, `created_at`, `updated_at`, `status` | 基础用户资料；`password_hash` 采用 Argon2id；当使用第三方登录且无密码时置空。 |
| `user_identities` | `id`, `user_id`, `provider`, `provider_uid`, `access_token`, `refresh_token`, `expires_at`, `scopes` | 存储第三方身份映射；`access_token` 等敏感字段使用 KMS 或 envelope encryption 加密。 |
| `sessions` | `id`, `user_id`, `refresh_token_hash`, `user_agent`, `ip`, `expires_at`, `created_at` | 管理长生命周期刷新令牌；支持多设备登录与吊销。 |

### 领域模型

```go
// internal/auth/domain/user.go
type User struct {
    ID           string
    Email        string
    DisplayName  string
    Status       UserStatus
    PasswordHash string
}

type Identity struct {
    ID         string
    UserID     string
    Provider   ProviderType
    ProviderID string
    Tokens     OAuthTokens
}
```

## 认证流程

### 用户名/密码

1. 客户端提交 `POST /api/auth/login`，携带 `email`、`password`。
2. 服务端校验密码（Argon2id + constant time compare）。
3. 签发短期 `access_token`（JWT，15 分钟）与长期 `refresh_token`（存储于 `sessions` 表，30 天）。
4. 前端持久化 `refresh_token` 于 HttpOnly Secure Cookie；`access_token` 存储在内存（React Query cache）。
5. 访问受保护 API 时，通过 `Authorization: Bearer <access_token>` 头部。

### Google 登录（OAuth 2.0 + OpenID Connect）

1. 前端访问 `GET /api/auth/google/login`，服务器重定向至 Google 授权端点。
2. 用户同意后，Google 回调 `/api/auth/google/callback`，附带 `code` 与 `state`。
3. 服务端使用授权码交换 `id_token` 与 `access_token`。
4. 通过 `id_token` 校验用户邮箱、sub；若首次登录：
   - 创建 `users` 记录（若邮箱已存在则合并）；
   - 创建 `user_identities` 记录。
5. 生成本地访问令牌与刷新令牌，流程同用户名登录。
6. 刷新第三方 access token 采用后台任务（基于 existing scheduler）或在使用前 lazy-refresh。

### 微信登录（扫码 / 授权码）

- 国际版网页扫码流程（QR Connect）：
  1. 前端调用 `GET /api/auth/wechat/login`，获取带 state 的授权 URL，嵌入二维码。
  2. 用户在微信客户端扫码确认；微信回调 `/api/auth/wechat/callback`（需公网可访问）。
  3. 服务端使用 `code` 调用 `https://api.weixin.qq.com/sns/oauth2/access_token` 获取 `openid`、`unionid`（如有）和令牌。
  4. 使用 `openid/unionid` 查找或创建 `user_identities`，关联至 `users`。
  5. 返回本地 access/refresh token。
- 国内 H5 登录类似，但需在微信内嵌 WebView 中触发授权；前端在检测到 `MicroMessenger` UA 时走该分支。

## 令牌与会话管理

- **Access Token**：JWT，使用 `internal/security` 新增 `jwt` 包签发。载荷包含 `sub`, `email`, `roles`, `exp`，使用 `HS256` 或 `EdDSA`。
- **Refresh Token**：随机 256 bit 字符串，仅通过 HttpOnly cookie 下发。数据库存储 Argon2id 哈希以防泄漏。
- **会话吊销**：`POST /api/auth/logout` 将当前刷新令牌标记为失效，并从客户端清理 cookie。
- **中间件**：`internal/server/http/middleware.go` 新增 `AuthenticationMiddleware` 验证 access token 并注入 `context.Context`。

## 配置

| 环境变量 | 说明 |
| --- | --- |
| `AUTH_JWT_SECRET` / `AUTH_JWT_PRIVATE_KEY` | 签名密钥。 |
| `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET` | Google OAuth 应用配置。 |
| `WECHAT_APP_ID` / `WECHAT_APP_SECRET` | 微信开放平台配置。 |
| `AUTH_REDIRECT_BASE_URL` | 构造 OAuth 回调 URL。 |
| `AUTH_REFRESH_TOKEN_TTL_DAYS` | 刷新令牌有效期。 |

这些变量通过现有 `internal/config` 系统加载，并在 `internal/di` 中注入 `AuthConfig`。

## 前端集成

- 在 `web/app` 目录新增 `auth` 相关路由：
  - `/login` 页面展示三种登录入口。
  - 使用 React Query 调用 `/api/auth/login`。
  - 第三方登录按钮触发窗口跳转或二维码弹窗。
- 公共组件 `AuthProvider` 使用 React Context 存储用户信息与 access token，并在 token 过期时触发刷新。
- API 调用通过 `lib/apiClient.ts` 拦截器自动附带 access token，并处理 401 -> refresh -> retry。

## 安全与合规

- 所有认证端点强制 HTTPS；开发环境允许 HTTP 但需设置 `Secure=false` cookie。
- 使用 CSRF Token 保护用户名/密码登录与登出；OAuth 流程使用 `state` 验证。
- 登录失败次数超过阈值时调用 `internal/security` 的速率限制器，触发告警。
- 记录审计日志：`internal/observability` 新增事件类型 `auth.login`, `auth.logout`, `auth.identity.linked`。
- 定期轮换 JWT 密钥；通过版本号在 JWT header (`kid`) 标识。

## 迁移计划

1. **数据层**：在 `internal/storage/migrations` 添加三张表的迁移脚本，并为 `user_identities(provider, provider_uid)` 建唯一索引。
2. **后端**：实现 `internal/auth` 模块、HTTP 端点、中间件、DI wiring。
3. **前端**：实现登录页面、上下文 provider、API 拦截器。
4. **回归测试**：
   - 单元测试：`internal/auth` domain/service；`middleware` token 验证。
   - 集成测试：模拟 OAuth 回调，验证账号合并逻辑。
   - E2E：通过 Playwright 覆盖三种登录路径（第三方采用模拟授权服务器）。
5. **部署**：配置环境变量，确保回调 URL 指向公开域名；对外暴露 `/api/auth/*`。

## 风险与缓解

| 风险 | 缓解措施 |
| --- | --- |
| OAuth 回调 URL 与环境不一致 | 在配置中强制校验域名，利用 feature flags 在灰度环境验证。 |
| 微信登录需要公网可访问回调 | 使用反向代理或云函数暴露回调端点；开发环境采用模拟服务器。 |
| 多账号合并冲突 | 采用邮箱匹配 + 用户确认流程；必要时在前端展示“关联账号”弹窗。 |
| 令牌泄漏 | 使用 HttpOnly Cookie，启用 SameSite=Lax/None；数据库中仅存哈希。 |
| 第三方 API 不可用 | 提供降级路径：仅保留本地账号登录，并在 UI 提示。 |

## 后续扩展

- 引入 WebAuthn/FIDO2，提供无密码登录。

## 技术方案调研与校验

- **Google OAuth**：按照 [Google Identity Web server OAuth 流程](https://developers.google.com/identity/protocols/oauth2/web-server#httprest_1) 校验，设计中的授权码交换、`id_token` 验证与 `state` 防重放机制均与官方推荐一致。
- **微信扫码登录**：参考 [微信开放平台网站应用扫码登录文档](https://developers.weixin.qq.com/doc/oplatform/Website_App/WeChat_Login/Wechat_Login.html)，确认需要预先在开放平台配置授权回调域名，并在回调阶段使用 `appid`+`secret`+`code` 换取 `access_token` 与 `openid`，与当前流程匹配。
- **凭据安全性**：针对 `password_hash` 采用 Argon2id，符合 [OWASP Password Storage Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Password_Storage_Cheat_Sheet.html) 中“首选算法”建议；刷新令牌 HttpOnly Cookie、数据库哈希存储策略满足最小暴露面要求。
- **合规要求**：HTTPS 强制、CSRF 防护、审计日志等控制措施符合 [NIST 800-63B](https://pages.nist.gov/800-63-3/sp800-63b.html) 中对会话管理与凭据保护的关键条款，保障多因素扩展与风控上线的合规基础。
- 增加组织与角色管理，细化 `roles` 与权限模型。
- 统一 CLI 与 HTTP 的认证：CLI 通过 PAT（Personal Access Token）调用 `/api/auth/token` 获取 JWT。
- 集成设备指纹/风险评估，提高异常登录检测能力。
