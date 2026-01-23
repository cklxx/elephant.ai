Error: `npm --prefix web test` hung during the `next build` step (Turbopack) for >8 minutes with near-zero CPU; had to terminate the `next build` process. A retry with `NEXT_TELEMETRY_DISABLED=1 TURBOPACK=0` still stalled with zero CPU until killed.

Remediation: Re-run `npm --prefix web run build` separately to isolate; if it hangs again, try forcing Turbopack off (confirm env for Next 16), or capture build logs with `NEXT_DEBUG=1` to diagnose. Consider moving build out of `pretest` if this persists.
