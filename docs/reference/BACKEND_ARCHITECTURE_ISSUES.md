# Backend Architecture Issues Review
> Last updated: 2026-01-26

This document continues the backend architecture analysis for elephant.ai and records evidence-backed issues with prioritized recommendations.
It complements `docs/reference/ARCHITECTURE_AGENT_FLOW.md` and the refactor ledger.

## Method
- Scanned Go LOC for non-test files to surface complexity hotspots.
- Collected internal package dependency fan-out/in via `go list -json ./internal/...`.
- Inspected high fan-out packages and large files for responsibility sprawl.

## Snapshot (Signals)

### Largest non-test files (top 12)
```
1631 internal/output/cli_renderer.go
 887 internal/agent/domain/react/attachments.go
 874 internal/agent/app/coordinator/coordinator.go
 869 internal/agent/domain/formatter/formatter.go
 853 internal/agent/app/coordinator/workflow_event_translator.go
 801 internal/server/http/middleware.go
 783 internal/server/app/postgres_event_history_store.go
 762 internal/server/http/sse_handler_render.go
 760 internal/llm/openai_responses_client.go
 744 internal/server/app/event_broadcaster.go
 700 internal/tools/builtin/seedream_helpers.go
 696 internal/llm/openai_client.go
```

### Highest internal dependency fan-out (top 8)
```
26 alex/internal/server/bootstrap
26 alex/internal/tools/builtin
23 alex/internal/server/http
21 alex/internal/di
19 alex/internal/agent/app/coordinator
18 alex/internal/server/app
13 alex/internal/agent/app/preparation
10 alex/internal/llm
```

## Findings (Prioritized)

### P1 — Delivery layer depends on concrete tool implementations and domain formatting
**Evidence**
- `internal/server/http` imports 23 internal packages, including `internal/tools/builtin`, `internal/agent/domain`, and `internal/agent/domain/formatter`.
- This couples the HTTP surface to tool implementation details and ANSI/presentation formatting.

**Why it matters**
- Makes the server layer sensitive to tool changes and presentation tweaks.
- Blocks alternative tool registries or delivery surfaces from reusing the same handlers cleanly.

**Recommendation**
- Introduce a `server/app` façade (e.g., `ToolInfoProvider`, `EventFormatter`) that hides implementation details.
- Keep `internal/server/http` consuming only `server/app` + `agent/ports` interfaces.
- Move ANSI-specific rendering out of domain (`internal/agent/domain/formatter`) into output/presentation packages.

---

### P1 — Builtin tools are a monolithic package with cross-layer dependencies
**Evidence**
- `internal/tools/builtin` imports 26 internal packages, touching agent ports, LLM, memory, storage, MCP, sandbox, skills, and workflow.
- Large helpers (e.g., `seedream_helpers.go`, `sandbox_browser_dom.go`) exceed 650+ LOC.

**Why it matters**
- High fan-out makes tool evolution risky; any refactor can cascade across unrelated dependencies.
- Hard to reason about per-tool requirements and test isolation.

**Recommendation**
- Split builtins into subpackages per tool domain with narrow constructor interfaces.
- Introduce a small `builtin/registry` layer that wires tools from interfaces rather than concrete packages.
- Prefer `agent/ports/tools` interfaces for tool registration instead of direct dependency on concrete storage/memory packages.

---

### P1 — Domain layer contains presentation and log-serialization concerns
**Evidence**
- `internal/agent/domain/formatter/formatter.go` embeds ANSI color sequences and tool-specific display rules.
- `internal/agent/domain/react/tool_args.go` and `attachments.go` serialize arguments/attachments with `encoding/json` for logs.

**Why it matters**
- Domain should express semantics, not presentation; ANSI output is a delivery concern.
- Log-oriented JSON formatting inside domain complicates reuse and testing.

**Recommendation**
- Move display formatting to `internal/output` (CLI/TUI) or a dedicated `presentation` package.
- Keep domain returning typed tool metadata/arguments; allow output layers to format and redact.

---

### P2 — Large-file hotspots indicate responsibility sprawl
**Evidence**
- Multiple core files exceed 700–1600 LOC (see snapshot).

**Why it matters**
- Increases cognitive load, review friction, and merge conflicts.
- Large files often hide multiple responsibilities that deserve clear boundaries.

**Recommendation**
- Schedule incremental splits aligned with responsibility boundaries:
  - `cli_renderer.go` → event mapping vs layout rendering.
  - `middleware.go` → auth, rate limiting, and stream guards.
  - `sse_handler_render.go` → SSE event mapping vs streaming transport.
  - LLM clients → request/response parsing vs transport/streaming.

## Suggested Next Steps
1. Carve out delivery-facing formatting into a dedicated presentation module used by CLI and SSE.
2. Define a minimal `ToolInfoProvider` interface in `server/app` and remove HTTP handlers' dependency on builtins.
3. Map builtins into subpackages with explicit constructor interfaces to reduce cross-layer coupling.
