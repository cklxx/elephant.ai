# 2026-01-22 - cli sandbox shell connection refused

- Error: CLI tool list exposed `sandbox_shell_exec`, causing `sandbox request failed: connection refused` after CLI was injected into Docker without a sandbox service.
- Remediation: block sandbox_* tools in CLI presets so CLI uses local file/shell tools only.
