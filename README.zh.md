<p align="center">
  <img src="assets/banner.png" alt="elephant.ai banner" width="100%" />
</p>

<h1 align="center">elephant.ai</h1>

<p align="center">
  <strong>替你盯进度、抓重点、推动事情往前走的 Leader Agent。</strong><br/>
  常驻飞书，持续盯住在推进中的工作，自动跟进、汇总、预警，只在需要你判断时才来打扰你。
</p>

<p align="center">
  <a href="https://github.com/cklxx/elephant.ai/actions/workflows/ci.yml"><img src="https://github.com/cklxx/elephant.ai/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
  <a href="https://goreportcard.com/report/github.com/cklxx/elephant.ai"><img src="https://goreportcard.com/badge/github.com/cklxx/elephant.ai" alt="Go Report Card"/></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"/></a>
  <a href="README.md"><img src="https://img.shields.io/badge/Docs-English-blue.svg" alt="English Docs"/></a>
</p>

---

## elephant.ai 是什么？

elephant.ai 是常驻飞书的 **leader agent**。它不只是回答问题——它接手工作：持续跟踪进展，主动催状态、汇总结果，帮你筛出真正需要关注的事，替你扛住「盯、问、催、对齐」的协调开销。

底层它可以调度其他 agent 和工具，但对你来说，它就是那个靠谱、持续、不会掉球的人。CLI 和 Web 控制台随时可用，但飞书是主场。

---

## ✨ 为什么需要 Leader Agent？

| | |
|---|---|
| 📌 **持续接手** | 交出去的事不会掉。不再问「这事到哪了？」——leader agent 一直盯着，直到做完。 |
| 🔇 **注意力守门** | 把信息压缩成：现在该看什么、哪个风险在变大、哪件事需要你拍板。 |
| 🔄 **主动推进** | 自动催状态、汇总结果、卡住就升级。你再也不用问「进展呢？」 |
| 🤝 **代表你协调** | 很多工作本质上不是「做」，是「盯、问、催、对齐」。leader agent 替你扛住这部分。 |
| 🧠 **持续记忆** | 跨周、跨月记住对话、决策和上下文。上下文随时间积累，越用越懂你。 |
| 🛡️ **审批门控** | 知道什么时候该问你。敏感操作需要明确的人工确认。 |
| 🔌 **MCP 扩展** | 通过 Model Context Protocol 接入任意外部工具，无限集成。 |
| 🏠 **飞书原生** | WebSocket 网关——常驻群聊和私信，无需 `/slash` 指令，自然对话即可。 |

---

## 🚀 快速开始

**前置条件：** Go 1.24+、Node.js 20+、飞书机器人 token、LLM API Key。

```bash
# 1. 克隆并构建
git clone https://github.com/cklxx/elephant.ai.git && cd elephant.ai
make build

# 2. 配置（LLM Key + 飞书凭据）
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
export LLM_API_KEY="sk-..."
alex setup   # 交互式初始化向导

# 3. 启动全部服务
alex dev up

# 4. 在飞书中直接对话——或使用 CLI
./alex "总结最近 3 条对话并起草跟进邮件"
```

完整配置指南 → [`docs/guides/quickstart.md`](docs/guides/quickstart.md)

---

## 工作原理

```
你（飞书群聊或私信）
        ↓
  上下文组装          — 聊天记录 + 记忆 + 策略
        ↓
  ReAct 代理循环      — Think → Act → Observe
        ↓
  工具执行            — 搜索 · 代码 · 浏览器 · 文件 · MCP
        ↓
  回复送回飞书        — 附带实时进度和 emoji 反应
```

---

## 交互界面

| 界面 | 说明 |
|---|---|
| **飞书**（主场） | WebSocket 网关。常驻群聊/私信。实时工具进度、emoji 反应、审批流程。 |
| **Web 控制台** | Next.js 控制台，SSE 流式传输、产物渲染、成本追踪、会话历史。 |
| **CLI / TUI** | 交互式终端，流式输出。面向开发者和本地工作流。 |

---

## 内置技能

技能是由自然语言触发的 markdown 驱动工作流：

| 技能 | 功能 |
|---|---|
| `deep-research` | 多步骤网络研究与信息综合 |
| `meeting-notes` | 结构化摘要与待办事项提取 |
| `email-drafting` | 基于上下文的邮件撰写 |
| `ppt-deck` | 幻灯片生成 |
| `video-production` | 视频脚本和制作规划 |
| `research-briefing` | 简洁研究简报 |
| `best-practice-search` | 工程最佳实践检索 |

---

## 模型供应商

```
OpenAI · Anthropic (Claude) · DeepSeek · 豆包 (ARK)
OpenRouter · Ollama（本地） · Kimi · 通义千问
```

设置 `llm_provider: auto`——运行时从环境变量自动选择最优可用模型。

---

## 架构

```
交付层      飞书 · Web 控制台 · CLI · API Server
     ↓
应用层      协调 · 上下文组装 · 成本控制
     ↓
领域层      ReAct 循环 · 类型化事件 · 审批门控
     ↓
基础设施    多模型 · 记忆存储 · 工具注册表 · 可观测性
```

---

## 📖 文档

| | |
|---|---|
| [快速开始](docs/guides/quickstart.md) | 从克隆到运行 |
| [配置参考](docs/reference/CONFIG.md) | 完整配置模式和优先级规则 |
| [架构](docs/reference/ARCHITECTURE.md) | 运行时分层和执行模型 |
| [部署指南](docs/operations/DEPLOYMENT.md) | 生产部署 |
| [路线图](ROADMAP.md) | 后续规划 |

---

## 🤝 参与贡献

欢迎贡献。参见 [`CONTRIBUTING.md`](CONTRIBUTING.md) 了解开发环境、代码规范和 PR 流程。首次贡献？查找标记为 [`good first issue`](https://github.com/cklxx/elephant.ai/issues?q=label%3A%22good+first+issue%22) 的 issue。

请在参与前阅读 [`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md)，安全漏洞请通过 [`SECURITY.md`](SECURITY.md) 报告。

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=cklxx/elephant.ai&type=Date)](https://star-history.com/#cklxx/elephant.ai&Date)

---

## 许可证

[MIT](LICENSE) © 2025 cklxx
