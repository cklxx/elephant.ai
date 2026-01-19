# elephant.ai 项目理解报告

## 项目概述

elephant.ai 是一个面向生产环境的统一 AI 代理运行时，提供三个入口点：CLI/TUI、服务器和 Web 仪表板，共享相同的依赖注入容器和执行核心。

### 核心亮点

1. **统一运行时**：CLI/TUI、服务器和 Web 仪表板共享相同的执行核心和事件流
2. **多 LLM 支持**：OpenAI、Anthropic Claude、OpenRouter/DeepSeek/Antigravity 等
3. **自动提供者选择**：`llm_provider: auto` 自动选择最佳可用订阅
4. **类型化事件流**：在终端和 Web UI 中一致渲染的 artifact 感知事件
5. **内置可观测性**：结构化日志、OpenTelemetry 跟踪、Prometheus 指标和每会话成本核算
6. **检索层**：内存、技能、文档和外部上下文的检索，以及风险操作审批
7. **评估工具**：内置 SWE-Bench 等评估 harness，确保手动和自动运行的一致性

## 项目架构

```
Delivery (CLI, Server, Web) → Agent Application Layer → Domain Ports → Infrastructure Adapters
```

### 核心模块

1. **Delivery 层**
   - CLI/TUI: `cmd/alex`
   - HTTP + SSE 服务器: `cmd/alex-server`
   - Web 仪表板: `web/` (Next.js)

2. **Agent 核心**
   - `internal/agent/app`: 协调对话和工具调用
   - `internal/agent/domain`: 领域模型和事件定义
   - `internal/agent/ports`: 端口定义和边界

3. **基础设施适配器**
   - `internal/di`: 依赖注入容器
   - `internal/tools`: 类型化工具和安全策略
   - `internal/toolregistry`: 工具注册表
   - `internal/llm`: LLM 提供者抽象
   - `internal/session`: 会话管理
   - `internal/storage`: 持久化层
   - `internal/observability`: 可观测性
   - `internal/context`: 上下文管理和提示注入

## 关键路径

- **Agent 核心**: `internal/agent/` - ReAct 循环、审批和事件模型
- **上下文管理**: `internal/context/` + `internal/rag/` - 分层检索和摘要
- **可观测性**: `internal/observability/` - 日志、跟踪、指标和成本核算
- **工具系统**: `internal/tools/` + `internal/toolregistry/` - 类型化工具和安全策略
- **评估工具**: `evaluation/` - SWE-Bench 和回归 harness
- **部署**: `deploy/` - Docker Compose 入口点
- **Web UI**: `web/` - 实时事件流和仪表板

## 快速开始

### 前提条件

- Go 1.24+
- Node.js 20+ (Web UI)
- Docker (可选)

### 配置和运行

```bash
# 配置 LLM 提供者
export OPENAI_API_KEY="sk-..."
cp examples/config/runtime-config.yaml ~/.alex/config.yaml

# 运行后端 + Web
./dev.sh

# 构建并运行 CLI
make build
./alex "Map the runtime layers, explain the event stream, and produce a short summary."
```

## 项目路线图

### 短期目标 (MVP 切片)

1. **跨进程编排**: 从持久化状态恢复会话
2. **计划重排**: 工具失败后自动重排并发出事件
3. **工具 SLA 配置文件**: 记录每个工具的延迟/成本
4. **评估门禁自动化**: CI 任务自动运行评估并总结失败

### 长期目标

1. **多智能体协作**: 仲裁器和冲突解决策略
2. **高级学习系统**: 反馈循环和模型微调
3. **数据治理完善**: 数据分级和合规审计
4. **部署扩展性**: K8s 高可用部署

## 质量保证

### 代码质量

- Go lint + 测试: `./scripts/run-golangci-lint.sh run --timeout=10m ./...` 和 `make test`
- Web lint + 单元测试: `npm --prefix web run lint` 和 `npm --prefix web test`
- 端到端测试: `npm --prefix web run e2e` 和 `./dev.sh test`

## 贡献指南

- `CONTRIBUTING.md`: 贡献工作流程和代码标准
- `CODE_OF_CONDUCT.md`: 社区行为准则
- `SECURITY.md`: 漏洞报告流程
- `ROADMAP.md`: 项目路线图和贡献入口点

## 文档

- `docs/README.md`: 文档主页
- `docs/AGENT.md`: 运行时概述
- `docs/reference/ALEX.md`: 架构和开发参考
- `docs/reference/CONFIG.md`: 配置 schema
- `docs/guides/quickstart.md`: 快速开始指南
- `docs/operations/DEPLOYMENT.md`: 部署指南