# 2026-01-21 - acp executor host path operation rejected

- Summary: executor rejected file writes under host paths (e.g., `/Users/...`) instead of sandbox workspace.
- Remediation: default executor cwd to `/workspace` and keep executor runs inside sandbox; avoid passing host paths.
