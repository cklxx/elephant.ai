# elephant.ai — Proactive AI Assistant (Repo Rules)

## Project snapshot
- Proactive assistant across Lark/CLI/Web with persistent memory + skills.
- Architecture: Delivery → App layer → Domain (ReAct loop, approvals, context) → Infra (LLM/tools/memory/observability).
- Key packages: `internal/agent/`, `internal/llm/`, `internal/memory/`, `internal/tools/builtin/`, `internal/channels/`, `internal/observability/`, `web/`.

## Non‑negotiables (must follow)
- Assist **cklxx**; address as “cklxx” first; assume senior backend/db engineer.
- Prefer correctness/maintainability; start with a systematic view and a reasonable plan.
- Config examples: YAML only, use `.yaml` paths.
- Plans: non‑trivial work requires a plan file under `docs/plans/` and updates as work progresses.
- Practices: review `docs/guides/engineering-practices.md` before tasks; add missing practices if needed.
- Records: error/win entries live in `docs/error-experience/entries/` and `docs/good-experience/entries/`; summaries in `.../summary/entries/`; index files are index-only.
- Testing: use TDD when touching logic; run full lint + tests before delivery.
- After completing code changes, restart the project with `./dev.sh down && ./dev.sh`.
- Commits: always commit; split one solution into multiple incremental commits.
- Safety: avoid destructive operations/history rewrites unless explicitly requested.
- Architecture guardrails: keep `agent/ports` free of memory/RAG deps to avoid import cycles.
- Coding: avoid unnecessary defensive code; trust guaranteed invariants; no compatibility shims—refactor cleanly.

## Memory loading (minimal, repeatable)
- First run in repo: read latest 3–5 items from error/good entries + summaries + `docs/memory/long-term.md`, rank by recency/frequency/relevance, keep top 8–12 as active memory.
- Use summaries first; open full entries only if summaries are insufficient.
- Expand beyond active memory only when a known pattern is relevant or tests fail with known signatures.
- Update `docs/memory/long-term.md` `Updated:` to hour precision; refresh active set on first load each day.
