Summary: `npm --prefix web test` stalled in `next build` (Turbopack) for >8 minutes; build had to be killed.

Remediation: Retry `npm --prefix web run build`; if it stalls, disable Turbopack (`TURBOPACK=0`) or enable debug output to capture the hang point.
