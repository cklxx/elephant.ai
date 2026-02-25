# 2026-01-09 - auth db migration file not in container

- Summary: migrations failed when `psql` ran inside Docker with `-f /host/path`, because the file wasn't in the container.
- Remediation: pipe the migration file via stdin (or copy it into the container) before running `psql`.
