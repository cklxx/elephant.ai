# Task Split: Claude Code vs Codex

Created: 2026-02-01
Updated: 2026-02-01 22:00

## Splitting Principle

- **Claude Code** (interactive): 快速任务、架构设计与规划、前端任务、轻量多文件串联、快速验证迭代。强在即时反馈和快速周转。
- **Codex** (deep batch): 复杂实现、疑难 debug、大规模重构、需要深度推理的跨包实现。强在深度思考和一次性高质量交付。

---

## P0: Blocks North Star

### Codex Tasks (complex, needs deep reasoning)

| # | Task | Why Codex | Touches |
|---|------|-----------|---------|
| X1 | Design & implement `internal/lark/` API client layer | 架构决策多（thin wrapper vs facade、错误映射、token 刷新、限流），需要深度推理一次做对 | `internal/lark/`, `internal/channels/lark/`, config |
| X2 | ReAct checkpoint + resume | 核心引擎改造：序列化状态、定义恢复点、处理 tool-in-flight。逻辑复杂，需要完整思考 | `internal/agent/domain/react/` |
| X3 | Global tool timeout/retry strategy | 跨切面设计：retry policy、circuit breaker、timeout 继承。需要考虑所有 tool 的边界情况 | `internal/tools/`, `internal/toolregistry/` |
| X4 | Integration test: full calendar flow E2E | 需要 mock Lark client、approval 流程模拟、scheduler trigger → tool 执行 → 结果验证，多包协调 | `internal/tools/builtin/larktools/`, `internal/toolregistry/`, `internal/scheduler/` |
| X5 | Token counting: replace len/4 with tiktoken-go | 需要理解各 provider 的 tokenizer 差异，选择正确的 encoding，处理 fallback | `internal/llm/` |
| X6 | Graceful shutdown drain logic | 需要理解所有 in-flight 资源（tool 执行、SSE 连接、scheduler job），设计 drain 顺序 | `cmd/elephant/main.go`, 多个子系统 |

### Claude Code Tasks (quick, architectural guidance, wiring)

| # | Task | Why Claude Code | Touches |
|---|------|-----------------|---------|
| C1 | Calendar CRUD tools (update/delete) | 模式固定（照抄 calendar_create.go），快速出活 | `internal/tools/builtin/larktools/` |
| C2 | Task CRUD tools (update/delete) | 同上，模式固定 | `internal/tools/builtin/larktools/` |
| C3 | Tool registration wiring | 在 registry.go 加几行注册代码，快速 | `internal/toolregistry/registry.go` |
| C4 | Extend approval gate for new tools | 已有 Dangerous flag 机制，按现有模式扩展 | `internal/toolregistry/` |
| C5 | Unit tests for C1-C2 | 照抄现有 test pattern，快速覆盖 | `*_test.go` |
| C6 | NSM metric stubs | 在 MetricsCollector 加 3 个 counter/histogram 定义，机械操作 | `internal/observability/metrics.go` |
| C7 | Wire scheduler reminders (basic) | 在现有 OKR trigger 模式上加 calendar trigger，模式清晰 | `internal/scheduler/` |

---

## P1: Quality

### Codex Tasks

| # | Task | Why Codex |
|---|------|-----------|
| X7 | NSM metric collection wiring | 度量点分布在多个包（task 完成时算 WTCR、用户反馈算 Accuracy），需要全局思考 |
| X8 | Memory restructuring (D5) | 架构改造：layered FileStore、daily summary pipeline、long-term extraction。跨 memory/context/agent |
| X9 | Tool policy framework (D1) | 需要设计 allow/deny 规则、per-context 过滤、policy 评估引擎。新子系统 |

### Claude Code Tasks

| # | Task | Why Claude Code |
|---|------|-----------------|
| C8 | Scheduler enhancement: job persistence config | 配置层面的改动 + 存储后端选择，快速决策 |
| C9 | Calendar conflict detection util | 纯函数：给定时间范围查冲突，逻辑简单 |
| C10 | Proactive context injection: calendar summary builder | 纯函数：今日事件 → markdown 摘要，快速实现 |

---

## P2: Next Wave

### Codex Tasks

| # | Task | Why Codex |
|---|------|-----------|
| X10 | Replan + sub-goal decomposition | ReAct 核心扩展：何时触发 replan、子目标分解策略、状态管理。需要深度设计 |
| X11 | Scheduler enhancement (D4) full | Job 持久化、cooldown、并发控制。需要考虑故障恢复和状态一致性 |
| X12 | Coding Agent Gateway | 全新子系统：plan → generate → test → fix pipeline。从零设计 |
| X13 | Shadow Agent framework | 自迭代框架 + mandatory approval gates。复杂度高 |

### Claude Code Tasks

| # | Task | Why Claude Code |
|---|------|-----------------|
| C11 | Calendar/Tasks full CRUD 补全 | P0 基础上补 batch 操作、multi-calendar 支持 |
| C12 | Proactive reminders: intent → draft → confirm UI | 串联前后端，快速迭代 |

---

## Execution Order

```
Phase 1 (now):
  Codex:  X1 (Lark API client layer)
  Claude: C1,C2,C3,C4,C5 (CRUD tools + tests + wiring)

Phase 2 (after X1 lands):
  Codex:  X4 (E2E integration test)
  Claude: C6,C7 (metrics stubs + scheduler reminders)

Phase 3 (P1):
  Codex:  X2,X3,X5,X6 (checkpoint, timeout, token, shutdown)
  Claude: C8,C9,C10 (scheduler config, conflict detection, summary builder)

Phase 4 (P1 continued):
  Codex:  X7,X8,X9 (NSM wiring, memory D5, tool policy D1)

Phase 5 (P2):
  Codex:  X10,X11,X12,X13 (replan, scheduler D4, coding gateway, shadow agent)
  Claude: C11,C12 (CRUD 补全, proactive UI)
```

---

## Codex Prompt Template

```
Context: elephant.ai Go project.

Codebase references (read these first):
- [pattern file] for implementation pattern
- [related files] for integration points
- docs/roadmap/roadmap.md for overall context

Task: [specific deliverable]

Requirements:
- [concrete specs with types, API calls, error handling]
- [test coverage expectations]
- [integration points to respect]

Constraints:
- Follow existing patterns exactly (BaseTool, ports.ToolDefinition, shared.LarkClientFromContext)
- Run `go vet ./...` and `go test ./...` before delivering
- No unnecessary defensive code; trust context invariants
```
