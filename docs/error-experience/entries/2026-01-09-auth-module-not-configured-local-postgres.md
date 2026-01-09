# 2026-01-09 - auth module not configured local postgres

- Error: `Authentication module not configured` when `auth.database_url` pointed to localhost without a running Postgres.
- Remediation: start `auth-db` via Docker (e.g. `scripts/setup_local_auth_db.sh`) or clear `auth.database_url` in `config.yaml` for in-memory auth.
