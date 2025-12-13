# Evaluation system status and TODO

## Current state
- Local and server evaluations capture auto-scores, task-level snapshots, and derived agent profiles while persisting artifacts to the evaluation results directory for reuse across restarts.
- Stored evaluations and profiles can be listed, queried, shown, and deleted through the shared datastore, manager, HTTP API, and CLI subcommands (agents/history/list/show/delete) with filtering by agent, time window, score floor, dataset path/type, and tags.
- CLI and web entry points reuse the same evaluation manager so runs triggered locally or via the dashboard produce consistent reports, indexing, and recovery from on-disk snapshots.

## TODO and gaps
- Realtime web updates via SSE/WebSockets for job progress and worker-level streaming without manual refreshes.
- Export/share flows for completed evaluations (Markdown/JSON downloads, permalinks) and richer search across agents and histories.
- Access control, quotas, and retention policies for evaluation artifacts and agent profiles, including cleanup/archival of old runs.
- Broader filtering beyond current fields (owners, free-text search) plus dataset catalog discovery and cross-agent comparison views.
- Hardening and coverage for edge cases such as SSE reconnect, API error handling, and dataset ingestion/validation.
