# Plan: OpenClaw proactive memory + tooling research (2026-01-31)

## Goal
- Explain how openclaw implements proactivity and long-term memory, list its tools, and clarify why it works well on macOS.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Review openclaw docs/source to extract proactive triggers, memory architecture, and retention policies.
2. Catalog built-in tools/integrations and how they are exposed.
3. Identify macOS-specific integrations or platform choices that improve usability.
4. Produce a concise research report with citations.
5. Update `docs/memory/long-term.md` timestamp and plan progress.
6. Run full lint + tests.
7. Commit changes in incremental steps.

## Progress
- 2026-01-31: Plan created; engineering practices reviewed.
- 2026-01-31: Collected OpenClaw docs on memory, hooks, cron, tools, plugins, and macOS companion.
- 2026-02-01: Updated long-term memory timestamp.
- 2026-02-01: Ran `./dev.sh lint` (pass) and `./dev.sh test` (fails: data race in `internal/mcp` `TestProcessManagerReinitializesStopChan`).
- 2026-02-01: Added detailed OpenClaw proactive/memory/tools/macOS analysis from official docs.
- 2026-02-01: Re-ran `./dev.sh lint` and `./dev.sh test` (both pass; race not reproduced).
- 2026-02-01: Wrote detailed research doc in `docs/research/2026-02-01-openclaw-proactivity-memory-tools.md` and updated index.
- 2026-02-01: Ran `./dev.sh lint` and `./dev.sh test` (both pass).
