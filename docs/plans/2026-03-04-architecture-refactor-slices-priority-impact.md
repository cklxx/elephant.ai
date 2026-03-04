# Plan: Architecture Refactor Slices (Priority + Impact Surface)

Date: 2026-03-04
Owner: Codex

## Scope
- 仅整理重构切片、优先级和影响面。
- 不包含天数、排期、里程碑时间。
- 本清单默认在现有行为不变前提下做增量重构。

## Slices

| Slice ID | Priority | Slice | Impact Surface |
|---|---|---|---|
| A01 | P0 | 消除 `app -> delivery` 反向依赖（scheduler API 下沉到 app/domain 端口） | `internal/app/scheduler`、`internal/app/di`、`internal/delivery/server/schedulerapi`、`internal/delivery/server/bootstrap` |
| A02 | P0 | 调度通知接口改为 channel-agnostic port（移除 `SendLark`/`SendMoltbook` 式接口） | `internal/app/scheduler`、`internal/delivery/server/bootstrap/notifier.go`、`internal/delivery/channels/lark`、`internal/delivery/channels/*` |
| A03 | P0 | ReAct domain 剥离 shell/filesystem I/O（改为 ports + infra adapter） | `internal/domain/agent/react/background.go`、`internal/domain/agent/react/context_artifact_compaction.go`、`internal/domain/agent/ports`、`internal/infra/*` |
| A04 | P0 | `materialregistry` 剥离 outbound HTTP（domain 仅保留业务语义） | `internal/domain/materialregistry`、`internal/domain/*/ports`、`internal/infra/*` |
| A05 | P0 | 统一任务状态契约，去除 `waiting_input -> running` 语义降级映射 | `internal/domain/task`、`internal/delivery/taskadapters`、`internal/delivery/server/ports`、`web/lib/types/events` |
| A06 | P1 | LLM Provider 接入改为注册式能力描述（替代多处 switch） | `internal/infra/llm`、`internal/shared/config`、`internal/app/di/builder_llm.go` |
| A07 | P1 | 清理 OpenAI 通用链路中的 provider 特判（provider-specific 逻辑归属独立 adapter） | `internal/infra/llm/openai_client.go`、`internal/infra/llm/thinking.go`、`internal/infra/llm/openai_responses_*` |
| A08 | P1 | 订阅/模型选择与 provider 能力统一到单一来源（避免并行规则） | `internal/app/subscription`、`internal/shared/config/llm_profile.go`、`internal/delivery/server/http/runtime_models.go` |
| A09 | P1 | Channel 接入插件化（配置、启动、路由装配可扩展） | `internal/delivery/server/bootstrap`、`internal/shared/config/file_config.go`、`internal/delivery/channels/*` |
| A10 | P1 | Prompt 组装移除硬编码 channel/tool 分支（策略注入） | `internal/app/context/manager_prompt*.go`、`internal/app/agent/preparation/service.go`、`internal/domain/agent/presets` |
| A11 | P1 | Tool registry + policy 由 metadata 驱动（减少静态常量耦合） | `internal/app/toolregistry`、`internal/infra/tools/policy.go`、`internal/domain/agent/ports/tool` |
| A12 | P1 | Memory 抽象后端无关化（摆脱文件路径语义） | `internal/infra/memory/engine.go`、`internal/app/di/builder_session.go`、`internal/app/context/manager_memory.go`、`internal/delivery/server/http/api_handler_memory.go` |
| A13 | P2 | 拆分胖 `ContextManager` 接口（窗口/压缩/注入/持久化分离） | `internal/domain/agent/ports/agent/context.go`、`internal/app/context`、`internal/domain/agent/react` |
| A14 | P2 | 统一事件失败语义与因果字段透传（生成/翻译/持久化一致） | `internal/domain/agent/react/events.go`、`internal/app/agent/coordinator/workflow_event_translator*.go`、`internal/delivery/server/app/file_event_history_store.go` |
| A15 | P2 | 控制 workflow/event payload 增长（快照瘦身与输出分级） | `internal/domain/workflow`、`internal/domain/agent/react/workflow.go`、`internal/app/agent/coordinator`、`web/hooks/useSSE` |

## Notes
- 执行顺序按 `P0 -> P1 -> P2`，同级切片可并行，但要求无写冲突。
- 每个切片建议独立 PR，避免跨切片混改。
- 验证基线统一使用：`make check-arch`、`alex dev lint`、`alex dev test`。
