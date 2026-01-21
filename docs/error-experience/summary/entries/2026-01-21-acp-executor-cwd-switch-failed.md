# 2026-01-21 - acp executor cwd switch failed

- Summary: ACP executor prompt failed to switch to `/workspace` when the directory did not exist on the host.
- Remediation: default `acp_executor_cwd` to `/workspace` only if it exists; otherwise use the current working directory.
