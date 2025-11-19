# RAG 预加载顺序问题说明

## 问题原因
- **召回历史被改写**：早期的 `ensureSystemPromptMessage` 会把第一个 `system` 角色的消息视为系统提示词，并将其内容替换为最新 prompt，这会把 `user_history` 类型的召回摘要整个覆盖成一段文本，导致 KV cache 失效。
- **RAG 结果插入位置错误**：RAG 预加载阶段注入的 `rag_preload` 消息默认放在消息数组最前面，在进入 ReAct 循环时会排在系统提示词之前，破坏了“系统提示 → 用户输入 → 检索上下文”的预期顺序。

> **更新（2025）**：现在 RAG 网关只负责写入检索计划摘要，并不会自动执行工具。`code_search` 作为标准工具注册给代理，模型需要在真正需要检索时自行调用，同时准备阶段会追加一条调试消息来提示推荐的检索/搜索/抓取动作。【F:internal/toolregistry/registry.go†L259-L279】【F:internal/agent/app/execution_preparation_service.go†L308-L338】

## 修复方案
- **精确识别系统提示词**：`internal/agent/domain/react_engine.go` 中的 `ensureSystemPromptMessage` 只会重写 `source=system_prompt` 的消息，带有 `user_history` 等其他来源的 `system` 消息保持原样，并将真正的系统提示移到列表开头。【F:internal/agent/domain/react_engine.go†L1184-L1222】
- **延迟挂载 RAG 上下文**：`extractPreloadedContextMessages` 会在进入循环前暂存 `rag_preload` 消息，待新的用户输入追加到 `state.Messages` 后再把这些上下文按原顺序拼接到用户消息之后（该逻辑保留以兼容旧会话记录）。【F:internal/agent/domain/react_engine.go†L1230-L1267】
- **端到端校验**：`internal/agent/app/execution_preparation_service_rag_test.go` 验证 RAG 信号、事件以及新的检索计划摘要消息都会写入状态，`internal/agent/domain/react_engine_test.go` 继续断言系统提示保持首位、RAG 上下文紧跟最新的用户输入。【F:internal/agent/app/execution_preparation_service_rag_test.go†L205-L320】【F:internal/agent/domain/react_engine_test.go†L64-L104】

通过上述调整，历史召回、系统提示和 RAG 预加载内容都能按照「系统 → 用户 → 检索上下文」的顺序稳定传入模型，消除了上下文错乱和 KV cache 失效的问题。
