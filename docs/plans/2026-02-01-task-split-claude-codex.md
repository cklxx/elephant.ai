# Task Split: Claude Code vs Codex

Created: 2026-02-01
Updated: 2026-02-01 23:30

## Splitting Principle

- **Claude Code**: 快速任务、架构设计与规划、前端、拆解后的子步骤、需要交互反馈的迭代。也能做特定复杂任务（尤其是架构设计、跨文件串联、前端）。
- **Codex**: 不可拆的深度实现，需要长时间推理和一次性正确交付的核心引擎改造。
- **核心策略**: 能拆则拆，拆成子步骤给 Claude Code 快速迭代；只有真正不可拆的原子复杂任务才留给 Codex。

---

## P0: Blocks North Star

### Lark API Client Layer — 拆解

原任务 "Design & implement `internal/lark/`" 拆为 4 步:

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C1 | 设计 Lark API client 接口 + 目录结构 | Claude Code | 架构决策，需要交互讨论 |
| C2 | 实现 auth token 管理 (app_access_token 获取/缓存/刷新) | Claude Code | 模式固定 (HTTP call + cache)，快速 |
| C3 | 实现 Calendar API wrapper (list/get/create/patch/delete) | Claude Code | 按 Lark Open API 文档逐个包装，模式重复 |
| C4 | 实现 Task API wrapper (list/get/create/patch/delete) | Claude Code | 同上 |

### Calendar & Task Tools — Claude Code

| # | Task | Owner | Why |
|---|------|-------|-----|
| C5 | Calendar CRUD tools (update/delete) | Claude Code | 照抄 calendar_create.go 模式 |
| C6 | Task CRUD tools (update/delete) | Claude Code | 照抄 task_manage.go 模式 |
| C7 | Tool registration wiring | Claude Code | registry.go 加几行 |
| C8 | Extend approval gate for new tools | Claude Code | Dangerous flag 按现有模式 |
| C9 | Unit tests for C5-C6 | Claude Code | 照抄现有 test pattern |

### Scheduler Reminders — Claude Code

| # | Task | Owner | Why |
|---|------|-------|-----|
| C10 | Wire calendar trigger into scheduler | Claude Code | 在现有 OKR trigger 模式上加，模式清晰 |

### E2E Integration Test — Codex

| # | Task | Owner | Why |
|---|------|-------|-----|
| X1 | Full calendar flow E2E test | Codex | mock Lark client + approval 流程 + scheduler trigger + 结果验证，需要一次性把所有 mock 和断言想清楚 |

---

## P1: M0 Quality

### ReAct Checkpoint + Resume — 拆解

原任务不可完全拆，但可以分阶段:

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C11 | 定义 checkpoint schema (JSON/protobuf) + state 序列化接口 | Claude Code | 架构设计 + 接口定义，需要讨论 |
| X2 | 实现 checkpoint 写入/恢复 + tool-in-flight recovery | Codex | 核心引擎改造，状态机逻辑复杂，需要深度推理一次做对 |
| C12 | Checkpoint 集成测试 + CLI resume 命令 | Claude Code | 基于 X2 的接口写测试和 CLI 入口，快速 |

### Global Tool Timeout/Retry — 拆解

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C13 | 定义 timeout/retry config schema + ToolPolicy 接口 | Claude Code | 配置设计 |
| X3 | 实现 retry middleware (exponential backoff + circuit breaker + context propagation) | Codex | 边界情况多（partial failure、context cancel、nested timeout），需要深度推理 |
| C14 | 集成到 registry wrapper chain + 配置加载 | Claude Code | 接线工作 |

### Graceful Shutdown — 拆解

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C15 | 定义 Drainable 接口 + 各子系统实现 drain() | Claude Code | 每个子系统加一个方法，模式固定 |
| C16 | main.go 中按顺序调用 drain + 超时兜底 | Claude Code | 逻辑简单：for each drainable, drain with timeout |

### Other P1 — Claude Code

| # | Task | Owner | Why |
|---|------|-------|-----|
| C17 | NSM metric stubs (WTCR/TimeSaved/Accuracy counters) | Claude Code | MetricsCollector 加定义 |
| C18 | Token counting: integrate tiktoken-go | Claude Code | 替换 len/4，单文件改动，provider→encoding 映射表 |

---

## P2: Next Wave (M1)

### Memory Restructuring (D5) — Codex

| # | Task | Owner | Why |
|---|------|-------|-----|
| X4 | Memory D5: layered FileStore + daily summary + long-term extraction | Codex | 跨 memory/context/agent 的架构改造，需要完整设计数据流和迁移策略 |

### Tool Policy Framework (D1) — 拆解

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C19 | 设计 policy schema (YAML rules, per-context scoping) | Claude Code | 配置格式设计 |
| X5 | 实现 policy evaluation engine + registry integration | Codex | 规则匹配逻辑、优先级冲突解决、性能要求，需要深度推理 |
| C20 | Default policies + 文档 | Claude Code | 快速 |

### Replan + Sub-goal Decomposition — Codex

| # | Task | Owner | Why |
|---|------|-------|-----|
| X6 | Replan trigger detection + sub-goal state machine | Codex | ReAct 核心扩展，何时 replan、如何分解、子目标状态管理，不可拆 |

### Scheduler Enhancement (D4) — 拆解

| # | Sub-task | Owner | Why |
|---|----------|-------|-----|
| C21 | Job persistence: 选择存储后端 + schema 设计 | Claude Code | 架构决策 |
| X7 | 实现 JobStore + cooldown + concurrency control + 故障恢复 | Codex | 状态一致性和并发逻辑复杂 |
| C22 | 集成测试 + config wiring | Claude Code | 快速 |

### Other P2 — Claude Code

| # | Task | Owner | Why |
|---|------|-------|-----|
| C23 | Calendar/Tasks full CRUD 补全 (batch, multi-calendar) | Claude Code | P0 模式扩展 |
| C24 | Calendar conflict detection util | Claude Code | 纯函数 |
| C25 | Proactive context injection: calendar summary builder | Claude Code | 纯函数 |
| C26 | Proactive reminders: intent → draft → confirm flow | Claude Code | 串联前后端 |

---

## P3: Future (M2+)

| # | Task | Owner | Why |
|---|------|-------|-----|
| X8 | Coding Agent Gateway | Codex | 全新子系统从零设计 |
| X9 | Shadow Agent framework | Codex | 全新子系统 + mandatory approval gates |

---

## Summary

| Owner | Count | 任务类型 |
|-------|-------|---------|
| Claude Code | 26 | 架构设计、接口定义、模式复制、接线、测试、配置、前端、纯函数 |
| Codex | 9 | 核心引擎改造、复杂状态机、跨包深度实现、全新子系统 |

**拆解效果**: 原 13 个 Codex 任务 → 9 个，其中 4 个被拆解后大部分子步骤转给 Claude Code。

---

## Execution Order

```
Phase 1 — P0 Core (parallel):
  Claude: C1→C2→C3,C4 (Lark client layer, sequential)
          C5,C6,C7,C8,C9 (CRUD tools + wiring, parallel)
          C10 (scheduler reminders)

Phase 2 — P0 Validation:
  Codex:  X1 (E2E integration test, after Phase 1)

Phase 3 — P1 (parallel):
  Claude: C11 (checkpoint schema) → C13 (timeout config) → C15,C16 (shutdown)
          C17,C18 (metrics, token counting)
  Codex:  X2 (checkpoint engine, after C11)
          X3 (retry middleware, after C13)

Phase 4 — P1 Integration:
  Claude: C12 (checkpoint test + CLI), C14 (retry wiring)

Phase 5 — P2 (parallel):
  Claude: C19 (policy schema) → C20, C21 (scheduler schema) → C22
          C23,C24,C25,C26 (CRUD, conflict, summary, reminders)
  Codex:  X4 (memory D5), X5 (policy engine), X6 (replan), X7 (scheduler D4)

Phase 6 — P3:
  Codex:  X8 (coding gateway), X9 (shadow agent)
```

---

## Codex Prompt Template

---

## Execution Status

| Task | Status | Commit |
|------|--------|--------|
| C1 | DONE | Lark API client interface + directory |
| C2 | DONE | Auth token management |
| C3 | DONE | Calendar API wrapper |
| C4 | DONE | Task API wrapper |
| C5 | DONE | Calendar CRUD tools |
| C6 | DONE | Task CRUD tools |
| C7 | DONE | Tool registration wiring |
| C8 | DONE | Approval gate extension |
| C9 | DONE | Unit tests for C5-C6 |
| C10 | DONE | Calendar trigger in scheduler |
| C11 | DONE | `ceba6d70` checkpoint schema |
| C12 | TODO | Checkpoint integration test + CLI resume not implemented yet |
| C13 | DONE | `be5bbc01` tool policy config |
| C14 | DONE | `25030852` retry wiring + config load |
| C15 | DONE | `953ddd80` drainable interface |
| C16 | DONE | `8f123f6f` drain wiring |
| C17 | DONE | `f209cf5f` NSM metric stubs |
| C18 | DONE | `7e6f9148` tiktoken token counting |
| C19 | DONE | `f68e75a5` policy schema + rules |
| C20 | DONE | `ba058157` default policy rules |
| C21 | DONE | `774b398a` JobStore + FileJobStore |
| C22 | DONE | `e4f7517d` scheduler integration tests + config wiring |
| C23 | DONE | `741509dd` batch ops + multi-calendar |
| C24 | DONE | `1c0b2735` calendar conflict detection |
| C25 | DONE | `b7315928` calendar summary builder |
| C26 | DONE | `74b6b963` reminder pipeline |

**Claude Code: 25/26 done** (C12 pending: checkpoint integration test + CLI resume)

```
Context: elephant.ai Go project.

Codebase references (read these first):
- [pattern file] for implementation pattern
- [related files] for integration points
- docs/roadmap/roadmap.md for overall context

Task: [specific deliverable]

Pre-work done by Claude Code:
- [interface/schema already defined at path]
- [config structure at path]

Requirements:
- [concrete specs with types, API calls, error handling]
- [test coverage expectations]
- [integration points to respect]

Constraints:
- Follow existing patterns exactly
- Run `go vet ./...` and `go test ./...` before delivering
- No unnecessary defensive code; trust context invariants
```
