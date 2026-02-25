# Plan: OpenClaw-style Markdown Memory (2026-02-02)

## Goal
Replace the legacy memory system (DB/hybrid/entries/daily summaries) with a pure Markdown memory layout:
- `~/.alex/memory/MEMORY.md` (long-term)
- `~/.alex/memory/memory/YYYY-MM-DD.md` (daily log)

## Scope
- Remove legacy memory stores (file/postgres/hybrid), retention, daily summarizer, long-term extractor.
- Replace memory tools with `memory_search` and `memory_get`.
- Wire context/system prompt to load MEMORY.md + today/yesterday daily logs.
- Keep compaction flush + stale session capture by appending to daily logs.
- Update docs/config to match the new system.

## Plan
1) Add new Markdown memory engine + tests.
2) Replace memory tools + tool registry wiring.
3) Remove auto memory hooks and legacy memory stores/tests.
4) Inject memory content into system prompt and add memory SOP doc.
5) Update config + docs + presets + dev endpoints.
6) Run full lint + tests; fix failures.

## Progress
- 2026-02-02: Plan created.
- 2026-02-02: Removed legacy memory stores/hooks/tools; added Markdown memory engine + tools.
- 2026-02-02: Injected Markdown memory into system prompt; wired compaction flush + stale session capture to daily logs.
- 2026-02-02: Updated config/docs/dev UI to reflect Markdown memory.
