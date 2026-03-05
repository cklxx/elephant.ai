Summary: `command not found` 不能直接等同于“本机缺依赖”；必须先验证执行目录、依赖安装与 PATH，再下结论并优先使用 `npm exec` 这类稳健调用。

## Metadata
- id: errsum-2026-03-05-lint-missing-eslint-misdiagnosis
- tags: [summary, lint, eslint, diagnosis]
- derived_from:
  - docs/error-experience/entries/2026-03-05-lint-missing-eslint-misdiagnosis.md
