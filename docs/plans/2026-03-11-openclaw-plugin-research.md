# OpenClaw 插件机制研究与 elephant.ai 借鉴方案

Date: 2026-03-11

---

## 1. OpenClaw 概述

OpenClaw（前身 Clawdbot/Moltbot）是开源自主 AI Agent，GitHub 247k+ stars。运行在用户自有设备上，通过 WhatsApp / Telegram / Slack / Feishu 等消息平台交互。架构上采用 **plugin-first** 设计。

### 1.1 核心架构

| 组件 | 职责 |
|------|------|
| **Gateway** | WebSocket 控制面，连接消息平台，分发消息到 Agent Runtime |
| **Agent Runtime** | AI 循环：context 组装 → LLM 调用 → tool 执行 → 流式回复 → 状态持久化 |
| **Skills** | 模块化能力扩展，以 `SKILL.md`（YAML frontmatter + markdown）形式定义 |
| **Memory** | 持久化层：JSONL transcript + Markdown memory |

Agent Loop：`receive → route → context + LLM + tools → stream → persist`

### 1.2 插件机制

**安装方式：**
```bash
openclaw plugins install <npm-spec | local-path | tarball>
```
解压到 `~/.openclaw/extensions/<id>/`，通过 `package.json` 的 `openclaw.extensions` 字段声明入口。

**四级优先级：**

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | config | 显式路径指定 |
| 2 | workspace | 项目级 |
| 3 | user-global | 用户全局 |
| 4（最低） | bundled | 内置 |

**插件能力：**
- 注册自定义 slash command（不经过 LLM 直接执行）
- Context engine 插件，接管 session context 编排
- 通过 `openclaw/plugin-sdk` 访问核心 API
- 支持 `.ts` 源码发布（jiti 实时转译）

**安全模型：** 插件为 trusted code，依赖安装禁用 lifecycle scripts。

### 1.3 Skills-as-Markdown

Skill 是一个文件夹，包含 `SKILL.md`：

```yaml
name: skill-name
description: ...
triggers:
  intent_patterns: [...]
  tool_signals: [...]
capabilities: [...]
requires_tools: [...]
chain:
  - skill: another-skill
```

关键设计：**选择性注入**——每轮对话只注入相关 skill，避免 prompt 膨胀。

### 1.4 生态

- **ClawHub**：Skill registry，支持自动发现和安装
- **A2A 协议**：Agent-to-Agent 跨 server 通信（v0.3.0）
- **模型提供者外部化**：LLM provider 作为可插拔包动态加载
- 第三方插件：SecureClaw（安全审计）、MemOS（长期记忆）、IronClaw（Rust 重写）等

---

## 2. elephant.ai 当前可扩展性

### 2.1 现有扩展点

| 扩展点 | 机制 | 位置 | 成熟度 |
|--------|------|------|--------|
| Tool 注册 | `ToolRegistry.Register(ToolExecutor)` + 装饰器链（SLA → ID → Retry → Approval → Validation → Executor） | `internal/app/toolregistry/` | 生产级 |
| 外部 Agent | Bridge + Registry 自动检测（claude_code / codex / kimi / generic_cli） | `internal/infra/external/registry.go` | 生产级 |
| Skills | 文件发现 + YAML metadata + chaining + approval gates | `internal/infra/skills/` | 生产级 |
| Channel 插件 | `PluginFactory` 闭包模式（Lark / Web / CLI） | `internal/delivery/channels/plugin.go` | 生产级 |
| LLM Provider | Model registry + client factory | `internal/infra/llm/` | 生产级 |
| Hooks | 事件驱动主动行为（memory capture / OKR） | `internal/app/di/` | Beta |

### 2.2 与 OpenClaw 的差距

| 维度 | OpenClaw | elephant.ai | 差距 |
|------|----------|-------------|------|
| 外部插件加载 | npm 包 / tarball 热加载 | 编译期注册，无外部加载器 | **大** |
| Skill 生态 | ClawHub registry + 安装 CLI | 文件系统扫描 + env var | **中** |
| Skill 选择性注入 | 每轮按相关性注入 top-K | 有 trigger 匹配但粒度较粗 | **中** |
| 优先级模型 | 统一 4 级（config > workspace > user > bundled） | skill 有类似逻辑但未统一到 tool/config | **小** |
| 多 Agent 互操作 | A2A 协议跨 server | subagent + external bridge（单机内） | **中** |
| 模型提供者 | 外部包动态加载 | 内置 client factory | **小** |

---

## 3. 借鉴方案

### 3.1 Skill 选择性注入优化（P0 — 投入小、收益大）

**现状：** Skill 匹配基于 `intent_patterns` + `tool_signals`，匹配后全量注入候选 skill 内容。

**改进：**
- 引入相关性评分（intent match score + 上下文 embedding 相似度）
- 每轮只注入 top-K（K=3~5）最相关 skill
- 对未命中 skill 只保留 name + one-line description 作为 fallback 列表
- 若 LLM 主动请求某 skill，下轮再注入完整内容

**收益：** 降低 prompt token 消耗，提升 LLM 响应准确度和速度。

**影响范围：** `internal/infra/skills/` 的匹配逻辑 + `internal/app/context/` 的注入逻辑。

### 3.2 Skill CLI 与版本化（P1 — 生态基础）

**目标：** 让 skill 成为可分享、可版本化的一等公民。

**设计：**
```bash
alex skills list                     # 列出已安装 skill
alex skills install <git-url>        # 从 Git repo 安装
alex skills install <local-path>     # 从本地目录安装
alex skills uninstall <name>         # 卸载
alex skills update [name]            # 更新
```

**Skill 包结构：**
```
my-skill/
├── SKILL.md           # 元数据 + 提示词
├── scripts/           # 可选的脚本
└── skill.lock.yaml    # 版本锁定（自动生成）
```

**安装位置：** `~/.alex/skills/<name>/`，遵循现有优先级链（`ALEX_SKILLS_DIR` > user dir > repo）。

**收益：** 团队间 skill 共享，减少重复编写，为未来 skill registry 打基础。

### 3.3 统一四级优先级模型（P1 — 架构一致性）

**目标：** 将 tool、skill、config 的解析统一为一致的优先级链。

| 优先级 | 来源 | 说明 |
|--------|------|------|
| 1（最高） | runtime config | YAML 显式配置 |
| 2 | workspace | 项目 `.alex/` 目录 |
| 3 | user-global | `~/.alex/` 目录 |
| 4（最低） | bundled | 代码库内置 |

**实现：** 抽象 `PriorityResolver[T]` 泛型接口，skill / tool / config 各自实现。

### 3.4 外部 Tool 子进程协议（P2 — 开放性）

**现状：** External agent bridge 已支持子进程调用（claude_code / codex / generic_cli），但仅限 agent 粒度。

**改进：** 泛化为通用 tool plugin 协议：

```yaml
# ~/.alex/tools/my-tool/tool.yaml
name: my-custom-tool
description: "..."
command: "python3 run.py"
args_schema:
  type: object
  properties:
    query: { type: string }
```

**执行流程：**
1. 启动时扫描 `~/.alex/tools/` 目录
2. 解析 `tool.yaml`，生成 `ToolDefinition`
3. 执行时：stdin 传入 JSON args → 子进程执行 → stdout 返回 JSON result
4. 自动接入现有装饰器链（SLA / Retry / Approval）

**安全边界：** 子进程隔离，不共享内存；timeout + resource limit 由现有 SLA 机制管控。

**收益：** 任意语言编写 tool 扩展，无需修改 Go 代码库。

### 3.5 A2A 跨进程协作（P3 — 中长期）

**目标：** 让多个 elephant.ai 实例（或异构 agent）能协作。

**方向：**
- 基于现有 event 体系定义轻量 RPC 协议
- 复用 `runtime/hooks/bus.go` 的事件类型，增加跨进程传输层
- 与现有 team / subagent 体系对齐，leader agent 可调度远程 worker

**前置条件：** 3.4 的外部 tool 协议稳定后再推进。

---

## 4. 不建议照搬的设计

| OpenClaw 设计 | 不适用原因 |
|--------------|-----------|
| 进程内插件（无沙箱） | elephant.ai 原则 safety > correctness，进程内插件风险过高 |
| npm 生态依赖 | Go 项目，引入 npm 生态增加不必要的复杂度 |
| TypeScript 插件 SDK | 语言栈不匹配 |

---

## 5. 建议路线图

```
Phase 1（近期）
  ├── 3.1 Skill 选择性注入优化
  └── 3.3 统一优先级模型

Phase 2（中期）
  ├── 3.2 Skill CLI 与版本化
  └── 3.4 外部 Tool 子进程协议

Phase 3（远期）
  └── 3.5 A2A 跨进程协作
```

每个 Phase 完成后评估收益再决定下一步优先级。
