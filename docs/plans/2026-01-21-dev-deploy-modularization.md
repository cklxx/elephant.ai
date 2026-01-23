# Plan: Modularize dev.sh & deploy.sh (2026-01-21)

## Goal
- Split shared logic from `dev.sh` and `deploy.sh` into reusable script modules.
- Keep behavior stable while reducing duplication.

## Constraints
- Bash scripts; no JSON config examples.
- Keep changes small and reviewable.
- Run full lint + tests after refactor.

## Plan
1. Extract common helpers (logging, process, ports, http) into `scripts/lib/common/`.
2. Extract ACP host helpers into `scripts/lib/acp_host.sh` and reuse in both scripts.
3. Update `dev.sh` and `deploy.sh` to source shared modules; remove duplicated functions.
4. Verify script flow manually (sanity check) and run `./dev.sh lint` + `./dev.sh test`.

## Progress
- 2026-01-21: Plan created.
- 2026-01-21: Added common script libs and ACP host helper module; updated dev.sh/deploy.sh to use shared helpers.
- 2026-01-21: Logged ACP executor missing-event incident in error experience entries.
- 2026-01-21: Ran `./dev.sh lint` and `./dev.sh test` (web tests emit intermittent happy-dom AbortError logs but pass).
