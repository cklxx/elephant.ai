# Plan: 补充 server architecture audit 方案

**Created:** 2026-01-31
**Owner:** cklxx
**Status:** In Progress

## Scope
- 补齐审计方案中缺失的 remediation/落地步骤与依赖关系
- 明确 TaskProgressTracker、一致性/回收/背压策略
- 明确路由重构兼容性策略与回归测试矩阵
- 明确事件常量落点与依赖方向，避免 import cycle
- 明确健康检查降级输出契约与 WriteTimeout 方案

## Steps
1. 盘点缺口并补充对应 Phase/子步骤与风险说明
2. 补全 TaskProgressTracker 设计约束与回收策略
3. 补全路由重构兼容性与测试矩阵
4. 补全健康检查降级与 WriteTimeout 实现路径
5. 运行全量 lint + tests，更新状态

## Progress
- 2026-01-31: Step 1-4 done (补齐 remediation/兼容矩阵/一致性/超时策略)
- 2026-01-31: Step 5 pending
