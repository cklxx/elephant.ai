Summary: Kernel minute-level repeated cycles on a shared subscription profile caused bursty LLM traffic and intermittent API rate-limit failures.
Remediation: Restore conservative default cadence (`0,30 * * * *`), keep pinned profile routing for kernel dispatches, and retain real-tool-action success guard.
