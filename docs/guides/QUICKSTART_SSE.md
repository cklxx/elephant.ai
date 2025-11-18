# ALEX SSE 服务快速开始
> Last updated: 2025-11-18


⚡ 3分钟快速启动 ALEX SSE 服务和 Web 界面

## 🚀 方式一: Docker Compose（推荐）

### 1. 准备环境变量

```bash
# 创建 .env 文件
cat > .env << EOF
OPENAI_API_KEY=sk-your-api-key-here
ALEX_MODEL=gpt-4
EOF
```

### 2. 启动所有服务

```bash
# 构建并启动
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 3. 访问应用

- **Web 界面**: http://localhost:3000
- **API 文档**: http://localhost:8080/health
- **SSE 测试**: http://localhost:8080/api/sse?session_id=test

### 4. 测试 SSE 连接

```bash
# 新终端：监听 SSE 事件
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=demo"

# 另一个终端：提交任务
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{"task": "What is 2+2?", "session_id": "demo"}'
```

### 5. 停止服务

```bash
docker-compose down
```

---

## 🛠️ 方式二: 本地开发

### 1. 启动后端

```bash
# 设置 API Key
export OPENAI_API_KEY="sk-your-key"

# 构建并运行
make server-run

# 或直接运行
go run cmd/alex-server/main.go
```

### 2. 启动前端（新终端）

```bash
cd web

# 安装依赖
npm install

# 配置环境变量
echo "NEXT_PUBLIC_API_URL=http://localhost:8080" > .env.local

# 启动开发服务器
npm run dev
```

### 3. 访问应用

打开浏览器访问: http://localhost:3000

---

## ✅ 验证安装

### 健康检查

```bash
# Server 健康检查
curl http://localhost:8080/health

# 应返回
# {"status":"ok","timestamp":"2025-10-02T..."}

# Web 访问测试
curl -I http://localhost:3000
```

### 完整流程测试

```bash
# 运行集成测试
./scripts/integration-test.sh http://localhost:8080
```

---

## 📝 快速使用指南

### Web 界面使用

1. **访问主页**: http://localhost:3000
2. **输入任务**: 在文本框输入任务（如 "分析这个项目的架构"）
3. **点击 Execute**: 开始执行
4. **实时查看**: 观察 SSE 事件流实时显示工具调用和结果

### API 使用

#### 创建任务

```bash
curl -X POST http://localhost:8080/api/tasks \
  -H "Content-Type: application/json" \
  -d '{
    "task": "Write a hello world in Python",
    "session_id": "my-session"
  }'
```

#### 订阅 SSE 事件

```bash
curl -N -H "Accept: text/event-stream" \
  "http://localhost:8080/api/sse?session_id=my-session"
```

#### 查看会话

```bash
# 列出所有会话
curl http://localhost:8080/api/sessions

# 获取特定会话
curl http://localhost:8080/api/sessions/my-session
```

---

## 🐛 常见问题

### 问题 1: SSE 连接失败

**症状**: 前端显示 "Disconnected"

**解决**:
```bash
# 检查后端是否运行
curl http://localhost:8080/health

# 检查 Web 环境变量
cat web/.env.local
# 应该有: NEXT_PUBLIC_API_URL=http://localhost:8080
```

### 问题 2: CORS 错误

**症状**: 浏览器控制台显示 CORS 错误

**解决**:
- 确保后端 CORS 中间件已启用
- 检查 `internal/server/http/middleware.go` 配置

### 问题 3: API Key 错误

**症状**: 任务执行失败

**解决**:
```bash
# 验证 API Key
echo $OPENAI_API_KEY

# 测试 OpenAI 连接
curl https://api.openai.com/v1/models \
  -H "Authorization: Bearer $OPENAI_API_KEY"
```

---

## 🎯 下一步

1. **浏览文档**
   - 完整文档: `DEPLOYMENT.md`
   - 架构设计: `docs/design/SSE_WEB_ARCHITECTURE.md`
   - 实现总结: `SSE_IMPLEMENTATION_SUMMARY.md`

2. **开发指南**
   - 后端开发: `internal/server/README.md`
   - 前端开发: `web/README.md`

3. **生产部署**
   - Docker: 参考 `docker-compose.yml`
   - Kubernetes: 参考 `k8s/deployment.yaml`

---

## 📊 端口说明

| 服务 | 端口 | 说明 |
|------|------|------|
| Web 前端 | 3000 | Next.js 开发服务器 |
| API 服务 | 8080 | ALEX SSE Server |
| Redis | 6379 | 会话存储（可选） |
| Nginx | 80 | 反向代理（生产环境） |

---

## 🔗 有用的命令

```bash
# Docker Compose
docker-compose up -d              # 启动所有服务
docker-compose logs -f alex-server # 查看后端日志
docker-compose logs -f web        # 查看前端日志
docker-compose down               # 停止所有服务

# Make 命令
make server-build                 # 构建后端
make server-run                   # 运行后端
make server-test                  # 运行测试

# NPM 命令（在 web/ 目录）
npm run dev                       # 开发模式
npm run build                     # 构建生产版本
npm run start                     # 启动生产服务器
```

---

**🎉 开始使用 ALEX SSE 服务吧！**

遇到问题？查看 [DEPLOYMENT.md](DEPLOYMENT.md) 获取详细说明。
