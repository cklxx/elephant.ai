# 当前架构审视报告

## 总览
本次审查聚焦 `internal/agent` 相关代码在所谓“六边形架构”实践中的落实情况，重点观察边界抽象、职责划分以及领域层的独立性。以下问题均来自主干分支的最新代码。

## 主要架构问题

### 1. 边界对象大量使用 `any`，导致类型逃逸
- `ports.ExecutionEnvironment` 及 `ports.AgentCoordinator` 接口以 `any` 表达领域状态、服务以及配置对象，只能通过注释说明真实类型。【F:internal/agent/ports/tools.go†L17-L63】
- `AgentCoordinator.ExecuteTask` 需要对 `env.State`、`env.Services` 和执行结果做显式类型断言，违背端口隔离的初衷，也让编译期无法发现跨层破坏。【F:internal/agent/app/coordinator.go†L111-L133】【F:internal/agent/app/coordinator.go†L318-L333】

**影响**：
1. 端口层无法表达真正的领域协议，导致上层必须了解具体实现细节。
2. 类型断言一旦失败会在运行期崩溃，难以在测试阶段捕获。
3. Mock 或替换实现非常困难——必须复制内部结构细节。

**优化建议**：
- 在 `ports` 层定义明确的领域 DTO/接口，例如 `TaskStateProvider`、`DomainServices`，并在 `domain` 层实现，以此消除对 `any` 的依赖。
- 对 `SaveSessionAfterExecution` 等方法返回值设计成结构化结果类型，避免跨层传递未定义的领域对象。

### 2. 领域层直接依赖基础设施实现，破坏六边形架构
- `domain.ReactEngine` 直接引入 `internal/utils` 的日志实现并在构造函数中实例化具体 logger。【F:internal/agent/domain/react_engine.go†L3-L32】
- 领域层同时引用 `internal/agent/types` 以读取输出上下文，从而反向依赖更外层模块。【F:internal/agent/domain/react_engine.go†L44-L54】

**影响**：
1. 领域层无法在没有 `utils.Logger` 的情况下独立测试或复用。
2. Logger 的初始化逻辑固定在领域对象内部，不利于自定义和观察性扩展。
3. `types` 包向领域层泄漏具体上下文表示，形成环状耦合。

**优化建议**：
- 在 `ports` 层引入最小化的 `Logger` 接口，并通过构造函数注入，实现基础设施可插拔。
- 将上下文读取逻辑下沉至应用层，在调用 `SolveTask` 前将所需的 agent level 作为参数显式传入，领域层不再依赖 `types` 包。

### 3. 应用服务承担过多职责，缺乏分层协作
`AgentCoordinator` 同时负责：
- 会话持久化、上下文压缩、LLM 客户端选择、费用回调注册、任务预分析、Prompt 装载、工具权限过滤等流程。【F:internal/agent/app/coordinator.go†L140-L315】
- 这些步骤穿插大量条件分支与 I/O（如 `os.Getwd`、LLM 请求、Preset 判断），造成巨大的“上帝对象”。

**影响**：
1. 单元测试需要准备大量依赖，难以覆盖。
2. 任一职责改动都可能触及其他逻辑，维护成本高。
3. 与六边形架构中“应用服务调度、领域服务处理”的分工不符。

**优化建议**：
- 引入独立的 `ExecutionPreparationService` 负责准备会话、系统提示与工具集合；`TaskAnalysisService` 封装预分析与 Prompt 选择；`CostTrackingDecorator` 负责注册回调。Coordinator 只聚焦编排流程。
- 将预分析所需的 LLM 客户端改为通过接口注入，而不是直接依赖 `llm.Factory`，提升可替换性。

### 4. 领域与端口重复定义消息结构，造成数据搬运
- `ports.Message` 与 `domain.Message` 结构体字段完全一致，却分别存在于端口层与领域层，需要多处转换函数来回拷贝数据。【F:internal/agent/ports/llm.go†L48-L55】【F:internal/agent/domain/types.go†L3-L25】【F:internal/agent/app/coordinator.go†L348-L370】

**影响**：
1. 转换函数易出现遗漏（如新增字段忘记同步）。
2. 领域模型与端口模型难以保持一致性，增加维护成本。
3. 造成不必要的内存复制与复杂度。

**优化建议**：
- 统一消息/工具调用的数据契约，可通过在端口层定义接口或共享结构体，并确保领域层与外层通过组合而不是复制来复用类型。
- 若担心循环依赖，可将通用 DTO 抽到独立的 `pkg/contract` 或 `internal/agent/model` 包，由端口和领域共同引用。

## 分阶段改进路线
1. **定义契约**：梳理 `domain` 与 `ports` 的数据结构，先完成 DTO/接口抽象，编写单元测试确保断言被移除。
2. **解耦领域依赖**：引入 Logger、上下文等接口，修改 `ReactEngine` 构造函数以依赖注入完成初始化，并调整调用方传参。
3. **拆分应用服务**：重构 `AgentCoordinator`，将预分析、提示管理、费用追踪拆分为独立服务；同时编写端到端测试覆盖编排流程。
4. **文档与示例更新**：更新架构文档，明确新的端口职责和依赖注入方式，避免后续实现再次退化。

## 结论
当前实现虽然借用了“Ports & Adapters”的命名，但在边界类型、安全性和职责划分方面仍存在明显缺陷。按上述建议逐步重构，可恢复六边形架构的核心价值：稳定的领域层、清晰的端口协议以及可替换的基础设施实现。
