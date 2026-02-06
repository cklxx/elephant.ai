# Plan: Domain dependency leak analysis (2026-02-06)

## Goal
Identify domain-layer dependency leaks under `internal/agent/domain` and React engine constructor wiring, then propose minimal-churn port interfaces and injection points.

## Steps
1. Scan `internal/agent/domain` and React engine wiring for imports of the specified packages.
2. Map each import to its call sites and describe the dependency direction.
3. Propose port interfaces and injection points with minimal churn, listing target files and function signatures.

## Progress
- 2026-02-06: Completed scan of domain + React wiring imports.
- 2026-02-06: Mapped call sites + dependency directions for id/clilatency/jsonx/async/workspace/pathutil.
- 2026-02-06: Drafted port interface + injection point proposals.
