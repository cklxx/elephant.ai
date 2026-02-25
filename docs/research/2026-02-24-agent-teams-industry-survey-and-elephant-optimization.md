# 2026-02-24 Agent Teams 行业实现调研与 elephant.ai 优化方案

Date: 2026-02-24

## 调研范围（Primary Sources）

- OpenAI Agents SDK（handoffs / orchestrating multiple agents）
  - https://openai.github.io/openai-agents-python/handoffs/
  - https://openai.github.io/openai-agents-js/guides/multi-agent/
- LangChain / LangGraph 多 Agent
  - https://docs.langchain.com/oss/python/langchain/multi-agent
  - https://langchain-ai.github.io/langgraph/concepts/multi_agent/
- Microsoft AutoGen Teams
  - https://microsoft.github.io/autogen/stable/user-guide/agentchat-user-guide/tutorial/teams.html
- CrewAI（Crews + Flows）
  - https://docs.crewai.com/
- Google ADK 多 Agent
  - https://google.github.io/adk-docs/agents/multi-agents/
- AWS Bedrock Multi-Agent Collaboration
  - https://docs.aws.amazon.com/bedrock/latest/userguide/agents-multi-agent-collaboration.html
  - https://docs.aws.amazon.com/bedrock/latest/userguide/create-multi-agent-collaboration.html
- Semantic Kernel Multi-Agent Orchestration
  - https://learn.microsoft.com/en-us/semantic-kernel/frameworks/agent/agent-orchestration/
- Azure AI Foundry Connected Agents
  - https://learn.microsoft.com/en-us/azure/ai-foundry/agents/how-to/connected-agents
- Anthropic 多 Agent 工程经验
  - https://www.anthropic.com/engineering/building-effective-agents

## 业界主流实现分类

1. Supervisor + Handoff
- 代表：OpenAI Agents SDK、Google ADK。
- 特点：主 agent 负责拆解和路由，子 agent 通过 handoff 接管子任务。
- 优势：职责边界清晰，适合权限隔离和可控执行。
- 劣势：中心路由质量决定上限，复杂任务下中心容易成为瓶颈。

2. Stateful Graph / DAG
- 代表：LangGraph、AWS Bedrock multi-agent collaboration。
- 特点：显式节点、状态、依赖和阶段；强调可恢复、可观测、可审计。
- 优势：生产可控性强，失败恢复路径清楚。
- 劣势：建模成本高，灵活探索能力弱于自由对话团队。

3. Team Conversation Runtime
- 代表：AutoGen Teams、CrewAI、Semantic Kernel 多 Agent 编排。
- 特点：多个 agent 在共享上下文中轮次协作，使用 selector/termination 策略控制收敛。
- 优势：开放任务下探索性强，多视角评审能力好。
- 劣势：token 成本高，容易出现“循环对话”或收敛不稳定。

4. Managed Platform Composition
- 代表：Azure Connected Agents、AWS 托管能力。
- 特点：平台提供 agent 连接、工具治理、跨 agent 协作结构。
- 优势：集成快、平台治理能力强。
- 劣势：平台能力边界和配额限制明显，深度定制受限。

## 对 elephant.ai 的结论（利弊映射）

当前项目最匹配的路线仍是：**Supervisor + 阶段化 DAG + 明确文件记录**。

原因：
- 你们已经有 `team_dispatch` + `bg_dispatch/bg_status/bg_collect` + external bridge。
- 业务目标偏工程交付，不是纯研究型开放协作。
- 需要把“多 agent 可观测”和“可追责”作为一等公民。

不建议当前阶段引入“全对话型 swarm”作为主路径：
- 会显著放大上下文成本与终止不确定性。
- 与你们现有审批/可审计要求冲突更大。

## 本次优化落地方向（已对应代码改造）

1. File-based 团队运行记录（核心）
- 每次 `team_dispatch` 记录 team run JSON（run/stage/role/task/dependency/config/prompt preview）。
- 默认路径：`${session_dir}/_team_runs/*.json`。
- 记录失败不阻塞主流程，但会回传 `team_run_record_error` 元数据。

2. 多 Agent 流程控制统一在 Team DAG
- 仍保持“阶段串行、阶段内并行”。
- `team_dispatch` 输出 `team_run_id` 和 `team_run_record_path`，直接支持追踪和审计。

3. 扩展执行器能力：`codex` / `claude_code` / `kimi`
- `kimi` 已纳入 detect/config/registry/bridge/coding-defaults 全链路。
- coding task 的执行控制（sandbox/approval/retry/verify）与 codex 策略一致。

## 建议的下一步增强

1. Team Run Index
- 在 `_team_runs` 目录旁增加增量索引（按 session/team/run_id 反查），避免全目录扫描。

2. Stage Gate Policy
- 为每个 stage 增加可选 `success_criteria`（例如必须包含测试通过信号），不满足时自动阻断下一 stage。

3. Artifact-first Collect
- `bg_collect` 增加按 team run 聚合视图，优先输出“改动文件/命令/验证结果/风险”统一摘要。

4. Resume 与重放
- 在 team run record 中追加 resume token（stage cursor + pending tasks），支持故障后恢复而不是重跑全链路。

## 对 cklxx 关注点的直接回应

- “要用 files base 作为记录和流程控制”：已落地 team run file recorder + metadata 回传。
- “分配 codex/claude code/kimi cli 执行任务”：已落地 `kimi` external agent 全链路支持，并可在 `external_agents.teams.roles[].agent_type` 中混编三者。
