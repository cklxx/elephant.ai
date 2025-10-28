# Sandbox Execution & Environment Persistence Design

This document outlines the web-console side design for running every tool invocation inside the managed sandbox and capturing a reusable environment blueprint per session.

## Goals

1. **Simple visibility into tool calls.** The conversation page now presents each tool run as plain text both in-stream and in a dedicated summary list so operators can audit behaviour quickly.
2. **Sandbox-only execution contract.** All tools exposed through the web UI are assumed to execute inside the sandbox runtime. Each summary call carries a `sandboxLevel` classification (`standard`, `filesystem`, or `system`) so elevated access is plainly visible.
3. **Automatically generated environments.** Each session accumulates tool metadata and produces a persisted environment blueprint with recommended sandbox capabilities and a todo checklist that supports automatic and manual completion states.

## Implementation Checklist

- [x] Plain-text session tool summaries that highlight sandbox classifications for filesystem and system calls.
- [x] Automatic sandbox enforcement messaging across the conversation stream and summary list.
- [x] Session-scoped environment plan generation with persisted blueprints and todo completion heuristics.
- [x] UI indicators for todo completion status, including an "all done" state banner in the environment summary card.
- [x] Share/export actions for distributing sandbox blueprints outside the web console.
- [ ] Backend orchestration hook to consume the generated sandbox blueprints and launch managed environments.

## Remaining Work

- [ ] Integrate with backend orchestration to automatically spin up sandbox containers from saved blueprints.
- [ ] Capture audit logs from exported plans so the server can reason about historical sandbox guarantees.

## Architecture Overview

- **Event aggregation** collects raw SSE events into `ToolCallSummary` structures, capturing status, timing, previews, and sandbox requirements including the `sandboxLevel` classification.
- **Environment planning** (`buildEnvironmentPlan`) consumes summaries and emits a `SessionEnvironmentPlan` containing:
  - Sandbox strategy (`required` vs `recommended`).
  - Unique tools used, with notes that highlight file-system or system level access.
  - A blueprint describing recommended sandbox capabilities and persistence hints.
- **Session store persistence** saves the plan via Zustand's `persist` middleware so reloading the page retains sandbox state per session.
- **UI presentation** shows:
  - A compact “tool calls this session” list with text-only entries and an inline reminder that every tool runs inside the sandbox runtime.
  - A sandbox environment card detailing capabilities, timestamps, notes, and a persisted assurance banner that highlights sandbox-only execution.
  - A todo checklist derived from tool activity so operators can track sandbox follow-ups per session, with automatic check marks when items are satisfied (e.g. no active tools, blueprint saved) and manual overrides preserved per item.
  - Header actions for sharing (clipboard/Web Share API) and exporting (JSON download) the current sandbox blueprint for collaboration.

## Sandbox Usage Guidance

- **System & file tools.** Any tool name containing shell or system verbs (`shell`, `bash`, `system`, `process`, `exec`, `command`) is promoted to the `system` sandbox level, while file verbs (`file`, `fs`, `write`, `read`, `download`, `upload`) become `filesystem`. The UI highlights this and the environment planner adds `filesystem-proxy`, `sandbox-auditing`, and `command-runner` capabilities.
- **Other tools.** Non-sensitive tools still default to sandbox execution (`standard` level) and require baseline capabilities (`network-isolation`, `process-isolation`, `tool-cache`, `sandbox-enforced`).
- **Persistence.** Plans recommend per-session persistence to keep sandbox state available for follow-up tasks without leaking across sessions.

## Environment Lifecycle

1. **Session start:** When the first event arrives, an empty plan is generated with recommended sandbox defaults.
2. **Tool execution:** Each tool call updates the summaries; the plan is regenerated with the new blueprint, todo completion states, and saved.
3. **Reload/resume:** Stored plans are rehydrated from local persistence, maintaining per-session sandbox hints and completed checklist items, including manual overrides where an operator explicitly checked or unchecked a todo.
4. **Future extensions:** The blueprint can be sent to backend orchestration to instantiate or resume sandbox containers matching the capabilities listed here.

This design keeps the operator informed while enforcing a sandbox-first execution pipeline for every tool surfaced through the web interface.
