Summary: 全局复扫后先做非 `web/` 低风险项（R-03/R-06/Q-08/E-12/R-08 子集），用 explorer 收敛、worker 按 ownership 并行落地，再由主 agent 跑全量门禁，是高吞吐且低冲突的推进方式。

## Metadata
- id: goodsum-2026-03-03-subagent-global-simplify-rescan-wave2-wave3b
- tags: [summary, subagent, simplify, non-web]
- derived_from:
  - docs/good-experience/entries/2026-03-03-subagent-global-simplify-rescan-wave2-wave3b.md
