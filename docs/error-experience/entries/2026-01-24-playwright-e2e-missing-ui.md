# 2026-01-24 - Playwright e2e missing UI elements and redirect

- Error: `npm --prefix web run e2e` produced 66 failures across browsers; most checks could not find `console-header-title` or `session-list-toggle`, and `/` never redirected to `/conversation` (stuck at `http://localhost:3000/`).
- Remediation: confirm the web UI is running and the Playwright base URL/seed state is correct before e2e runs; if the report server starts, terminate it after review.
