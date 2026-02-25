Summary: Refresh 401 was caused by auth Postgres client exhaustion, which triggered development-mode fallback to memory stores and lost refresh sessions after restart.
Remediation: Cleaned stale server processes and introduced configurable auth DB pool cap (`database_pool_max_conns` / `AUTH_DATABASE_POOL_MAX_CONNS`) with default `4`.
