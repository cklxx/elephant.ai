# ALEX 基础流程与架构复盘（2025Q1）

**目标**：务实评估当前主干分支的端到端执行链路，识别与业界最佳实践不符之处，并输出可落地的改进计划。

## 1. 当前执行流程快照
1. **服务启动与依赖装配**：`cmd/alex-server/main.go` 在 `run()` 里解析配置、初始化依赖注入容器并启动 HTTP 与 SSE 服务，随后拉起任务存储和事件广播器，再把它们交给服务器协调器处理异步任务。【F:cmd/alex-server/main.go†L20-L149】
2. **依赖注入容器**：`internal/di/container.go` 的 `BuildContainer` 负责创建 LLM 工厂、工具注册表、会话存储、成本追踪器与 MCP 初始化逻辑；构造过程中直接尝试请求远端模型客户端并注册 Git/MCP 工具。【F:internal/di/container.go†L46-L164】
3. **接口层路由**：`internal/server/http/router.go` 注册 REST 与 SSE 路由，绑定到 `ServerCoordinator`、`EventBroadcaster` 等适配器，同时为静态资源提供前端代理。【F:internal/server/http/router.go†L20-L122】
4. **任务调度**：`ServerCoordinator.ExecuteTaskAsync` 将 HTTP 请求转换为任务，写入任务存储后在后台 goroutine 中执行，并通过事件广播器推送状态变化；该流程目前会丢弃上游请求上下文。【F:internal/server/app/server_coordinator.go†L40-L168】
5. **领域执行循环**：`AgentCoordinator` 使用 `ExecutionPreparationService` 构建执行环境，随后实例化 `ReactEngine` 驱动 ReAct 循环以触发工具调用、子代理推理和消息压缩。【F:internal/agent/app/coordinator.go†L37-L187】【F:internal/agent/app/execution_preparation_service.go†L45-L230】【F:internal/agent/domain/react_engine.go†L17-L236】

## 2. 与最佳实践对齐情况
| 维度 | 现状 | 评估 |
| --- | --- | --- |
| **分层架构** | Hexagonal 层次清晰，应用层负责 orchestration，领域层聚焦推理循环。 | 👍 结构合理，但依赖注入仍存在隐藏耦合。 |
| **可测试性** | 关键组件具备接口抽象，`ReactEngine`、工具注册表可被替身；但协调器内部仍直接实例化部分依赖。 | ⚠️ 中等，需要进一步解耦构造逻辑。 |
| **弹性与隔离** | LLM 工厂缓存共享客户端；后台任务未感知取消；DI 启动需要真实外部依赖。 | ⚠️ 有显著风险。 |
| **可观测性** | 日志覆盖基础流程，但缺少对上下文压缩、成本追踪的结构化指标。 | ⚠️ 待提升。 |
| **运维体验** | CLI/Server/Web 统一通过容器启动，部署脚本完善；但缺乏最小化配置和健康探针。 | ⚠️ 仍有欠缺。 |

## 3. 核心问题与改进建议
### 3.1 成本追踪与客户端缓存耦合
- **症状**：`llm.Factory` 按 provider+model 缓存单例客户端，`CostTrackingDecorator.Attach` 通过共享回调写入成本信息，最后写入者覆盖先前配置，导致多会话并发时统计互相污染。【F:internal/llm/factory.go†L14-L105】【F:internal/agent/app/cost_tracking_decorator.go†L25-L74】
- **最佳实践参照**：每个会话/请求应持有独立的计费通道，可使用装饰器或中间件隔离副作用。
- **改进计划**：
  1. 为 `llm.Factory` 增加可选择“非共享实例”或“轻量代理”创建路径，让需要隔离的调用获得独立客户端句柄。
  2. 将 `CostTrackingDecorator` 改为返回实现 `ports.LLMClient` 的包装器，在包装器内部上报成本，避免修改被缓存客户端的回调状态。
  3. 在 `ExecutionPreparationService` 中显式请求隔离客户端，并将成本流水写入 session 维度的指标结构。
  4. 补充并发单元测试覆盖多会话同时执行时的成本准确性。

### 3.2 异步任务丢失上下文
- **症状**：`ExecuteTaskAsync` 创建后台 goroutine 时使用 `context.Background()`，无法传播 HTTP 超时/取消信息；任务结束原因也未写回存储或 SSE 事件。【F:internal/server/app/server_coordinator.go†L92-L168】
- **最佳实践参照**：异步执行需保留原始上下文或派生上下文，并提供取消/超时反馈。
- **改进计划**：
  1. 使用 `context.WithCancelCause` 从请求上下文派生后台上下文，将取消函数与任务 ID 关联存入 `TaskStore`。
  2. 在 `executeTaskInBackground` 中监听 `ctx.Done()`，向 `AgentCoordinator` 新增 `CancelTask` 或通过工具注册表触发中断逻辑。
  3. 扩展任务存储与 SSE 事件负载，记录终止原因（完成/取消/超时/失败），便于 UI 呈现与运维排障。

### 3.3 依赖注入阶段副作用过重
- **症状**：容器启动时即尝试初始化 Git/MCP 工具并访问外部模型提供商；缺少 feature flag 控制，导致本地/CI 若未配置 API Key 即直接失败。【F:internal/di/container.go†L72-L154】
- **最佳实践参照**：DI 构造应可在离线/测试模式下运行，副作用应延迟到显式 `Start()` 调用。
- **改进计划**：
  1. 将 Git/MCP 注册挪到惰性初始化流程（例如工具首次被请求时）；提供 `EnableMCP`、`EnableGitTools` 配置开关。
  2. 在 `cmd/alex-server` 中引入 `Start()`/`Shutdown()` 生命周期，方便在测试里跳过重型外部依赖。
  3. 为工具注册与 MCP 初始化添加健康探针接口，暴露给运维和 UI。

### 3.4 协调器构造逻辑冗杂
- **症状**：`NewAgentCoordinator` 内部直接实例化 `prompts.Loader`、`TaskAnalysisService`、`CostTrackingDecorator`，导致测试难以替换实现，且难以覆盖多模型或定制化场景。【F:internal/agent/app/coordinator.go†L49-L120】
- **最佳实践参照**：应用层应通过构造函数注入依赖或使用选项模式，让调用方决定具体实现。
- **改进计划**：
  1. 为协调器新增 `CoordinatorOption` 以注入 prompt loader、分析服务、成本装饰器的自定义实现；默认实现由 DI 层提供。
  2. 把预设解析、工作目录选择等职责提取到独立组件（例如 `PresetResolver`），减少协调器内部状态。
  3. 扩展 `internal/agent/app` 测试覆盖自定义依赖注入路径，确保可替换性。

### 3.5 缺少可观测性与运行诊断
- **症状**：成本追踪、上下文压缩、MCP 初始化缺乏结构化日志或指标；难以评估长任务或高并发场景的健康度。【F:internal/agent/app/execution_preparation_service.go†L120-L230】【F:internal/server/app/event_broadcaster.go†L23-L156】
- **最佳实践参照**：关键路径应输出可聚合的指标（token 使用、队列延迟、重试状态），并与 UI/运维面板对齐。
- **改进计划**：
  1. 在执行准备阶段记录上下文截断与工具过滤指标，暴露给 SSE。
  2. 扩展成本追踪器结构，分会话累积 tokens/costs 并输出到日志/metrics sink。
  3. 为事件广播器添加缓冲深度与失败计数观测。

## 4. 迭代路线图（建议）
| 阶段 | 范围 | 关键交付 | 验收标准 |
| --- | --- | --- | --- |
| Sprint 1 | 成本隔离 + 任务上下文 | LLM 客户端包装器、取消感知执行、任务存储终止原因字段 | 并发测试验证成本准确；取消请求 100% 停止后台执行 |
| Sprint 2 | DI 解耦 + 配置旗帜 | 惰性工具注册、`Start()/Shutdown()` 生命周期、MCP 健康探针 | 无 API Key 下 `make test` 可运行；健康端点提供状态 |
| Sprint 3 | 可替换协调器依赖 | Prompt/分析装饰器选项、PresetResolver、定制依赖单测 | 自定义依赖注入测试通过；`go test ./internal/agent/...` 绿 |
| Sprint 4 | 可观测性扩展 | Token/成本指标、事件广播器指标、文档更新 | 监控字段在 SSE/日志可见；README/运维手册同步 |

此外，本次复盘已清理过时的验收与环境评估文档，避免历史结论干扰当前规划。

## 5. 风险与缓解
- **并发行为回归**：引入新的客户端包装可能导致性能下降。→ 预留基准测试，并在 CI 中记录冷/热启动延迟对比。
- **配置复杂度提升**：新增旗帜可能使用户困惑。→ README 与 `docs/operations` 提供“最小可运行配置”示例，并在默认配置中保持兼容。
- **惰性初始化失败**：延迟注册可能导致首次调用时暴露错误。→ 在启动阶段执行预热检查（可选），并提供可观测事件。

---
*本报告基于 2025Q1 主干分支代码审阅，建议在完成上述改进后同步补充端到端回归测试与运维文档。*
