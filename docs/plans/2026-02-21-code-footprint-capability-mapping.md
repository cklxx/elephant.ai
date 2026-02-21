# 2026-02-21 代码量与功能映射全量报告计划

## 目标
- 生成覆盖仓库 tracked 文件的导出清单（尽量覆盖所有文件）。
- 建立代码量与功能实现的一一映射，细分到模块内子模块。
- 基于数据给出代码量合理性评估和可执行优化点。

## 产物
- `docs/analysis/2026-02-21-repo-file-catalog.csv`
- `docs/analysis/2026-02-21-code-file-capability-mapping.csv`
- `docs/analysis/2026-02-21-code-footprint-capability-mapping-report.md`

## 执行步骤
- [x] 收集基线：tracked 文件总量、目录分布。
- [x] 使用 subagent 并行扫描 Domain/App-Delivery/Infra/Web 架构问题。
- [x] 生成全量文件导出（路径、类型、代码量、模块归属）。
- [x] 生成代码文件功能映射导出（模块/子模块/能力）。
- [x] 编写合理性评估与优化建议报告。
- [x] 校验导出覆盖率并记录统计口径。
- [x] 完成 mandatory code review（按仓库 skill 流程）。

## 统计口径
- 文件覆盖范围：`git ls-tree -r --name-only HEAD`（HEAD 快照）。
- 代码量口径：按文件行数（`\n` 分隔）与字节数。
- 模块归属：按路径前缀和目录层级映射。
- 功能映射：按路径规则 + 文件名关键词的启发式归类。

## 风险与控制
- 启发式归类可能存在少量误差：在报告中标注“自动推断”，并给出人工抽样建议。
- 历史遗留大文件会拉高复杂度：报告中拆分“存量合理性”与“新增约束策略”。
