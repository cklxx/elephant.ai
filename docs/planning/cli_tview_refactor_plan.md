# CLI Chat Experience Refactor Plan

## 1. Current Architecture Overview

### 1.1 Entry Points & Command Handling
- `cmd/alex/cli.go` is the entry point that routes commands. Non-recognized commands fall back to `RunTaskWithStreamOutput`, which executes a free-form task with streaming output.【F:cmd/alex/cli.go†L16-L66】
- Chat mode relies on `NativeChatUI` (`cmd/alex/tui_native.go`), which currently runs a simple line-mode chat without true terminal widgets or scrollback management.【F:cmd/alex/tui_native.go†L37-L106】

### 1.2 Streaming Output Pipeline
- `RunTaskWithStreamOutput` instantiates `StreamingOutputHandler`, registers a `StreamEventBridge`, and delegates to `Coordinator.ExecuteTask` for execution.【F:cmd/alex/stream_output.go†L51-L103】
- `StreamingOutputHandler` uses `internal/output.CLIRenderer` to stringify domain events for stdout. Events are handled imperatively: `fmt.Print` is called per event, and state tracking for tools/subtasks is done ad-hoc inside the handler.【F:cmd/alex/stream_output.go†L19-L195】
- Subagent progress is appended to the terminal in plaintext with manual ANSI formatting. State is not retained beyond ephemeral prints, and there is no scrollable history; the terminal relies on native scrollback.【F:cmd/alex/stream_output.go†L139-L195】

### 1.3 MCP Management UX
- MCP commands (`alex mcp ...`) operate synchronously and print tables via `fmt.Printf`. There is no integration with chat mode; MCP status changes are not reflected live inside the interactive session.【F:cmd/alex/mcp.go†L12-L120】

## 2. Pain Points & Limitations

| Area | Current Behavior | Issues |
| --- | --- | --- |
| Chat mode | Line-by-line prompt, no widgets | No scrollback management, no message grouping, cannot view long history comfortably, no streaming panes.【F:cmd/alex/tui_native.go†L37-L137】 |
| Streaming | Renderer streams strings directly to stdout | Hard to provide structure for UI; lack of decoupled model makes alternate UIs painful.【F:cmd/alex/stream_output.go†L19-L195】 |
| Subagent visibility | ANSI prints per event | Progress overwrites lines, final state easy to miss, no persistent view per subagent, complex to follow when multiple subtasks run.【F:cmd/alex/stream_output.go†L139-L195】 |
| MCP lifecycle | Managed via CLI commands | No async startup feedback, cannot see MCP discovery/loading during chat, failure modes hidden unless command executed manually.【F:cmd/alex/mcp.go†L12-L120】 |
| Extensibility | Tight coupling between domain events and `fmt.Print` | Introducing new UI (tview) requires re-implementing event handling logic; no shared stateful model. |

## 3. Goals for tview-based Refactor

1. **Event-driven UI layer**: Decouple domain events into a shared state store (messages, tool runs, subagent status) that the TUI consumes.
2. **Rich chat layout**: Implement multi-pane interface using `tview` with dedicated regions for history, live stream, subagent dashboards, and input box.
3. **Asynchronous MCP integration**: Load MCP servers in background, stream readiness/errors into UI, allow manual refresh without blocking chat.
4. **Streaming transcript**: Render incremental messages in a scrollable text view with markdown-friendly formatting and colorization.
5. **Persistent history**: Maintain full scrollback; allow infinite scrolling through message history and tool outputs per agent.
6. **Subagent telemetry**: Provide collapsible panels per subagent showing queued tasks, running tool, completed steps, errors, and durations.
7. **Accessibility & fallback**: Keep existing simple CLI mode as fallback (e.g., `--no-tui`).

## 4. High-Level Architecture

```
Coordinator Events → Event Bus → State Store (models) → UI Controller → tview Widgets → Terminal
```

1. **Event Bus**: Fan-out `ports.AgentEvent` & `builtin.SubtaskEvent` into typed channels.
2. **State Store**: Maintain in-memory models
   - Chat messages (user/assistant/system)
   - Tool executions per agent
   - Subagent tasks & progress snapshots
   - MCP server registry statuses
3. **UI Controller**: Goroutine that listens for store updates and triggers tview redraws via `Application.QueueUpdate`.
4. **Renderer**: Replace `CLIRenderer` usage with structured message building (allow fallback renderer for legacy mode).

## 5. Detailed Implementation Plan

### 5.1 Infrastructure Layer
- **Event Aggregator**: Create `cmd/alex/ui/eventhub` to convert `AgentEvent` + MCP signals into typed structs.
- **State Models**: Define Go structs for `ChatMessage`, `ToolRun`, `SubagentTask`, `MCPServerStatus`, each with thread-safe mutations (e.g., `sync.RWMutex` or channels).
- **Legacy Compatibility**: Keep `StreamingOutputHandler` but refactor to share the event aggregation so it consumes structured updates.

### 5.2 tview UI Layout
1. **Root Layout**: Flex layout with three rows
   - Header (session info, MCP status summary)
   - Body (two columns)
     - Left: chat transcript (scrollable) + live stream pane
     - Right: subagent dashboard (accordion of agents) + MCP server panel
   - Footer: input box + command hints
2. **Chat Transcript**: `tview.TextView` with dynamic buffer; support markdown-like formatting via color tags.
3. **Live Stream Pane**: Show currently streaming tool output with auto-scroll; when a tool completes, append to transcript.
4. **Subagent Dashboard**: For each subagent: `tview.List` or `Table` showing task states with status icons, percent complete, last update timestamp.
5. **MCP Panel**: Table with server name, status, uptime, last error, with indicators for async load.
6. **Input Handling**: `tview.InputField` capturing enter to submit tasks; support slash commands (`/quit`, `/mcp restart foo`, etc.).
7. **Scrollback**: Enable navigation keys (PgUp/PgDn, g/G) and search (optional stretch).

### 5.3 Asynchronous MCP Loading
- Spawn goroutines on startup to initialize MCP servers.
- Publish status updates (starting, running, error) to event hub.
- Reflect statuses in header and MCP panel; allow manual refresh command that triggers background reload while UI remains responsive.

### 5.4 Streaming Integration
- Buffer assistant/tool output incrementally; push tokens into live pane while simultaneously updating aggregated message once tool completes.
- Implement per-agent logs: store structured events keyed by agent ID to enable filtered views.

### 5.5 Subagent Command Output Optimization
- For each `builtin.SubtaskEvent`, maintain progress struct with fields: task summary, start time, active tool, completed tools, total tokens, error state.
- Show spinner/progress bar while running; upon completion, collapse to summary entry retaining start/end timestamps.
- Support expanding a task to view detailed tool log.

### 5.6 Persistence & Session Recovery (Stretch)
- Optional: load prior session history from store when chat starts to populate transcript.

## 6. Milestones & Deliverables

| Milestone | Scope | Key Tasks | Acceptance Criteria |
| --- | --- | --- | --- |
| M1: Event Hub & State Store | Shared event infrastructure | Implement event hub, thread-safe models, adapter from Coordinator events | Unit tests proving events update store correctly; legacy CLI still works. |
| M2: Basic tview Shell | Minimal chat UI with transcript + input | Render user/assistant messages; support command submission; integrate with event store | Can send task and see streamed answer in scrollable transcript; fallback CLI works. |
| M3: Streaming & Tool View | Live streaming pane + tool completion | Display token stream, tool start/complete updates, consolidate to transcript | Tool runs visible with timestamps; tests for event ordering. |
| M4: Subagent Dashboard | Dedicated panel per subagent | Show queued/running/completed states, durations, errors | Running subtasks visibly update progress; QA script demonstrating multiple subagents. |
| M5: Async MCP Integration | MCP status in UI | Background loader, status panel, manual refresh command | MCP start/restart shown without blocking; simulated failure surfaces error badge. |
| M6: Polish & QA | Keyboard navigation, theming, docs | Add help overlay, ensure resizing works, document usage | Manual QA checklist passed; README section updated. |

## 7. Acceptance & Testing Strategy

1. **Unit Tests**
   - Event hub mapping functions (domain event → store updates).
   - Store methods ensure thread safety & expected snapshots.
2. **Integration Tests**
   - Simulate Coordinator emitting events; verify UI controller receives updates (can be headless by inspecting store state).
   - MCP loader mock to assert async updates.
3. **Manual QA Checklist**
   - Launch TUI, send requests, ensure transcript scrolls infinitely.
   - Trigger multiple subagents (e.g., planning tasks) and verify dashboards update per agent.
   - Restart MCP server from UI command; observe status transitions without freezing.
   - Resize terminal; confirm layout adapts without artifacts.
   - Switch to legacy mode (`alex --no-tui`) and confirm old renderer still functions.
4. **Performance Validation**
   - Stress test with long outputs (>10k tokens) to ensure no crashes, scrolling remains responsive.
   - Monitor goroutine leaks with `pprof` or `go test -run TestLeak`. Ensure background watchers stop on exit.
5. **Documentation & Sign-off**
   - Update README/CLI docs with new usage.
   - Provide demo recording or screenshots for stakeholders.
   - Stakeholder review of UI flows against goals (chat flow, MCP management, subagent visibility).

## 8. Risks & Mitigations

| Risk | Impact | Mitigation |
| --- | --- | --- |
| tview concurrency misuse causing panics | High | Strictly use `Application.QueueUpdate`; add integration tests and lints. |
| Event duplication between legacy & new UI | Medium | Centralize event processing in shared hub; decouple renderers. |
| Performance degradation with large histories | Medium | Implement capped in-memory log with pagination; optionally persist to disk. |
| MCP startup failures blocking UI | High | Run loads in separate goroutines with context cancellation; surface errors in UI without blocking main loop. |

## 9. Timeline Estimate

- Weeks 1-2: M1, architecture groundwork, tests.
- Weeks 3-4: M2 & M3 (basic UI + streaming integration).
- Weeks 5-6: M4 subagent dashboard & instrumentation.
- Weeks 7-8: M5 async MCP integration; extend commands.
- Week 9: M6 polish, docs, final QA & acceptance.

Total: ~9 weeks with overlap possible if multiple contributors work in parallel (event hub + UI workstreams).

## 10. Implementation Status Snapshot

- ✅ Event hub, store, and tview chat panes are wired together with live MCP telemetry.
- ✅ Keyboard navigation, follow-mode toggles, and a built-in `?` shortcut overlay document the controls.
- ✅ Legacy fallback can be launched with `alex --no-tui` or `ALEX_NO_TUI=1` for terminals that cannot render the full TUI.
- ✅ Contextual search within panes, persisted session replay, and spinner indicators for long-running MCP startups are complete.
- ✅ Slash commands inside the TUI start fresh sessions, list saved ones, and load a prior conversation without leaving the shell.
- ✅ `/mcp` commands in the chat let operators refresh status or restart servers inline, completing the asynchronous MCP management milestone.
- ✅ Subagent dashboards now include agent attribution plus start/finish timestamps and runtime totals, making long-running or stalled subtasks easy to diagnose.
- ✅ Transcripts can be exported on demand with `/export`, producing shareable Markdown logs using sanitized filenames.
- ✅ Verbose logging can be inspected or toggled live with `/verbose`, keeping the CLI renderer in sync with the TUI without leaving the session.
- ✅ The status bar aggregates cumulative per-agent token usage so operators can monitor total consumption alongside tool and MCP activity.
- ✅ Cost tracking is surfaced in the status bar with total spend and per-model breakdowns pulled from the shared cost tracker so operators can monitor budgets in real time.
- ✅ `/cost` surfaces a per-session cost rollup in the transcript, giving operators an on-demand audit trail alongside the live status bar totals.
- ✅ A headless regression harness drives the chat UI against a simulated terminal so the event hub, store, and rendering pipeline are covered by automated tests.
- ✅ Default follow behaviour for the transcript and live stream panes can be configured via runtime config or `ALEX_TUI_FOLLOW_*` environment toggles so operators can opt out of auto-scroll after resets.
- ✅ Default follow behaviour can also be inspected and persisted directly from the TUI with `/follow`, which writes the desired settings back to `~/.alex-config.json` for future sessions.

## 11. Remaining Work & Follow-ups

- **Cross-platform QA matrix** – validate the tview layout on Windows terminals and popular macOS emulators, documenting any keymap adjustments or font requirements.
- **CI hardening for the headless harness** – capture screen diffs or logs on failure and wire nightly runs so regressions surface with actionable artifacts.
- **High-volume transcript optimisations** – profile and, if necessary, paginate rendering for transcripts that exceed ~5k messages to protect redraw latency.
- **Real-time cost streaming** – explore wiring the cost tracker to push incremental updates without waiting for `/cost` or manual refreshes, especially for long-running background sessions.
- **Configuration persistence** – persist UI preferences (follow mode, focused pane, verbose toggle) so they survive restarts alongside the restored transcript.

