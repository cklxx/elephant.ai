<p align="center">
    <img src="web/public/elephant-rounded.png" alt="elephant.ai mascot" width="76" height="76" />
</p>

# elephant.ai

[![CI](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml/badge.svg)](https://github.com/cklxx/Alex-Code/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cklxx/Alex-Code)](https://goreportcard.com/report/github.com/cklxx/Alex-Code)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**飞书原生的主动性个人 Agent。**

[English](README.md)

elephant.ai 作为一等成员常驻在你的飞书群聊和私信中——不是你需要主动召唤的机器人。它理解对话上下文、跨会话记忆、用内置技能主动发起工作、自主执行真实任务。CLI 和 Web 控制台随时可用，但飞书是主场。

---

## 为什么是飞书原生

大多数 AI 助手在工作流之外——另一个应用、另一个标签页、另一次上下文切换。elephant.ai 不一样：

| 能力 | 在飞书中如何工作 |
|---|---|
| **常驻在线** | 通过 WebSocket 常驻群聊和私信，无需特殊指令，自然对话即可。 |
| **理解上下文** | 自动获取近期聊天记录作为上下文，先理解对话再回复。 |
| **持续记忆** | 跨会话记住对话、决策和上下文，不用重复说明背景。 |
| **自主执行** | 完整的 Think → Act → Observe 循环。搜索、写代码、生成文档、浏览网页——一条飞书消息触发全部。 |
| **实时反馈** | 工作时实时展示工具执行进度和 emoji 反应。 |
| **内置技能** | 深度研究、会议纪要、邮件撰写、PPT 生成、视频制作——自然语言触发。 |
| **审批门控** | 知道什么时候该先征求同意。高风险操作在聊天中需要明确的人工审批。 |

---

## 北极星：日程 + 任务闭环（M0）

核心切片完全在飞书内闭环：**读取日程/待办 → 提出行动建议 → 审批后执行写入 → 到期主动提醒/跟进**。

已具备的基础能力：
- **日历工具：** 查询/创建/更新/删除日程（`lark_calendar_*`）
- **任务工具：** 列出/创建/更新/删除任务（`lark_task_manage`）
- **主动提醒：** scheduler 定时检查未来日程/任务并在飞书内提醒

状态以 `docs/roadmap/roadmap.md` 为准；配置细节见 `docs/reference/CONFIG.md`。

---

## 工作原理

```
你（飞书群聊 / 私信）
        ↓
   elephant.ai 运行时
        ↓
  ┌─────────────────────────────────┐
  │  上下文组装                      │
  │  (聊天记录 + 记忆 +             │
  │   策略 + 会话状态)              │
  ├─────────────────────────────────┤
  │  ReAct 代理循环                  │
  │  Think → Act → Observe          │
  ├─────────────────────────────────┤
  │  工具执行                        │
  │  (搜索、代码、浏览器、文件、     │
  │   产物、MCP 服务器)             │
  ├─────────────────────────────────┤
  │  可观测性                        │
  │  (追踪、指标、成本)             │
  └─────────────────────────────────┘
        ↓
  回复送回飞书
```

---

## 交互界面

- **飞书**（主场）— WebSocket 网关，自动保存消息到记忆，注入近期聊天记录作为上下文，实时工具进度，emoji 反应，群聊和私信支持，计划审查和审批流程。
- **Web 控制台** — Next.js 应用，SSE 流式传输、产物渲染、成本追踪、会话管理。适合回顾历史对话和复杂产出。
- **CLI / TUI 命令行** — 交互式终端，流式输出和工具审批提示。面向偏好命令行的开发者。

---

## 内置技能

技能是 markdown 驱动的工作流，在飞书中用自然语言描述需求即可触发：

| 技能 | 说明 |
|---|---|
| `deep-research` | 多步骤网络搜索与信息综合 |
| `meeting-notes` | 结构化会议纪要和待办事项 |
| `email-drafting` | 基于上下文的邮件撰写 |
| `ppt-deck` | 幻灯片生成 |
| `video-production` | 视频脚本和制作规划 |
| `research-briefing` | 研究简报生成 |
| `best-practice-search` | 工程最佳实践检索 |

---

## 模型供应商

支持多个供应商并自动选择最优可用模型：

- **OpenAI** — Chat API + Responses API (GPT-4o, o-series)
- **Anthropic** — Claude API (Claude 3.5/4 family, extended thinking)
- **ByteDance ARK** — 支持推理力度控制
- **DeepSeek** — 通过 OpenAI 兼容网关接入
- **OpenRouter** — 接入 100+ 模型
- **Ollama** — 本地模型，零云依赖
- **Antigravity** — OpenAI 兼容网关

设置 `llm_provider: auto`，运行时从 CLI 认证和环境变量自动解析最优模型。

---

## 快速开始

前置条件：Go 1.24+、Node.js 20+（Web UI）、Docker（可选）。

```bash
# 1. 配置模型供应商
export OPENAI_API_KEY="sk-..."
# 或: ANTHROPIC_API_KEY, CLAUDE_CODE_OAUTH_TOKEN, CODEX_API_KEY, ANTIGRAVITY_API_KEY
cp examples/config/runtime-config.yaml ~/.alex/config.yaml

# 2. 在 ~/.alex/config.yaml 中配置飞书机器人凭据
#    channels:
#      lark:
#        enabled: true
#        app_id: "cli_xxx"
#        app_secret: "xxx"
#        cards_enabled: true
#        card_callback_verification_token: "${LARK_VERIFICATION_TOKEN}"
#        card_callback_encrypt_key: "${LARK_ENCRYPT_KEY}"
#    回调地址: /api/lark/card/callback
#
# 可选：开启日程/任务主动提醒（scheduler）
#    runtime:
#      proactive:
#        scheduler:
#          enabled: true
#          calendar_reminder:
#            enabled: true
#            schedule: "*/15 * * * *"
#            look_ahead_minutes: 120
#            channel: "lark"
#            user_id: "ou_xxx"
#            chat_id: "oc_xxx"

# 3. 一起启动后端和前端
./dev.sh

# 4. 或者构建 CLI
make build
./alex
./alex "总结最近 3 条飞书对话并起草跟进邮件"
```

```bash
# 开发命令
./dev.sh status    # 检查服务状态
./dev.sh logs server
./dev.sh logs web
./dev.sh down      # 停止服务
```

配置参考：[`docs/reference/CONFIG.md`](docs/reference/CONFIG.md)

---

## 架构

```
交付层 (飞书, Web, CLI)
  → 代理应用层
  → 领域端口 (ReAct 循环, 事件, 审批)
  → 基础设施适配器 (LLM, 工具, 记忆, 存储, 可观测性)
```

| 层级 | 核心包 |
|---|---|
| 交付层 | `internal/channels/lark/`, `cmd/alex-server`, `web/`, `cmd/alex` |
| 代理核心 | `internal/agent/{app,domain,ports}` — ReAct 循环、类型化事件、审批门控 |
| 工具 | `internal/tools/builtin/` — 搜索、代码执行、浏览器、文件、产物、媒体 |
| 记忆 | `internal/memory/` — 持久化存储（Postgres、文件、内存）含分词 |
| 上下文 | `internal/context/`, `internal/rag/` — 分层检索与摘要 |
| 模型 | `internal/llm/` — 多供应商自动选择与流式传输 |
| MCP | `internal/mcp/` — JSON-RPC 工具服务器，用于外部集成 |
| 可观测性 | `internal/observability/` — OpenTelemetry 追踪、Prometheus 指标、成本核算 |
| 存储 | `internal/storage/`, `internal/session/` — 会话持久化与历史 |
| 依赖注入 | `internal/di/` — 所有界面共享的依赖注入 |

---

## 可用工具

- **网络搜索与浏览** — 搜索引擎和 ChromeDP 浏览器自动化
- **代码执行** — 多语言沙箱代码运行器
- **文件操作** — 读、写、管理文件
- **产物生成** — PDF、图片和结构化输出
- **媒体处理** — 图片、音频和视频处理
- **飞书集成** — 发送消息、获取聊天记录、管理对话、日程与任务
- **记忆管理** — 跨会话存储和召回信息
- **MCP 服务器** — 通过 Model Context Protocol 连接任意外部工具

---

## 质量与运维

```bash
# 代码检查与测试
./dev.sh lint
./dev.sh test
npm --prefix web run e2e

# 评估工具 (SWE-Bench, 回归测试)
# 见 evaluation/ 目录
```

可观测性：结构化日志、OpenTelemetry 追踪、Prometheus 指标、按会话成本核算——全部内置。

---

## 文档

| 文档 | 说明 |
|---|---|
| [`docs/README.md`](docs/README.md) | 文档首页 |
| [`docs/reference/ARCHITECTURE_AGENT_FLOW.md`](docs/reference/ARCHITECTURE_AGENT_FLOW.md) | 架构和执行流程 |
| [`docs/reference/CONFIG.md`](docs/reference/CONFIG.md) | 配置模式和优先级 |
| [`docs/guides/quickstart.md`](docs/guides/quickstart.md) | 从克隆到运行 |
| [`docs/operations/DEPLOYMENT.md`](docs/operations/DEPLOYMENT.md) | 部署指南 |
| [`AGENTS.md`](AGENTS.md) | 代理工作流与安全规则 |
| [`ROADMAP.md`](ROADMAP.md) | 路线图与贡献队列 |

---

## 贡献

参见 [`CONTRIBUTING.md`](CONTRIBUTING.md) 了解工作流和代码规范，[`CODE_OF_CONDUCT.md`](CODE_OF_CONDUCT.md) 了解社区准则，[`SECURITY.md`](SECURITY.md) 了解漏洞报告。

基于 [MIT](LICENSE) 许可证。
