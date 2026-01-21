# 2026-01-21 - acp executor cwd switch failed

- Summary: ACP executor prompt failed to switch to `/workspace` when the directory was missing or inaccessible on the host.
- Remediation: keep `/workspace` as the executor CWD in prompts, but skip `chdir` when the host ACP server cannot enter that directory.
