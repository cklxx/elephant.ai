Summary: 跨渠道的 team runtime 能力应使用一级 CLI 命令，不应挂在 `lark` 子命令；skills 紧凑提示必须保留 runner 信号，确保 LLM 能直接推断 `shell_exec + python3 skills/<name>/run.py` 调用路径。

## Metadata
- id: errsum-2026-03-05-user-correction-team-cli-and-artifacts-should-be-first-class
- tags: [summary, user-correction, cli, skills]
- derived_from:
  - docs/error-experience/entries/2026-03-05-user-correction-team-cli-and-artifacts-should-be-first-class.md
