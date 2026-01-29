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
