# ALEX 配置参考
> Last updated: 2025-12-14

本文档是 **ALEX 核心运行时配置（LLM / presets / runtime 行为）** 的唯一说明。无论是 `alex` CLI 还是 `alex-server`，都通过同一套配置加载逻辑（`internal/config`）构建运行时配置。

> 说明：部分子系统（例如 MCP servers 的 `.mcp.json`、observability 示例 YAML 等）有独立的配置文件；本文聚焦于 `~/.alex-config.json` + managed overrides 这条“核心运行时配置链路”。

---

## 目标与原则（只有一个 config）

- **唯一 schema**：`internal/config.RuntimeConfig`（运行时配置快照）。
- **唯一加载入口**：`internal/config.Load`（defaults → file → env → overrides）。
- **唯一“可持久化覆盖层”**：`internal/config/admin`（managed overrides；CLI `alex config set/clear` 与 server 共用）。
- 工程侧通过测试 `internal/config/env_usage_guard_test.go` 限制新增 `os.Getenv` 的散落使用，避免出现“第二套配置系统”。

---

## 配置来源与优先级（从低到高）

1. **Defaults**：内置默认值（用于开箱即用/本地开发兜底）。
2. **Main config file**：`~/.alex-config.json`（或 `ALEX_CONFIG_PATH` 指定的路径）。
3. **Environment variables**：例如 `LLM_PROVIDER` / `LLM_MODEL` 等。
4. **Managed overrides**：`alex config set` 写入的 overrides 文件（位置见 `alex config path`）。

> 重要坑位：**managed overrides 会覆盖环境变量**。如果你在容器里设置了 env 但效果不生效，优先检查 overrides 文件是否有同字段。

---

## 文件：主配置 `~/.alex-config.json`

### 路径解析

- 默认：`$HOME/.alex-config.json`
- 可覆盖：`ALEX_CONFIG_PATH=/path/to/alex-config.json`

### 推荐最小示例

见 `examples/config/core-config-example.json`（与 quickstart 同步）。

```json
{
  "llm_provider": "openrouter",
  "llm_model": "deepseek/deepseek-chat",
  "llm_vision_model": "openai/gpt-4o-mini",
  "api_key": "your-openrouter-key-here",
  "base_url": "https://openrouter.ai/api/v1",
  "tool_preset": "safe"
}
```

---

## 文件：Managed Overrides（可选）

Managed overrides 是“**可持久化的最后一层覆盖**”，用于快速切模型/切 base_url/临时调参，不需要改主配置文件。

### 路径解析

- 执行 `alex config path` 查看实际路径
- 默认：`$HOME/.alex/runtime-overrides.json`
- 可覆盖：`CONFIG_ADMIN_STORE_PATH=/path/to/runtime-overrides.json`

### 示例

见 `examples/config/runtime-overrides-example.json`：

```json
{
  "llm_model": "deepseek/deepseek-chat",
  "llm_vision_model": "openai/gpt-4o-mini"
}
```

### CLI 操作

```bash
alex config
alex config set llm_model gpt-4o-mini
alex config set llm_vision_model gpt-4o-mini
alex config clear llm_vision_model
alex config path
```

---

## 环境变量

说明：为减少歧义与维护成本，runtime loader **只识别一套 canonical 环境变量名**（不再支持历史别名）。

- `OPENAI_API_KEY`：OpenAI-compatible API key
- `LLM_PROVIDER`：`openrouter` / `openai` / `deepseek` / `ollama` / `mock`
- `LLM_MODEL`：默认模型
- `LLM_VISION_MODEL`：有图片附件时使用的 vision 模型（可选）
- `LLM_BASE_URL`：OpenAI-compatible base URL（如 `https://api.openai.com/v1`、`https://openrouter.ai/api/v1`、Ollama 地址等）
- `TAVILY_API_KEY`：`web_search` 工具
- `ARK_API_KEY`：Seedream/Ark 工具

### 网络与代理（非 RuntimeConfig 字段）

ALEX 的出站 HTTP 请求默认遵循 Go 标准代理环境变量：`HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`。

本地开发时经常出现“代理地址指向 `127.0.0.1:xxxx` 但代理进程未启动”的情况。默认模式下 ALEX 会 **自动绕过不可达的 loopback 代理**，避免所有出站请求都因为 `proxyconnect ... connection refused` 失败（日志会给出 warning）。

- `ALEX_PROXY_MODE`：`auto`（默认） / `strict` / `direct`
  - `auto`：遵循标准代理 env；若 loopback 代理不可达则自动绕过；并始终对 `localhost/127.0.0.1/::1` 目标直连。
  - `strict`：严格遵循代理 env；代理不可用会直接失败。
  - `direct`：忽略代理 env，全部直连。

---

## 字段参考（JSON keys）

> 说明：主配置与 managed overrides 使用同一套字段名（snake_case），只识别这一套 schema。

### LLM 相关

- `llm_provider`：provider 选择；默认 `openrouter`（当 `api_key` 为空时会自动降级为 `mock` 供本地跑通）。
- `llm_model`：默认模型。
- `llm_vision_model`：vision 模型；当检测到图片附件时优先使用（见下节）。
- `api_key`：API key（生产建议用 env 注入，不要提交到 git）。
- `base_url`：OpenAI-compatible base URL。
- `max_tokens`：请求 `max_tokens`。
- `temperature`：采样温度；显式写入 `0` 会被保留。
- `top_p`：Top-P 采样。
- `stop_sequences`：stop 序列列表。

### 工具与运行体验

- `tool_preset`：工具权限预设（仅 CLI）：`safe` / `read-only` / `full`。Web 模式下忽略该字段并默认启用全部非本地工具。
- 运行时工具模式由入口决定：`alex` 为 CLI 模式、`alex-server` 为 Web 模式（非本地工具全开、禁用本地文件/命令）。
- `agent_preset`：agent 预设（按项目内 presets 定义）。
- `verbose`：verbose 模式（CLI/Server 的输出更详细）。
- `session_dir`：会话存储目录（支持 `~` 与 `$ENV` 展开）。
- `cost_dir`：cost 存储目录（支持 `~` 与 `$ENV` 展开）。

### 外部工具 keys

- `tavily_api_key`：Tavily web search key。
- `ark_api_key`：Ark key。

### Seedream（Ark）模型/端点

- `seedream_text_endpoint_id`
- `seedream_image_endpoint_id`
- `seedream_text_model`
- `seedream_image_model`
- `seedream_vision_model`
- `seedream_video_model`

---

## 多模态（Vision）配置与行为

- 当 **用户消息携带图片附件**（或 task 文本引用图片占位符）时：
  - 若配置了 `llm_vision_model`，会在执行准备阶段把本次请求模型切到 `llm_vision_model`；
  - 否则继续使用 `llm_model`（可能导致 provider/model 不支持图片而失败）。
- 这层切换发生在 agent 的准备阶段；agent 只需要“表达附件/模态”，provider 的差异由 LLM 基础层抹平。

---

## 最佳实践与常见坑位（业界经验）

- **分离 secrets 与非 secrets**：生产环境建议用 env（K8s Secret / Docker secret）注入 `OPENAI_API_KEY`、`TAVILY_API_KEY`、`ARK_API_KEY`；主配置文件存放非敏感参数（model/base_url/preset）。
- **明确优先级**：遇到“配置没生效”，按顺序排查：
  1) `alex config` 看当前快照；2) `alex config path` 看 overrides；3) 环境变量；4) 主配置文件。
- **谨慎使用 managed overrides**：它会覆盖 env；在容器/多环境切换时，常见的坑是忘记清掉 overrides。
- **修改主配置文件需要重启 `alex-server`**：server 启动时会构建 DI container；主配置文件 `~/.alex-config.json` 的改动不会自动热更新（managed overrides 可通过 UI/CLI 更新）。
- **Vision 模型必须真支持图片**：很多文本模型不支持 image；建议明确配置 `llm_vision_model`，并用 provider 对应的 vision model 名称。
- **OpenAI-compatible base_url 通常需要带 `/v1`**：例如 OpenAI `https://api.openai.com/v1`、OpenRouter `https://openrouter.ai/api/v1`；少了 `/v1` 常见报错是 404/路径不匹配。
- **控制图片体积**：base64 会显著膨胀 payload，且不同 provider 有请求大小上限；优先使用可访问的远程 URL 或在入库/上传阶段做压缩/缩放。
- **Ollama 仅接受 inline base64 图片**：如果你给 attachment 只填了远程 `uri`，需要确保同时提供 `data`（或 data URI）才能走 `messages[].images`。
- **避免把大体积 data URI 打进日志**：图片常以 `data:image/...;base64,...` 出现；项目已在 LLM request log 里做脱敏，但仍建议避免在业务日志中打印原始附件。
- **工具调用安全**：只开启需要的 `tool_preset`（CLI）；并避免让模型“发明未声明工具”。项目已在基础层对 tool-call 解析做了 declared-tools 过滤，但 preset 仍是第一道闸。
