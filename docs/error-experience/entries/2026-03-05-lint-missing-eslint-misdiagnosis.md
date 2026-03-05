# 2026-03-05 lint missing eslint misdiagnosis

## Summary
在 `alex dev lint` 报 `eslint: command not found` 时，未先验证执行上下文与 Node 依赖安装状态，就把现象直接表述为“本机缺 eslint”，导致结论不够严谨。

## Rule
- 遇到 `command not found` 类错误，必须先做三步验证再给结论：
  1. 确认当前工作目录与脚本调用链（谁在执行命令）。
  2. 确认依赖是否已安装（如 `web/node_modules/.bin/eslint`、`npm ls eslint`、`npm ci` 状态）。
  3. 确认 PATH 是否包含本地 bin 或脚本是否应通过 `npm exec`/`pnpm exec` 调用。
- 未完成验证前，只能描述为“当前执行上下文未找到 eslint”，不能下“环境缺失”定性结论。

## Prevention
- 在修复前固定执行：`cd web && npm ls eslint || true && ls node_modules/.bin/eslint || true`。
- 对仓库脚本优先改成对 PATH 更稳健的调用方式（如 `npm exec eslint`），减少环境差异导致的假阴性。

## Metadata
- id: err-2026-03-05-lint-missing-eslint-misdiagnosis
- tags: [lint, eslint, diagnosis, process]
- links: []
