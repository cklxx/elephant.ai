# 2026-01-09 - psql missing use docker exec

- Error: `psql` was missing on host, so local auth DB setup skipped migrations/seeding.
- Remediation: use Docker exec inside `alex-auth-db` (script now falls back) or install `psql`.
