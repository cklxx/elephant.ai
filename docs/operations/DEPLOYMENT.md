# ALEX 部署指南

Updated: 2026-02-10

本文档基于当前仓库实现，覆盖本地开发、Docker Compose 部署与自定义 Kubernetes 部署要点。

## 1. 本地开发部署（推荐）

前置要求：
- Go 1.24+
- Node.js 20+
- Docker（可选，但建议用于 sandbox 与本地服务依赖）

快速启动：

```bash
make build
alex setup
alex dev up
```

常用运维命令：

```bash
alex dev status
alex dev logs server
alex dev logs web
alex dev down
```

默认访问地址：
- Web: `http://localhost:3000`
- API/SSE: `http://localhost:8080`
- Health: `http://localhost:8080/health`

## 2. Docker Compose 部署

仓库内置 Compose 文件：
- `deploy/docker/docker-compose.dev.yml`
- `deploy/docker/docker-compose.yml`

启动（开发版）：

```bash
export LLM_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
docker compose -f deploy/docker/docker-compose.dev.yml up -d
```

启动（标准版）：

```bash
docker compose -f deploy/docker/docker-compose.yml build
docker compose -f deploy/docker/docker-compose.yml up -d
```

查看日志与停止：

```bash
docker compose -f deploy/docker/docker-compose.yml logs -f
docker compose -f deploy/docker/docker-compose.yml down
```

## 3. Kubernetes 部署（自定义清单）

仓库当前不再维护内置 K8s manifests。建议流程：
1. 构建并推送镜像（server + web）。
2. 使用 Secret 注入敏感配置（API keys、JWT secret、DB URL 等）。
3. 通过 ConfigMap 挂载 `config.yaml`（或设置 `ALEX_CONFIG_PATH`）。
4. 自定义 Deployment/Service/Ingress/HPA 并按环境治理资源。

示例镜像构建：

```bash
docker build -f deploy/docker/Dockerfile.server -t your-registry/alex-server:latest .
docker build -f web/Dockerfile -t your-registry/alex-web:latest ./web
```

## 4. 配置与密钥

主配置文件：`~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH` 指向的路径）。

建议通过环境变量做 YAML 插值，例如：
- `LLM_API_KEY`
- `AUTH_JWT_SECRET`
- `AUTH_DATABASE_URL`
- `ALEX_SESSION_DATABASE_URL`

示例（节选）：

```yaml
runtime:
  llm_provider: "auto"
  api_key: "${LLM_API_KEY}"

auth:
  jwt_secret: "${AUTH_JWT_SECRET}"
  database_url: "${AUTH_DATABASE_URL}"

session:
  database_url: "${ALEX_SESSION_DATABASE_URL}"
```

完整字段见：`docs/reference/CONFIG.md`。

## 5. 运行观测与排障

结构化运行日志：
- `alex-service.log`
- `alex-llm.log`
- `alex-latency.log`

默认目录：
- `ALEX_LOG_DIR`（默认 `$HOME`）
- `ALEX_REQUEST_LOG_DIR`（默认 `${PWD}/logs/requests`）

开发进程日志（`alex dev`）：默认写到 `logs/`（如 `logs/server.log`, `logs/web.log`）。

更多说明见：`docs/reference/LOG_FILES.md`。
