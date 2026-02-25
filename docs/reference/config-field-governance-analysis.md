# 配置字段与选项治理分析（全域）
Last updated: 2026-02-25

## 1. 文档目的

本文档面向工程治理，不替代 `CONFIG.md` 的“使用说明”角色。重点回答：

1. 字段和选项到底做什么。
2. 影响面与依赖耦合在哪里。
3. 哪些是必须保留、可删除、或需要迁移后删除。
4. 如何按低风险顺序统一收口。

## 2. 事实依据（代码位置）

### 2.1 配置模型与默认值

- `internal/shared/config/types.go`
- `internal/shared/config/load.go`
- `internal/shared/config/file_config.go`
- `internal/shared/config/runtime_env_loader.go`
- `internal/shared/config/provider_resolver.go`

### 2.2 运行时映射与消费

- `internal/app/di/runtime_config.go`
- `internal/app/di/container.go`
- `internal/app/di/container_builder.go`
- `internal/app/di/builder_hooks.go`
- `internal/app/agent/coordinator/config_resolver.go`
- `internal/app/agent/preparation/service.go`

### 2.3 服务端配置消费

- `internal/delivery/server/bootstrap/config.go`
- `internal/delivery/server/bootstrap/foundation.go`
- `internal/infra/attachments/store.go`

### 2.4 工具安全与审批

- `internal/domain/agent/ports/tools.go`
- `internal/app/toolregistry/policy.go`
- `internal/app/toolregistry/retry.go`
- `internal/app/agent/kernel/executor.go`

### 2.5 默认配置样例

- `configs/config.yaml`

## 3. 配置生效链（从输入到行为）

```text
defaults
  -> YAML file
  -> env overrides
  -> provider/profile resolve
  -> RuntimeConfig
  -> DI Config projection
  -> module consumption (agent/tool/scheduler/server/channels/attachments)
```

关键点：

- `load.go` 明确执行优先级：默认值 -> 文件 -> env -> overrides -> normalize/resolve。
- `runtime_config.go` 是 runtime 到 DI 的主收口点。
- `bootstrap/config.go` 叠加 server/channels/session/attachments 的服务侧配置。

## 4. 全域分类分析（作用、影响面、耦合、必要性）

| 配置域 | 主要字段族 | 作用 | 影响面 | 耦合等级 | 必要性 |
| --- | --- | --- | --- | --- | --- |
| `runtime.*` | `llm_*`, `api_key`, `base_url`, `tool_policy`, `http_limits`, `proactive`, `external_agents` | 核心运行时行为与能力开关 | 全系统 | 高 | 必保留 |
| `server.*` | `port`, `enable_mcp`, `stream_*`, `rate_limit_*`, `non_stream_timeout_seconds` | 服务入口与流控 | 服务稳定性 | 高 | 必保留 |
| `channels.lark.*` | `enabled`, `app_id`, `app_secret`, `persistence.*`, `browser.*`, `plan_review_*`, `auto_upload_*` | Lark 通道行为 | 通道体验与状态持久化 | 中高 | 保留并收口 |
| `runtime.proactive.*` | `prompt.*`, `skills.*`, `scheduler.*`, `timer.*`, `attention.*` | 主动上下文/调度/反馈 | agent 行为和成本 | 高 | 主体保留 |
| `runtime.proactive.memory.index.*` | `db_path`, `chunk_tokens`, `min_score`, `embedder_model` | 记忆索引质量/成本 | memory/rag | 中 | 必保留 |
| `runtime.external_agents.*` | `codex/claude_code/kimi.*`, `approval_policy`, `sandbox` | 外部代理编排与权限 | 安全与自动化 | 中高 | 必保留并强校验 |
| `attachments.*` | `provider`, `dir`, `cloudflare_*`, `presign_ttl` | 附件后端选择 | 附件可用性 | 中 | 保留并结构化 |
| `observability.*` | `logging.*`, `metrics.*`, `tracing.*` | 监控与诊断 | 全链路 | 高 | 必保留 |
| `task_execution.*` | `lease_*`, `max_in_flight`, `resume_claim_batch_size` | 任务租约并发控制 | 任务执行 | 高 | 必保留 |
| `event_history.*` | `retention_days`, `max_sessions`, `session_ttl_seconds`, `max_events` | SSE 历史与回放 | 会话与流体验 | 中高 | 必保留 |

## 5. 高风险选项清单（默认行为、失败模式、建议）

### 5.1 `ACP_EXECUTOR_AUTO_APPROVE`

- 代码证据
  - 默认值：`internal/shared/config/load.go`
  - env 覆盖：`internal/shared/config/runtime_env_loader.go`
  - 执行启用：`internal/app/agent/kernel/executor.go`
- 失败模式
  - 高风险工具可能绕过审批直接执行。
- 建议
  - 布尔改枚举：`approval_mode: manual | auto | audit_only`。
  - 生产默认 `manual`，并在启动时打印当前审批模式。

### 5.2 `external_agents.*` 的 `approval_policy` + `sandbox`

- 代码证据
  - 默认结构与默认值：`internal/shared/config/types.go`
  - 消费：`internal/app/di/container_builder.go`, `internal/infra/external/registry.go`
- 失败模式
  - 代理被启用后在高权限沙箱中自动执行，风险放大。
- 建议
  - 启动校验规则：`enabled=true` 必须显式声明安全策略组合。
  - 禁止隐式 fallback 到危险组合。

### 5.3 `Dangerous` 与 `SafetyLevel` 双轨

- 代码证据
  - 定义与 fallback：`internal/domain/agent/ports/tools.go`
  - policy/retry 使用：`internal/app/toolregistry/policy.go`, `retry.go`
- 失败模式
  - 仅布尔 `Dangerous` 容易语义不全或漏标。
- 建议
  - 逐步收口到 `SafetyLevel(L1-L4)` 单轨，`Dangerous` 只做过渡兼容。

### 5.4 `enable_mcp`

- 代码证据
  - 字段：`internal/shared/config/file_config.go`
  - 消费：`internal/delivery/server/bootstrap/config.go`, `internal/app/di/container.go`
- 失败模式
  - 主服务可运行但 MCP 能力静默失效，现场排障成本高。
- 建议
  - 增加清晰降级日志和 readiness 标识。

## 6. 可删除与收敛项（按证据分类）

### 6.1 可立即删除（低风险）

1. `runtime.proactive.scheduler.leader_lock_enabled`
2. `runtime.proactive.scheduler.leader_lock_name`
3. `runtime.proactive.scheduler.leader_lock_acquire_interval_seconds`

证据：

- 定义：`internal/shared/config/types.go` 与 `file_config.go`
- merge：`internal/shared/config/proactive_merge.go`
- 调度消费侧：`internal/app/scheduler/scheduler.go` 只消费注入的 `LeaderLock interface`，不消费上述字段。

4. `runtime.proactive.scheduler.triggers[].approval_required`
5. `runtime.proactive.scheduler.triggers[].risk`

证据：

- 定义：`internal/shared/config/types.go` 与 `file_config.go`
- merge：`internal/shared/config/proactive_merge.go`
- 调度注册：`internal/app/scheduler/scheduler.go` 仅使用 `name/schedule/task/channel/user_id/chat_id`。

### 6.2 保留但需收敛

1. `runtime.proactive.attention.*`
- 有定义和 merge/load 默认补全（`types.go`, `proactive_merge.go`, `load.go`），当前未形成明确业务消费链。
- 处理建议：短期标注 `reserved` 或移除；二选一，不建议“存在但无行为”长期保留。

2. `proactive.prompt.bootstrap_files` 双默认源
- 代码默认与 `configs/config.yaml` 同时维护，易漂移。
- 建议只保留一个默认源（优先代码），YAML 仅写覆盖项。

### 6.3 迁移后删除

1. `ToolMetadata.Dangerous`
- 待 `SafetyLevel` 覆盖全部工具后，删除 fallback 路径。

## 7. 影响链与耦合关系

### 7.1 LLM 与工具策略链

```text
runtime.llm_* + runtime.tool_policy
  -> RuntimeConfig
  -> di.Config
  -> llmclient + toolregistry(policy/retry)
  -> provider request + approval behavior
```

影响：全局且安全敏感。

### 7.2 Proactive 链

```text
runtime.proactive.*
  -> di builder/hooks
  -> preparation/coordinator/scheduler/timer/memory
  -> 主动触发、上下文注入、技能行为
```

影响：高，跨层（app/domain/infra）耦合明显。

### 7.3 External agents 链

```text
runtime.external_agents.*
  -> di container_builder
  -> external registry
  -> local binary execution (approval/sandbox/env)
```

影响：高，安全与资源双敏感。

### 7.4 Server/channels/attachments 链

```text
server.* + channels.lark.* + attachments.*
  -> bootstrap config/foundation
  -> HTTP/SSE/task/event history + Lark gateway + attachment store
```

影响：对外行为、稳定性和用户体验。

## 8. 统一收口设计（建议）

### 8.1 收口原则

1. 单一事实源：配置语义只在 `internal/shared/config` 定义。
2. 单向映射：`RuntimeConfig -> di.Config -> 模块配置`。
3. 风险枚举化：安全相关配置不使用松散布尔。
4. 组合可验证：启动期校验危险组合，拒绝不安全运行。

### 8.2 目标结构（示例）

```yaml
runtime:
  llm: {}
  execution: {}
  safety:
    approval_mode: manual
    tool_default_safety_level: L1
  proactive: {}
  integrations:
    external_agents: {}
server:
  http: {}
  stream_guard: {}
  task_execution: {}
  event_history: {}
channels:
  lark: {}
attachments:
  provider: local
  local: {}
  cloudflare: {}
observability: {}
```

## 9. 分阶段实施路线（可执行）

### Phase 1（Quick Wins，1-2天）

1. 删除无消费字段：`leader_lock_*`、`trigger.approval_required/risk`。
2. 收敛 `bootstrap_files` 双默认源到单一来源。
3. MCP 关闭时增加显式降级日志。

验收建议：

- `go test ./internal/shared/config/...`
- `go test ./internal/app/scheduler/...`

### Phase 2（安全收口，2-4天）

1. `ACP_EXECUTOR_AUTO_APPROVE` 改为 `approval_mode` 枚举。
2. external agents 增加组合校验（enabled + approval_policy + sandbox）。
3. 统一工具安全等级为 `SafetyLevel` 主路径。

验收建议：

- `go test ./internal/app/toolregistry/...`
- `go test ./internal/app/agent/kernel/...`

### Phase 3（解耦重构，3-5天）

1. 减少 runtime 结构跨层直接引用，强化 DI 边界。
2. 拆分 proactive/external_agents builder，降低 container_builder 膨胀。

验收建议：

- `go test ./internal/app/di/... ./internal/app/agent/...`

### Phase 4（持续治理）

1. `config validate` 纳入 CI。
2. 新增字段必须声明：作用、默认源、消费者、风险等级、迁移策略。
3. 配置变更必须同步 `CONFIG.md` 与治理文档差异说明。

## 10. 结论

优先做“删除无消费字段 + 安全策略枚举化”。这两步收益最高、风险可控，且能显著降低后续配置治理成本。随后再推进跨层解耦，形成长期可演进的配置架构。

