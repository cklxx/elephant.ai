# 2026-02-02 - dev.sh sandbox env_flags unbound

## Error
- `./dev.sh down && ./dev.sh` failed: `scripts/lib/common/sandbox.sh: line 63: env_flags[@]: unbound variable`.

## Impact
- Local dev restart aborted during sandbox startup.

## Notes
- Happened after sandbox recreation when ACP port mapping was missing.

## Remediation Ideas
- Initialize `env_flags` array before use or guard unset array access.

## Status
- mitigated (guarded env_flags expansion in sandbox helper)
