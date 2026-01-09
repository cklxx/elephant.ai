# 2026-01-09 - dev.sh auth db setup skipped without psql

- Error: `./dev.sh` skipped local auth DB auto-setup when `psql` was missing, even though Docker could run migrations.
- Remediation: remove the `psql` gate in `dev.sh` and let the setup script fall back to `docker exec`.
