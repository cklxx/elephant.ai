Error: `npm --prefix web test` hung during the `next build` step (Turbopack) for >8 minutes with near-zero CPU; had to terminate the `next build` process.

Remediation: Re-run `npm --prefix web run build` separately to isolate; if it hangs again, try `NEXT_TELEMETRY_DISABLED=1` and `TURBOPACK=0`, or capture build logs with `NEXT_DEBUG=1` to diagnose. Consider moving build out of `pretest` if this persists.
