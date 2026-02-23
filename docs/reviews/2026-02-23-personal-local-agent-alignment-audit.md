# 2026-02-23 Personal Local Agent Alignment Audit

**Date**: 2026-02-23
**Scope**: repository-wide (`internal/`, `web/`, `cmd/`, `scripts/`, `tests/`, `configs/`, `docs/`)
**Method**: 4-way parallel subagent audit + repo-wide file inventory scan

## Audit Goal

围绕新的核心目标检查当前实现一致性：
- 个人本地 agent（single-user leverage）
- 主动性（proactive）且可覆盖（overrideable）
- 上下文压缩（context compression）
- 注意力节省（attention saving）
- subagent 并行杠杆能力

## Repository-Wide Coverage Snapshot

`rg --files` + 代码型文件统计（2026-02-23）:
- Total code-ish files: **2712**
- By top-level dir:
  - `internal`: 1022
  - `docs`: 771
  - `web`: 352
  - `skills`: 158
  - `evaluation`: 141
  - `cmd`: 77
  - `scripts`: 71
  - `tests`: 31
  - `configs`: 19
- By extension (top):
  - `.go`: 1164
  - `.md`: 832
  - `.tsx`: 173
  - `.ts`: 172
  - `.yaml`: 131
  - `.py`: 130

并行审计分区:
1. Partition A: `internal/app`, `internal/domain`（`internal/agent` 不存在）
2. Partition B: `internal/tools`, `internal/memory`, `internal/llm`, `internal/channels`, `internal/observability`
3. Partition C: `web`, `cmd`
4. Partition D: `scripts`, `tests`, `configs`, `docs`

## Conforming Points (already aligned)

1. Context compression pipeline is implemented and integrated (`internal/app/context/manager_compress.go`, `internal/domain/agent/react/solve.go`).
2. Proactive yet overrideable behavior exists through orchestrator gates and approval abstractions (`internal/domain/agent/react/runtime.go`, `internal/domain/agent/ports/tools/approval.go`).
3. Subagent orchestration and event partitioning are structurally present (`internal/domain/agent/react/tool_batch.go`, `web/components/agent/ConversationEventStream.tsx`).
4. Attention-saving UX primitives exist (intermediate panel, final summary composition, quick prompts) (`web/components/agent/IntermediatePanel.tsx`, `web/components/agent/TaskCompleteCard/index.tsx`).
5. Tool safety and policy enforcement are in place (`internal/infra/tools/policy.go`, `internal/infra/tools/approval_executor.go`).
6. Process guardrails are present (TDD/lint/test/pre-push/architecture checks) (`docs/guides/engineering-practices.md`, `scripts/pre-push.sh`, `scripts/check-arch.sh`).

## Non-Conforming / Gaps

1. Background subagent dispatch has no explicit manager-level concurrency cap, risking single-user local resource contention under bursty `bg_dispatch` (`internal/domain/agent/react/background.go`).
2. Token estimation mismatch: `TrimMessages` used content-only token counting while compaction path counts tool calls/thinking/attachments, causing budget inconsistency (`internal/app/context/trimmer.go` vs `internal/app/context/manager_compress.go`).
3. Memory indexing lacks explicit compression-efficiency telemetry (chunk/cache/refresh metrics), weakening evidence for attention-saving impact (`internal/infra/memory/indexer.go`).
4. Web intermediate progress panel is hidden until a tool call exists, losing early-stage “current goal/plan” visibility (`web/components/agent/IntermediatePanel.tsx`).
5. CLI path lacks first-class “running task status watch” command; strong completion output but weaker in-flight observability (`cmd/alex/cli.go`, `cmd/alex/stream_output.go`).

## Optimization Backlog (prioritized)

### High
1. Add background task concurrency limit in manager and wire from runtime config.
2. Unify trimming token estimator with full message token accounting.
3. Add regression tests for dispatch saturation and token accounting consistency.

### Medium
1. Add memory index observability metrics: chunk count, overlap usage, embedding cache hit ratio, refresh latency.
2. Surface pre-tool-call stage summaries in web event stream (goal/plan before first tool).
3. Add CLI `status/watch` command for in-flight subagent progress snapshots.

### Low
1. Add per-session compression event visibility in UI (compact but inspectable).
2. Extend eval suites with explicit “attention-saving + single-user leverage” benchmarks.

## Immediate Action Plan (this change set)

1. Update core goal narrative in primary docs and roadmap to “personal local agent leverage”.
2. Implement high-priority code fixes: background concurrency limit + token estimation alignment.
3. Add/adjust tests and run full lint + tests.
4. Perform mandatory code review workflow before merge.

