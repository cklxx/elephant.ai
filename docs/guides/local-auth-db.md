# 本地用户登录数据库方案

> 目的：为需要真实用户登录的本地开发环境提供一份“可以立刻落地”的后端数据库拓扑、建表 SQL、启动顺序与待办清单，同时记录当前登录系统存在的结构性问题，方便团队分工推进。

## 1. 现状问题

| 问题 | 影响 | 证据 |
| --- | --- | --- |
| 仅有内存仓库 | 账号 / 刷新令牌保存在 `cmd/alex-server/main.go:402-410` 中的 `authAdapters.NewMemoryStores()`，一旦重启服务或扩容多实例即全部丢失。 | `cmd/alex-server/main.go:365-452` |
| 默认未启用认证 | 需要 `AUTH_JWT_SECRET` 才会构建 `AuthHandler`，在 `.env.example`、`docker-compose.dev.yml` 中均未配置，开发者默认无法命中 `/api/auth/*`。 | `cmd/alex-server/main.go:333-379` |
| API 没有鉴权门控 | Router 从未检查 Authorization 头部；即使成功登录也无法保护 `/api/tasks`、`/api/sessions`。 | `internal/server/http/router.go:16-104` |
| Web 前端没有登录入口 | `web/app` 仅包含 `conversation/`、`sessions/` 两个页面，无 `/login`、无 token 注入逻辑。 | `web/app` 目录结构 |
| 数据模型只存在于设计文档 | `docs/design/AUTH_SYSTEM_DESIGN.md` 描述了 `users` / `user_identities` / `sessions` 表，但代码中没有任何数据库实现或迁移脚本。 | 文档与代码对照 |

> 结论：要让“用户登录”可在本地验证，必须先补齐持久化、配置、迁移与最小化的启动脚本，再逐步实现仓库适配层与前端入口。

## 2. 本地开发拓扑

```
┌────────────────────────────┐      ┌─────────────────────┐
│ docker-compose.dev.yml     │      │ 本地 CLI / cURL     │
│                            │      │ (注册 / 登录)       │
│  • alex-server (Go)        │◄────►│ 或 Postman          │
│  • web (Next.js)           │      └─────────────────────┘
│  • sandbox (tools)         │
│  • auth-db (Postgres 15)   │◄────┐
└──────────┬─────────────────┘     │ 5432/tcp
           │                        │
           ▼                        │
  auth-db-data (Docker volume) ─────┘
```

新增的 `auth-db` 容器负责三类数据：

1. `auth_users` – 账号、邮箱、密码哈希、状态。
2. `auth_user_identities` – Google / WeChat 等外部身份。
3. `auth_sessions` – 刷新令牌会话、UA、IP、过期时间。

数据库启动参数（可通过环境变量覆盖）：

| 变量 | 默认值 | 用途 |
| --- | --- | --- |
| `AUTH_DB_USER` | `alex` | Postgres 用户名 |
| `AUTH_DB_PASSWORD` | `alex` | 密码 |
| `AUTH_DB_NAME` | `alex_auth` | 数据库名 |
| `AUTH_DB_PORT` | `5432` | 暴露到宿主机的端口 |

Compose 会把这些变量注入容器，同时暴露 `AUTH_DATABASE_URL=postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable` 供 `alex-server` 使用。

## 3. 数据库建表脚本

`migrations/auth/001_init.sql` 定义了最小可用 Schema（Postgres）：

```sql
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS auth_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email CITEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    password_hash TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status IN ('active', 'disabled'))
);

CREATE TABLE IF NOT EXISTS auth_user_identities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    provider TEXT NOT NULL,
    provider_uid TEXT NOT NULL,
    access_token TEXT,
    refresh_token TEXT,
    expires_at TIMESTAMPTZ,
    scopes TEXT[],
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, provider_uid)
);

CREATE TABLE IF NOT EXISTS auth_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth_users(id) ON DELETE CASCADE,
    refresh_token_hash TEXT NOT NULL,
    user_agent TEXT,
    ip TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS idx_auth_sessions_user ON auth_sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_auth_sessions_expiry ON auth_sessions (expires_at);
```

未来可以在此目录继续追加迁移（002、003…），并在 CI/部署阶段统一执行。

## 4. 启动步骤

1. **准备环境变量**
   ```bash
   cp .env.example .env
   export AUTH_JWT_SECRET="dev-secret-change-me"
   export AUTH_DATABASE_URL="postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable"
   export AUTH_REDIRECT_BASE_URL="http://localhost:8080"
   ```
   > 备注：目前 `alex-server` 仅消费 `AUTH_JWT_SECRET`；`AUTH_DATABASE_URL` 先写入 `.env`，等仓库实现落地后直接复用。

2. **启动 Docker 服务**
   ```bash
   docker compose -f docker-compose.dev.yml up auth-db -d
   docker compose -f docker-compose.dev.yml up alex-server web
   ```

3. **执行迁移**
   ```bash
   psql postgres://alex:alex@localhost:5432/postgres \
     -c "CREATE DATABASE alex_auth OWNER alex;"
   psql postgres://alex:alex@localhost:5432/alex_auth \
     -f migrations/auth/001_init.sql
   ```

4. **手动创建测试账号（可选）**
   ```sql
   INSERT INTO auth_users (email, display_name, password_hash)
   VALUES (
     'dev@example.com',
     'Dev Admin',
     '$argon2id$v=19$m=65536,t=3,p=4$uGxm...'
   );
   ```
   Argon2 哈希可通过 `./alex hash-password <plain>`（待实现）或在线工具生成。

5. **调用 API 验证**
   ```bash
   curl -X POST http://localhost:8080/api/auth/login \
     -d '{"email":"dev@example.com","password":"secret"}' \
     -H "Content-Type: application/json"
   ```
   查看响应是否带回 `access_token`、`refresh_token`，并在 `auth_sessions` 表中出现记录。

## 5. 后续落地清单

| 模块 | 工作项 |
| --- | --- |
| Go 服务 | 实现 `ports.UserRepository/IdentityRepository/SessionRepository` 的 Postgres 适配层；在 `buildAuthService` 中读取 `AUTH_DATABASE_URL` 并注入对应实现。 |
| API 中间件 | 引入 `Authorization` 校验，将 `/api/tasks/*`、`/api/sessions/*` 挂在 JWT 校验之后。 |
| CLI/工具 | 提供 `alex auth seed-user` 或 `scripts/seed_auth_user.go` 以生成 Argon2 密码。 |
| Web 前端 | 新建 `/login` 页面、封装 `authClient`，在 Layout 中根据 token 状态决定显示内容。 |
| 文档 & CI | 将 `migrations/auth` 纳入 CI（例如 `scripts/db-migrate.sh`），确保 PR 自动验证迁移是否可执行。 |

完成以上步骤后，用户登录即可在本地与云端共享同一套数据模型，实现“登录后访问控制”的闭环。
