## 2026-02-23 Project-Wide Code Simplification

### Context
- Goal: simplify the project codebase aggressively while preserving behavior.
- Constraint: prioritize low-risk, behavior-preserving simplifications first (DRY helpers, normalization helpers, branch flattening).
- Source inputs: parallel subagent scans for `internal/domain`, `internal/app`, `internal/infra`.

### Best-Practice Basis
- Effective Go: keep code simple, avoid duplicated logic, and prefer clear helper functions over repeated inline patterns.
- Go Code Review Comments: consolidate repeated code paths and normalize inputs once at boundaries.
- Uber Go Style Guide: reduce branching complexity and centralize invariants.

### Scope for This Change Set
1. Domain cloning simplification:
- unify map cloning and percentage computation helpers in domain event/message paths.
- simplify attachment persistence and merge loops in ReAct attachment state handling.

2. App-layer normalization simplification:
- centralize tool mode + default preset normalization in `presets`.
- replace repeated per-call context cancellation guards in selection store.

3. Mechanical simplification:
- run `gofmt -w -s` on touched Go files.

### Execution Plan
- [x] Gather candidate simplifications via subagents.
- [x] Implement low-risk simplifications in domain + app packages.
- [x] Add/adjust tests for new helper behavior.
- [x] Run full lint + full tests.
- [x] Mandatory code review pass (P0-P3 format) and fix findings.
- [in_progress] Create incremental commits and merge to `main`.

### Risks and Mitigation
- Risk: helper extraction can accidentally change defaults.
- Mitigation: keep existing public behavior, add targeted tests for normalization and merge semantics.
