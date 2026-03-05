# 2026-03-05 · 用户要求“一并提交”后必须先核定完整脏改动集合

## Context
- 用户纠正“那不是你的改动吗，一并提交并测试好”。
- 当前 `main` 工作树存在大批已暂存改动，不止此前口头提到的两个文件。

## Symptom
- 若只按局部文件继续推进，会遗漏真实待提交范围，导致合回受阻或提交不完整。

## Root Cause
- 过早基于局部观察下结论，没有先做完整 `git status --porcelain -b` 与 `git diff --cached --stat` 归并确认。

## Remediation
- 收到“全部提交/一并提交”类纠正时，先执行并汇报：
  - `git status --porcelain -b`
  - `git diff --cached --stat`
  - `git diff --stat`
- 只有在范围确认后才进入测试与提交流程。
- 明确区分“默认测试”与“真实 E2E 门控测试”，避免把环境依赖型 E2E当作单测失败处理。

## Follow-up
- 将“先核定全量脏改动再提交”作为默认提交前置动作。

## Metadata
- id: err-2026-03-05-user-correction-commit-all-dirty-main-diff
- tags: [user-correction, commit-discipline, scope-control]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-commit-all-dirty-main-diff.md
