# elephant.ai 开源传播策略

**目标：** 在开源社区建立技术影响力，获取 GitHub stars、contributors 和开发者认知。
**受众：** 后端/全栈开发者、AI/LLM 工具用户、Lark 企业用户、自托管爱好者、**工程经理和技术 Team Lead**。
**差异化定位：** 开源 Leader Agent——不是聊天机器人，而是主动拥有和管理任务的 AI 领导力代理，Lark-native 且可自托管。

> **定位升级说明（2026-03）：** 基于 [Leader Agent 品类分析](../../output/research/leader-agent-category-analysis.md) 的研究结论，将核心定位从 "AI 队友" 升级为 "Leader Agent"。研究显示该品类 VC 投资 H1 2025 达 $2.8B，Gartner 预测 2026 年底 40% 企业应用将内嵌任务型 AI Agent，市场规模预计 2030 年达 $52.6B。

---

## 核心定位：Leader Agent

### 为什么不叫 "AI 助手" 或 "AI 队友"

行业已从 chatbot → copilot → agent 演进（参见 Product Hunt 2025 趋势、Sam Altman "2025 is the year agents join the workforce"）。"助手" 暗示被动响应，"队友" 暗示平级协作。**Leader Agent** 传达的是：

1. **主动任务拥有权**：不等你问，主动识别、分解、执行任务
2. **协调成本压缩**：知识工作者 60% 时间花在 "关于工作的工作" 上（Asana），Leader Agent 接管这层协调开销
3. **注意力守护**：每次上下文切换需 25 分钟恢复（UC Irvine），Leader Agent 减少不必要的打断
4. **可信委托**：审批门控 + 可观测性，让人敢于把任务交出去

### 一句话定位

**英文：**
> Leader Agent for Lark — proactively owns tasks, compresses coordination overhead, guards your attention. Open source, self-hosted.

**中文：**
> 飞书原生 Leader Agent——主动接管任务、压缩协调成本、守护深度工作时间。开源，可自托管。

### 核心差异化三点（更新）

1. **Leader, not assistant**：主动拥有任务全生命周期，不是被动等指令
2. **Coordination compression**：持久记忆 + 跨会话上下文，消除重复沟通和信息搜寻
3. **Trusted autonomy**：审批门控 + 完整可观测性，让委托可控可追溯

---

## 目标受众（更新）

### 原有受众
- 后端/全栈开发者
- AI/LLM 工具用户
- Lark 企业用户
- 自托管爱好者

### 新增受众：工程经理和技术 Team Lead

**为什么：**
- 他们是协调成本的最大承受者——McKinsey 数据显示管理者 28% 时间在邮件、20% 在信息搜寻
- 他们有预算决策权，能推动团队采用
- "Leader Agent" 定位天然对 leadership 角色有吸引力——"AI Chief of Staff" 的隐喻（Xembly 已验证此定位有市场共鸣）

**触达渠道：**
- LinkedIn 文章（管理者密度高）
- Engineering Manager Slack 社区
- LeadDev / Engineering Leadership 相关会议
- 掘金/InfoQ 的技术管理类文章

---

## 现状评估

**优势**
- 技术深度真实：ReAct loop、多 Provider、MCP、审批门控，不是玩具项目
- Lark-native 是独特护城河：没有同类竞品
- 多模型支持（8 个 provider）覆盖中国和国际市场
- 完整可观测性栈，面向生产的设计
- **新增：** 定位精准契合 "Leader Agent" 新兴品类趋势

**劣势**
- GitHub 知名度几乎为零
- 无 Demo 视频，外部人很难快速理解产品价值
- 尚无社区（Discord/论坛）

---

## 第一阶段：基础建设（1–2 周）

### 1. GitHub 仓库优化

**About 区域（仓库描述）**
```
Leader Agent for Lark — proactively owns tasks, compresses coordination overhead, guards your attention. Persistent memory, autonomous ReAct execution, 8 LLM providers. Open source, self-hosted.
```

**Topics 标签**（在仓库 Settings → Topics 添加）
```
leader-agent  ai-agent  lark  feishu  llm  autonomous-agent  react-agent
ai-chief-of-staff  coordination  mcp  openai  golang  self-hosted  proactive-ai
```

**GitHub Social Preview**
- 使用 `assets/banner.png`（已生成）
- 路径：Settings → Social preview → Upload image

**Release 发布**
- 基于 `CHANGELOG.md` 发布第一个正式 Release（v0.3.0）
- 附上 pre-built binary（`make build` 产物）

---

### 2. 初始传播：中文社区（优先）

目标读者最密集，转化效率最高。

**V2EX（强烈推荐，Show and Tell 节点）**

标题示例：
> 开源了一个飞书原生 Leader Agent — elephant.ai，不是聊天机器人，是主动管理任务的 AI 领导力代理

正文结构：
1. 一句话核心定位（Leader Agent vs 聊天机器人的区别）
2. 核心能力截图/GIF（Lark 对话场景，展示主动任务接管）
3. 为什么不做成 "又一个 AI 助手"——协调成本问题
4. Quick Start 命令
5. GitHub 链接

**掘金（技术深度文章）**

文章题目：《从 AI 聊天机器人到 Leader Agent：我用 Go 构建了一个主动管理任务的飞书 AI 代理》
重点：Leader Agent 品类概念 + 技术实现，吸引工程师和技术管理者。

**SegmentFault / 知乎**

转发掘金文章，适当调整标题偏向产品侧。

---

### 3. 初始传播：英文社区

**Hacker News（Show HN）**

标题：
> Show HN: elephant.ai – Open-source Leader Agent for Lark (proactive task ownership, persistent memory, 8 LLM providers)

时机：周二/周三上午 9–10 点（US Eastern），竞争最低。
正文：3 段，控制在 200 字内。第一段说 why（coordination costs crushing knowledge work），第二段说 what（Leader Agent that owns tasks），第三段给链接。

**Reddit**
- `r/selfhosted`：强调自托管、无云依赖
- `r/LocalLLaMA`：强调 Ollama 本地模型支持
- `r/golang`：强调 Go 实现、架构设计
- `r/productivity`：**新增**——强调注意力守护、协调成本压缩

**Product Hunt（可选，适合积累初始 star）**

发布时机：门面文档完成后，选一个周二发布。
Tagline: "Leader Agent for Lark — owns your tasks so you can do deep work"
需要：hunter 推荐 + 提前联系支持者。

---

## 第二阶段：内容矩阵（1–2 个月）

持续输出技术内容，建立搜索引擎权重和开发者信任。

### 技术博客系列（推荐平台：Medium + 掘金双发）

**文章 1：为什么我们需要 Leader Agent，而不是又一个 AI 聊天机器人** *(新增)*
- 核心论点：chatbot → copilot → agent 的行业演进，以及为什么 "主动任务拥有" 是下一步
- 引用数据：60% 时间花在 "work about work"，每次上下文切换 25 分钟恢复成本
- 目标读者：工程经理、技术 Lead、AI 工具选型者

**文章 2：协调成本正在杀死你的团队——Leader Agent 如何压缩协调开销** *(新增)*
- 核心论点：基于 "AI as Coordination-Compressing Capital"（arXiv）框架，解释 Agent 如何消除协调摩擦
- 数据支撑：$450B/年上下文切换成本、1200 次/天应用切换、28% 时间在邮件
- 目标读者：关注团队效能的管理者、DevOps/SRE Lead

**文章 3：注意力是最稀缺的资源——AI Agent 如何守护深度工作时间** *(新增)*
- 核心论点：Cal Newport 的 Deep Work 框架 + Leader Agent 的注意力守护机制
- 展示 elephant.ai 的具体功能：主动任务接管、审批门控（减少打断）、持久记忆（消除重复沟通）
- 目标读者：个人效能关注者、知识工作者

**文章 4：为什么我把 AI Agent 做进飞书，而不是做成独立 App**
- 核心论点：AI Agent 在工作流内 vs 工作流外的本质差异
- 目标读者：工程效能团队、AI 工具选型者

**文章 5：Context Engineering > Prompt Engineering**
- 核心论点：提示词优化的天花板，以及 elephant.ai 的上下文组装策略
- 包含：代码片段、架构图

**文章 6：用 Go 实现多 Provider LLM 路由：设计与陷阱**
- 吸引 Go 开发者，讲 `internal/infra/llm/` 的设计决策
- 附带 OpenRouter / Ollama 接入实战

**文章 7：Approval Gates — 如何让 AI Agent 在做危险操作前停下来**
- 吸引关注 AI 安全的读者
- 讲审批门控的设计哲学和实现——这也是 "可信委托" 叙事的关键

**文章 8：ReAct Loop 生产化：持久化、超时、错误恢复**
- 深度技术，讲 `internal/domain/agent/` 的内核设计
- 目标读者：想自己实现 Agent 的工程师

### Demo 视频（2–3 分钟）

**场景脚本（Lark 场景）：**
```
00:00 – 00:15  开场：elephant.ai 不是聊天机器人，是 Leader Agent
00:15 – 00:45  场景1：@elephant.ai "帮我整理今天的会议要点并生成跟进任务"
               → 展示主动任务分解 + 实时 emoji 进度 + 结构化输出
00:45 – 01:15  场景2：无需再次说明背景，它记住了之前所有上下文
               → 展示跨会话记忆带来的协调成本压缩
01:15 – 01:45  场景3："帮我调研竞品 X 最近的融资动态，整理成表格"
               → 展示 web search + ReAct 多步自主执行
01:45 – 02:15  场景4：高风险操作触发审批门控
               → 展示可信委托（Trusted Autonomy）
02:15 – 02:35  Web Console 简要展示（成本追踪、会话历史）
02:35 – 03:00  "Stop managing tasks. Start leading." + Quick Start + GitHub 链接
```

录制工具推荐：Loom（免费，可直接分享链接）。

---

## 竞品对比与差异化定位

### 品类地图

基于 [Leader Agent 品类分析](../../output/research/leader-agent-category-analysis.md)，当前市场分为四层：

| 层级 | 代表产品 | 定位 | elephant.ai 的差异 |
|------|----------|------|-------------------|
| **通用 AI Agent 平台** | Lindy ($54M), Dust ($21.5M, Sequoia) | No-code agent builder, 企业内部 AI | 开源 + Lark-native + 可自托管 |
| **AI Chief of Staff** | Xembly ($15M), Motion ($550M) | 会议/日程/待办自动化 | 更深的技术栈（ReAct loop, MCP）+ 开发者友好 |
| **大厂 Agent** | ChatGPT Agent, Project Mariner, Claude Computer Use | 通用 web/desktop agent | Lark 嵌入（工作流内而非工作流外）+ 自托管 |
| **AI 编码工具** | Cursor, Claude Code, Devin ($10.2B) | IDE/CLI 编码助手 | 面向全工作流（不限于编程）|

### 详细竞品矩阵

| 维度 | elephant.ai | Xembly (AI Chief of Staff) | Lindy AI | Dust | Cursor / Claude Code |
|------|-------------|--------------------------|----------|------|---------------------|
| **定位** | 工作流内 Leader Agent | AI Chief of Staff | No-code AI Agent builder | Enterprise AI Agents | IDE/CLI 编码助手 |
| **嵌入位置** | 飞书群聊（Lark-native） | Slack/Teams/Calendar | 多渠道（web, email, calendar） | Slack/Notion/内部工具 | 编辑器/终端 |
| **主动性** | 主动任务识别与执行 | 主动会议跟进与日程管理 | 用户配置后自动执行 | 用户触发 | 被动响应指令 |
| **记忆** | 持久跨会话记忆 | 会议上下文 | 有限上下文 | 企业知识库连接 | 仅当前会话 |
| **安全机制** | 审批门控 + 完整可观测性 | 基础权限 | 基础权限 | 企业级权限 | 沙盒/权限系统 |
| **模型** | 8 个 Provider 自动选优 | 固定模型 | 多模型 | 多模型 | 单 Provider 锁定 |
| **自托管** | 完整自托管 | SaaS only | SaaS only | SaaS only | 云服务为主 |
| **开源** | 完全开源 | 闭源 | 闭源 | 开源（部分） | 闭源 |
| **技能系统** | 15+ 内置 Skill + MCP 扩展 | 内置功能 | 可视化 workflow | 内部 API 连接 | 内置工具 |
| **融资** | 开源项目 | $20M | $54M | $21.5M | Cursor: $400M+ |
| **适用场景** | 团队协作、日常工作流 | 会议跟进、日程管理 | 多场景自动化 | 企业知识问答与执行 | 软件开发 |

### 核心差异叙事（更新）

elephant.ai 不是又一个 AI 聊天机器人或编码工具。它是 **Leader Agent**——

1. **vs 聊天机器人（ChatGPT等）：** 聊天机器人等你问才回答。Leader Agent 主动识别、分解、拥有任务。（"Chatbots offer assistance; Agents offer labor."）
2. **vs AI Chief of Staff（Xembly等）：** Xembly 证明了定位有效，但它是闭源 SaaS。elephant.ai 开源、可自托管、技术栈更深（ReAct + MCP + 多 Provider）。
3. **vs 通用 Agent 平台（Lindy, Dust）：** 它们是水平平台（build any agent）。elephant.ai 是垂直产品（Lark-native Leader Agent），开箱即用。
4. **vs AI 编码工具（Cursor, Devin）：** 它们只解决编程场景。elephant.ai 面向全工作流——会议跟进、信息搜寻、任务协调、团队沟通。
5. **独特护城河：** 唯一一个 Lark-native + 开源 + 可自托管 + 生产级设计的 Leader Agent。

---

## 第三阶段：社区运营（持续）

### GitHub Issues 运营

**标签体系建立：**
```
good first issue    — 适合新贡献者，附详细说明
help wanted         — 需要社区协助
bug                 — 可复现问题
enhancement         — 功能请求
skill-request       — 新 Skill 请求
provider-request    — 新 LLM 供应商请求
documentation       — 文档改进
leader-agent        — Leader Agent 核心能力相关
```

**每月至少新增 3–5 个 `good first issue`**，降低贡献门槛。

### Release Notes

每次 Release 对应一篇简短的更新说明（500 字内）：
- 发在 GitHub Releases
- 同步到 V2EX / 掘金（每 1–2 个月发一次，不要过于频繁）

### Discord 社区（积累到 50+ stars 后建立）

频道建议：
- `#general` — 一般讨论
- `#help` — 使用问题
- `#showcase` — 用户展示
- `#dev` — 开发者讨论
- `#announcements` — 版本公告
- `#leader-agent-ideas` — Leader Agent 能力讨论与需求收集

### Conference Talk 投稿计划

| 会议 | 时间 | 话题方向 | 优先级 |
|------|------|----------|--------|
| GopherCon | 年度 | Multi-Provider LLM Architecture in Go | 高 |
| KubeCon AI Day | 年度 | Self-Hosted AI Agents: From Dev to Production | 中 |
| QCon | 半年度 | From Chatbots to Leader Agents: The Next Evolution | 高 |
| ArchSummit | 半年度 | 生产级 Leader Agent 的架构演进 | 中 |
| LeadDev | 半年度 | **新增** — How Leader Agents Compress Coordination Costs for Engineering Teams | 高 |

CFP 提交时机：通常提前 3-6 个月。优先投 Lightning Talk（10 分钟），降低首次入选门槛。

### Awesome Lists 收录

提交 PR 到以下 Awesome Lists（仓库有 50+ stars 后进行）：

- [awesome-ai-agents](https://github.com/e2b-dev/awesome-ai-agents) — 最大的 AI Agent 列表
- [awesome-lark](https://github.com/nicephil/awesome-lark) — 飞书生态
- [awesome-golang](https://github.com/avelino/awesome-go) — Go 项目列表
- [awesome-selfhosted](https://github.com/awesome-selfhosted/awesome-selfhosted) — 自托管项目
- [awesome-mcp](https://github.com/punkpeye/awesome-mcp-servers) — MCP 生态

每个 Awesome List PR 需符合其 contribution guidelines，通常要求：项目活跃、有文档、有 CI、有 License。

---

## GitHub 仓库设置操作清单

以下设置需手动在 GitHub Settings 中配置：

### About 区域
- **Description**: `Leader Agent for Lark — proactively owns tasks, compresses coordination overhead, guards your attention. Persistent memory, autonomous execution, 15+ skills, 8 LLM providers. Open source.`
- **Website**: GitHub Pages URL
- **Topics**: `leader-agent`, `ai-agent`, `lark`, `llm`, `autonomous-agent`, `ai-chief-of-staff`, `golang`, `react-loop`, `mcp`, `open-source`

### Social Preview
- Settings → Social preview → Upload `assets/banner.png`

### Discussions
- Settings → Features → 启用 Discussions
- 创建分类：
  - **Announcements** — 版本发布、重要公告（仅维护者可发帖）
  - **Q&A** — 使用问题（启用 "Mark as answer"）
  - **Ideas** — 功能建议和讨论
  - **Show and tell** — 社区展示

### Sponsorship
- Settings → Sponsor → 考虑启用 GitHub Sponsors
- 或创建 `.github/FUNDING.yml`：
  ```yaml
  github: cklxx
  ```

---

## KPI 参考

| 指标 | 第1个月目标 | 第3个月目标 |
|------|------------|------------|
| GitHub Stars | 100 | 500 |
| GitHub Forks | 10 | 50 |
| Contributors | 2 | 10 |
| HN 点赞 | >50 | — |
| 掘金文章阅读 | >3000 | >10000 |
| LinkedIn 文章阅读 | >1000 | >5000 |

---

## 执行顺序

```
Week 1  GitHub 仓库 About/Topics/Social Preview → 发布 v0.3.0 Release
Week 1  V2EX Show and Tell 帖子（Leader Agent 定位）
Week 2  博客文章 #1（为什么需要 Leader Agent）
Week 2  Hacker News Show HN
Week 3  博客文章 #2（协调成本压缩）
Week 3  LinkedIn 文章（面向工程经理）
Week 4  录制 Demo 视频 + 发到 YouTube/B站
Month 2 博客文章 #3-5（注意力守护 + 技术深度系列）
Month 2 Reddit 系列发布（含 r/productivity）
Month 3 Product Hunt 发布（视 stars 积累情况）
Month 3 博客文章 #6-8（技术深度系列）
持续    月度 Release Notes + Good First Issues 更新
```

---

## 附：文案素材库

**一句话定位（英文）**
> Leader Agent for Lark — proactively owns tasks, compresses coordination overhead, guards your attention. Open source, self-hosted.

**一句话定位（中文）**
> 飞书原生 Leader Agent——主动接管任务、压缩协调成本、守护深度工作时间。开源，可自托管。

**核心差异化三点**
1. Leader, not assistant（主动拥有任务，不是被动等指令）
2. Coordination compression（持久记忆消除重复沟通，跨会话上下文消除信息搜寻）
3. Trusted autonomy（审批门控 + 可观测性，让委托可控可追溯）

**对工程经理的价值主张**
> 你的团队 60% 的时间在 "关于工作的工作" 上。elephant.ai 接管协调层——会议跟进、信息汇总、任务分发——让团队回到深度工作。

**SEO 目标搜索词（更新）**
- "leader agent open source"
- "ai chief of staff lark"
- "lark ai agent open source"
- "feishu ai assistant self-hosted"
- "coordination cost ai agent"
- "attention management ai"
- "golang ai agent react loop"
- "proactive ai agent"
- "open source leader agent"
- "ai agent vs chatbot"

**数据弹药库（引用自 Leader Agent 品类分析）**
- 知识工作者 60% 时间花在 "work about work"（Asana）
- 每次上下文切换需 25 分 26 秒恢复（UC Irvine）
- 每天 1200 次应用切换（Harvard Business Review）
- 上下文切换每年成本 $450B（美国）
- AI Agent 市场 $7.8B（2025）→ $52.6B（2030），CAGR 46.3%
- Gartner：2026 年底 40% 企业应用将内嵌任务型 AI Agent
