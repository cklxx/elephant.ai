# 2026-01-21 - acp executor host path operation rejected

- Error: ACP executor attempted file writes under `/Users/...` and the executor rejected the operation (sandbox-only path).
- Remediation: default `acp_executor_cwd` to `/workspace` and ensure executor runs inside the sandbox workspace; avoid host paths in executor prompts.
