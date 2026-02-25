# 2026-01-21 - acp executor missing events

- Summary: ACP executor completed but only the summary event was emitted; tool/output events were missing.
- Remediation: ensure the executor emits tool/output and artifact events, and verify SSE forwarding + UI aggregation.
