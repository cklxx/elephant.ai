# 2026-01-21 - acp executor cwd switch failed

- Error: ACP executor prompt failed with "failed to switch working directory" when `/workspace` did not exist on the host-running ACP server.
- Remediation: default `acp_executor_cwd` to `/workspace` only when the path exists; otherwise fall back to the current working directory.
