# 2026-02-23 Agent Teams 主流实现与实践对比

Date: 2026-02-23

## 目标

对比当前主流 agent teams 实现路线，并结合本仓库现状（已有 `team_dispatch` + `bg_*` 背景任务框架 + external bridge）给出可落地融合策略。

## 主流实现范式（业界）

1. **Supervisor / Router 中枢编排**
   - 代表：OpenAI Agents SDK handoffs、LangChain/LangGraph supervisor pattern、Google ADK multi-agent orchestration。
   - 核心：由一个主 agent 负责拆解与路由，子 agent 专注单一职责。

2. **Team / Group Chat 对话协作**
   - 代表：Microsoft AutoGen Teams（RoundRobin/Selector/Swarm）、CrewAI crews。
   - 核心：多个 agent 在共享上下文中轮次协作，由策略决定发言和终止。

3. **Workflow 图/DAG 编排**
   - 代表：AWS Bedrock multi-agent collaboration（supervisor + specialist + routing）、LangGraph stateful graph。
   - 核心：显式阶段和依赖图，强调可观测、可回放、可审计。

4. **轻量单体 + 专家工具化**
   - 代表：Anthropic “Building effective agents”强调先从简单起步，逐步增加多 agent。
   - 核心：仅在复杂度和收益足够时引入多 agent，避免过度编排。

## 辩证分析：优劣与适用边界

| 范式 | 优势 | 劣势 | 适用场景 |
| --- | --- | --- | --- |
| Supervisor 中枢编排 | 结构清晰、权限边界明确、调试路径短 | 中枢易成瓶颈/单点；路由质量决定上限 | 需要稳定可控的生产编排 |
| Team 对话协作 | 协作灵活，适合开放性问题和多视角评审 | token 成本高；终止条件难；易“空转” | 研究、方案对齐、评审型任务 |
| DAG 工作流 | 可预测性强，审计和 SLO 管控好 | 前期建模成本高，灵活性较弱 | 工程交付、多阶段流水线 |
| 轻量单体 + 专家工具 | 实现成本低、鲁棒性高、认知负担小 | 对极复杂任务拆解能力有限 | 日常任务、低复杂度自动化 |

## 与本项目的融合判断

本仓库当前能力最匹配 **Supervisor + 阶段化 DAG**：

- 已有 `team_dispatch`（团队定义 + staged 依赖）
- 已有 `bg_dispatch/bg_status/bg_collect`（后台任务生命周期）
- 已有 external bridge（Codex/Claude Code）与 `workspace_mode`（shared/branch/worktree）

因此推荐路线：

1. 保持 `team_dispatch` 作为唯一团队入口（避免新增并行编排语义）。
2. 团队结构采用“阶段串行 + 阶段内并行”的最小 DAG（当前实现已支持）。
3. 默认团队模板优先落地在 **file-based config (`runtime.external_agents.teams`)**，而不是 hardcode。
4. 对高风险执行角色（如 codex）在 role `config` 中显式设置：
   - `approval_policy=never`
   - `sandbox=danger-full-access`
   - `workspace_mode=worktree`
5. 通过 `bg_status/bg_collect` 做可观测闭环，避免 team 内部隐式状态机膨胀。

## 为什么不优先引入更复杂编排器

- 项目已有可用编排骨架，新增二级 orchestrator 会提升认知和维护成本。
- 结合 Anthropic 的建议，应先验证“最小有效多 agent”收益，再决定是否升级为更复杂的 swarm/selector。
- 现阶段交付目标是“可靠可控 + 可配置落地”，不是“最大化自治复杂度”。

## 本次落地结论

- 继续沿用现有 `team_dispatch` 模型。
- 修复并打通 `external_agents.teams` 配置加载链路。
- 以 YAML 配置实现团队定义，支持 file-based 管理和 CLI 全权限模式（role config 覆盖）。

## 参考资料（Primary Sources）

- OpenAI Agents SDK — Handoffs: https://openai.github.io/openai-agents-python/handoffs/
- OpenAI Cookbook — Orchestrating agents: https://cookbook.openai.com/examples/agents_sdk/parallel_agents
- Anthropic — Building effective agents: https://www.anthropic.com/engineering/building-effective-agents
- LangChain Docs — Multi-agent patterns: https://docs.langchain.com/oss/python/langchain/multi-agent
- Microsoft AutoGen Docs — AgentChat Teams: https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- Google ADK Docs — Multi agents: https://google.github.io/adk-docs/agents/multi-agents/
- AWS Bedrock Docs — Multi-agent collaboration: https://docs.aws.amazon.com/bedrock/latest/userguide/agents-multi-agent-collaboration.html
- CrewAI Docs: https://docs.crewai.com/

