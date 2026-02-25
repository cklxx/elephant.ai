# 2026-01-09 - dev.sh auth db setup gated on psql

- Summary: `./dev.sh` skipped auth DB setup when `psql` was missing even though Docker could run migrations.
- Remediation: remove the `psql` gate and fall back to `docker exec` in the setup script.
