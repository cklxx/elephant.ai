# SWE-bench Verified 提交与落地调研（2026-02-08）

## 目标
- 明确如何进入 SWE-bench Verified 榜单。
- 明确当前（2026-02-08）提交资格限制与替代路径。
- 给出本仓库可执行的最小落地方案与风险点。

## 结论（先看）
- 上榜路径是：产出预测结果 -> 使用 `sb-cli` 提交 -> 在 `SWE-bench/experiments` 仓库走审核/复现实验流程。
- `SWE-bench Verified` 目前不是“随时可交即收录”；官方在 2025-11-18 公告后要求更严格研究资格（开源 + 技术报告/论文 + 学术/研究机构属性）。
- 若不满足上述资格，官方给出的替代是先走可接受的其他榜单（例如 Multimodal），同时准备后续 Verified 资格材料。

## 官方流程要点
1. 运行评测并生成规范化预测结果（predictions）。
2. 使用 `sb-cli submit` 提交对应 split（如 `test`）。
3. 在 `SWE-bench/experiments` 完成材料与复现审查。
4. 通过后更新 leaderboard；若申请“verified”标记，还需按官方要求提交可复现实验信息并接受抽样复跑。

## 关键约束（2026-02-08）
- 官方提交页明确：自 2025-11-18 起，Verified/Multilingual 提交只接受满足研究条件的提交方。
- 这意味着工程侧“能跑通”并不等于“可上 Verified 榜单”；资格材料是并列必需项。

## 对本仓库的落地建议

### A. 技术准备（已可并行）
- 数据与流程：
  - 固化 `swe_bench verified` 的运行配置（模型、温度、max_turns、timeout、worker、成本上限）。
  - 产出结构化 artifacts：preds、summary、失败案例剖析、关键 trace。
- 评估准备：
  - 在 foundation 离线评测中保留 `SWE-bench Verified Readiness` 集合作为预检门。
  - 只有预检稳定达标后，再触发高成本真实评测。

### B. 提交准备（新增门槛）
- 资格材料：
  - 开源方法说明（复现实验步骤、版本、依赖、命令）。
  - 技术报告/论文链接（至少技术报告）。
  - 提交主体的研究属性说明（机构/团队信息）。
- 审核材料：
  - `sb-cli` 提交记录（run id）。
  - `experiments` PR 材料与复现说明。

### C. 风险与对策
- 风险：资格不满足导致无法进入 Verified。
  - 对策：先按可提交榜单完成工程闭环，再并行补齐研究材料。
- 风险：结果可复现性不足导致审核不通过。
  - 对策：冻结配置、锁定依赖、提供一键复现实验脚本与数据校验摘要。

## 建议的执行顺序
1. 每次改动先跑 foundation-suite（含 verified-readiness 集合）作为低成本闸门。
2. 闸门稳定后再跑小规模真实实例（3/10/50）做成本可控回归。
3. 准备技术报告与开源复现材料。
4. 使用 `sb-cli` 提交并在 `experiments` 发起审核。

## 参考来源
- SWE-bench 提交页：https://www.swebench.com/submit.html
- sb-cli 提交文档：https://www.swebench.com/sb-cli/user-guide/submit/
- SWE-bench experiments 仓库：https://github.com/SWE-bench/experiments
- sb-cli 仓库：https://github.com/SWE-bench/sb-cli
