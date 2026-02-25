# 2026-01-22 - cli sandbox shell connection refused

- Summary: CLI exposed sandbox_* tools and hit `sandbox request failed: connection refused` after Docker injection without a sandbox service.
- Remediation: block sandbox_* tools in CLI presets so CLI relies on local tools.
