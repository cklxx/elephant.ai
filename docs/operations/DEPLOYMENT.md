# ALEX 部署指南
> Last updated: 2025-11-18


本文档提供了 ALEX SSE 服务和 Web 界面的完整部署指南。

## 目录

1. [本地开发部署](#本地开发部署)
2. [Docker Compose 生产部署](#docker-compose-生产部署)
3. [Kubernetes 集群部署](#kubernetes-集群部署)
4. [配置说明](#配置说明)
5. [监控与日志](#监控与日志)

---

## 本地开发部署

### 前置要求

- Go 1.23+
- Node.js 20+
- Docker (可选)
- Redis (可选)

### 快速启动

#### 方式 1: 原生运行

```bash
# 1. 启动后端服务
export OPENAI_API_KEY="sk-..."
make server-run

# 2. 启动前端 (新终端)
cd web
npm install
npm run dev

# 访问: http://localhost:3000
```

#### 方式 2: Docker Compose 开发模式（位于 `deploy/docker/`）

```bash
# 启动所有服务
export OPENAI_API_KEY="sk-..."
docker compose -f deploy/docker/docker-compose.dev.yml up

# 访问: http://localhost:3000
# API: http://localhost:8080
```

---

## Docker Compose 生产部署

### 1. 准备环境变量

创建 `.env` 文件：

```bash
# .env
OPENAI_API_KEY=sk-xxxxx
LLM_PROVIDER=openai
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL=gpt-4o-mini
# Optional: enable vision routing when images are attached
# LLM_VISION_MODEL=gpt-4o-mini
ALEX_VERBOSE=false
AUTH_JWT_SECRET=change-me-in-prod
AUTH_DATABASE_URL=postgres://alex:alex@auth-db:5432/alex_auth?sslmode=disable
AUTH_REDIRECT_BASE_URL=https://alex.yourdomain.com
```

> 登录服务在生产模式下默认开启。启动栈之前，请先通过 `psql "$AUTH_DATABASE_URL" -f migrations/auth/001_init.sql` 初始化认证表，并确保证书/Secret 能够持久化保存刷新令牌。

### 2. 构建和启动

```bash
# 构建镜像
docker compose -f deploy/docker/docker-compose.yml build

# 启动所有服务
docker compose -f deploy/docker/docker-compose.yml up -d

# 查看日志
docker compose -f deploy/docker/docker-compose.yml logs -f

# 停止服务
docker compose -f deploy/docker/docker-compose.yml down
```

### 3. 服务端点

- **Web 界面**: http://localhost:3000
- **API 服务**: http://localhost:8080
- **SSE 事件流**: http://localhost:8080/api/sse?session_id=xxx&replay=session
- **Health Check**: http://localhost:8080/health
- **Redis**: localhost:6379

### 4. Nginx 反向代理（推荐）

使用 Nginx 作为统一入口：

```bash
# 启动包含 Nginx 的完整栈
docker compose -f deploy/docker/docker-compose.yml up -d

# 访问通过 Nginx: http://localhost
```

**特性：**
- 统一入口（端口 80/443）
- SSE 连接优化（禁用缓冲）
- 速率限制保护
- CORS 配置
- SSL/TLS 支持（需配置证书）

---

## Kubernetes 集群部署

> ⚠️ 仓库当前不再内置 Kubernetes 清单；如需在集群部署，请基于下述镜像与配置要点编写自定义 manifest 并使用 `kubectl` 应用。

### 1. 准备镜像

```bash
# 构建并推送镜像到仓库
docker build -f deploy/docker/Dockerfile.server -t your-registry/alex-server:v1.0.0 .
docker build -f web/Dockerfile -t your-registry/alex-web:v1.0.0 ./web

docker push your-registry/alex-server:v1.0.0
docker push your-registry/alex-web:v1.0.0
```

### 2. 配置 Secret

在集群中创建包含所需凭据的 Secret：

```bash
# 使用 base64 编码 API Key
echo -n "sk-your-api-key" | base64

# 或使用 kubectl create secret
kubectl create secret generic alex-secrets \
  --from-literal=OPENAI_API_KEY=sk-your-api-key \
  -n alex-system
```

### 3. 部署到集群

使用自行维护的 Kubernetes 清单应用 Deployment/Service/Ingress 等资源，并检查部署状态：

```bash
# 应用所有资源
kubectl apply -f your-manifest.yaml

# 查看部署状态
kubectl get pods -n alex-system
kubectl get svc -n alex-system
kubectl get ingress -n alex-system

# 查看日志
kubectl logs -f deployment/alex-server -n alex-system
kubectl logs -f deployment/alex-web -n alex-system
```

### 4. 访问服务

#### 通过 Ingress（生产环境）

配置 DNS 记录指向 Ingress IP：

```bash
# 获取 Ingress IP
kubectl get ingress alex-ingress -n alex-system

# 访问
https://alex.yourdomain.com
```

#### 通过 Port Forward（开发/测试）

```bash
# 转发 Web 服务
kubectl port-forward svc/alex-web-service 3000:3000 -n alex-system

# 转发 API 服务
kubectl port-forward svc/alex-server-service 8080:8080 -n alex-system

# 访问
http://localhost:3000
```

### 5. 水平扩展

```yaml
# HPA 已配置，自动根据 CPU/内存扩展
# 手动扩展：
kubectl scale deployment alex-server --replicas=5 -n alex-system
```

### 6. 滚动更新

```bash
# 更新镜像
kubectl set image deployment/alex-server \
  alex-server=your-registry/alex-server:v1.1.0 \
  -n alex-system

# 查看滚动状态
kubectl rollout status deployment/alex-server -n alex-system

# 回滚
kubectl rollout undo deployment/alex-server -n alex-system
```

---

## 配置说明

### 环境变量

#### ALEX Server (Go)

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `OPENAI_API_KEY` | ✅ | - | OpenAI API Key |
| `LLM_PROVIDER` | ❌ | `openai` | LLM provider |
| `LLM_BASE_URL` | ❌ | `https://api.openai.com/v1` | API Base URL |
| `LLM_MODEL` | ❌ | `deepseek/deepseek-chat` | LLM 模型 |
| `LLM_VISION_MODEL` | ❌ | - | 图片附件存在时使用的 vision 模型 |
| `ALEX_VERBOSE` | ❌ | `false` | 详细日志 |
| `ALEX_SESSION_DATABASE_URL` | ✅ | - | Web 模式 Session 持久化 Postgres 连接串（可与 `AUTH_DATABASE_URL` 共用） |
| `ALEX_WEB_SESSION_DIR` | ❌ | `~/.alex-web-sessions` | Web 模式会话侧文件产物（journals、migration marker）路径 |
| `SESSION_STORE_PATH` | ❌ | `/data/sessions` | 兼容旧配置：同 `ALEX_WEB_SESSION_DIR` |
| `ALEX_WEB_MAX_TASK_BODY_BYTES` | ❌ | `20971520` | `/api/tasks` POST 请求体上限（字节），需要更大附件时调高 |
| `REDIS_URL` | ❌ | - | Redis 连接地址（可选） |
| `PORT` | ❌ | `8080` | HTTP 监听端口 |

#### Web Frontend (Next.js)

| 变量 | 必需 | 默认值 | 说明 |
|------|------|--------|------|
| `NEXT_PUBLIC_API_URL` | ❌ | `auto` | ALEX Server API 地址（nginx 同源默认 auto 即可） |
| `NODE_ENV` | ❌ | `development` | 运行环境 |
| `PORT` | ❌ | `3000` | HTTP 监听端口 |

### 持久化存储

#### Docker Compose

Volume 自动创建：
- `alex-sessions`: Web 会话侧文件产物（journals 等）
- `redis-data`: Redis 持久化

#### Kubernetes

需配置 PVC（已包含在 deployment.yaml）：
- `alex-sessions-pvc`: 10Gi（会话存储）
- `redis-pvc`: 5Gi（Redis 数据）

---

## 监控与日志

### 健康检查端点

```bash
# Server health
curl http://localhost:8080/health

# Response
{"status": "ok", "timestamp": "2025-10-02T10:00:00Z"}
```

### 查看日志

#### Docker Compose

```bash
# 所有服务
docker compose -f deploy/docker/docker-compose.yml logs -f

# 特定服务
docker compose -f deploy/docker/docker-compose.yml logs -f alex-server
docker compose -f deploy/docker/docker-compose.yml logs -f web
```

#### Kubernetes

```bash
# 实时日志
kubectl logs -f deployment/alex-server -n alex-system

# 查看所有 Pod
kubectl logs -l app=alex-server -n alex-system --tail=100

# 导出日志
kubectl logs deployment/alex-server -n alex-system > alex-server.log
```

### 监控指标（可选）

集成 Prometheus + Grafana：

```yaml
# 在 Deployment 中添加
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "8080"
  prometheus.io/path: "/metrics"
```

---

## 故障排查

### 问题 1: SSE 连接失败

**症状**: 前端显示 "Disconnected"

**解决方案**:
1. 确认 nginx 同源代理是否生效（默认 `NEXT_PUBLIC_API_URL=auto` 即可）
2. 验证 CORS 设置
3. 检查 Nginx 代理配置（`proxy_buffering off`）
4. 查看浏览器控制台错误

```bash
# 测试 SSE 连接
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=test"
```

### 问题 2: API 调用超时

**症状**: 任务执行无响应

**解决方案**:
1. 检查 `OPENAI_API_KEY` 是否有效
2. 验证网络连接到 OpenAI
3. 检查速率限制

```bash
# 测试 API Key
export OPENAI_API_KEY="sk-..."
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

### 问题 3: 内存/CPU 过高

**解决方案**:
1. 调整 Docker 资源限制
2. 减少并发任务数
3. 启用 Redis 会话存储（减少内存占用）
4. 配置 HPA 自动扩展

---

## 生产环境最佳实践

### 1. 安全性

- [ ] 使用 HTTPS（配置 SSL 证书）
- [ ] 启用身份验证（JWT/OAuth）
- [ ] 配置 API 速率限制
- [ ] 定期更新依赖和镜像
- [ ] 使用 Secret 管理工具（Vault、Sealed Secrets）

### 2. 可靠性

- [ ] 配置 Health Check 和 Readiness Probe
- [ ] 设置资源限制（CPU/Memory）
- [ ] 启用日志聚合（ELK、Loki）
- [ ] 配置监控告警（Prometheus + Alertmanager）
- [ ] 实施备份策略（会话数据）

### 3. 性能优化

- [ ] 启用 CDN（静态资源）
- [ ] 配置 Redis 缓存
- [ ] 使用连接池
- [ ] 优化镜像大小（多阶段构建）
- [ ] 配置 HPA 自动扩展

### 4. 运维

- [ ] CI/CD 自动部署
- [ ] 蓝绿部署或金丝雀发布
- [ ] 定期备份和恢复演练
- [ ] 文档化运维流程
- [ ] 建立监控仪表板

---

## 快速命令参考

```bash
# 本地开发
make server-run                    # 启动后端
cd web && npm run dev              # 启动前端

# Docker Compose
docker compose -f deploy/docker/docker-compose.yml up -d          # 启动生产环境
docker compose -f deploy/docker/docker-compose.dev.yml up         # 启动开发环境
docker compose -f deploy/docker/docker-compose.yml logs -f        # 查看日志
docker compose -f deploy/docker/docker-compose.yml down           # 停止服务

# Kubernetes（需使用自定义 manifest）
kubectl apply -f your-manifest.yaml           # 部署
kubectl get all -n alex-system                # 查看状态
kubectl logs -f deployment/alex-server -n alex-system  # 查看日志
kubectl port-forward svc/alex-web-service 3000:3000 -n alex-system  # 端口转发

# 测试
curl http://localhost:8080/health             # 健康检查
curl -N http://localhost:8080/api/sse?session_id=test&replay=session  # 测试 SSE
```

---

## 支持

遇到问题？

1. 查看 [故障排查](#故障排查) 部分
2. 检查 GitHub Issues: https://github.com/cklxx/Alex-Code/issues
3. 查看完整文档: `docs/` 目录

---

**文档版本**: v1.0
**更新时间**: 2025-10-02
