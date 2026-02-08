# 2026-02-08 — Dev Tools Workbench Consolidation + Log Virtualization

## Situation
- Dev debugging surfaces were fragmented (`conversation-debug`, `log-analyzer`, multiple config pages), and large log datasets degraded frontend responsiveness.

## Action
- Consolidated entry surfaces into workbench routes (`/dev/diagnostics`, `/dev/configuration`, `/dev/operations`) and updated `dev.sh logs-ui` to open diagnostics.
- Introduced shared dev-tools primitives (`useDebouncedValue`, virtualized list, reusable JSON/highlight renderers).
- Added structured log workbench with virtualized `log_id` sidebar and deferred payload expansion to avoid eager heavy stringify/render work.
- Upgraded diagnostics event stream rendering from full list mapping to virtualized viewport rendering.

## Result
- Debug workflow is centralized and easier to operate from one place.
- Large log/event sets avoid full DOM growth and unnecessary payload rendering, improving interaction smoothness.
