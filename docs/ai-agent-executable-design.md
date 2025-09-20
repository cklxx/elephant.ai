## Minimal ReAct Agent 方案设计（精简、可控、可执行）

### 设计目标
- 仅实现标准 ReAct 循环：Thought → Action(tool) → Observation。
- 强约束、低耦合：最少组件、最短路径、可观测、可测试。
- 直接对接现有代码：`ReactCore`、`ToolRegistry`、`session.Manager`、`LLM`。

### 最小组件
- ReActLoop：单文件/单结构体，负责迭代与退出条件。
- Prompt：一条模板（含工具使用规范与输出格式），无阶段化拆分。
- ToolGateway：对 `ToolRegistry` 的极薄封装，统一超时与错误封装。
- Memory：仅使用 `session.Manager` 最近 N 条消息；超出阈值触发简单摘要（可选，开关）。
- LLM：直接使用现有 `llm.Client`，不引入路由与中间件。

### ReAct 循环（伪代码）

```text
init(max_iter, max_tokens, tool_timeout)
messages = [system_prompt, user_goal]

for i in [1..max_iter]:
  resp = LLM(messages, tools=ToolRegistry.defs, stream=false)
  if resp.no_tool_calls:
    return finalize(resp.assistant_message)

  for call in resp.tool_calls:
    result = ToolGateway.invoke(call.name, call.args, timeout=tool_timeout)
    messages.append(tool_message(result))

  messages.append(assistant_ack("Observed results. Continuing."))

return truncate_and_finalize(messages)
```

### 核心接口（最小化）

```go
type ReActConfig struct {
  MaxIterations   int
  MaxHistory      int // 最近消息条数
  ToolTimeout     time.Duration
}

type ReActLoop struct {
  LLM llm.Client
  Sessions *session.Manager
  Tools *agent.ToolRegistry
  Logger Logger // 英文日志
  Cfg ReActConfig
}

func (r *ReActLoop) Run(ctx context.Context, goal string) (string, error)
```

### Prompt（单模板，示意）

```text
You are a concise coding agent. Use tools when necessary.
Format:
- Think: brief reasoning
- Action: {"tool":"name","args":{...}} or "finish"
- Observation: will be provided by system
Rules:
- Prefer minimal steps, deterministic outputs, English logs.
- If enough info, choose "finish" with the final answer.
```

### ToolGateway（极薄封装）

- 输入：`name + args`
- 行为：从 `ToolRegistry` 获取工具，设置统一超时（context.WithTimeout），捕获 panic，返回标准结果：`{success, data|string, error, duration}`。
- 不做重试/熔断/并发池，保持最小可控。后续按需加。

### 退出条件（严格、可控）
- LLM 未产生工具调用且输出为“finish”。
- 达到 `MaxIterations`。
- 工具调用连续失败 ≥ K（默认 3），直接返回已知信息的最佳答案。

### 记忆与上下文（最小策略）
- 仅注入最近 `MaxHistory` 条用户/助手/工具消息。
- 可选开启“简单摘要”：当历史超阈值，用单轮 LLM 将早期消息压缩为 1 段摘要，替换到最前部（默认关闭）。

### 日志与事件（轻量）
- 统一英文日志（level, iter, tool, duration, error）。
- 可选事件钩子：OnIterationStart/End、OnToolStart/End（函数指针或接口）。

### 与现有代码的映射
- `internal/agent/core.go`：将 `ExecuteTaskCore` 精简为 `ReActLoop.Run` 的包装；保留 `MaxIter`、`ToolRegistry`、会话写回。
- `internal/agent/tool_registry.go`：保持不变，新增 `ToolGateway` 封装在 `internal/agent/tools/gateway.go`。
- `internal/session/session.go`：直接取当前会话消息，按 `MaxHistory` 裁剪；可选调用现有压缩。
- `internal/llm/*`：直接用当前客户端，无需路由/工厂修改。

### 配置（仅三项）
- `MaxIterations`（默认 50）
- `MaxHistory`（默认 40）
- `ToolTimeout`（默认 60s）

### 验收标准
- 典型任务在 ≤ 20 次迭代内收敛；无不必要的并发与子 Agent 调用。
- 工具错误不导致崩溃；达到失败阈值能优雅退出并给出当前最佳答案与原因。
- 历史裁剪后对话稳定，日志清晰（英文）。

### 后续可选增强（不在本方案内）
- 轻微的工具重试/并发度控制；
- 单轮反思（失败后一次修正）；
- 模型路由与事件总线。

---

## 基于本方案的优化清单（按优先级）

- 强化单环路 ReAct
  - 将 `ReactCore.ExecuteTaskCore` 精简为单一 ReAct 循环，移除循环内的“待处理消息整合/子 agent 分支/并发分支”。
  - 默认 `MaxIterations=50`；循环中不追加新用户输入。

- 工具调用最小化
  - 新增极薄 `ToolGateway`：统一 `context.WithTimeout` + panic 保护 + 结构化错误封装。
  - 绕过 `utils.NewToolExecutor`、display 格式化与流式回调转换。
  - 工具失败仅写入一次工具消息，避免重复噪音。

- 会话与消息
  - 只注入最近 `MaxHistory`（默认 40）条消息；默认关闭 AI 压缩（保留开关）。
  - 仅 ReAct 循环写回 `assistant/tool` 消息，避免多处写回。

- Prompt 收敛
  - 统一单模板 Prompt；删除阶段化/复杂 fallback。
  - 强化 “若足够信息则 finish” 约束，减少无谓工具调用。

- LLM 使用收敛
  - 仅使用单个 `llm.Client` 与固定模型；禁用路由与中间件。
  - 统一关闭 streaming，使用固定温度与最大 token。

- 子 Agent/并发去耦
  - 默认禁用并发与子 Agent；`subagent` 仅作为普通工具 behind flag。
  - 移除循环内并发逻辑与特殊回调转换。

- 日志与可观测（轻量）
  - 统一英文日志、无 emoji；字段含 level/iter/tool/duration/error/call_id。
  - 可选事件钩子：OnIterationStart/End、OnToolStart/End（默认关闭）。

- 错误与退出
  - 统一最大连续工具失败阈值（默认 3）→ 提前退出并返回“当前最佳答案 + 失败原因”。
  - 仅在 ReAct 入口与工具层做 panic 恢复，内部不再层层 recover。

- 配置收敛
  - 仅暴露 3 项：`MaxIterations`、`MaxHistory`、`ToolTimeout`；其他设默认值。
  - 文档化默认值与典型建议值。

- 依赖与路径
  - 保留静态工具；MCP/动态工具默认关闭或延长 TTL。
  - `internal/context/message` 的池化/压缩逻辑不在主路径直接调用。

- 兼容与清理
  - 在 `ReactCore` 增加 `minimal_react` 开关灰度切换，旧路径保留。
  - 清理循环内与 ReAct 无关分支（如主 agent 待处理消息融合），迁至循环外预处理。

---

## 快速落地步骤（建议 1 日内完成）

1) 默认配置：`MaxIterations=50`、`MaxHistory=40`、`ToolTimeout=60s`。
2) 新增 `ToolGateway`（`internal/agent/tools/gateway.go`）：统一超时与错误封装。
3) 在 `ReactCore` 增加 `MinimalReAct` 开关；实现走单模板 Prompt + 单 LLM + `ToolGateway` 的精简路径。
4) 关闭并发/子 Agent/压缩/阶段化 prompt 的开关（默认关闭）。
5) 统一英文日志与字段，移除 emoji；仅入口与工具层打点。

---

## 进阶考量（完善与稳态化）

### 安全与合规
- Prompt 侧：加入“拒绝高风险请求”的固定条款；要求工具调用仅限白名单。
- 工具侧：对外部 I/O 工具（shell/web/file）强制参数白名单与路径沙箱；统一 `ToolTimeout` + 上限输出大小（例如 64KB）。
- 日志侧：敏感信息脱敏（token、路径、URL 参数中的秘钥）；错误信息仅记录摘要与错误码。

### 确定性与可控性
- 固定模型、固定温度（建议 0.2-0.4）；Prompt 稳定化（避免易抖动字段）。
- 约束 Action 输出 JSON Schema（严格解析，失败即反馈重新输出）。
- 工具结果输出做截断与类型校验，防止长输出刷屏。

### 成本与性能
- Token 预算可视化：每次调用记录 prompt/completion 估算，按迭代累积。
- MaxHistory 与工具输出截断是主控点；必要时单轮摘要以常数成本换线性增长。
- 禁用并发与子 Agent，避免不可控的 token 放大。

### 稳定性与回退
- 单一 panic 恢复点：ReAct 入口与 ToolGateway；异常时返回“已完成进度 + 建议人工介入”。
- 工具失败阈值触发软退出；若 LLM 输出非预期格式，进行一次提示重试再失败退出。

### 测试与评测
- 单元测试：
  - ReAct 循环在无工具、单工具、多工具场景的收敛性与写回一致性。
  - ToolGateway：超时/异常/大输出截断/类型校验。
  - Prompt 解析：Action JSON 解析失败时的重试路径。
- 集成评测（`evaluation/swe_bench` 可扩展）：
  - 关键任务样例：编辑单文件、跨文件读写、网络检索（如有）。
  - 指标：成功率、平均迭代、平均 token、工具失败率。

### 迁移与兼容
- 保留旧路径开关；默认启用 `minimal_react`，CI 中保留旧路径回归样例以对照。
- 文档列出“旧 → 新”映射：哪些功能默认关闭（并发/子 Agent/阶段化 Prompt/压缩等）。

### SLO 与可观测
- SLO 建议：
  - 成功率 ≥ 80%（内部样例集）。
  - 平均迭代 ≤ 20；p95 工具延迟 ≤ 3s；p95 任务时长 ≤ 60s。
  - token 成本 p95 ≤ 1.5× 旧实现。
- 可观测：
  - 指标：iteration_count、tool_latency_bucket、tool_error_total、token_used_total。
  - 日志：英文、结构化、task_id/iter/call_id 贯通。

### 文档与运营
- 在 README/Docs 标注“Minimal ReAct 模式”为默认；附带开关说明与典型配置。
- 事故手册：工具频繁失败/LLM 解析失败/超时的处置流程（回退、关闭功能、采集样本）。



