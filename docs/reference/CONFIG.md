# ALEX 配置参考
> Last updated: 2026-01-31

本文档是 **ALEX 主配置文件（`~/.alex/config.yaml`）** 的说明，覆盖 runtime 以及 server/auth/session/analytics/attachments 等段。`alex` CLI 与 `alex-server` 共享 runtime 配置；`alex-server` 额外读取其他段完成服务侧配置。

> 说明：主配置文件与 managed overrides 统一放在 `~/.alex/config.yaml`。MCP servers 仍使用各自的 `.mcp.json`（多 scope）。

---

## 目标与原则（只有一个 config）

- **唯一主配置文件**：`~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH`）。
- **运行时 schema**：`internal/config.RuntimeConfig`（runtime 快照）。
- **加载入口**：
  - runtime：`internal/config.Load`（defaults → file(runtime) → overrides）。
  - server 侧：`internal/config.LoadFileConfig`（读取 server/auth/session/analytics/attachments 等段）。
- **唯一“可持久化覆盖层”**：`internal/config/admin`（managed overrides；CLI `alex config set/clear` 与 server 共用，写入同一 YAML）。
- 工程侧通过测试 `internal/config/env_usage_guard_test.go` 限制新增 `os.Getenv` 的散落使用，避免出现“第二套配置系统”。

---

## 配置来源与优先级（从低到高）

1. **Defaults**：内置默认值（用于开箱即用/本地开发兜底）。
2. **Main config file**：`~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH` 指定的路径）。
3. **Managed overrides**：`alex config set` 写入 `config.yaml` 的 `overrides` 段（位置见 `alex config path`）。

---

## 文件：主配置 `~/.alex/config.yaml`

### 路径解析

- 默认：`$HOME/.alex/config.yaml`
- 可覆盖：`ALEX_CONFIG_PATH=/path/to/config.yaml`

### 推荐最小示例（远程 provider）

```yaml
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o-mini"
  base_url: "https://api.openai.com/v1"
```
完整示例见 `examples/config/runtime-config.yaml`。

---

## 段：Managed Overrides（可选）

Managed overrides 是“**可持久化的最后一层覆盖**”，用于快速切模型/切 base_url/临时调参，不需要改主配置文件。

### 示例

```yaml
overrides:
  llm_model: "deepseek/deepseek-chat"
  llm_vision_model: "openai/gpt-4o-mini"
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

## 其他配置段（apps / server / auth / session / analytics / attachments）

这些段由不同组件读取：`apps` 由内置 `apps` 工具读取（CLI/Server 共用）；其余段由
`alex-server` 在启动时读取，用于 Web/服务端配置；CLI 侧忽略。

### apps

用于扩展内置 app 插件列表（`apps` 工具会合并内置与自定义条目；`id` 相同则自定义覆盖）。

字段：

- `plugins`：自定义 app 插件列表
  - `id`：唯一标识（小写推荐）
  - `name`：展示名称
  - `description`：简短描述
  - `capabilities`：能力点列表
  - `integration_note`：接入/权限说明
  - `sources`：开源实现来源（GitHub 链接）

示例：

```yaml
apps:
  plugins:
    - id: "internal-chat"
      name: "Internal Chat"
      description: "Internal chat connector."
      capabilities:
        - "send"
        - "receive"
      integration_note: "Requires internal auth."
      sources:
        - "https://github.com/example/internal-chat"
```

### server

- `port`：HTTP 端口（默认 `8080`）。
- `enable_mcp`：是否启用 MCP 探针（默认 `true`）。
- `max_task_body_bytes`：`/api/tasks` POST 请求体上限（字节，默认 20 MiB）。
- `allowed_origins`：CORS 允许来源列表。
- `stream_max_duration_seconds`：流式请求最大持续时间（秒，默认 2h）。
- `stream_max_bytes`：单条流式连接最大输出字节数（默认 64 MiB）。
- `stream_max_concurrent`：同时允许的流式连接数（默认 128）。
- `rate_limit_requests_per_minute`：HTTP 请求速率限制（每分钟，默认 600）。
- `rate_limit_burst`：速率限制突发配额（默认 120）。
- `non_stream_timeout_seconds`：非流式请求超时（秒，默认 30）。
- `event_history_retention_days`：事件历史保留天数（默认 30；设置为 0 关闭自动清理）。
- `event_history_max_sessions`：内存事件历史保留的最大会话数（默认 100；设置为 0 表示不限制）。
- `event_history_session_ttl_seconds`：内存事件历史空闲 TTL（秒，默认 3600；设置为 0 表示不启用）。
- `event_history_max_events`：单个会话内存事件历史最大条数（默认 1000；设置为 0 表示不限制）。

### auth

- `jwt_secret`
- `access_token_ttl_minutes`
- `refresh_token_ttl_days`
- `state_ttl_minutes`
- `redirect_base_url`
- `database_url`
- `bootstrap_email` / `bootstrap_password` / `bootstrap_display_name`
- `google_client_id` / `google_client_secret` / `google_auth_url` / `google_token_url` / `google_userinfo_url`

### session

- `database_url`
- `dir`
- `pool_max_conns`
- `pool_min_conns`
- `pool_max_conn_lifetime_seconds`
- `pool_max_conn_idle_seconds`
- `pool_health_check_seconds`
- `pool_connect_timeout_seconds`
- `cache_size`：Session 读取缓存大小（默认 256；设置为 0 关闭缓存）。

### analytics

- `posthog_api_key`
- `posthog_host`

### attachments

- `provider`：`local` / `cloudflare`
- `dir`：本地存储目录（provider=local 时必填）
- `cloudflare_account_id` / `cloudflare_access_key_id` / `cloudflare_secret_access_key`
- `cloudflare_bucket` / `cloudflare_public_base_url` / `cloudflare_key_prefix`
- `presign_ttl`：预签名 TTL（未配置 `cloudflare_public_base_url` 时建议 `4h`，例如 `4h`）

### web

- `api_url`：仅供部署脚本读取（用于 `NEXT_PUBLIC_API_URL`）。

### channels

#### channels.lark

- `enabled`：是否启用 Lark 网关（默认 false）。
- `app_id` / `app_secret`：Lark 应用凭证。
- `base_domain`：Lark API 域名（默认 `https://open.larkoffice.com`）。
- `session_prefix`：会话 ID 前缀（默认 `lark`），用于派生稳定的 chat session ID；Lark 不注入 session history，多轮聊天依赖 `auto_chat_context` 与 Markdown 记忆加载。
- `reply_prefix`：回复前缀。
- `allow_groups` / `allow_direct`：是否响应群聊/私聊。
- `agent_preset` / `tool_preset` / `tool_mode`：通道级 preset/mode（Lark 默认 `tool_preset: lark-local`）。
- `workspace_dir`：Lark 本地工具工作区根目录（默认进程 working dir）。
- `cards_enabled`：是否启用 Lark 交互卡片（默认 true）。
- `cards_plan_review` / `cards_results` / `cards_errors`：分别控制计划确认/结果/错误卡片发送。结果卡片在存在附件时会升级为“附件卡片”（仅一条卡片，按钮点击后发送文件/图片）；若卡片构建失败则回退为文本回复 + 原有附件发送。
- `card_callback_verification_token` / `card_callback_encrypt_key`：卡片交互回调配置（用于 `/api/lark/card/callback` 回调校验/解密）。
- `auto_upload_files`：本地文件写入/替换后自动上传附件（默认 true）。
- `auto_upload_max_bytes`：自动上传单文件大小上限（默认 2MB）。
- `auto_upload_allow_ext`：自动上传允许扩展名白名单（默认常见文档/图片）。
- `browser`：本地浏览器配置（`cdp_url` / `chrome_path` / `headless` / `user_data_dir` / `timeout_seconds`）。
- `reply_timeout_seconds`：单条消息执行超时（秒）。
- `react_emoji`：随机表情池（逗号/空格分隔）。同一次请求会在开始/结束分别随机挑选不同表情；少于 2 个时会回退默认池。
- `memory_enabled`：启用 Markdown 记忆加载（MEMORY.md + daily logs）。
- `show_tool_progress`：是否在 Lark 显示工具执行进度。
- `auto_chat_context` / `auto_chat_context_size`：自动拉取近期聊天上下文。
- `plan_review_enabled`：启用 plan review（计划确认/反馈注入）。
- `plan_review_require_confirmation`：回复中是否提示 “OK/修改意见”。
- `plan_review_pending_ttl_minutes`：plan review pending 记录 TTL（分钟）。

示例（YAML）：

```yaml
channels:
  lark:
    enabled: true
    app_id: "cli_test"
    app_secret: "secret"
    session_prefix: "lark"
    allow_groups: true
    allow_direct: true
    agent_preset: "default"
    tool_preset: "lark-local"
    tool_mode: "cli"
    workspace_dir: "/Users/bytedance/code/elephant.ai"
    cards_enabled: true
    cards_plan_review: true
    cards_results: true
    cards_errors: true
    card_callback_verification_token: "${LARK_VERIFICATION_TOKEN}"
    card_callback_encrypt_key: "${LARK_ENCRYPT_KEY}"
    auto_upload_files: true
    auto_upload_max_bytes: 2097152
    auto_upload_allow_ext: [".txt", ".md", ".json", ".yaml", ".yml", ".csv", ".log", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".pdf", ".docx", ".xlsx", ".pptx"]
    browser:
      cdp_url: ""
      chrome_path: ""
      headless: true
      user_data_dir: "~/.alex/chrome"
      timeout_seconds: 60
    reply_timeout_seconds: 180
    react_emoji: "WAVE, Get, THINKING, MUSCLE, THUMBSUP, OK, THANKS, APPLAUSE, LGTM"
    memory_enabled: true
    show_tool_progress: true
    auto_chat_context: true
    auto_chat_context_size: 20
    plan_review_enabled: true
    plan_review_require_confirmation: true
    plan_review_pending_ttl_minutes: 60
```

### observability

- 仍由 `internal/observability` 读取 `observability` 段（日志/metrics/tracing）。

---

## 环境变量（用于路径与插值）

说明：runtime loader **不再把环境变量作为覆盖层**。环境变量只用于：

- **定位配置文件**：`ALEX_CONFIG_PATH=/path/to/config.yaml`
- **在 YAML 中插值**：使用 `${ENV}`（例如 `runtime.api_key: ${OPENAI_API_KEY}`）

推荐使用 env 承载 secrets，然后在 `config.yaml` 里引用（示例）：

- `OPENAI_API_KEY`：OpenAI-compatible API key
- `ANTHROPIC_API_KEY`：Claude (Anthropic) API key
- `CLAUDE_CODE_OAUTH_TOKEN`：Claude Code OAuth token
- `ANTHROPIC_AUTH_TOKEN`：Claude OAuth token (备用)
- `CODEX_API_KEY`：OpenAI Responses / Codex API key
- `ANTIGRAVITY_API_KEY`：Antigravity API key
- `OPENAI_BASE_URL`：OpenAI base URL override
- `ANTHROPIC_BASE_URL`：Anthropic base URL override
- `CODEX_BASE_URL`：Responses / Codex base URL override
- `ANTIGRAVITY_BASE_URL`：Antigravity base URL override
- `ALEX_CLI_AUTH_PATH`：CLI auth.json 路径覆盖
- `ALEX_SKILLS_DIR`：Skills 根目录（包含各技能目录/文件的 `skills/` 路径）
- `TAVILY_API_KEY`：`web_search` 工具
- `ARK_API_KEY`：Seedream/Ark 工具
- `AUTH_JWT_SECRET` / `AUTH_DATABASE_URL`
- `ALEX_SESSION_DATABASE_URL`
- `GOOGLE_CLIENT_SECRET`
- `CLOUDFLARE_ACCOUNT_ID` / `CLOUDFLARE_ACCESS_KEY_ID` / `CLOUDFLARE_SECRET_ACCESS_KEY`

> 插值规则：`${VAR}` 会被替换为环境变量值；如需字面量 `$`，可写成 `$$`。

### 网络与代理（非 RuntimeConfig 字段）

ALEX 的出站 HTTP 请求默认遵循 Go 标准代理环境变量：`HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`。

本地开发时经常出现“代理地址指向 `127.0.0.1:xxxx` 但代理进程未启动”的情况。默认模式下 ALEX 会 **自动绕过不可达的 loopback 代理**，避免所有出站请求都因为 `proxyconnect ... connection refused` 失败（日志会给出 warning）。

- `ALEX_PROXY_MODE`：`auto`（默认） / `strict` / `direct`
  - `auto`：遵循标准代理 env；若 loopback 代理不可达则自动绕过；并始终对 `localhost/127.0.0.1/::1` 目标直连。
  - `strict`：严格遵循代理 env；代理不可用会直接失败。
  - `direct`：忽略代理 env，全部直连。

---

## 字段参考（runtime/overrides keys）

> 说明：`runtime` 与 `overrides` 使用同一套字段名（snake_case），只识别这一套 schema。

### LLM 相关

- `llm_provider`：provider 选择；默认 `openai`（当 `api_key` 为空时会自动降级为 `mock`，但 `ollama` 不需要密钥）。支持 `openai` / `openai-responses` / `codex` / `openrouter` / `deepseek` / `anthropic` / `antigravity` / `ollama` / `mock` / `auto` / `cli`。
- `llm_model`：默认模型。
- `llm_vision_model`：vision 模型；当检测到图片附件时优先使用（见下节）。
- `api_key`：API key（生产建议用 env 注入，不要提交到 git）。
- `base_url`：OpenAI-compatible base URL。
- `sandbox_base_url`：AIO Sandbox API 根地址（**不含 `/v1`**）。
- `max_tokens`：请求 `max_tokens`。
- `temperature`：采样温度；显式写入 `0` 会被保留。
- `top_p`：Top-P 采样。
- `stop_sequences`：stop 序列列表。
- `user_rate_limit_rps`：按用户的 LLM 调用速率限制（默认 1.0）。
- `user_rate_limit_burst`：按用户的 LLM 调用突发配额（默认 3）。

`llm_provider: auto` 会优先读取 env key（含 Claude OAuth），若缺失再回退到 CLI 登录。`llm_provider: cli` 则优先读取 CLI 登录，再回退到 env key。CLI 订阅优先级：Codex → Antigravity → Claude → OpenAI。`*_BASE_URL` 可覆盖基座地址。

参见：`docs/reference/external-agents-codex-claude-code.md`（Codex/Claude Code 外部代理与 CLI 调用说明）。

### 工具与运行体验

- `tool_preset`：工具权限预设：`sandbox` / `safe` / `read-only` / `full` / `architect`。Web 模式下如显式配置会生效（默认不配置则保持 Web 模式默认允许集）。
- 运行时工具模式由入口决定：`alex` 为 CLI 模式、`alex-server` 为 Web 模式（默认禁用本地文件/命令）。
- `agent_preset`：agent 预设（按项目内 presets 定义）。
- `verbose`：verbose 模式（CLI/Server 的输出更详细）。
- `session_dir`：会话存储目录（支持 `~` 与 `$ENV` 展开）。
- `cost_dir`：cost 存储目录（支持 `~` 与 `$ENV` 展开）。
- `tool_max_concurrent`：工具调用最大并发数（默认 8）。

### Proactive 记忆（proactive.memory）

- `proactive.enabled`：总开关（默认 true）。
- `proactive.memory.enabled`：Markdown 记忆加载开关（从 `~/.alex/memory/` 读取 `MEMORY.md` 与 `memory/YYYY-MM-DD.md`）。
- `proactive.memory.index.enabled`：本地向量索引开关（SQLite + sqlite-vec）。
- `proactive.memory.index.db_path`：索引数据库路径（默认 `~/.alex/memory/index.sqlite`；用户隔离路径会落在 `~/.alex/memory/<user-id>/index.sqlite`）。
- `proactive.memory.index.chunk_tokens`：分块 token 上限（默认 400）。
- `proactive.memory.index.chunk_overlap`：分块重叠 token 数（默认 80）。
- `proactive.memory.index.min_score`：检索最小分数阈值（默认 0.35）。
- `proactive.memory.index.fusion_weight_vector`：向量检索权重（默认 0.7）。
- `proactive.memory.index.fusion_weight_bm25`：BM25 权重（默认 0.3）。
- `proactive.memory.index.embedder_model`：Ollama embedding 模型（默认 `nomic-embed-text`）。

示例（YAML）：

```yaml
runtime:
  proactive:
    enabled: true
    memory:
      enabled: true
      index:
        enabled: true
        db_path: "~/.alex/memory/index.sqlite"
        chunk_tokens: 400
        chunk_overlap: 80
        min_score: 0.35
        fusion_weight_vector: 0.7
        fusion_weight_bm25: 0.3
        embedder_model: "nomic-embed-text"
```

### ACP 执行器配置（executor 适配层）

- `acp_executor_addr`：ACP executor 地址（`http://host:port`）。默认先读 `ACP_PORT` / `.pids/acp.port`（配合 `ACP_HOST`），否则回退 `http://127.0.0.1:9000`。
- `acp_executor_cwd`：executor 工作目录（绝对路径）。默认 `/workspace`；若 host 侧目录不存在会跳过 `chdir`。
- `acp_executor_mode`：executor 工具模式（`sandbox` / `safe` / `read-only` / `full`）。默认 `sandbox`。
- `acp_executor_auto_approve`：自动批准 executor 的权限请求（布尔）。默认 `true`。
- `acp_executor_max_cli_calls`：单次任务最大 CLI 调用次数。
- `acp_executor_max_duration_seconds`：单次任务最大执行时长（秒）。
- `acp_executor_require_manifest`：是否强制产出 artifact manifest。

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

- **分离 secrets 与非 secrets**：生产环境建议用 env（K8s Secret / Docker secret）注入 `OPENAI_API_KEY`、`TAVILY_API_KEY`、`ARK_API_KEY`、`AUTH_JWT_SECRET`、`AUTH_DATABASE_URL`、`CLOUDFLARE_*` 等敏感字段，并在 `config.yaml` 中用 `${ENV}` 引用；主配置文件存放非敏感参数（model/base_url/preset/ports）。
- **明确优先级**：遇到“配置没生效”，按顺序排查：
  1) `alex config` 看当前快照；2) `alex config path` 打开 `config.yaml`；3) 检查 `overrides` 是否覆盖了 `runtime`。
- **谨慎使用 managed overrides**：`overrides` 会覆盖同名 `runtime` 字段；在容器/多环境切换时，常见的坑是忘记清掉 overrides。
- **修改主配置文件需要重启 `alex-server`**：server 启动时会构建 DI container；主配置文件 `~/.alex/config.yaml` 的改动不会自动热更新（managed overrides 可通过 UI/CLI 更新）。
- **Vision 模型必须真支持图片**：很多文本模型不支持 image；建议明确配置 `llm_vision_model`，并用 provider 对应的 vision model 名称。
- **OpenAI-compatible base_url 通常需要带 `/v1`**：例如 OpenAI `https://api.openai.com/v1`、OpenRouter `https://openrouter.ai/api/v1`；少了 `/v1` 常见报错是 404/路径不匹配。
- **Responses API 仍使用 `/v1` base_url**：不要把 `/responses` 写进 `base_url`，只需要在 `llm_provider` 里选择 `openai-responses` / `codex`。
- **控制图片体积**：base64 会显著膨胀 payload，且不同 provider 有请求大小上限；优先使用可访问的远程 URL 或在入库/上传阶段做压缩/缩放。
- **Ollama 仅接受 inline base64 图片**：如果你给 attachment 只填了远程 `uri`，需要确保同时提供 `data`（或 data URI）才能走 `messages[].images`。
- **避免把大体积 data URI 打进日志**：图片常以 `data:image/...;base64,...` 出现；项目已在 LLM request log 里做脱敏，但仍建议避免在业务日志中打印原始附件。
- **工具调用安全**：只开启需要的 `tool_preset`（CLI）；并避免让模型“发明未声明工具”。项目已在基础层对 tool-call 解析做了 declared-tools 过滤，但 preset 仍是第一道闸。
