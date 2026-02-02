# Phase 7: Remaining P2 Tasks — Claude-Owned

Created: 2026-02-02
Purpose: Complete remaining Claude-owned P2 gaps identified after Phase 6.

---

## Remaining P2 Items (Claude-owned, not blocked)

### Batch 1 (parallel, no dependencies)

| # | Task | Priority | Description | Current State |
|---|------|----------|-------------|---------------|
| C41 | Decision memory | P2 | Record key decisions (what/why/context/outcome) for future reference. New `internal/memory/decision.go` with DecisionEntry, DecisionStore interface, file-based implementation. | ~0% — no existing code |
| C42 | Entity memory | P2 | Extract people/projects/concepts from conversations, build entity store. New `internal/memory/entity.go` with Entity type, EntityStore, extraction helpers. | ~0% — no existing code |
| C43 | Signal collection framework | P2 | Centralized signal collector: failure trajectories (error chains), user feedback (approval accept/reject), implicit signals (retries, tool failures, latency outliers). Extend beyond skills-only feedback. | ~20% — `skills/feedback.go` exists but skill-centric only |
| C44 | Dynamic model selection router | P2 | Extract routing logic from `preparation/analysis.go` into dedicated `internal/llm/router/` module. Task complexity → model tier mapping, context length awareness, cost/latency tradeoff config. | ~40% — ad-hoc in preparation service |

### Batch 2 (after Batch 1)

| # | Task | Priority | Description | Current State |
|---|------|----------|-------------|---------------|
| C45 | Tool SLA dynamic routing | P2 | Use existing SLA metrics (`tools/sla.go`) for tool selection decisions. SLA-based fallback (prefer tool with best P95), integrate with degradation chain. | ~50% — SLA metrics collected but unused for routing |
| C46 | Evaluation set construction | P2 | Hierarchical eval sets: easy/medium/hard/expert tiers, per-domain stratification, eval set versioning, composition rules. | ~40% — flat JSON dataset exists |
| C47 | Evaluation automation enhancements | P2 | Baseline snapshot management, regression detection (threshold alerts), trend tracking. Build on existing `evaluation/agent_eval/`. | ~70% — metrics/analyzer exist, no baseline/regression |

---

## Execution Status

| Task | Status | Commit |
|------|--------|--------|
| C41 | | |
| C42 | | |
| C43 | | |
| C44 | | |
| C45 | | |
| C46 | | |
| C47 | | |

---
