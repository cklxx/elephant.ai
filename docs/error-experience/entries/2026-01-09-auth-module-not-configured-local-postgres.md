# 2026-01-09 - auth module not configured local postgres

- Error: `Authentication module not configured` when `AUTH_DATABASE_URL` pointed to localhost without a running Postgres.
- Remediation: start `auth-db` via Docker (e.g. `scripts/setup_local_auth_db.sh`) or clear auth DB envs for in-memory auth.
