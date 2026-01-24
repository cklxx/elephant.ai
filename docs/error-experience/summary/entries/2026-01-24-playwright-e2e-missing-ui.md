# 2026-01-24 - Playwright e2e missing UI elements and redirect

- Summary: `npm --prefix web run e2e` failed across all browsers because the console UI never loaded (missing `console-header-title` and `session-list-toggle`) and `/` did not redirect to `/conversation`.
- Remediation: ensure the web app is running and Playwright base URL/seed state is correct before executing e2e; shut down the report server after review.
