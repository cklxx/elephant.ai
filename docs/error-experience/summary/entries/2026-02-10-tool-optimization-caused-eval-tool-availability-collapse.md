Summary: 大规模工具优化后，`web/full/default` 下可用工具集显著收缩，导致 foundation 评测出现 `N/A` 激增（E2E `177`，Current `138`）与 deliverable 质量归零，评测结果被 availability 问题主导。
Remediation: 先恢复工具注册覆盖并增加 pre-eval inventory parity gate，再进行语义/提示词优化。
