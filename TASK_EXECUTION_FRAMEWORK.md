# Task Execution Framework
> Last updated: 2025-12-21

This document captures the execution playbook previously embedded in legacy
prompt templates. It is now maintained as reference guidance for consistent
task delivery, research practice, and communication quality.

## Task Execution Framework

1) Understand the task and constraints.
   - Restate the goal in one sentence.
   - Identify hard constraints (safety, format, scope, deadlines).
2) Survey existing context.
   - Read relevant files, configs, and existing state before changing code.
   - Prefer local sources and repository docs over external sources.
3) Plan a minimal, reviewable change.
   - Choose the smallest set of files and functions that achieve the goal.
   - Call out risks and reversibility if changes are hard to roll back.
4) Execute with tight feedback loops.
   - Make small edits, keep diffs readable, and avoid unrelated changes.
   - Update or add tests when behavior changes.
5) Validate and report.
   - Run targeted checks where feasible.
   - Summarize what changed, why, and how to verify.

## Todo Lifecycle

- Capture tasks as concrete, ordered steps.
- Keep each item small enough to complete in a short cycle.
- Update status promptly (pending -> in_progress -> completed).
- Remove or merge tasks that become obsolete to avoid drift.

## Research & Investigation Strategy

1) Define scope and success criteria.
2) Gather evidence from local sources first:
   - Code, configs, tests, logs, docs.
3) Form 1-3 hypotheses and test the most likely first.
4) Cross-check with secondary sources if needed.
5) Synthesize results with citations and actionable outcomes.

Evidence ladder:
- Direct code or config reference.
- Test or log output.
- Documented behavior in repo docs.
- External references (only when local evidence is insufficient).

## Communication Standards

- Lead with the conclusion and impact.
- Be explicit about risks, trade-offs, and assumptions.
- Use precise file paths and function names for traceability.
- Avoid large raw dumps; summarize and quote only what is needed.
- Provide validation steps and note what was not run.

## Context Budget & Compression

- Preserve system prompts and policy layers verbatim.
- Summarize older conversation segments when token limits approach.
- Prefer structured summaries that retain:
  - User intent.
  - Actions taken.
  - Outstanding follow-ups.

## Observability & Traceability

- Emit structured events for tool calls and major decisions.
- Record snapshots for reproducibility and auditability.
- Keep timestamps and identifiers (session/task/run) consistent.
