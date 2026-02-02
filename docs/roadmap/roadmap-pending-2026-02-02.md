# Roadmap: Pending or Unverified (2026-02-02)

## Scope
- Source: `docs/roadmap/roadmap.md`.
- Pending = any status not **Done**.
- Unverified = **Done** items with no local `*_test.go` in their code path (heuristic).
- Non-ASCII roadmap labels are normalized to ASCII here.

## Pending

### P1: M0 Quality

| Item | Status | Code path |
|------|--------|-----------|
| ReAct checkpoint + resume | **Runtime done, wiring pending** | `internal/agent/domain/react/` |

### P2: Next Wave (M1)

#### Agent Core & Memory

| Item | Status | Code path |
|------|--------|-----------|
| Replan + sub-goal decomposition | **Not started** | `internal/agent/domain/react/`, `internal/agent/planner/` |
| Memory restructuring (D5) | **Not started** | `internal/memory/` |

#### Tool Chain & Scheduler

| Item | Status | Code path |
|------|--------|-----------|
| Tool SLA profile + dynamic routing | **Not started** | `internal/tools/router.go` |

#### DevOps Foundations

| Item | Status | Code path |
|------|--------|-----------|
| Evaluation automation | **In progress** | `internal/devops/evaluation/` |
| Evaluation set construction (evaluation dataset build) | **In progress** | `evaluation/` |

### P3: Future (M2+)

#### Coding Agent Gateway

| Item | Status | Code path |
|------|--------|-----------|
| Gateway abstraction | **Not started** | `internal/coding/gateway.go` |
| Multi-adapter framework | **Not started** | `internal/coding/adapters/` |
| Local CLI auto-detect | **Not started** | `internal/coding/adapters/detect.go` |
| Task translation | **Not started** | `internal/coding/task.go` |
| Build/test/lint verification | **Not started** | `internal/coding/verify*.go` |
| Fix loop | **Not started** | `internal/coding/fix_loop.go` |
| Auto commit + PR | **Not started** | `internal/coding/deliver.go` |

#### Shadow Agent & DevOps

| Item | Status | Code path |
|------|--------|-----------|
| Shadow Agent framework | **Not started** | `internal/devops/shadow/` |
| Coding Agent dispatch | **Not started** | `internal/devops/shadow/dispatcher.go` |
| Verification orchestration | **Not started** | `internal/devops/shadow/verify_orchestrator.go` |
| Mandatory human approval | **Not started** | `internal/devops/shadow/approval.go` |
| PR automation | **Not started** | `internal/devops/merge/` |
| Release automation | **Not started** | `internal/devops/release/` |
| Agent-driven ops | **Not started** | `internal/devops/ops/` |
| Self-healing | **Not started** | `internal/devops/ops/` |

#### Advanced Agent Intelligence

| Item | Status | Code path |
|------|--------|-----------|
| Multi-agent collaboration | **Not started** | `internal/agent/orchestration/` |
| Multi-path sampling + voting | **Not started** | `internal/agent/domain/react/voting.go` |
| Confidence modeling | **Not started** | `internal/agent/domain/confidence.go` |
| User preference learning | **Not started** | `internal/memory/preferences.go` |

#### Deep Lark Ecosystem

| Item | Status | Code path |
|------|--------|-----------|
| Lark Docs read/write | **Not started** | `internal/lark/docs/` |
| Lark Sheets/Bitable | **Not started** | `internal/lark/sheets/`, `internal/lark/bitable/` |
| Lark Wiki | **Not started** | `internal/lark/wiki/` |
| Meeting preparation assistant | **Not started** | `internal/lark/calendar/` |
| Meeting notes auto-generation | **Not started** | `skills/meeting-notes/` |
| Calendar suggestions | **Not started** | `internal/lark/calendar/` |

#### Platform & Interaction

| Item | Status | Code path |
|------|--------|-----------|
| macOS Companion (D6) | **Not started** | `macos/ElephantCompanion/` |
| Node Host Gateway | **Not started** | `internal/tools/builtin/nodehost/` |
| Cross-surface session sync | **Not started** | `internal/session/` |
| Unified notification center | **Not started** | `internal/notification/` |
| Web execution replay | **Not started** | `web/components/agent/` |
| CLI pipe mode + daemon | **Not started** | `cmd/alex/` |

#### Data Processing

| Item | Status | Code path |
|------|--------|-----------|
| PDF parsing | **Not started** | `internal/tools/builtin/fileops/` |
| Excel/CSV processing | **Not started** | `internal/tools/builtin/fileops/` |
| Audio transcription | **Not started** | `internal/tools/builtin/media/` |
| Data analysis + visualization | **Not started** | `internal/tools/builtin/data/` |
| User-defined skills | **Not started** | `internal/skills/custom.go` |
| Skill composition | **Not started** | `internal/skills/compose.go` |

#### Self-Evolution (M3)

| Item | Status | Code path |
|------|--------|-----------|
| Self-fix loop | **Not started** | `internal/devops/evolution/self_fix.go` |
| Prompt auto-optimization | **Not started** | `internal/devops/evolution/prompt_tuner.go` |
| A/B testing framework | **Not started** | `internal/devops/evaluation/ab_test.go` |
| Knowledge graph | **Not started** | `internal/memory/knowledge_graph.go` |
| Cloud execution environments | **Not started** | `internal/environment/` |

## Unverified (heuristic)

| Item | Evidence | Code path |
|------|----------|-----------|
| Graceful shutdown | No local `*_test.go` found under `internal/lifecycle/` | `cmd/alex/main.go`, `internal/lifecycle/` |
