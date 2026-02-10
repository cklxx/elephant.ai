# Task Execution Framework

This document defines the standard operating procedures for task execution,
context management, and communication within the agent system.

---

## TODO Lifecycle

Defines how tasks progress through their lifecycle stages.

- **Pending**: Task created, awaiting assignment or prerequisites.
- **In Progress**: Actively being worked on by an agent or user.
- **Blocked**: Waiting on external input, dependency, or approval.
- **Completed**: All acceptance criteria met, validated, and committed.
- **Cancelled**: No longer relevant or superseded by another task.

---

## Research & Investigation Strategy

Guidelines for systematic investigation before implementation.

- Read existing code and tests before proposing changes.
- Search for related patterns, prior art, and architectural decisions in the codebase.
- Verify assumptions with concrete evidence (logs, tests, traces).
- Document findings in plan files before starting implementation.

---

## Communication Standards

Rules for clear, actionable communication during task execution.

- Use concise, imperative language for task descriptions.
- Provide context for decisions (why, not just what).
- Surface blockers and risks early.
- Include verification steps for every deliverable.

---

## Context Budget & Compression

Strategies for managing context window usage efficiently.

- Prioritize recent and relevant context over exhaustive history.
- Compress earlier conversation turns when approaching token limits.
- Preserve system prompts and critical state across compressions.
- Use structured summaries rather than verbatim replay.

---

## Observability & Traceability

Standards for maintaining audit trails and operational visibility.

- Log key decisions, state transitions, and tool invocations.
- Attach session and turn identifiers to all operations.
- Record error experiences for post-incident learning.
- Emit structured events for downstream consumers.

---

## Autonomous Exploration and Environment Injection

Rules for high-agency execution with safe environment awareness.

- Default stance: execute first for reversible local actions; ask only when irreversibility, external side effects, or explicit consent boundaries are involved.
- Exploration loop is mandatory: inspect current state -> choose best deterministic action -> execute -> verify evidence -> iterate.
- Treat host CLIs as first-class capabilities. If a dedicated tool is missing, use shell and local binaries available on PATH.
- Discover tools proactively using deterministic checks (`command -v`, `--version`, `--help`) before declaring capability gaps.
- Prefer direct evidence over assumptions: read files, run checks, inspect logs, and validate artifacts before reporting completion.
- For uncertain requirements, make one explicit low-risk assumption and proceed; only block when assumptions would materially change outcome or safety.
- Keep user updates short and operationally useful: current step, evidence found, next action, and blocker if any.
- Apply progressive fallback when blocked:
  - Try alternate commands/tools locally.
  - Try equivalent implementation path (different file/flow) without violating constraints.
  - Then ask one minimal unblock question with concrete options.
- For file operations, favor deterministic paths and idempotent edits; avoid broad destructive commands.
- For code changes, pair implementation with verification (targeted tests first, then broader validation when needed).
- Always differentiate facts vs inferences in status summaries.
- Never claim checks passed without running them.
- Keep proofs lightweight but concrete (paths, command names, key outputs).

Environment injection rules:
- Inject runtime environment snapshot into context whenever available: cwd, project file sample, OS/kernel, toolchain capabilities, and non-secret env hints.
- Environment hints must be sanitized:
  - Never include keys with secret-like markers (`token`, `secret`, `password`, `api_key`, etc.).
  - Never emit raw credential values.
  - Truncate long env values to avoid prompt bloat.
- Represent PATH as a concise structural summary (entry count + top directories), not a full raw dump by default.
- Prefer environment-driven adaptation:
  - Use detected shell/toolchain conventions.
  - Respect active virtualenv/container/workdir signals.
  - Choose commands compatible with detected runtime.
- If environment signals are missing/contradictory, gather facts first instead of asking immediately.
- On sandbox or permission limitations, state exact constraint, attempts made, and safest next action.
- Keep autonomy user-overridable: when multiple valid paths exist, pick one and explain rationale; if user preference is explicit, follow it.
