# Context Learning Benchmark 难题映射（2026-02-08）

## 目标
- 用最新 long-context/context-learning benchmark 的“高难失败模式”指导本地离线评测设计。
- 把 benchmark 难题模式映射为可执行的 foundation case 集合，形成持续回归基线。

## 参考基准（最新可用）

1. **LongBench v2 (ACL 2025)**
   - 关注现实长上下文任务，强调模型在“感知-推理”链路上的短板。
   - 映射：多文档聚合、长对话历史、代码仓理解、结构化长文处理。

2. **NoLiMa (2025)**
   - 关注 lexical mismatch（关键词不重叠）的潜在匹配检索难题。
   - 映射：memory + search 的语义检索链路，不依赖字面 token 命中。

3. **Sequential-NIAH (2025)**
   - 关注顺序依赖与链式 needle 检索，而不只是单 needle 命中。
   - 映射：多步顺序证据提取（find/read/shell/grep）与时序一致性。

4. **RULER (2024)**
   - 长上下文综合任务（检索、聚合、多跳等）上的可扩展评估。
   - 映射：多跳 symbol tracing、跨段聚合与一致性核验。

5. **BABILong (2024)**
   - bAbI 风格任务扩展到长上下文，考察事实链和干扰鲁棒性。
   - 映射：fact-chain、counting、long noise 中的关键信息保持。

6. **InfiniteBench (2024)**
   - 超长（100k+）上下文的真实/合成混合任务评估。
   - 映射：分块策略规划、跨源融合、长流程审计痕迹。

## 已落地映射

- 新集合：`evaluation/agent_eval/datasets/foundation_eval_cases_context_learning_hard.yaml`
- 新增规模：20 cases
- 已纳入 suite：`evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- 运行结果：`context-learning-hard` `20/20`

示例映射：
- NoLiMa -> `context-hard-literal-mismatch-retrieval`
- Sequential-NIAH -> `context-hard-sequential-needles`
- LongBench v2 -> `context-hard-longbenchv2-*`
- RULER -> `context-hard-multi-hop-tracing`
- BABILong -> `context-hard-babilong-fact-chain`
- InfiniteBench -> `context-hard-infinitebench-*`

## 设计原则
- 让 case 明确暴露“难点模式”，而不是堆砌普通工具调用。
- 保持 expected tools 为“可解释的第一步路由集合”。
- 用 `x/x` 规模和分集合结果持续跟踪回归。

## Sources
- LongBench v2 (ACL Anthology): https://aclanthology.org/2025.acl-long.1341/
- NoLiMa (arXiv): https://arxiv.org/abs/2503.19815
- Sequential-NIAH (arXiv): https://arxiv.org/abs/2507.15807
- RULER (arXiv): https://arxiv.org/abs/2404.06654
- BABILong (arXiv): https://arxiv.org/abs/2406.10149
- InfiniteBench (arXiv): https://arxiv.org/abs/2402.13718
