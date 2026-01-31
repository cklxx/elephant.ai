# Plan: Remove WeChat support

**Status**: In Progress
**Date**: 2026-01-31

## Goals
- Remove WeChat channel support (config + code paths) from backend/server.
- Remove WeChat OAuth/login UI and provider wiring from web.
- Clean configuration/docs/tests/dependencies referencing WeChat.
- Remove WeChat entries from local runtime config (keep file): `/Users/bytedance/.alex/config.yaml`.

## Plan
1. Inventory WeChat references (backend, web, config, docs) and decide removals/updates.
2. Remove backend WeChat channel code paths and config structs; update tests.
3. Remove web WeChat OAuth flow and translations; update tests/types.
4. Clean docs/config examples and dependencies (go.mod/go.sum, env examples).
5. Run full lint + tests; fix or record issues.
6. Commit in multiple incremental commits.

## Progress Log
- 2026-01-31: Plan created; WeChat references enumerated; execution started.
- 2026-01-31: Removed WeChat channel code, config structs, auth endpoints, and web OAuth UI. Updated README/config docs and cleaned deps.
- 2026-01-31: `./dev.sh lint` passed; `./dev.sh test` failed on a pre-existing data race in `internal/agent/app/coordinator` (SerializingEventListener). WeChat removal changes compiled but full suite still blocked.
