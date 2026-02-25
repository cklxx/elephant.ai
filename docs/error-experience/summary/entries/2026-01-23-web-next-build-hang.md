# 2026-01-23 - web next build hang

- Summary: `npm --prefix web test` stalled in `next build` (Turbopack) for >8 minutes; retry with `NEXT_TELEMETRY_DISABLED=1 TURBOPACK=0` also stalled and was killed.
- Remediation: retry `npm --prefix web run build`; if it stalls, disable Turbopack (`TURBOPACK=0`) or enable debug output to capture the hang point.
