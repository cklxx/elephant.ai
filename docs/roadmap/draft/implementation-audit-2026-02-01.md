# 实现状态审计报告

> **审计日期:** 2026-02-01
> **审计范围:** 全部四条 Track + 共享基础设施
> **方法:** 逐文件核实代码实现，对照 Roadmap 标注

## OKR-First 说明

该审计报告基于原始功能拆解维度，但与当前 OKR-First Roadmap 保持一致：
- Track 1 → O1/KR1.*
- Track 2 → O2/KR2.*
- Track 3 → O3/KR3.*
- Track 4 → O4/KR4.*

如需按 OKR 维度追踪进展，可直接按 Track 对照本报告。

---

## 审计摘要

| Track | M0 完成度 | M1 完成度 | 关键发现 |
|-------|----------|----------|---------|
| Track 1 Agent Core | ~85% | ~15% | ReAct/LLM/上下文/记忆基础扎实；RAG 用 chromem-go 非 pgvector，无 BM25；Token 计数为估算(÷4) |
| Track 2 系统交互层 | ~75% | ~5% | 69+ 工具（非 83）；5 档权限预设（非 3）；12 个技能（非 13）；`internal/coding/` 不存在 |
| Track 3 Lark 生态 | ~90% | ~10% | IM 基础完备；群聊消息全量监听和引用回复已实现（Roadmap 标为待实现）；Lark 全生态(Docs/Sheets/Wiki/Calendar/Tasks)均未实现 |
| Track 4 Shadow DevOps | ~60% | 0% | `internal/devops/` 目录不存在；evaluation/ 套件已实现；共享基础设施完备 |
| 共享基础设施 | ~95% | — | Observability/Config/Auth/Errors/Storage/DI 全部已实现 |

---

## Track 1: 推理与 Agent 核心循环

### 1.1 ReAct 核心循环

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| Think→Act→Observe 循环 | ✅ | ✅ | `runtime.go:runIteration()` → `think()` → `planTools()` → `executeTools()` → `observeTools()` |
| 状态机 (prepare→execute→summarize→persist) | ✅ | ⚙️ | prepare/execute 已实现，persist 缺少正式 checkpoint 机制 |
| 并行工具调用 + 去重 + 超时 | ✅ | ✅ | `tool_batch.go` worker pool + context timeout + CallID 去重 |
| 结构化事件流 | ✅ | ✅ | `events.go` BaseEvent 含唯一 ID + 序列号 + 因果链 |
| 子 Agent 委派 | ✅ | ✅ | `background.go` BackgroundTaskManager + ExternalExecutor 接口 |
| 异常路径全覆盖 | ⚙️ | ⚙️ | LLM 超时(context cancel)、工具失败(tool_batch finalize)✅；上下文溢出❌ |
| 断点快照 | ⚙️ | ⚙️ | 附件快照、子 Agent 状态快照存在；无正式 checkpoint/savepoint |
| 重启自动续跑 | ❌ | ❌ | 无 `Resume()` 函数 |
| 优雅退出 | ❌ | ⚙️ | `handleCancellation()` 处理 context cancel；缺 SIGTERM handler |
| 事件一致性 | ⚙️ | ✅ | `id.NewEventID()` + `SeqCounter.Next()` atomic + 幂等消费 |
| Replan 机制 | ❌ | ❌ | 无 `Replan()` API |
| 子目标分解 | ❌ | ⚙️ | `plan` 工具支持 simple/complex；无自动 DAG 拆解 |
| 用户干预点 | ⚙️ | ✅ | `maybeTriggerPlanReview()` + `runtime_user_input.go` 用户注入 |

### 1.2 LLM 推理引擎

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 统一 LLM 接口 | ✅ | ✅ | 5 个 client 实现（Anthropic/OpenAI/OpenAI-Responses/ARK/Ollama）；OpenAI-compatible 覆盖 DeepSeek/OpenRouter 等 |
| 流式输出 | ✅ | ✅ | `StreamComplete()` 接口 + retry wrapper；各 provider 均有 SSE 解析 |
| 重试与降级 | ✅ | ✅ | `retry_client.go` 指数退避 + `circuit_breaker.go` 熔断 + `user_rate_limit_client.go` 限流 |
| Extended Thinking | ✅ | ✅ | `thinking.go` 检测 Claude 3-7/thinking 模型 + budget token |
| Reasoning Effort | — | ✅ | `thinking.go` 检测 ARK 端点 + O1/O3/R1/DeepSeek reasoning effort |
| 动态模型选择 | ❌ | ⚙️ | `thinking.go` 按模型名切换行为；无成本/复杂度路由 |
| Token 预算管理 | ❌ | ⚙️ | `manager_compress.go:EstimateTokens()` 粗估(÷4)；非精确编码器 |
| 成本核算 | ⚙️ | ✅ | `observability/metrics.go` llmCost counter + `storage/cost_store.go` 会话级成本 |

### 1.3 上下文工程

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 四层分层拼装 | ✅ | ✅ | `manager_window.go:BuildWindow()` System/Policy/Task/Memory |
| 动态压缩 | ✅ | ✅ | `manager_compress.go:AutoCompact()` 阈值 0.8 触发 |
| Token 预算窗口 | ✅ | ✅ | `manager_window.go` 基于 TokenLimit 的滑动窗口 |
| SOP 解析 | ✅ | ✅ | `sop_resolver.go:ResolveKnowledgeRefs()` |
| Lark 聊天历史注入 | ✅ | ✅ | `channels/lark/chat_context.go:fetchRecentChatMessages()` |
| 主动上下文注入 | ⚙️ | ⚙️ | `runtime.go:applyIterationHook()` 每轮注入记忆 + `ProactiveContextRefreshEvent` |
| 上下文优先级排序 | ❌ | ❌ | 固定层序组装，无相关性排序 |

### 1.4 记忆系统

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 对话记忆存储 | ✅ | ✅ | `memory/service.go` Save()/Recall() |
| 向量检索 | ✅ | ⚙️ | **chromem-go 后端（非 pgvector）**；`rag/store.go` cosine similarity；**无 BM25** |
| 保留策略 | ✅ | ✅ | `memory/retention.go` + `filterExpiredEntries()` |
| 消息自动入库 | ⚙️ | ⚙️ | Lark 群聊消息在 channels 层处理，非 memory 包直接管理 |

**Roadmap 需修正：**
- 向量检索：后端为 chromem-go 而非 pgvector，无 BM25 混合排序
- Token 计数为粗估（÷4），非 tiktoken

---

## Track 2: 系统交互层

### 2.1 工具引擎

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 工具数量 | 83 个 | **69+** | `toolregistry/registry.go` 注册 69+ 工具（含 static/dynamic/mcp 三类） |
| 工具注册与元数据 | ✅ | ✅ | `ToolDefinition` 含 Name/Description/Parameters(JSON Schema)/Metadata |
| 权限预设 | 3 档 | **5 档** | Full/ReadOnly/Safe/Sandbox/Architect（`presets/tools.go`） |
| 并发执行 | ✅ | ✅ | `tool_batch.go` worker pool + WaitGroup |
| 超时/重试/限流 | ✅ | ⚙️ | web fetch 有 TTL 缓存；code_execute 有超时；**无全局工具超时/重试策略** |
| SLA 基线采集 | ❌ | ❌ | 未实现 |

### 2.2 沙箱与执行环境

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 代码执行沙箱 | ✅ | ✅ | `code_execute_local.go` Python/Go/JS/Bash + 超时控制 |
| Shell 执行 | ✅ | ✅ | `bash_local.go`(本地) + `sandbox_shell.go`(沙箱) + build tag 切换 |
| 浏览器自动化 | ✅ | ✅ | `sandbox_browser.go` browser_action/info/screenshot/dom |
| 文件操作 | ✅ | ✅ | fileops(本地) + sandbox(沙箱) 双路径 + path_resolver |

### 2.3 Coding Agent Gateway

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| `internal/coding/` 目录 | ❌ 待实现 | ❌ **不存在** | 确认未创建 |
| Gateway 抽象接口 | ❌ | ❌ | 不存在；`acp_executor` 具有部分类似功能（gRPC 调度） |
| CLI adapter | ❌ | ❌ | 无 Claude Code/Codex/Kimi adapter |
| 构建验证 | ❌ | ❌ | 无 |

### 2.4 数据与文件处理

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| 文本文件读写 | ✅ | ✅ | file_read/file_write/file_edit/list_files |
| 图像生成 | ✅ | ✅ | text_to_image/image_to_image (Seedream via ARK) |
| PPT 生成 | ✅ | ✅ | pptx_from_images (gofpdf) |
| 视频生成 | ✅ | ✅ | video_generate (Seedream video model) |

### 2.5 技能系统

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| Markdown 驱动技能 | 13 个 | **12 个** | a2ui-templates, best-practice-search, deep-research, email-drafting, json-render-templates, meeting-notes, moltbook-posting, okr-management, ppt-deck, research-briefing, self-test, video-production |
| 技能注册与暴露 | ✅ | ✅ | `internal/skills/skills.go` + `tools/builtin/session/skills.go` |

**Roadmap 需修正：**
- 工具数量：83 → 69+
- 权限预设：3 档 → 5 档（Full/ReadOnly/Safe/Sandbox/Architect）
- 技能数量：13 → 12
- 超时/重试/限流：✅ → ⚙️（仅部分工具有超时，无全局策略）

---

## Track 3: 人类集成交互 — Lark 全生态

### 3.1 Lark IM 消息层

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| WebSocket 事件循环 | ✅ | ✅ | `gateway.go:Start()` + `handleMessage()` + 10min 去重缓存 |
| 群聊历史获取 | ✅ | ✅ | `chat_context.go:fetchRecentChatMessages()` 分页 20-50 条 |
| Emoji 进度标记 | ✅ | ✅ | `emoji_reactions.go` 14 种 emoji + 工具 start/end 标记 |
| 审批门禁 | ✅ | ✅ | `cli_approver.go` + `toolregistry` dangerous tool wrapper |
| 富附件发送 | ✅ | ✅ | `gateway.go:sendAttachments()` 图片/PDF/Doc/XLS/PPT/MP4 上传 |
| 主动发送消息 | ✅ | ✅ | `larktools/send_message.go` |
| **群聊消息自动感知** | ❌ 待实现 | **✅ 已实现** | `gateway.go:handleMessage()` 处理所有消息，无 @mention 过滤 |
| 主动摘要 | ❌ | ❌ | 未实现 |
| 定时提醒 | ❌ | ⚙️ | `internal/scheduler/` cron + LarkNotifier 基础设施存在；无聊天提取 |
| 智能卡片交互 | ❌ | ❌ | 只识别 interactive 消息类型，无卡片构建能力 |
| **消息引用回复** | ❌ 待实现 | **✅ 已实现** | `sdk_messenger.go:ReplyMessage()` + `lark_send_message` 从消息上下文派生 reply target（无显式 reply_to 参数） |
| 消息类型丰富化 | ❌ | ❌ | 仅文本消息 |

### 3.2 Lark Open API 封装层

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| `internal/lark/` 目录 | ❌ 待实现 | ❌ **不存在** | 直接使用 Lark SDK (`larksuite/oapi-sdk-go/v3`) |
| 统一 API Client | ❌ | ❌ | SDK 直连，无封装层 |
| 认证管理 | ❌ | ⚙️ | SDK 内部处理 token 刷新 |
| 限流与重试 | ❌ | ❌ | 无限流包装；进度消息有 2s flush interval |
| 错误码映射 | ❌ | ❌ | SDK 错误直传 |

### 3.3 Lark 全生态接入

| 模块 | 状态 | 说明 |
|------|------|------|
| Lark Docs (读写/评论) | ❌ | `internal/lark/docs/` 不存在 |
| Lark Sheets/Bitable | ❌ | `internal/lark/sheets/` 不存在 |
| Lark Wiki | ❌ | `internal/lark/wiki/` 不存在 |
| Lark Calendar | ❌ | `internal/lark/calendar/` 不存在 |
| Lark Tasks | ❌ | `internal/lark/tasks/` 不存在 |
| Lark Approval | ❌ | `internal/lark/approval/` 不存在 |

### 3.4 Lark 工具注册

| 工具名 | Roadmap | 实际 | 说明 |
|--------|---------|------|------|
| `lark_send_message` | ✅ | ✅ | send_message.go + reply target 从消息上下文派生（无显式 reply_to 参数） |
| `lark_chat_history` | ✅ | ✅ | chat_history.go + 分页/时间过滤 |
| `lark_doc_read` | ❌ | ❌ | |
| `lark_doc_write` | ❌ | ❌ | |
| `lark_doc_comment` | ❌ | ❌ | |
| `lark_sheet_read` | ❌ | ❌ | |
| `lark_sheet_write` | ❌ | ❌ | |
| `lark_wiki_search` | ❌ | ❌ | |
| `lark_wiki_write` | ❌ | ❌ | |
| `lark_calendar_query` | ❌ | ❌ | |
| `lark_calendar_create` | ❌ | ❌ | |
| `lark_task_manage` | ❌ | ❌ | |
| `lark_approval_submit` | ❌ | ❌ | |

### 3.5 Web Dashboard

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| SSE 流式渲染 | ✅ | ✅ | `lib/api.ts` + `hooks/useSSE/` (6 文件) |
| 对话界面 | ✅ | ✅ | `app/conversation/` 全套组件 |
| 附件/工具可视化 | ✅ | ✅ | A2UI + artifact rendering |
| 会话管理 | ✅ | ✅ | `useSessionStore.ts` |
| 成本追踪 | ✅ | ✅ | Cost component + API |
| 子 Agent 执行树 | ⚙️ | ⚙️ | CLI 有 subagent_display；Web 部分 |

### 3.6 CLI 界面

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| TUI 交互 | ✅ | ✅ | `tui_*.go` 7 个文件 |
| 审批 | ✅ | ✅ | `cli_approver.go` |
| 会话持久化 | ✅ | ✅ | session state storage |

**Roadmap 需修正：**
- 群聊消息自动感知：❌ → ✅（已实现，无 @mention 过滤）
- 消息引用回复：❌ → ✅（sdk_messenger.go ReplyMessage）
- 定时提醒：❌ → ⚙️（scheduler 基础设施存在）

---

## Track 4: 自主迭代升级 — 影子 Agent DevOps

### 4.1 DevOps 模块

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| `internal/devops/` 目录 | — | ❌ **不存在** | 整个 Track 4 代码架构未创建 |
| 信号采集 | ❌ | ❌ | |
| Shadow Agent 框架 | ❌ | ❌ | |
| 合并自动化 | ❌ | ❌ | |
| 发布自动化 | ❌ | ❌ | Makefile 有 build-all/release，但未抽象到 devops 层 |

### 4.2 评测套件

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| SWE-Bench 套件 | ✅ | ✅ | `evaluation/swe_bench/` ~17K 行 Go 代码 |
| Agent Eval 套件 | ✅ | ✅ | `evaluation/agent_eval/` ~13K 行 Go 代码 |
| Lint + 单元测试 | ✅ | ✅ | `ci.yml` golangci-lint + `go test -race` + Codecov |

### 4.3 可观测基础（共享基础设施）

| 项目 | Roadmap 标注 | 实际状态 | 说明 |
|------|-------------|---------|------|
| OpenTelemetry Traces | ✅ | ✅ | `tracing.go` OTLP + Zipkin exporters |
| Prometheus Metrics | ✅ | ✅ | `metrics.go` 16K 行；15+ 指标（LLM/Tool/HTTP/SSE） |
| 结构化日志 | ✅ | ✅ | `logger.go` context-aware structured logging |
| 会话成本核算 | ✅ | ✅ | `storage/cost_store.go` JSONL + session 索引 |

---

## 共享基础设施

| 模块 | 状态 | 代码量 | 关键实现 |
|------|------|--------|---------|
| **Observability** | ✅ 完备 | ~2K 行 | OpenTelemetry + Prometheus + 结构化日志 |
| **Config** | ✅ 完备 | ~6.5K 行 | YAML 配置 + 环境变量覆盖 + 60+ 配置项 |
| **Auth** | ✅ 完备 | ~2.4K 行 | JWT + OAuth(Google) + Argon2id + Postgres/Memory store |
| **Errors** | ✅ 完备 | ~28K 行 | 错误分类 + 指数退避重试 + 熔断器 |
| **Storage** | ✅ 基础 | ~278 行 | 文件 JSONL 成本存储 + session 索引 |
| **DI** | ✅ 完备 | ~10.6K 行 | Container + Builder + lazy init |
| **CI/CD** | ✅ 完备 | — | Lint/Test/Build/Release/Security workflows |
| **Docker** | ✅ 完备 | — | Multi-stage Dockerfile × 3 + Docker Compose |

---

## 需要修正的 Roadmap 标注

### 数字修正

| 位置 | 原值 | 实际值 | 原因 |
|------|------|--------|------|
| Track 2 工具数量 | 83 个 | 69+ | registry.go 实际注册数 |
| Track 2 权限预设 | 3 档 | 5 档 | Full/ReadOnly/Safe/Sandbox/Architect |
| Track 2 技能数量 | 13 个 | 12 个 | skills/ 目录实际计数 |
| Track 1 LLM 提供商 | 7+ | 5 client + 7+ config | 5 个 client 实现，通过 OpenAI-compatible 支持更多 |

### 状态修正（已实现但标为待实现）

| 位置 | 项目 | 原标注 | 修正为 | 证据 |
|------|------|--------|--------|------|
| Track 3 M1 | 群聊消息自动感知 | ❌ 待实现 | ✅ 已实现 | `gateway.go:handleMessage()` 无 @mention 过滤 |
| Track 3 M1 | 消息引用回复 | ❌ 待实现 | ✅ 已实现 | `sdk_messenger.go:ReplyMessage()` |
| Track 1 M0 | 事件一致性 | ⚙️ 部分 | ✅ 已实现 | unique ID + atomic seq + idempotent |
| Track 1 M0 | 用户干预点 (M1) | ⚙️ 部分 | ✅ 已实现 | plan review + user input injection |

### 状态修正（标为已实现但实际为部分）

| 位置 | 项目 | 原标注 | 修正为 | 原因 |
|------|------|--------|--------|------|
| Track 2 M0 | 超时/重试/限流 | ✅ 已实现 | ⚙️ 部分 | 仅 web fetch TTL + code_execute timeout，无全局策略 |
| Track 1 M0 | 向量检索 | ✅ 已实现 | ⚙️ 部分 | chromem-go 非 pgvector，无 BM25 |

---

## 按 M0 优先级的关键缺口

### P0 — M0 未完成项（阻塞开箱即用）

| Track | 缺口 | 影响 |
|-------|------|------|
| T2 | Coding Agent Gateway 不存在 | 编码能力完全不可用 |
| T2 | 本地 CLI 自动探测 | Codex/Claude Code 订阅无法自动识别 |
| T1 | 重启自动续跑 | 进程重启后丢失执行状态 |
| T3 | Lark Open API 封装层 | 后续 Lark 生态接入无基础 |

### P1 — M0 质量提升项

| Track | 缺口 | 影响 |
|-------|------|------|
| T1 | Token 计数精确化 | 成本估算不准、上下文裁剪不精确 |
| T1 | ✅ 已完成：优雅退出 (SIGTERM) | 工具调用中断风险已缓解 |
| T2 | 全局工具超时/重试 | 工具调用挂起无兜底 |
| T4 | ✅ 已完成：评测 CI 门禁（可选） | 支持手动/Tag 触发快速评测 |

---

## TODO/DONE（2026-02-01 后续）

- [x] 优雅退出（CLI SIGTERM 处理，主进程取消上下文并 shutdown）
- [x] 评测 CI 门禁（manual + `eval/` tag 触发，quick eval 脚本）
- [ ] 全局工具超时/重试：本轮暂缓（待决策）
