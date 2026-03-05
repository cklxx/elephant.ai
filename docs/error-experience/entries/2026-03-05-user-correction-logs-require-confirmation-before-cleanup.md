# 2026-03-05 · 清理主目录时误删 logs，未先确认“运行证据目录”是否仍在使用

## Context
- 任务目标：按用户要求清理主目录无用文件与目录。
- 实际执行：删除了 `logs/` 等忽略目录与构建产物。
- 用户纠正：`我靠 logs 确定不要吗`，明确指出 `logs` 仍是其排障依据。

## Symptom
- 将 `logs/` 直接归类为“可清理缓存目录”，导致删除了本地历史运行日志。

## Root Cause
- 清理动作中把“Git 忽略”错误等同于“业务无价值”。
- 未执行“运行证据目录二次确认”步骤，直接批量清理。

## Remediation
- 任何清理请求中，以下目录默认视为“运行证据目录”，禁止无确认删除：
  - `logs/`
  - `artifacts/`
  - `docs/reports/`（仅允许按文件级明确指令删除）
- 仅当用户明确点名可删时，才清理上述目录。
- 批量清理前先做 dry-run 列表确认，并在执行前单独列出“证据目录”项。

## Follow-up
- 后续执行“主目录清理”任务统一采用顺序：
  1) 删除用户点名文件；
  2) 删除明显二进制/缓存产物；
  3) 证据目录单独确认后再执行。

## Metadata
- id: err-2026-03-05-user-correction-logs-require-confirmation-before-cleanup
- tags: [user-correction, cleanup, logs, safety]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-logs-require-confirmation-before-cleanup.md
