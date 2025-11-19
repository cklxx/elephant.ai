# 本地认证数据库指南

为了在开发环境中调试登录/鉴权功能，需要引入可持久化的认证数据库。本指南介绍当前问题、推荐的拓扑结构、Postgres 初始化脚本，以及如何使用新的 Docker Compose 服务启动环境，最后还列出了后续落地任务清单。

## 背景与现状问题

目前 `cmd/alex-server` 仍然默认使用内存版 `authAdapters.NewMemoryStores()`。这会导致以下限制：

- 账户、刷新令牌在进程重启后立即丢失，无法复现登录相关缺陷。
- 无法支撑多实例部署，每个副本都会维护自己的内存状态。
- 缺乏统一的认证入口：服务只有在设置 `AUTH_JWT_SECRET` 时才会注册 `/api/auth/*` 路由，而 `.env`、`docker-compose.dev.yml` 默认都未配置。
- HTTP 路由缺少 `Authorization` 校验，中后台接口在登录成功后仍未受保护。
- Web 前端没有 `/login` 页面，也没有把令牌注入到请求上下文中，用户即便拿到了账号也没有入口登录。

因此需要一个持久化的认证数据库来支撑后续开发与调试。

## 推荐拓扑

```
┌──────────────┐     ┌─────────────────────────┐
│ alex-web(dev)├────▶│ alex-server (Go backend) │
└──────────────┘     └──────────────┬──────────┘
                                   │  AUTH_DATABASE_URL
                             ┌─────▼─────┐
                             │ Postgres  │
                             │ (auth-db) │
                             └───────────┘
```

- `alex-server` 通过 `AUTH_DATABASE_URL` 访问 `auth-db` 服务。
- `alex-web` 在登录后将访问令牌注入后续请求。
- Postgres 使用持久化卷保存用户、身份和会话数据。

## Postgres 初始化

当首次启动 Postgres 时，可以使用仓库内提供的迁移脚本初始化数据库结构：

```bash
psql "$AUTH_DATABASE_URL" -f migrations/auth/001_init.sql
```

脚本将创建以下对象：

- `pgcrypto` 与 `citext` 扩展，用于生成 UUID 和大小写不敏感的邮箱字段。
- `auth_users`：存储本地账号（邮箱、密码哈希）。
- `auth_user_identities`：外部身份提供方（如 OAuth）的映射表。
- `auth_sessions`：刷新令牌哈希、指纹（SHA-256）及设备信息，用于跨实例校验。
- `auth_states`：保存 OAuth state 随机串，保证多实例场景的回调校验。

## docker-compose.dev.yml 配置

开发 Compose 文件增加了 `auth-db` 服务，并让 `alex-server` 在启动前等待数据库健康：

```yaml
  alex-server:
    environment:
      - AUTH_JWT_SECRET=${AUTH_JWT_SECRET:-dev-secret-change-me}
      - AUTH_ACCESS_TOKEN_TTL_MINUTES=${AUTH_ACCESS_TOKEN_TTL_MINUTES:-15}
      - AUTH_REFRESH_TOKEN_TTL_DAYS=${AUTH_REFRESH_TOKEN_TTL_DAYS:-30}
      - AUTH_REDIRECT_BASE_URL=${AUTH_REDIRECT_BASE_URL:-http://localhost:8080}
      - AUTH_DATABASE_URL=${AUTH_DATABASE_URL:-postgres://alex:alex@auth-db:5432/alex_auth?sslmode=disable}
    depends_on:
      auth-db:
        condition: service_healthy

  auth-db:
    image: postgres:15
    environment:
      - POSTGRES_USER=${AUTH_DB_USER:-alex}
      - POSTGRES_PASSWORD=${AUTH_DB_PASSWORD:-alex}
      - POSTGRES_DB=${AUTH_DB_NAME:-alex_auth}
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U ${AUTH_DB_USER:-alex} -d ${AUTH_DB_NAME:-alex_auth}"]
```

通过 `.env` 文件即可覆盖默认连接信息，例如：

```
AUTH_JWT_SECRET=please-change-me
AUTH_DATABASE_URL=postgres://alex:alex@localhost:5432/alex_auth?sslmode=disable
AUTH_DB_PASSWORD=super-secret
AUTH_DB_IMAGE=postgres:15
```

如需在国内网络环境下加速镜像拉取，可将 `AUTH_DB_IMAGE` 改为镜像仓库地址，例如：

```
AUTH_DB_IMAGE=docker.m.daocloud.io/library/postgres:15
```

Compose 会直接使用该镜像，避免从 `docker.io` 拉取失败。

> 快捷方式：执行 `./scripts/setup-china-mirrors-all.sh` 会自动把 `.env` 里的 `AUTH_DB_IMAGE` 切换到上述国内镜像。

## 启动步骤

1. 复制 `.env.example` 为 `.env` 并按需修改认证相关变量。
2. 执行 `docker-compose -f docker-compose.dev.yml up --build`。
3. 待 `auth-db` 通过健康检查后，`alex-server` 会自动连接数据库。
4. 运行 `psql "$AUTH_DATABASE_URL" -f migrations/auth/001_init.sql` 初始化表结构。
5. `cmd/alex-server` 检测到 `AUTH_DATABASE_URL` 时会自动启用 Postgres 仓储与 JWT 鉴权中间件。
6. 通过下文的 SQL 示例写入至少一个可用账号，以便 Web 登录。

## 创建测试账号

### 通过环境变量自动注入

若只是想本地跑通登录流程，可直接在 `.env` 中提供启动时自动注入的账号信息（无论是内存仓储还是 Postgres 都生效）：

```
AUTH_BOOTSTRAP_EMAIL=admin@example.com
AUTH_BOOTSTRAP_PASSWORD=P@ssw0rd!
AUTH_BOOTSTRAP_DISPLAY_NAME=Admin
```

重启 `alex-server` 后会自动调用注册逻辑，若该邮箱尚不存在就会创建账号；已存在则跳过。这样即使暂时没有 `psql` 或 Postgres，也能快速拿到一个本地可用的用户。

### 使用 CLI 脚本（推荐）

仓库现在提供了 `cmd/auth-user-seed`，可以根据 `.env` 中的 `AUTH_DATABASE_URL` 自动向 Postgres 插入（或更新）一个本地账号：

```bash
# 默认会创建 admin@example.com / P@ssw0rd!
go run ./cmd/auth-user-seed
```

也可以通过参数覆盖邮箱、密码、显示名或订阅档位：

```bash
go run ./cmd/auth-user-seed \
  -email dev@example.com \
  -password 'MyStrongerP@ss' \
  -name 'Dev User' \
  -tier supporter \
  -points 100
```

该脚本会在 `auth_users` 表中执行 `UPSERT`，因此重复运行会直接更新已存在的账号，方便快速重置密码或切换订阅状态。

### 手动创建测试账号

如果暂时无法运行上述脚本，也可以直接使用 `psql` 向 `auth_users` 表写入测试账号。首先用下列 Argon2id 哈希（对应明文 `P@ssw0rd!`）作为密码：

```
argon2id$1$65536$4$X/2c361Hs7Z7BTh06+aZaQ$FN9oVAe9UTRi7adCznuGy7sQrKYhanWBDhVG3en+HV4
```

然后执行以下 SQL，将账号插入数据库：

```sql
INSERT INTO auth_users (id, email, display_name, status, password_hash, points_balance, subscription_tier, subscription_expires_at, created_at, updated_at)
VALUES (
  gen_random_uuid(),
  'admin@example.com',
  'Admin',
  'active',
  'argon2id$1$65536$4$X/2c361Hs7Z7BTh06+aZaQ$FN9oVAe9UTRi7adCznuGy7sQrKYhanWBDhVG3en+HV4',
  0,
  'free',
  NULL,
  NOW(),
  NOW()
)
ON CONFLICT (email) DO UPDATE
SET display_name = EXCLUDED.display_name,
    status = EXCLUDED.status,
    password_hash = EXCLUDED.password_hash,
    points_balance = EXCLUDED.points_balance,
    subscription_tier = EXCLUDED.subscription_tier,
    subscription_expires_at = EXCLUDED.subscription_expires_at,
    updated_at = NOW();
```

### 积分与订阅字段说明

- `points_balance`：平台级积分余额，默认 0，可由后台或业务逻辑累加／扣减。
- `subscription_tier`：订阅档位，当前支持：
  - `free`：免费版。
  - `supporter`：每月 20 美元，解锁支持者额度。
  - `professional`：每月 100 美元，解锁专业版额度。
- `subscription_expires_at`：订阅到期时间，针对付费档必填；免费档保持 `NULL` 即可。

如需生成自定义哈希，可运行任意脚本调用 `internal/auth/crypto` 包或根据同样的参数（Argon2id、t=1、m=65536、p=4、32 字节输出、16 字节盐）生成匹配格式的字符串后再插入。

## 后续落地清单

- ✅ **前端入口**：在 `web/app` 中实现 `/login` 页面，并在全局布局中根据令牌控制导航与请求头注入。
- ✅ **多集群**：`cmd/alex-server` 会在启用数据库仓储时每分钟自动清理过期的 `auth_states` 记录，避免历史 state 堆积。

完成以上工作后，登录体验即可在全链路达到生产级要求。
