# Plan: Architecture Refactor Slices (Priority + Impact Surface)

Date: 2026-03-04
Owner: Codex (authored) → ckl (reviewed + revised)

## Diagnosis

当前架构是**枚举驱动拼接**，不是**能力驱动抽象**。本计划是**止血单**，不是可扩展蓝图。

六个结构性病灶：

1. **分层是名义上的，不是约束性的。**
   `configs/arch/exceptions.yaml` 有 **123 条** import 豁免，过期时间**全部**是 `2026-03-31`。每加一个能力先想"加豁免"，架构检查形同虚设。关键反例：`scheduler.go:9` 直接 import delivery；`exceptions.yaml:146`。

2. **Domain 被 infra 污染，可移植性和可测试性差。**
   `background.go:8-10` + `:961`（tmux/exec）；`context_artifact_compaction.go:9-10` + `:251`（直接文件系统）；`attachment_migrator.go:10` + `:170`（直接 HTTP）。换运行环境/存储后端/沙箱策略牵一发而动全身。

3. **Channel 抽象是假的，接口里写死渠道名。**
   `executor.go:22-25`（`SendLark`/`SendMoltbook`）；`trigger.go:8-10`（lark 特定字段语义）；`manager_prompt_context.go:307-312`（Prompt 对 Lark 硬编码）；`lark_gateway.go` 与 `telegram_gateway.go` 两套复制装配。新增 WeChat/Slack 必改 scheduler + bootstrap + prompt + config，多点爆炸。

4. **Provider 能力模型分散且重复，多处 switch。**
   `llm/factory.go:193`、`builder_llm.go:30`、`llm_profile.go:72`、`provider_resolver.go:164`。更糟：`fetchProviderModels` 在两处重复实现：`catalog.go:414` 与 `runtime_models.go:83`。加一个 provider 不是"注册一次"，而是"全仓补 switch"。

5. **任务状态契约已发生语义塌缩。**
   Domain 有 `waiting_input`（`store.go:19`），但 server adapter 折叠为 `running`（`server_adapter.go:230-231`）。前端/自动恢复/告警都拿不到真实阻塞态。

6. **事件因果链在持久化层被截断。**
   Envelope 明确保留因果字段（`envelope.go:98-99`），但文件事件存储恢复时明确丢弃 correlation/causation（`file_event_history_store.go:247`）。跨层排障、回放追责、事件关联分析都会断链。

一句话：现在这套抽象是"功能能跑"，不是"系统可进化"。

---

## Scope

- 仅整理重构切片、优先级、影响面、依赖关系和验收标准。
- 不包含天数、排期、里程碑时间。
- 本清单默认在现有行为不变前提下做增量重构，**但修复语义塌缩（A05）和因果截断（A14）属于 bug fix，不受"行为不变"约束**。这两个切片需要兼容迁移策略（见 blueprint §0.3）。

---

## Slices

| Slice ID | Priority | Slice | Impact Surface | Depends On | Done Criteria | Verify Command |
|---|---|---|---|---|---|---|
| A01 | P0 | 消除 `app -> delivery` 反向依赖（scheduler API 下沉到 app/domain 端口） | `internal/app/scheduler/scheduler.go`、`internal/app/di`、`internal/delivery/schedulerapi/api.go`、`internal/delivery/server/bootstrap/scheduler.go`、`configs/arch/exceptions.yaml` | - | `internal/app/scheduler` 不再 import `internal/delivery/schedulerapi`；对应 arch exception 被清理或替换为目标依赖；`internal/infra/tools/builtin/scheduler` 同步清理。 | `grep -r "delivery/schedulerapi" internal/app/ internal/infra/ \| wc -l` → 0 |
| A02 | P0 | 调度通知接口改为 channel-agnostic port（移除 `SendLark`/`SendMoltbook` 式接口） | `internal/app/scheduler/executor.go:22-25`（`Notifier` 接口）、`internal/app/scheduler/trigger.go:8-10`（Trigger.Channel/UserID/ChatID 字段语义）、`internal/delivery/server/bootstrap/notifier.go`、`internal/delivery/channels/lark`、`internal/delivery/channels/*` | A01 | `Notifier` 接口改为 `Send(ctx, channel, target, content) error` 单方法；Trigger 结构体不再暴露 channel-specific 字段语义；新增 channel 不需要改 scheduler 接口层。 | `grep -n "SendLark\|SendMoltbook" internal/app/scheduler/` → 无匹配 |
| A03 | P0 | ReAct domain 剥离 shell/filesystem I/O（改为 ports + infra adapter） | `internal/domain/agent/react/background.go:8-10`（`os`/`os/exec`/`path/filepath`）、`internal/domain/agent/react/context_artifact_compaction.go:9-10`（`os`/`path/filepath`）、`internal/domain/agent/react/workflow.go`（审计点，当前无直接 import）、`internal/domain/agent/ports`、`internal/infra/*` | - | ReAct domain 目标文件不再直接 import `os`/`os/exec`/`path/filepath`；相关能力通过 ports 注入到 infra adapter。 | `grep -rn '"os"\|"os/exec"\|"path/filepath"' internal/domain/agent/react/` → 无匹配 |
| A04 | P0 | `materialregistry` 剥离 outbound HTTP（domain 仅保留业务语义） | `internal/domain/materialregistry/attachment_migrator.go:10,170`（`net/http`、`http.NewRequestWithContext`）、`internal/domain/materialregistry/ports`（已有 ports 目录） | - | `internal/domain/materialregistry` 不再直接使用 `net/http` 或 `http.NewRequestWithContext`；HTTP 调用移入 infra adapter 并通过 ports 注入。 | `grep -rn '"net/http"' internal/domain/materialregistry/` → 无匹配 |
| A05 | P0 | 统一任务状态契约，去除 `waiting_input -> running` 语义降级映射 | `internal/domain/task/store.go:19`（`StatusWaitingInput`）、`internal/delivery/taskadapters/server_adapter.go:230-231`（折叠点）、`internal/delivery/server/ports/task.go:13-19`（缺少 `TaskStatusWaitingInput`）、`internal/delivery/server/http/api_handler_tasks.go`、`web/lib/types/api/task.ts`、**`web/components/*` 中所有消费 `task.status` 的组件（需审计）** | - | server adapter 不再把 `waiting_input` 折叠为 `running`；`TaskStatusWaitingInput` 加入 server ports；前端 task API 类型显式包含 `waiting_input`；前端组件审计完成，正确处理新状态。**迁移**：后端通过 feature flag `TASK_STATUS_V2` 控制，默认 on；前端未就绪时可回滚。Flag 在前端部署确认后删除（≤2 周窗口）。 | `grep -n "StatusWaitingInput.*StatusRunning\|waiting_input.*running" internal/delivery/` → 无匹配；`go test ./internal/delivery/taskadapters/ -run TestWaitingInputPreserved` → pass |
| A06 | P0 | LLM Provider 接入改为注册式能力描述（替代多处 switch） | `internal/infra/llm/factory.go:193`（provider switch）、`internal/app/di/builder_llm.go:30`（credential switch）、`internal/shared/config/llm_profile.go:72`（normalizeProviderFamily switch）、`internal/shared/config/provider_resolver.go:164`（resolveProviderCredentials）、`internal/app/subscription/catalog.go:414` 与 `internal/delivery/server/http/runtime_models.go:83`（**重复的 `fetchProviderModels` 实现**） | - | provider 能力声明集中在注册表；新增 provider 不需要复制粘贴多处 switch 分支；`fetchProviderModels` 合并为单一实现。 | 新增 provider 只需在注册表加一条声明，不需改 factory/DI/config/subscription/delivery 中任何一处 |
| A07 | P0 | 清理 OpenAI 通用链路中的 provider 特判（provider-specific 逻辑归属独立 adapter） | `internal/infra/llm/openai_client.go:461-471`（`detectProvider` baseURL 嗅探）、`:476-478,516,544`（Kimi 特判）、`internal/infra/llm/base_client.go:101-105`（Kimi UA header）、`internal/infra/llm/thinking.go:97-115,173-181`（provider-specific reasoning 判定） | A06 | OpenAI 通用链路不再包含 provider 名称特判；差异逻辑下沉到独立 adapter 或注册表能力声明。 | `grep -n "isKimi\|detectProvider\|kimi.com\|moonshot\|shouldSendOpenAIReasoning\|shouldSendAnthropicThinking" internal/infra/llm/openai_client.go internal/infra/llm/base_client.go internal/infra/llm/thinking.go` → 无匹配；`go test ./internal/infra/llm/ -run TestProviderAdapterIsolation` → pass |
| A08 | P1 | 订阅/模型选择与 provider 能力统一到单一来源（避免并行规则） | `internal/app/subscription/selection.go:60-102`（provider switch）、`internal/shared/config/llm_profile.go:84-111`（API key 特殊前缀判定）、`internal/delivery/server/http/runtime_models.go:83-110,141-185`（并行 credential + header 判定） | A06 | 运行时模型可见性与可选性来自同一能力来源；删除并行判定分支。 | provider 能力查询收敛到注册表单一入口 |
| A09a | P1 | Channel 插件化第一步：抽象 channel registration 接口 | `internal/delivery/server/bootstrap`（`ChannelsConfig{Lark, Telegram}` 硬编码字段）、`internal/domain/agent/ports/agent`、`internal/app/di` | - | 完成 channel registration 抽象，bootstrap 依赖抽象接口而非固定 channel 细节。 | `ChannelsConfig` 不再有 channel-specific 命名字段 |
| A09b | P1 | Channel 插件化第二步：迁移 bootstrap 配置与路由装配 | `internal/delivery/server/bootstrap`、`internal/shared/config/file_config.go`、`internal/delivery/channels/*`（`lark_gateway.go` 与 `telegram_gateway.go` 存在复制装配） | A09a | 新 channel 接入通过插件注册 + 配置生效，不需要改动中心化启动装配逻辑。 | 新增 channel 只需实现接口 + 加配置，不改 bootstrap |
| A10 | P1 | Prompt 组装移除硬编码 channel/tool 分支（策略注入） | `internal/app/context/manager_prompt_context.go:307-322`（`buildChannelFormattingSection` 仅对 `"lark"` 硬编码）、`internal/app/agent/preparation/service.go`、`internal/domain/agent/presets` | A09b | `composeSystemPrompt` 不再硬编码 channel 分支；channel 格式化段由 channel 插件自描述提供。 | `grep -n 'channel.*lark\|"lark"' internal/app/context/manager_prompt*.go` → 无匹配 |
| A11 | P2 | Tool registry 内置默认规则外部化为 YAML（`DefaultPolicyRules` 可配置化） | `internal/infra/tools/policy.go`（`DefaultPolicyRules()` 硬编码 Go 常量）、`internal/app/toolregistry/policy.go` | - | `DefaultPolicyRules()` 读取 YAML 配置而非 Go 常量；policy 框架本身（`PolicySelector` + metadata 驱动匹配）保持不变。 | 内置规则数量 = YAML entries 数量，Go 代码无硬编码规则 |
| A12 | P1 | Memory 抽象后端无关化（摆脱文件路径语义） | `internal/infra/memory/engine.go:48`（`RootDir() string` 接口泄露）、`internal/app/context/manager_memory.go:84-90`（依赖 `RootDir()`）、`internal/delivery/server/http/api_handler_memory.go:47-50`（依赖 `RootDir()`）、`internal/app/di/builder_session.go` | - | `Engine` 接口移除 `RootDir()`；上层通过 Engine 方法获取内容而非自行拼路径。 | `grep -rn "\.RootDir()" internal/app/ internal/delivery/` → 无匹配（跨层）；`grep -n "RootDir" internal/infra/memory/engine.go` → 仅实现体内部；`go test ./internal/app/context/ -run TestMemoryWithoutRootDir` → pass |
| A13 | P1 | 拆分胖 `ContextManager` 接口（ISP 原则） | `internal/domain/agent/ports/agent/context.go`（8 方法混合 3+ 职责：token 估算/压缩、窗口构建、turn 记录、preload）、`internal/app/context`、`internal/domain/agent/react` | A03 | `ContextManager` 拆成职责明确接口：`WindowBuilder`（BuildWindow）、`Compressor`（Compress/AutoCompact/ShouldCompress/EstimateTokens/BuildSummaryOnly）、`TurnRecorder`（RecordTurn）。调用方只依赖所需的窄接口。 | `ContextManager` interface 方法数 ≤ 3；拆分后接口各自 ≤ 4 方法 |
| A14 | P1 | 统一事件失败语义与因果字段透传（生成/翻译/持久化一致） | `internal/domain/agent/events.go:48-49`（`GetCorrelationID`/`GetCausationID`）、`internal/domain/agent/envelope.go:98-99`（翻译层保留）、`internal/app/agent/coordinator/workflow_event_translator*.go`（failure 字段保留）、`internal/delivery/server/app/file_event_history_store.go:247`（**明确丢弃** `correlationID, causationID`） | - | **必须透传字段**：ErrorStr, PhaseLabel, Recoverable — 三层语义一致（已满足）。**因果字段**：correlationID/causationID 在持久化层恢复时不再丢弃（`file_event_history_store.go:247` 修复）；持久化 schema 加入这两个字段。 | 辅助告警：`grep -n 'correlationID.*causationID.*not persisted' internal/delivery/` → 无匹配；**行为验证**：`go test ./internal/delivery/server/app/ -run TestEventCausalityRoundTrip` → pass（写入带 correlationID+causationID 事件 → 新建 store 实例 → replay → 断言两字段值一致） |
| A15 | P2 | 控制 workflow/event payload 增长（快照瘦身与输出分级） | `internal/domain/workflow`（`WorkflowSnapshot.Nodes[].Output` 为 `any` 类型，无界）、`internal/domain/agent/react/workflow.go`、`internal/app/agent/coordinator`、`web/hooks/useSSE`（前端已有 cap：10K delta、50-event buffer、1000 history） | A14 | 后端 snapshot 输出分级落地（大 output 摘要化或引用化）；payload 体积策略与 A14 事件语义兼容。 | snapshot 单节点 output 有明确字节上限 |

---

## Risk Callouts

### LLM 热路径风险（A06 → A07 → A08）
A06/A07/A08 涉及 LLM 调用的所有路径，是系统最高频热路径。每个切片完成后必须跑**完整 LLM provider 集成测试**（不仅是 `alex dev test`），包括 OpenAI/Anthropic/Kimi/DeepSeek 各 provider 的端到端调用验证。

### 前端状态审计（A05）
TypeScript 定义虽然是 `status: string` 泛型，但前端 UI 逻辑（loading spinner、按钮禁用、状态标签渲染）很可能硬编码了 5 种状态假设。A05 必须包含 `web/components/*` 中所有消费 `task.status` 的组件审计。

### Exceptions 批量清理
123 条 arch exception 全部在 `2026-03-31` 过期。本计划各切片完成后应逐批清理对应 exception，而非等到过期日统一处理。每个切片的 PR 应包含对应 exception 的删除。

---

## Notes

- 执行顺序按 `P0 -> P1 -> P2`，同级切片可并行，但要求无写冲突且满足 `Depends On`。
- 每个切片建议独立 PR，避免跨切片混改。
- 每个切片完成后必须通过统一回归基线：`make check-arch`、`alex dev lint`、`alex dev test`。
- **A06/A07/A08 额外硬门禁**：`alex dev test-llm-integration`（全 provider 端到端）。此项为 PR CI 必过步骤，不是"建议跑"。
- 每个切片的 PR 必须包含对应 `configs/arch/exceptions.yaml` 条目的清理。
- A06/A07 从 P1 提升到 P0：provider 能力模型是 channel 插件化和订阅统一的前置条件，延后会导致 A08/A09/A10 在错误基础上构建。
- A13/A14 从 P2 提升到 P1：接口拆分是 domain 可测试性的基础设施；因果链截断是可观测性的硬伤，不该排在 payload 瘦身之后。
- A11 从 P1 降为 P2：policy 框架已经是 metadata-driven 设计（`PolicySelector` + globs/categories/tags），只是内置默认规则用 Go 常量，外部化优先级低于结构性问题。
