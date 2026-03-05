# 2026-03-05 · 用户要求复用既有授权实现但我先行重写授权路径

## Context
- 任务目标：将飞书工具统一为 CLI 并通过 skill 暴露给 agent。
- 用户补充纠正：授权部分应复用之前代码，统一运行时和环境变量；通过完整测试后再全量迁移并删除旧实现。

## Symptom
- 我先实现了新的授权管理器路径，偏离了“复用既有授权代码”的用户约束。

## Root Cause
- 在复杂重构中优先追求统一封装，未先锁定“哪些历史实现必须复用”这一硬约束。
- 纠偏前缺少“新要求落地为第一条技术决策”的显式步骤。

## Remediation
- 收到用户对实现路径的纠正后，立即执行：
  - 将纠正内容写成首条不可违反约束（本次：复用既有授权实现）。
  - 暂停新增实现，先改造为“旧实现为内核、新 CLI 为外壳”。
  - 统一运行时入口和环境变量命名，禁止并行多套授权分支。
  - 完整测试通过后，再批量迁移调用方并删除冗余旧代码。

## Follow-up
- 今后凡是涉及“迁移/重构 + 用户指定保留策略”的任务，先做“保留策略清单”，再写第一行代码。

## Metadata
- id: err-2026-03-05-user-correction-reuse-existing-auth-and-unified-runtime
- tags: [user-correction, migration, auth, runtime-unification]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-reuse-existing-auth-and-unified-runtime.md
  - docs/plans/2026-03-05-feishu-cli-skillification.md
