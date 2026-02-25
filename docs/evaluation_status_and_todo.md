# Evaluation system status and TODO

Updated: 2026-02-23

## Current state
- Local and server evaluations capture auto-scores, task-level snapshots, and derived agent profiles while persisting artifacts to the evaluation results directory for reuse across restarts.
- Stored evaluations and profiles can be listed, queried, shown, and deleted through the shared datastore, manager, HTTP API, and CLI subcommands (agents/history/list/show/delete) with filtering by agent, time window, score floor, dataset path/type, and tags.
- CLI and web entry points reuse the same evaluation manager so runs triggered locally or via the dashboard produce consistent reports, indexing, and recovery from on-disk snapshots.

### Feb 2026 eval campaign progress
- **Foundation suites**: Systematic expansion across 8+ rounds (r2–r12), covering tool coverage, prompt effectiveness, proactivity, and complex task delivery. Layered by hardness: Core-Hard / Frontier-Hard / Research-Frontier-Hard.
- **Suite pruning**: Multiple prune rounds to retire easy-pass cases and inject conflict-heavy replacements (under-500 threshold, r2/r3/r4 rounds).
- **Pass@k metrics**: pass@1 and pass@5 tracked per collection; foundation pass@1 saturation used as trigger for case retirement.
- **Routing optimization**: 17+ rounds of product routing optimization (r6–r18), semantic convergence, heuristic token matching fixes.
- **E2E systematic rebuild**: Rebuilt capability-layered evaluation (Foundation Core / Stateful-Memory / Delivery / Frontier Transfer).
- **Eval skill**: Dedicated `eval-systematic-optimization` skill for running evaluation rounds.
- **Eval automation**: `eval-path-automation-for-real-e2e` plan delivered automated evaluation pipelines.

## TODO and gaps
- Realtime web updates via SSE/WebSockets for job progress and worker-level streaming without manual refreshes.
- Export/share flows for completed evaluations (Markdown/JSON downloads, permalinks) and richer search across agents and histories.
- Access control, quotas, and retention policies for evaluation artifacts and agent profiles, including cleanup/archival of old runs.
- Broader filtering beyond current fields (owners, free-text search) plus dataset catalog discovery and cross-agent comparison views.
- Hardening and coverage for edge cases such as SSE reconnect, API error handling, and dataset ingestion/validation.
- Continuous eval integration: automated nightly eval runs with regression detection and Lark notification.
