# Plan: Systematic Log Cleanup and Governance (2026-02-24)

## Status
- in_progress

## Goal
- 系统性梳理项目日志调用，删除冗余日志，保留关键可观测性信号，并沉淀后续日志治理规则。

## Scope
- `internal/**`、`cmd/**`、`scripts/**` 中的日志调用。
- 日志治理文档（规则、分级、准入标准）。

## Steps
- [x] 读取工程规范与记忆文档，确认约束。
- [x] 创建独立 worktree 分支，隔离当前改动。
- [x] 全量扫描日志调用并建立分类清单（删除/降级/保留）。
- [x] 批量实施日志精简与级别治理。
- [x] 补充日志治理文档，形成系统化规则。
- [x] 运行格式化、lint、测试、代码评审。
- [x] 按功能分批提交并准备交付说明。

## Progress Log
- 2026-02-24 22:08: 计划创建，已完成规范与记忆加载，worktree `../elephant-log-cleanup` 初始化完成。
- 2026-02-24 22:22: 完成全仓日志点扫描，识别热点为 `react runtime`、`task execution service`、`event broadcaster`。
- 2026-02-24 22:35: 完成第一轮日志精简（删除重复日志、降低热路径日志级别、移除冗余前缀）。
- 2026-02-24 22:40: 输出系统化审计文档 `docs/analysis/2026-02-24-log-redundancy-audit.md`，并新增 `scripts/analysis/log_audit.sh` 审计脚本。
- 2026-02-24 23:20: 完成全链路校验（`./scripts/pre-push.sh` 全通过）并执行代码评审技能脚本。
