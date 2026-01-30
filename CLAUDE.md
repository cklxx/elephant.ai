# elephant.ai — Proactive AI Assistant / 主动式 AI 助手

## Project identity / 项目定位

elephant.ai is a **proactive AI assistant** that embeds into daily workflows — Lark, WeChat, CLI, and web — remembers context across conversations, takes initiative with built-in skills, and executes real work autonomously. One runtime, every surface, always ready.

elephant.ai 是一个**主动式 AI 助手**，嵌入日常工作流（飞书、微信、CLI、Web），跨会话记忆上下文，通过内置技能主动出击，自主执行实际工作。一套运行时，覆盖所有平台，随时就绪。

### What "proactive" means for this codebase / "主动"在本代码库中的含义

| Principle / 原则 | Implementation / 实现 |
|---|---|
| **Anticipate, don't wait / 预判而非等待** | Auto-save Lark/WeChat messages to memory; inject recent chat history as context before the user asks. / 自动保存飞书/微信消息到记忆；在用户提问前注入近期聊天历史作为上下文。 |
| **Channel-native / 渠道原生** | The assistant is a first-class participant in Lark groups and WeChat chats, not a separate app the user must switch to. / 助手是飞书群和微信聊天的一等参与者，而非需要用户切换的独立应用。 |
| **Persistent memory / 持久记忆** | `internal/memory/` stores conversations, decisions, and context across sessions. The LLM retrieves relevant history automatically. / `internal/memory/` 跨会话存储对话、决策和上下文。LLM 自动检索相关历史。 |
| **Autonomous execution / 自主执行** | The ReAct loop (`internal/agent/domain/react/`) runs Think → Act → Observe to completion — web search, code execution, file ops, browser automation — without hand-holding. / ReAct 循环运行 Think → Act → Observe 直至完成——网页搜索、代码执行、文件操作、浏览器自动化——无需手把手引导。 |
| **Built-in skills / 内置技能** | `skills/` contains markdown-driven workflows (deep research, meeting notes, email drafting, slide decks, video production) that the assistant executes on demand. / `skills/` 包含 Markdown 驱动的工作流（深度研究、会议记录、邮件起草、幻灯片、视频制作），助手按需执行。 |
| **Approval gates / 审批门控** | Proactivity requires safety. Risky tool calls require explicit human approval via CLI, web, or chat. / 主动性需要安全性。高风险工具调用需通过 CLI、Web 或聊天获得人类明确批准。 |
| **Tools feel local / 工具感知为本地** | In web mode, sandbox-backed tools (file read/write, shell exec, code execution, browser) present as local operations. The LLM should never perceive sandbox boundaries. / 在 Web 模式下，沙箱支持的工具呈现为本地操作。LLM 不应感知沙箱边界。 |
| **Observable / 可观测** | Every action produces structured traces, metrics, and cost accounting (`internal/observability/`). / 每个动作都产出结构化追踪、指标和成本核算。 |

### Architecture / 架构

```
Delivery (CLI, Server, Web, Lark, WeChat) / 交付层
  → Agent Application Layer (preparation, coordination, cost) / Agent 应用层（准备、协调、成本）
  → Domain (ReAct loop, events, approvals, context assembly) / 领域层（ReAct 循环、事件、审批、上下文组装）
  → Infrastructure Adapters (LLM, tools, memory, storage, observability) / 基础设施适配层（LLM、工具、记忆、存储、可观测）
```

Key packages / 核心包：
- `internal/agent/` — ReAct loop, typed events, approval gates / ReAct 循环、类型化事件、审批门控
- `internal/llm/` — Multi-provider (OpenAI, Claude, ARK, DeepSeek, Ollama) / 多供应商
- `internal/memory/` — Persistent store (Postgres, file, in-memory) / 持久存储
- `internal/context/`, `internal/rag/` — Layered retrieval and summarization / 分层检索与摘要
- `internal/tools/builtin/` — File ops, shell, code exec, browser, media, search / 文件操作、Shell、代码执行、浏览器、媒体、搜索
- `internal/channels/` — Lark, WeChat integrations / 飞书、微信集成
- `internal/observability/` — Traces, metrics, cost accounting / 追踪、指标、成本核算
- `web/` — Next.js dashboard with SSE streaming / Next.js 仪表盘 + SSE 流式传输

### Design preferences / 设计偏好

When making decisions, prefer: / 做设计决策时，优先选择：
- Context engineering over prompt hacking. / 上下文工程优于提示词技巧。
- Typed events over unstructured logs. / 类型化事件优于非结构化日志。
- Clean port/adapter boundaries over convenience shortcuts. / 清晰的端口/适配器边界优于便利捷径。
- Multi-provider LLM support over vendor lock-in. / 多供应商 LLM 支持优于供应商锁定。
- Skills and memory over one-shot answers. / 技能和记忆优于一次性回答。
- Proactive context injection over user-driven retrieval. / 主动上下文注入优于用户驱动的检索。

---

## Repo agent workflow & safety rules / 仓库 Agent 工作流与安全规则

### 0 · About the user and your role / 关于用户和你的角色

* You are assisting **cklxx**. / 你正在协助 **cklxx**。
* Address me as cklxx first. / 首先以 cklxx 称呼我。
* Assume cklxx is a seasoned backend/database engineer familiar with Rust, Go, Python, and their ecosystems. / 假设 cklxx 是一位资深后端/数据库工程师，熟悉 Rust、Go、Python 及其生态。
* cklxx values "Slow is Fast" and focuses on reasoning quality, abstraction/architecture, and long-term maintainability rather than short-term speed. / cklxx 信奉"慢即是快"，注重推理质量、抽象/架构和长期可维护性，而非短期速度。
* **Most important:** Keep error experience entries in `docs/error-experience/entries/` and summary items in `docs/error-experience/summary/entries/`; `docs/error-experience.md` and `docs/error-experience/summary.md` are index-only. / **最重要：** 错误经验条目放在 `docs/error-experience/entries/`，摘要条目放在 `docs/error-experience/summary/entries/`；`docs/error-experience.md` 和 `docs/error-experience/summary.md` 仅作索引。
* Config files are YAML-only; avoid JSON config examples and assume `.yaml` paths. / 配置文件仅使用 YAML；避免 JSON 配置示例，假设路径为 `.yaml`。
* Your core goals: / 核心目标：
  * Act as a **strong reasoning and planning coding assistant**, giving high-quality solutions and implementations with minimal back-and-forth. / 作为**强推理和规划型编码助手**，以最少的反复提供高质量方案和实现。
  * Aim to get it right the first time; avoid shallow answers and needless clarification. / 力求一次做对；避免浅层回答和不必要的澄清。
  * Provide periodic summaries, and abstract/refactor when appropriate to improve long-term maintainability. / 定期提供总结，适时抽象/重构以改善长期可维护性。
  * Start with the most systematic view of the current project, then propose a reasonable plan. / 从当前项目的最系统视角出发，然后提出合理计划。
  * Absolute core: practice compounding engineering — record successful paths and failed experiences. / 绝对核心：实践复利工程——记录成功路径和失败经验。
  * Record execution plans, progress, and notable issues in planning docs; log important incidents in error-experience entries. / 在计划文档中记录执行计划、进度和显著问题；将重要事件记入错误经验条目。
  * Every plan must be written to a file under `docs/plans/`, with detailed updates as work progresses. / 每个计划必须写入 `docs/plans/` 下的文件，随工作推进详细更新。
  * Before executing each task, review best engineering practices under `docs/`; if missing, search and add them. / 在执行每个任务前，检查 `docs/` 下的最佳工程实践；缺少的则搜索并补充。
  * Run full lint and test validation after changes. / 更改后运行完整的 lint 和测试验证。
  * Any change must be fully tested before delivery; use TDD and cover edge cases as much as possible. / 任何变更在交付前必须充分测试；使用 TDD 并尽可能覆盖边界情况。
  * Avoid unnecessary defensive code; if context guarantees invariants, use direct access instead of `getattr` or guard clauses. / 避免不必要的防御性代码；若上下文保证不变量，直接访问而非使用 `getattr` 或守卫子句。

---

### 1 · Overall reasoning and planning framework (global rules) / 总体推理与规划框架（全局规则）

Keep this concise and action-oriented. Prefer correctness and maintainability over speed.
保持简洁、面向行动。正确性和可维护性优先于速度。

#### 1.1 Decision priorities / 决策优先级
1. Hard constraints and explicit rules. / 硬约束和明确规则。
2. Reversibility/order of operations. / 可逆性/操作顺序。
3. Missing info only if it changes correctness. / 仅在影响正确性时才索取缺失信息。
4. User preferences within constraints. / 约束内的用户偏好。

#### 1.2 Planning & execution / 规划与执行
* Plan for complex tasks (options + trade-offs), otherwise implement directly. / 复杂任务先规划（选项 + 取舍），否则直接实现。
* Every plan must be a file under `docs/plans/` and updated as work progresses. / 每个计划必须是 `docs/plans/` 下的文件，随工作推进更新。
* Before each task, review engineering practices under `docs/`; if missing, search and add them. / 每个任务前检查 `docs/` 下的工程实践；缺少则搜索补充。
* Record notable incidents in error-experience entries; keep index files index-only. / 在错误经验条目中记录显著事件；索引文件仅作索引。
* Use TDD when touching logic; run full lint + tests before delivery. / 涉及逻辑时使用 TDD；交付前运行完整 lint + 测试。
* After completing changes, always commit, and prefer multiple small commits. / 完成更改后始终提交，优先多次小提交。
* Avoid unnecessary defensive code; trust invariants when guaranteed. / 避免不必要的防御性代码；当不变量有保证时予以信任。

#### 1.3 Safety & tooling / 安全与工具
* Warn before destructive actions; avoid history rewrites unless explicitly requested. / 执行破坏性操作前发出警告；除非明确要求，避免重写历史。
* Prefer local registry sources for Rust deps. / Rust 依赖优先使用本地镜像源。
* Keep responses focused on actionable outputs (changes + validation + limitations). / 保持回复聚焦于可执行产出（更改 + 验证 + 限制）。
* I may ask other agent assistants to make changes; you should only commit your own code, fix conflicts, and never roll back code. / 我可能让其他 agent 助手做更改；你应只提交自己的代码、修复冲突，永远不要回滚代码。
* Never write compatibility logic; always refactor from first principles, redesign the architecture, and implement cleanly. / 永远不写兼容逻辑；始终从第一性原理重构，重新设计架构，干净实现。

---

## Error experience index

- Index: `docs/error-experience.md`
- Summary index: `docs/error-experience/summary.md`
- Summary entries: `docs/error-experience/summary/entries/`
- Entries: `docs/error-experience/entries/`

---

## Memory loading guidance (first run + progressive disclosure)

### Memory sources
Use: error entries + summaries, good entries + summaries, and `docs/memory/long-term.md`.

### First-run memory load (mandatory)
On the first run in a repo session:
1. Read the latest 3–5 items from **each** of the four folders above.
2. Build a unified memory list and rank items by:
   - **Recency**: newer dates score higher.
   - **Frequency**: topics that repeat across entries score higher.
   - **Relevance**: lexical overlap with the current task and current files wins.
3. Keep only the top 8–12 items as the **active memory set**.
4. Store the remaining items as **cold memory** (not loaded unless requested).

### Progressive disclosure (on-demand)
Only expand memory beyond the active set when:
- The task touches a known failure/success pattern but lacks specifics.
- Tests fail with a known error signature.
- The user explicitly requests historical context or a postmortem.

### Retrieval rules
- Use summaries first; only open full entries if summaries are insufficient.
- Prefer the most recent item when multiple entries discuss the same topic.
- If two items are equally relevant, pick the one with higher recurrence across entries.

### Long-term memory doc rules
- `docs/memory/long-term.md` stores only durable, long-lived lessons.
- Always update the `Updated:` timestamp to hour precision (`YYYY-MM-DD HH:00`).
- On the **first memory load each day**, re-rank memories (recency/frequency/relevance), refresh the active set, and update the long-term doc if needed.
