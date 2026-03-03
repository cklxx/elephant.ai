# elephant.ai 开源传播策略

**目标：** 在开源社区建立技术影响力，获取 GitHub stars、contributors 和开发者认知。
**受众：** 后端/全栈开发者、AI/LLM 工具用户、Lark 企业用户、自托管爱好者。
**差异化定位：** 唯一真正 Lark-native 的开源 AI Agent，而非又一个 CLI 工具或聊天机器人。

---

## 现状评估

**优势**
- 技术深度真实：ReAct loop、多 Provider、MCP、审批门控，不是玩具项目
- Lark-native 是独特护城河：没有同类竞品
- 多模型支持（8 个 provider）覆盖中国和国际市场
- 完整可观测性栈，面向生产的设计

**劣势**
- GitHub 知名度几乎为零
- 无 Demo 视频，外部人很难快速理解产品价值
- 尚无社区（Discord/论坛）

---

## 第一阶段：基础建设（1–2 周）

### 1. GitHub 仓库优化

**About 区域（仓库描述）**
```
Proactive AI assistant for Lark — persistent memory, autonomous ReAct execution, 8 LLM providers. Open source, self-hosted.
```

**Topics 标签**（在仓库 Settings → Topics 添加）
```
ai-agent  lark  feishu  llm  autonomous-agent  react-agent
personal-assistant  mcp  openai  golang  self-hosted  proactive-ai
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
> 开源了一个飞书原生 AI Agent — elephant.ai，支持持久记忆、ReAct 自主执行和 8 个模型供应商

正文结构：
1. 一句话是什么（30 字内）
2. 核心能力截图/GIF（Lark 对话场景）
3. 与同类工具的差异（Lark-native vs 外部 app）
4. Quick Start 命令
5. GitHub 链接

**掘金（技术深度文章）**

文章题目：《我用 Go 写了一个常驻飞书的 AI Agent，支持持久记忆和自主执行》
重点：技术实现路径，吸引 Go 开发者，在文末引导到 GitHub。

**SegmentFault / 知乎**

转发掘金文章，适当调整标题偏向产品侧。

---

### 3. 初始传播：英文社区

**Hacker News（Show HN）**

标题：
> Show HN: elephant.ai – Open-source proactive AI agent that lives in Lark (persistent memory, ReAct loop, 8 LLM providers)

时机：周二/周三上午 9–10 点（US Eastern），竞争最低。
正文：3 段，控制在 200 字内。第一段说 why，第二段说 what，第三段给链接。

**Reddit**
- `r/selfhosted`：强调自托管、无云依赖
- `r/LocalLLaMA`：强调 Ollama 本地模型支持
- `r/golang`：强调 Go 实现、架构设计

**Product Hunt（可选，适合积累初始 star）**

发布时机：门面文档完成后，选一个周二发布。
需要：hunter 推荐 + 提前联系支持者。

---

## 第二阶段：内容矩阵（1–2 个月）

持续输出技术内容，建立搜索引擎权重和开发者信任。

### 技术博客系列（推荐平台：Medium + 掘金双发）

**文章 1：为什么我把 AI Agent 做进飞书，而不是做成独立 App**
- 核心论点：AI Agent 在工作流内 vs 工作流外的本质差异
- 目标读者：工程效能团队、AI 工具选型者

**文章 2：Context Engineering > Prompt Engineering**
- 核心论点：提示词优化的天花板，以及 elephant.ai 的上下文组装策略
- 包含：代码片段、架构图

**文章 3：用 Go 实现多 Provider LLM 路由：设计与陷阱**
- 吸引 Go 开发者，讲 `internal/infra/llm/` 的设计决策
- 附带 OpenRouter / Ollama 接入实战

**文章 4：Approval Gates — 如何让 AI Agent 在做危险操作前停下来**
- 吸引关注 AI 安全的读者
- 讲审批门控的设计哲学和实现

**文章 5：ReAct Loop 生产化：持久化、超时、错误恢复**
- 深度技术，讲 `internal/domain/agent/` 的内核设计
- 目标读者：想自己实现 Agent 的工程师

### Demo 视频（2–3 分钟）

**场景脚本（Lark 场景）：**
```
00:00 – 00:20  开场：elephant.ai 在飞书群聊中已存在
00:20 – 00:50  场景1：@elephant.ai "帮我整理今天的会议要点"
               → 展示实时 emoji 进度 + 最终结构化输出
00:50 – 01:20  场景2："基于刚才的会议，帮我起草一封跟进邮件"
               → 展示跨会话记忆（它记住了刚才的会议内容）
01:20 – 01:50  场景3："帮我查一下竞品 X 最近的融资动态"
               → 展示 web search + ReAct 多步执行
01:50 – 02:20  场景4：高风险操作触发审批门控
               → 展示安全机制
02:20 – 02:40  Web Console 简要展示（成本追踪、会话历史）
02:40 – 03:00  Quick Start 命令 + GitHub 链接
```

录制工具推荐：Loom（免费，可直接分享链接）。

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

---

## KPI 参考

| 指标 | 第1个月目标 | 第3个月目标 |
|------|------------|------------|
| GitHub Stars | 100 | 500 |
| GitHub Forks | 10 | 50 |
| Contributors | 2 | 10 |
| HN 点赞 | >50 | — |
| 掘金文章阅读 | >3000 | >10000 |

---

## 执行顺序

```
Week 1  GitHub 仓库 About/Topics/Social Preview → 发布 v0.3.0 Release
Week 1  V2EX Show and Tell 帖子
Week 2  掘金文章 #1（飞书原生定位）
Week 2  Hacker News Show HN
Week 3  掘金文章 #2（Context Engineering）
Week 4  录制 Demo 视频 + 发到 YouTube/B站
Month 2 博客文章 #3–5（每两周一篇）
Month 2 Reddit 系列发布
Month 3 Product Hunt 发布（视 stars 积累情况）
持续    月度 Release Notes + Good First Issues 更新
```

---

## 附：文案素材库

**一句话定位（英文）**
> Proactive AI agent that lives in Lark — remembers everything, executes autonomously, approves before acting.

**一句话定位（中文）**
> 常驻飞书的主动式 AI Agent——持久记忆、自主执行、高风险操作审批门控。

**核心差异化三点**
1. Lark-native（不是外挂工具，是群聊成员）
2. 持久记忆（跨会话，不是每次从零开始）
3. 生产级设计（审批门控、可观测性、多 Provider）

**目标搜索词**
- "lark ai agent open source"
- "feishu ai assistant self-hosted"
- "golang ai agent react loop"
- "proactive ai assistant"
- "open source personal ai agent"
