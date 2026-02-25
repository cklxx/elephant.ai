# Plan: OpenAI Single Key Bootstrap + Lark Docker Auto-Setup

## Goal

- Initialize with only `OPENAI_API_KEY` to get the system running locally.
- Lark standalone mode automatically ensures Docker sandbox is available.
- Config templates auto-created on first run.

## Scope

1. Bootstrap script (`scripts/setup_local_runtime.sh`) auto-creates `.env`, `~/.alex/config.yaml`, `~/.alex/test.yaml`
2. `dev.sh` calls bootstrap before starting services; adds `sandbox-up/down/status` commands
3. Lark scripts (`main.sh`, `test.sh`) integrate `load_dotenv`, bootstrap, Docker sandbox, auth DB
4. `worktree.sh` falls back to `.env.example` when `.env` missing
5. `.env.example` promotes `OPENAI_API_KEY` as the minimal required key
6. `runtime-config.yaml` cleaned of obsolete async event history comments
7. New `runtime-test-config.yaml` template (port 8081)

## Checklist

- [x] Create `scripts/setup_local_runtime.sh`
- [x] Create `examples/config/runtime-test-config.yaml`
- [x] Update `.env.example`
- [x] Update `dev.sh` (bootstrap + sandbox commands)
- [x] Update `scripts/lark/main.sh` (bootstrap + docker + dotenv + auth)
- [x] Update `scripts/lark/test.sh` (same)
- [x] Update `scripts/lark/worktree.sh` (.env.example fallback)
- [x] Clean `runtime-config.yaml`
- [x] Tests pass
- [x] Merged to main (4 incremental commits)
- [x] Old worktree (`elephant.ai-wt-openai-key`) and branch cleaned up

## Commits

1. `feat(bootstrap): add setup_local_runtime.sh and test config template`
2. `chore(config): promote OPENAI_API_KEY in .env.example, cleanup runtime config`
3. `feat(dev): wire bootstrap and sandbox-only commands into dev.sh`
4. `feat(lark): integrate bootstrap, dotenv, and docker sandbox into lark scripts`

## Status: COMPLETE (2026-02-08)
