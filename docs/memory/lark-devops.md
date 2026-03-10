# Lark & DevOps

Updated: 2026-03-10 18:00

## Lark Local Ops

- Record the real `alex-server` PID, not the wrapper shell PID.
- If auth DB startup hits `too many clients already`, clear orphan Lark processes and retry with backoff.
- Keep Lark loop auto-fix off by default. Enable it only with `LARK_LOOP_AUTOFIX_ENABLED=1`.
- Background progress listeners must call `Release()`, not `Close()`.
- Lark model selection should resolve at chat scope first, then fall back to legacy scope.
- Callback token and encrypt key can come from env expansion; missing callback config can silently disable callbacks.

## Process Management

- PID recovery and stop logic must verify process identity, not only `kill(0)`.
- Wait callbacks must confirm they are cleaning up the same process instance.
- Restart thresholds use `>= limit -> cooldown`, and cooldown backoff should not block unrelated work.

## Auth

- In local multi-process development, cap auth DB connections with `auth.database_pool_max_conns` or `AUTH_DATABASE_POOL_MAX_CONNS` (default `4`).
- JWT and OAuth config should support env fallback.
- In development-like environments, auth DB failures should degrade to memory stores instead of disabling auth entirely.
