# Session and Task Identifier Acceptance Checklist
> Last updated: 2025-11-18


This manual acceptance guide exercises the full identifier pipeline (session → task → subtask) across the CLI, HTTP API, SSE stream, and structured logs. Follow these steps after applying the identifier changes to verify end-to-end propagation.

## Prerequisites

- Go toolchain installed (1.24 or newer).
- Node.js 20+ with `pnpm` available if you plan to exercise the web UI.
- From the repository root, install Go dependencies and build the binaries once:

  ```bash
  go test ./...
  make build
  ```

## 1. Start the backend with structured logging enabled

Launch the HTTP/SSE server in a dedicated terminal so logs remain visible. The `ALEX_LOG_FORMAT=json` flag makes it easy to inspect identifier fields.

```bash
ALEX_LOG_FORMAT=json ALEX_LOG_LEVEL=debug ./alex-server
```

Expected output: the startup banner includes `session_id` once the first request is handled.

## 2. Submit a CLI task and capture the generated identifiers

In a new terminal, invoke the CLI without the TUI so the streamed JSON includes session and task identifiers.

```bash
./alex --no-tui --task "List repo root files" --json
```

Record the `session_id` and `task_id` from the CLI output. The `context` block preceding each chunk should display both values.

## 3. Verify task lineage through subagent delegation

Use the CLI to trigger the subagent tool, forcing the backend to spawn a child task.

```bash
./alex --no-tui --task "Use the subagent to run two simple shell echoes" --json
```

Confirm in the CLI stream that:

- The root task retains the session identifier observed in step 2.
- The streamed `tool_result` payload for the `subagent` tool includes `session_id`, `task_id`, and `parent_task_id`.

## 4. Inspect structured logs for lineage metadata

Switch to the server terminal and locate the log entry corresponding to the subagent invocation. Each JSON line should contain:

- `session_id` matching the CLI output.
- `task_id` representing the delegated subtask.
- `parent_task_id` referencing the root task from step 3.

Example snippet:

```json
{"level":"INFO","msg":"executing subtask","session_id":"session-...","task_id":"task-...","parent_task_id":"task-..."}
```

## 5. Stream SSE events directly (optional)

If you prefer to validate the SSE contract without the web UI, issue the following `curl` command using the session identifier from step 2:

```bash
curl -N "http://localhost:8080/api/sessions/<session_id>/events"
```

Observe that each event frame includes `session_id`, `task_id`, and, when applicable, `parent_task_id`.

## 6. Validate the web dashboard (optional)

If the web app is enabled, start the Next.js dev server:

```bash
cd web
pnpm install
pnpm dev
```

Open `http://localhost:3000/sessions/<session_id>` in a browser. The timeline cards and session sidebar should group events by task and display the captured identifiers.

## Completion Criteria

- CLI output consistently surfaces `session_id`, `task_id`, and `parent_task_id` for root and delegated work.
- Server logs emit the same identifiers for every subagent delegation, enabling aggregation in external tooling.
- SSE or web dashboard views reflect the lineage, allowing operators to trace subtasks back to their parent task.

Document the observed identifiers and screenshots (if applicable) before marking the change as accepted.
