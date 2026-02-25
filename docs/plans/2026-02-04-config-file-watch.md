# Plan: Async Config File Watcher

## Status: Completed
## Date: 2026-02-04

## Problem
The server only loads runtime config on startup or on-demand API calls, and file edits are not monitored. We need to watch the config YAML and refresh runtime config asynchronously so updates propagate without blocking request execution.

## Plan
1. Add a runtime config cache with async reload support and tests.
2. Add a file watcher with debounce to trigger reloads, and ensure update notifications are non-blocking.
3. Wire the watcher into bootstrap and config SSE updates; update tests.
4. Run full lint + tests.
5. Update long-term memory timestamp.

## Progress
- [x] Add runtime config cache with tests.
- [x] Add file watcher with debounce and non-blocking reloads.
- [x] Wire watcher into bootstrap/config SSE and update tests.
- [x] Run full lint + tests.
- [x] Update long-term memory timestamp.
