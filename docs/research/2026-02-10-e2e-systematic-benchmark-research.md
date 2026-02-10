# 2026-02-10 业界高难 Agent 评测集合调研与映射

## 1. 调研目标
- 为 elephant.ai 重新建立“系统化、多能力维度、端到端”的 Agent 评测集合。
- 优先纳入业界公认高难集合，避免继续堆叠低挑战样例。
- 保持与当前离线路由评测框架兼容（`expected_tools` + pass@1/pass@5 + deliverable check）。

## 2. 调研方法
- 以官方站点/论文/代码仓为主，聚焦可复用为“能力维度”的 benchmark 家族。
- 对每个 benchmark 提取：
  - 主要能力压力（工具调用、长程记忆、多轮承诺、交付产物、网页/GUI 操作、研究检索）。
  - 可迁移到本项目的离线路由评测信号。
  - 对应新增或重用的数据集文件。

## 3. 核心 benchmark 家族（高优先级）

| Benchmark | 主要能力压力 | 在本项目中的映射 |
|---|---|---|
| SWE-bench / SWE-bench Verified | 真实仓库修复、多文件定位、回归验证 | `industry_benchmark_swebench_verified_hard_plus` |
| TAU-bench / TAU2 企业多轮 | 长时序状态、承诺边界、审批门控 | `industry_benchmark_tau2_long_horizon_enterprise_hard` |
| GAIA | 通用助手复杂任务、工具组合、信息整合 | `industry_benchmark_general_assistant_gaia` |
| WebArena | 真实网站多步操作、状态正确性、网页证据 | 新增 `industry_benchmark_webarena_verified_webops_hard` |
| OSWorld / OSWorld-G | GUI/Computer-use、跨模态状态感知 | `industry_benchmark_osworld_g_grounded_computer_use_hard` |
| AgentBench | 多环境工具使用、跨域执行稳定性 | 新增 `industry_benchmark_agentbench_multidomain_tooluse_hard` |
| ToolSandbox | API/工具调用流程、参数化操作鲁棒性 | 作为 Tool-use 维度补充，优先映射到 AgentBench/Tool Coverage 组合 |
| BrowseComp | 稀疏线索检索、弱词面重叠、证据收敛 | 新增 `industry_benchmark_browsecomp_sparse_research_hard` |
| AgentLongBench | 长上下文记忆、线程连续性、长程任务纪律 | 新增 `industry_benchmark_agentlongbench_long_context_memory_hard` |
| RE-Bench | 前沿 ML R&D 开放任务、实验闭环 | `industry_benchmark_re_bench_frontier_ml_rd_hard` |
| MLE-bench | 实验生命周期、可复现与产物交付 | `industry_benchmark_mle_bench_experiment_lifecycle_hard` |
| CyBench | 安全运营流程、修复与证据打包 | `industry_benchmark_cybench_security_ops_hard` |

## 4. 新增集合（本轮）
- `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_webarena_verified_webops_hard.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_agentbench_multidomain_tooluse_hard.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_browsecomp_sparse_research_hard.yaml`
- `evaluation/agent_eval/datasets/foundation_eval_cases_industry_benchmark_agentlongbench_long_context_memory_hard.yaml`

新增设计原则：
- 每集合 12 case，覆盖“发现 -> 读取 -> 执行 -> 审批 -> 交付”完整链路。
- 强化隐式提示词，不显式点名工具，逼近真实用户表达。
- 对包含文件交付的任务强制 `deliverable` 契约，支持 good/bad 抽样检查。

## 5. 系统化维度重建（E2E）
新端到端 suite：
- `evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml`

结构采用四层：
1. Foundation Core：工具覆盖、提示词有效性、主动性、安全与可用性恢复。  
2. Stateful & Personalization：记忆、用户习惯/Soul、多轮连续性。  
3. Delivery & Value：有价值工作流、复杂产物交付、复杂 coding/deep research。  
4. Frontier Transfer：SWE/TAU/WebArena/OSWorld/AgentBench/BrowseComp/AgentLongBench/RE/MLE 等高难迁移集合。  

## 6. 采用标准（纳入/淘汰）
纳入标准：
- 能映射到明确能力维度。
- 能形成可诊断冲突簇（`expected => top1`）。
- 能支持端到端交付契约检查（文件/证据/发布）。

淘汰标准：
- 连续多轮 pass@1 接近 100% 且不再提供新失败模式。
- 与已有集合高度重叠且不增加诊断价值。

## 7. 参考来源
- SWE-bench 官方：https://www.swebench.com/
- SWE-bench Verified 论文：https://arxiv.org/abs/2406.12045
- GAIA 官方：https://hal.cs.princeton.edu/gaia
- WebArena 论文：https://arxiv.org/abs/2307.13854
- WebArena 代码：https://github.com/web-arena-x/webarena
- OSWorld 官方：https://os-world.github.io/
- AgentBench 论文：https://arxiv.org/abs/2308.03688
- ToolSandbox 论文：https://arxiv.org/abs/2408.04682
- BrowseComp 介绍（OpenAI）：https://openai.com/index/introducing-deep-research/
- AgentLongBench 论文：https://arxiv.org/abs/2501.16503
- RE-Bench 论文：https://arxiv.org/abs/2411.15114
- MLE-bench 论文：https://arxiv.org/abs/2410.07095
