# Context-First Multi-Agent Architecture Design

Goal: build a context-first multi-agent system where the server keeps a single long-lived "architect" context, and execution happens in a VM-bound CLI executor that can be hot-swapped (Codex / Claude Code / Gemini CLI) as long as it is ACP-ready. The shared context is backed by a traceable session ledger (event stream), not implicit memory, and the SSE stream reuses the same events.

Context and constraints:
- The server-side agent is the "architect" and must not switch brains across agents; only tools/environments change.
- The VM-side agent is the "executor" and only runs CLI actions, reads/writes the workspace, and returns artifacts.
- All inputs, tool calls, outputs, diffs, test reports, and artifacts are persisted as session events for audit and replay.
- Artifacts are first-class: each execution ends with an artifact manifest event; the server stores references + checksums, not large blobs.
- ACP is the only execution protocol boundary; swapping an executor is a single adapter replacement.
- Config examples must be YAML-only; assume `.yaml` paths.

Non-goals:
- Designing a new wire protocol beyond ACP.
- Building a general multi-brain coordinator that mixes reasoning across agents.
- Storing large artifacts directly in the server session store.

Principles (hard invariants):
1) One architect context per session; no cross-agent brain switching.
2) No implicit memory for sharing; every shared state is an event.
3) Executors are swappable; the ACP surface stays constant.
4) Artifacts are mandatory and structured; text-only outputs are insufficient.
5) Convergence is enforced by protocol and scheduler limits, not operator discipline.

Repository defaults (alex-server):
- `alex-server` boots with the **architect** agent preset by default.
- Tool access is restricted to the **architect** tool preset in web mode unless overridden.
- The executor is invoked via `acp_executor`; local shell/file tools remain disabled in web mode.

---

## Roles and responsibilities

Architect (server-side):
- Allowed actions: search, plan, clarify, interpret results, update task plan.
- Forbidden actions: direct filesystem mutation, local CLI execution, artifact creation.
- Context: single long-lived "architect" context that persists across tasks in the same session.

Executor (VM-side):
- Allowed actions: run CLI, read/write workspace, run tests, build artifacts, produce diffs.
- Forbidden actions: global planning and requirement negotiation beyond explicit clarify tasks.
- Context: per-task execution context sent from the architect via ACP.

Adapter boundary:
- The only swappable module is the ACP executor adapter (Codex, Claude Code, Gemini CLI, etc.).
- Adapter changes must not affect the server workflow or event schema.

---

## Architecture overview

High-level components:
- Architect service (server)
- Session ledger (event stream + store)
- Artifact store (object store or filesystem with checksums)
- ACP executor adapter (per CLI provider)
- Sandbox executor runtime (existing sandbox; workspace, tools, test runners)
- SSE gateway (reuses session events)

Flow sketch:

```
User
  |
  v
Architect (search/plan/clarify)
  | writes events
  v
Session ledger -----> SSE (frontend)
  |
  v
ACP adapter -------> VM executor (CLI)
  |                    |
  | results + artifacts v
  +-----------------> events + manifests
```

---

## Sandbox execution layer (existing sandbox, portable by design)

VM-side execution is provided by the existing sandbox service. The executor runtime is a thin ACP adapter that:
- Opens a sandbox session for each ACP session (stable mapping).
- Executes CLI commands inside the sandbox workspace.
- Collects stdout/stderr, exit codes, diffs, and artifacts.
- Emits events back to the session ledger via ACP.

Portability constraints:
- The executor adapter must be the only component that knows sandbox-specific APIs.
- The sandbox layer should remain OCI-compatible to keep migration cost low.
- Environment fingerprints must include sandbox image/runtime identifiers to keep replays reproducible.

Implementation notes:
- Reuse current sandbox integration and session pinning strategy (`docs/operations/SANDBOX_INTEGRATION.md`).
- Preserve the ACP boundary so swapping the sandbox or CLI executor is a single adapter change.

Sandbox configuration (project wiring):
- Use the existing runtime config key `runtime.sandbox_base_url` in `~/.alex/config.yaml`.
- This config is already consumed by the built-in sandbox tools and registry wiring, so the executor adapter can rely on it without adding new config knobs.
- Canonical config reference: `docs/reference/CONFIG.md` and `docs/operations/SANDBOX_INTEGRATION.md`.

```yaml
runtime:
  sandbox_base_url: "http://localhost:18086"
```

ACP executor config (project wiring):
- Use runtime keys to locate the ACP executor and enforce convergence limits.
- These are shared by `alex` and `alex-server` but primarily consumed by the server-side architect.

```yaml
runtime:
  acp_executor_addr: "127.0.0.1:18088"
  acp_executor_cwd: "/workspace/project"
  acp_executor_auto_approve: false
  acp_executor_max_cli_calls: 12
  acp_executor_max_duration_seconds: 900
  acp_executor_require_manifest: true
```

---

## Project alignment notes (ACP + sandbox)

ACP alignment (current repo):
- ACP framing and method surface are defined in `docs/reference/ACP.md`.
- Only one `session/prompt` runs per session at a time; avoid parallel prompts on the same session.
- `session/set_mode` can be used to keep the architect in read-only mode while the executor runs in full mode.

Sandbox alignment (current repo):
- Sandbox access is wired in the tool registry with `sandbox_shell_exec` and the `sandbox_file_*` tools.
- Sandbox client uses per-session pinning with a TTL; keep session IDs stable across calls for isolation.
- Sandbox calls carry `X-Session-ID` for isolation (see `docs/operations/SANDBOX_INTEGRATION.md`).

Executor adapter guidance:
- Treat sandbox APIs as the implementation detail behind the executor adapter, not the architect.
- Keep the ACP interface stable; only the adapter changes when switching sandbox or CLI provider.
- Include sandbox runtime identifiers in the environment fingerprint for reproducible replay.

---

## Session ledger (shared context)

The session ledger is the single source of truth for shared context. All components read/write events; no implicit state sharing is allowed.

Minimal event envelope:
- `event_id`: monotonic, unique
- `parent_event_id`: causal link to prior step (optional)
- `session_id`
- `timestamp`
- `actor`: `architect` | `executor` | `system`
- `tool`: tool name or `cli`
- `event_type`
- `payload_ref`: pointer to the payload blob in the event store

Payloads are stored separately and referenced by `payload_ref` to keep events small. Large blobs (logs, diffs, artifacts) live in the artifact store and are referenced from payloads.

Event categories (non-exhaustive):
- `session.config`: session limits and execution policy (see below)
- `task.clarify`: clarify question/answer pairs and decisions
- `task.snapshot`: context snapshot for executor handoff
- `tool.call` / `tool.result`: architect-side tools
- `cli.run` / `cli.stdout` / `cli.stderr`: executor-side CLI execution
- `file.diff`: patch/diff output
- `test.report`: test run summary
- `artifact.manifest`: artifact list for a completed executor run
- `task.summary`: event-level summary referencing original events
- `task.error`: structured error + failure category

Context snapshots:
- The architect materializes a snapshot event before handoff.
- A snapshot references a bounded list of relevant events and has its own content hash for replay.

---

## ACP handoff contract (executor boundary)

The architect sends a "task package" to the executor via ACP, combining a context snapshot and the per-task instruction. The executor only receives this package; it does not infer additional context.

Project-first ACP reference:
- The authoritative ACP contract is the repo implementation in `docs/reference/ACP.md`.
- External protocol specs are only used for background framing, not as a substitute for the project ACP surface.
- Use ACP methods exactly as defined (initialize, session/new, session/load, session/prompt, session/update, session/set_mode, session/cancel).

Task package (YAML representation):

```yaml
session_id: session-xxx
task_id: task-yyy
context_ref: event://session/task.snapshot/abcd
instruction:
  goal: "Add a CLI command and tests"
  scope:
    include_paths:
      - cmd/
      - internal/cli/
    exclude_paths:
      - migrations/
  acceptance:
    - "go test ./... passes"
    - "new command is documented"
  constraints:
    max_cli_calls: 8
    max_duration_seconds: 900
    no_touch:
      - docs/error-experience.md
      - docs/error-experience/summary.md
runtime:
  cwd: /workspace/project
  tool_mode: full
  environment_fingerprint: env://hash/1234
```

The executor responds with ACP `session/update` events that are persisted into the same session ledger. On completion, it must emit an artifact manifest event.

---

## Clarify-driven closed loop

Each task unit follows a strict loop that must converge:
1) Architect splits the request into minimal task units.
2) Architect clarifies inputs until boundaries are explicit (scope, constraints, acceptance).
3) Architect emits `task.snapshot` and sends the task package via ACP.
4) Executor performs CLI actions and emits CLI/tool events.
5) Executor publishes `artifact.manifest` + diffs + test reports.
6) Architect reads results, updates the plan, and either:
   - Accepts and moves to next unit, or
   - Starts another clarify round with a smaller scope.

The architect never directly executes tools; all actions flow through the executor and the session ledger.

---

## Artifacts as first-class outputs

Every executor run ends with a required `artifact.manifest` event. The manifest is the contract for downstream rendering and retrieval.

Artifact manifest (YAML representation):

```yaml
manifest_id: artifact-manifest-001
session_id: session-xxx
generated_at: 2026-01-20T12:34:56Z
environment_fingerprint: env://hash/1234
artifacts:
  - type: patch
    ref: artifact://patches/patch-0001.diff
    generated_by: "git diff"
    checksum: sha256:...
    preview_ref: artifact://previews/patch-0001.txt
  - type: test_report
    ref: artifact://reports/go-test.txt
    generated_by: "go test ./..."
    checksum: sha256:...
  - type: build_output
    ref: artifact://build/cli-binary.tar
    generated_by: "make build"
    checksum: sha256:...
```

Artifact types supported:
- `patch`, `workspace_snapshot`, `build_output`, `test_report`, `log_bundle`, `doc`, `table`, `image`

Server storage rules:
- Store only references + checksums in the session ledger.
- Keep large blobs in artifact storage (filesystem or object store).
- Provide optional previews for UI (small text or image).

---

## Concurrency model

Default is serial execution to maximize convergence. Concurrency is allowed only when file domains do not overlap.

Rules:
- Do not concurrently mutate the same file domain.
- Partition by workspace shard or task domain.
- After parallel tasks, run a single merge task that produces a conflict report event and a consolidated patch.

---

## SSE event stream

The SSE stream reuses the session ledger without a separate event model. Each SSE event includes the minimal envelope:
- `event_id`, `parent_event_id`, `actor`, `tool`, `timestamp`, `payload_ref`.

The frontend subscribes to the session stream and renders:
- timeline view (what happened, why, by whom)
- artifact view (download/preview)
- error view (failure category + raw logs)

---

## Convergence controls (hard stops)

Convergence is enforced by protocol and scheduler constraints, recorded in the session config event.

Default limits (example):
- `max_clarify_rounds_per_task`: N (e.g., 3)
- `max_cli_calls_per_task`: M (e.g., 8)
- `max_cli_duration_seconds`: T (e.g., 900)
- `max_cli_memory_mb`: R (e.g., 4096)
- `max_same_error_retries`: K (e.g., 2)
- `context_summarize_threshold`: S (e.g., 200 events)

Session config event (YAML):

```yaml
session_id: session-xxx
config:
  limits:
    max_clarify_rounds_per_task: 3
    max_cli_calls_per_task: 8
    max_cli_duration_seconds: 900
    max_cli_memory_mb: 4096
    max_same_error_retries: 2
    context_summarize_threshold: 200
  failure_policy:
    on_repeated_error: downgrade_executor
    on_timeout: cancel_and_report
  summarization:
    mode: event_level
    keep_original_refs: true
```

Failure handling:
- Same error K times triggers downgrade strategy (swap executor or shrink scope).
- Timeouts and memory violations are hard stops with immediate event capture.
- Summaries must retain pointers to original events for audit.

---

## End-to-end demo scenario (acceptance)

Scenario: "Add a new CLI tool to an existing repo and cover it with tests."

Task units:
1) Clarify constraints and acceptance criteria (scope, files, tests, forbidden paths).
2) Implement code changes and run tests via the executor.
3) Produce artifacts and write a short change summary.

Acceptance criteria:
- Architect emits a `task.snapshot` before execution.
- Executor returns a patch + test report artifact manifest.
- Session ledger contains all tool calls, stdout/stderr, and diffs.
- SSE stream renders progress + artifacts without extra mapping.

---

## Risks and mitigations

- Event volume growth: use payload_ref + artifact store, and summarize only at event level.
- Executor divergence: require ACP readiness checks and capability handshake on startup.
- Concurrency conflicts: enforce domain partitioning and require merge task reports.
- Context drift: pin architect context; executor receives only snapshot + task package.

---

## Open questions / follow-ups

- Artifact retention policy and GC strategy for large blobs.
- How to expose executor capability metadata in ACP (read-only vs. full mode).
- Minimum required event taxonomy for analytics and cost reporting.
- Whether to standardize environment fingerprints across executors.

---

## Research references (web)

These references ground the protocol framing, event-ledger model, SSE stream, and sandbox portability choices:

- JSON-RPC 2.0 specification (ACP transport baseline): https://www.jsonrpc.org/specification
- LSP base protocol framing (Content-Length headers): https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/
- Server-Sent Events spec (EventSource + `text/event-stream`): https://html.spec.whatwg.org/dev/server-sent-events.html
- Event sourcing pattern (append-only event log + replay): https://martinfowler.com/eaaDev/EventSourcing.html
- OCI runtime spec (portable sandbox runtime contract): https://specs.opencontainers.org/runtime-spec/
- gVisor docs (OCI-compatible sandbox runtime `runsc`): https://gvisor.dev/docs/
