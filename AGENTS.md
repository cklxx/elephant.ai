# elephant.ai — Proactive AI Assistant (Repo Rules)

## Project snapshot
- Proactive assistant across Lark/CLI/Web with persistent memory + skills.
- Architecture: Delivery → App layer → Domain (ReAct loop, approvals, context) → Infra (LLM/tools/memory/observability).
- Key packages: `internal/agent/`, `internal/llm/`, `internal/memory/`, `internal/tools/builtin/`, `internal/channels/`, `internal/observability/`, `web/`.

## Non‑negotiables (must follow)
- Assist **cklxx**; address as “cklxx” first; assume senior backend/db engineer.
- Prefer correctness/maintainability; start with a systematic view and a reasonable plan.
- Start fresh from `main`: create a new worktree on a new branch and copy `.env` into the worktree before coding; when done, merge back into `main` (prefer fast-forward) and then remove the temporary worktree (and optionally delete the branch).
- From scratch, cut a new worktree branch off main and copy .env, then write code. After finishing, merge back to main.
- Config examples: YAML only, use `.yaml` paths.
- Plans: non‑trivial work requires a plan file under `docs/plans/` and updates as work progresses.
- Practices: review `docs/guides/engineering-practices.md` before tasks; add missing practices if needed.
- Records: error/win entries live in `docs/error-experience/entries/` and `docs/good-experience/entries/`; summaries in `.../summary/entries/`; index files are index-only.
- Testing: use TDD when touching logic; run full lint + tests before delivery.
- Commits: always commit; split one solution into multiple incremental commits.
- Safety: avoid destructive operations/history rewrites unless explicitly requested.
- Architecture guardrails: keep `agent/ports` free of memory/RAG deps to avoid import cycles.
- Coding: avoid unnecessary defensive code; trust guaranteed invariants; no compatibility shims—refactor cleanly.
- Prefer using subagents for parallelizable tasks to improve execution speed.

## Memory loading (minimal, repeatable)
- First run in repo: read latest 3–5 items from error/good entries + summaries + `docs/memory/long-term.md`, rank by recency/frequency/relevance, keep top 8–12 as active memory.
- Use summaries first; open full entries only if summaries are insufficient.
- Expand beyond active memory only when a known pattern is relevant or tests fail with known signatures.
- Update `docs/memory/long-term.md` `Updated:` to hour precision; refresh active set on first load each day.

## Code Review — mandatory step after coding

### When to trigger
Code review is **mandatory** before every commit or merge. The following scenarios trigger a review:
- Feature development complete, ready to commit
- Bug fix complete, ready to merge
- Refactoring complete, ready to submit PR
- User explicitly requests a review

### Review process

1. **Determine scope**: Run `git diff --stat` to identify the scope of changes; record the number of files and lines changed.
2. **Load skill**: Execute the 7-step workflow defined in `skills/code-review/SKILL.md`.
3. **Review by dimension**: Check SOLID/architecture, security/reliability, code quality/edge cases, and cleanup plan in order.
4. **Generate report**: Output a structured review report organized by severity level (P0–P3).
5. **Confirm fixes**: Present the report and wait for the user to confirm the resolution approach; re-verify after fixes are applied.

### Review reference checklists

Load the following reference files during review for specific check items:
- `skills/code-review/references/solid-checklist.md` — SOLID principles and code smells
- `skills/code-review/references/security-checklist.md` — Security and reliability (including race condition checks)
- `skills/code-review/references/code-quality-checklist.md` — Error handling, performance, edge cases, observability
- `skills/code-review/references/removal-plan.md` — Dead code identification and cleanup plan template

### Review principles

- **Understand before judging**: Read the full context of changes before reviewing; understand the design intent.
- **Prioritize by severity**: P0/P1 must be fixed, P2 creates a follow-up, P3 is optional.
- **Provide fix suggestions**: For every issue, not only identify the problem but also provide a concrete fix recommendation.
- **Respect architectural decisions**: Do not push personal preferences; focus on correctness and security.
- **Go/Rust specifics**: Perform targeted checks for language-specific issues in the project's primary languages.

### Integration with existing workflow

Code review is part of the coding → testing → review → commit pipeline:

```
Coding complete
  → lint + test pass
  → Code review (this skill)
  → Fix issues found during review
  → Re-run lint + test
  → Commit (incremental commits)
  → Merge to main
```

Code review does not replace lint and tests; it supplements them by catching architecture, security, and logic issues that automated tools cannot detect.
