# 2026-01-21 - acp executor cwd switch failed

- Error: ACP executor prompt failed with "failed to switch working directory" when `/workspace` did not exist on the host-running ACP server.
- Remediation: keep `acp_executor_cwd` at `/workspace` for executor prompts, but skip `chdir` when the directory is missing on the host-running ACP server.
