# 2026-01-21 - acp executor missing events

- Error: ACP executor runs completed but only emitted the final summary event; tool/output events were missing from the stream.
- Remediation: ensure executor emits tool/output and artifact events (even for minimal runs) and verify SSE forwarding + UI aggregation.
