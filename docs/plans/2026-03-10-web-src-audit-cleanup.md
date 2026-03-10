Status: completed

# Web Src Audit Cleanup

## Goal

Audit `web/src/` and adjacent frontend entrypoints for unused components, dead imports, stale API calls, and duplicated logic. Remove dead code and simplify only low-risk frontend paths.

## Plan

1. Inventory the actual frontend source layout and cross-reference components and hooks against live imports.
2. Delete confirmed dead code and simplify duplicated logic where behavior is preserved.
3. Run proportionate frontend validation and review, then commit and merge back to `main` without pushing.

## Outcome

- Confirmed `web/src/` no longer exists; the active frontend lives under `web/app`, `web/components`, `web/hooks`, and `web/lib`.
- Removed a dead visualizer UI subtree that had no page entrypoints:
  - `web/components/visualizer/*`
  - `web/hooks/useVisualizerStream.ts`
  - `web/hooks/__tests__/useVisualizerStream.test.tsx`
- Removed other zero-reference frontend code:
  - `web/components/agent/IntermediatePanel.tsx`
  - `web/components/agent/tooling/ToolCallLayout.tsx`
  - `web/hooks/useAgentStreamStore.ts`
  - their orphaned tests
- The deleted visualizer code was the only frontend caller of the old `/api/visualizer/*` fetches, so stale API usage was removed from the UI layer without changing server routes.
