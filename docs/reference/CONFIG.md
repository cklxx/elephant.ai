# ALEX 配置参考

> Last updated: 2026-03-10

**唯一主配置文件**：`~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH`）。CLI 与 `alex-server` 共享 runtime 配置；server 额外读取 server/auth/session/analytics/attachments 等段。

---

## 配置优先级（低 → 高）

1. **内置默认值** — 开箱即用兜底。
2. **主配置文件** — `~/.alex/config.yaml`（或 `ALEX_CONFIG_PATH`）。
3. **Managed overrides** — `alex config set` 写入的 `overrides` 段。

---

## 最小配置示例

```yaml
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o-mini"
  base_url: "https://api.openai.com/v1"
```

完整示例见 `examples/config/runtime-config.yaml`。

---

## Managed Overrides

最后一层覆盖，用于快速切模型/切 base_url，不需要改主配置。

```yaml
overrides:
  llm_model: "deepseek/deepseek-chat"
  llm_vision_model: "openai/gpt-4o-mini"
```

CLI 操作：

```bash
alex config                        # 查看当前快照
alex config set llm_model gpt-4o   # 设置覆盖
alex config clear llm_model        # 清除覆盖
alex config path                   # 配置文件路径
```

---

## Runtime 字段参考

> `runtime` 与 `overrides` 使用同一套字段名（snake_case）。

### LLM

| 字段 | 说明 | 默认 |
|------|------|------|
| `llm_provider` | Provider 选择。支持 `openai` / `openai-responses` / `codex` / `openrouter` / `deepseek` / `anthropic` / `antigravity` / `ollama` / `llama.cpp` / `mock` / `auto` / `cli` | `openai` |
| `llm_model` | 默认模型 | — |
| `llm_vision_model` | Vision 模型，检测到图片附件时优先使用 | — |
| `api_key` | API key（生产建议用 env 注入） | — |
| `base_url` | OpenAI-compatible base URL | — |
| `max_tokens` | 请求 max_tokens | — |
| `temperature` | 采样温度（显式写 `0` 会保留） | — |
| `top_p` | Top-P 采样 | — |
| `stop_sequences` | Stop 序列列表 | — |
| `llm_cache_size` | LLM 响应缓存条数 | — |
| `llm_cache_ttl_seconds` | LLM 缓存 TTL（秒） | — |
| `llm_request_timeout_seconds` | LLM 请求超时（秒） | — |
| `llm_fallback_rules` | 模型降级规则 | — |
| `user_rate_limit_rps` | 按用户 LLM 调用速率限制 | `1.0` |
| `user_rate_limit_burst` | 按用户 LLM 突发配额 | `3` |
| `kimi_rate_limit_rps` | Kimi provider 速率限制 | — |
| `kimi_rate_limit_burst` | Kimi provider 突发配额 | — |

**Provider 选择逻辑：**

- `auto`：优先读取 env key（含 Claude OAuth），缺失时回退 CLI 登录。
- `cli`：优先 CLI 登录，再回退 env key。CLI 订阅优先级：Codex → Antigravity → Claude → OpenAI。
- `api_key` 优先级：`runtime.api_key` / override > provider-specific env（如 `OPENAI_API_KEY`）> `LLM_API_KEY`。

#### llama.cpp（本地推理）

`llm_provider: "llama.cpp"` 走 llama-server 的 OpenAI-compatible API。

```bash
alex llama-cpp pull <hf_repo> <gguf_file>   # 下载模型
llama-server -m "<model.gguf>" --port 8080   # 启动
```

```yaml
runtime:
  llm_provider: "llama.cpp"
  base_url: "http://127.0.0.1:8080/v1"
```

### 工具与运行

| 字段 | 说明 | 默认 |
|------|------|------|
| `tool_preset` | 工具预设：`safe` / `read-only` / `full` / `architect` | `full` |
| `toolset` | 工具实现：`default`（沙箱）/ `local` / `lark-local`（本地执行） | `default` |
| `agent_preset` | Agent 预设 | — |
| `tool_max_concurrent` | 工具调用最大并发数 | `8` |
| `max_iterations` | ReAct 最大迭代次数 | — |
| `profile` | 运行 profile：`quickstart` / `standard` / `production` | `standard` |
| `environment` | 运行环境标识 | — |
| `verbose` | Verbose 模式 | `false` |
| `disable_tui` | 禁用 TUI | `false` |
| `follow_transcript` | 跟随 transcript 输出 | `false` |
| `follow_stream` | 跟随流式输出 | `false` |
| `session_dir` | 会话存储目录（支持 `~` 和 `$ENV`） | — |
| `cost_dir` | Cost 存储目录 | — |
| `session_stale_after_seconds` | 会话过期时间（秒） | — |

### Tool Policy

| 字段 | 说明 | 默认 |
|------|------|------|
| `tool_policy.enforcement_mode` | `enforce`（拒绝）/ `warn_allow`（告警放行） | `enforce` |

### 浏览器

| 字段 | 说明 | 默认 |
|------|------|------|
| `browser.connector` | 连接方式：`cdp` / `chrome_extension` | `cdp` |
| `browser.cdp_url` | CDP URL（`ws://...` 或 `http://host:port`） | — |
| `browser.chrome_path` | Chrome 路径（无 cdp_url 时生效） | — |
| `browser.headless` | 是否 headless | — |
| `browser.user_data_dir` | Chrome user-data-dir | — |
| `browser.timeout_seconds` | 浏览器工具超时（秒） | — |
| `browser.bridge_listen_addr` | Extension Bridge 监听地址（仅 loopback） | `127.0.0.1:17333` |
| `browser.bridge_token` | Extension Bridge token | — |

### HTTP 限制

| 字段 | 说明 | 默认 |
|------|------|------|
| `http_limits.*` | HTTP 并发/超时限制 | — |

### ACP 执行器

| 字段 | 说明 | 默认 |
|------|------|------|
| `acp_executor_addr` | ACP executor 地址 | `http://127.0.0.1:9000` |
| `acp_executor_cwd` | 工作目录 | `/workspace` |
| `acp_executor_mode` | 工具模式：`safe` / `read-only` / `full` | `host` |
| `acp_executor_auto_approve` | 自动批准权限请求 | `true` |
| `acp_executor_max_cli_calls` | 单次任务最大 CLI 调用次数 | — |
| `acp_executor_max_duration_seconds` | 单次任务最大时长（秒） | — |
| `acp_executor_require_manifest` | 强制产出 artifact manifest | — |

### External Agents

| 字段 | 说明 |
|------|------|
| `external_agents.max_parallel_agents` | 外部 agent 最大并发数 |
| `external_agents.claude_code.*` | Claude Code bridge 参数（binary/model/mode/budget/timeout/env） |
| `external_agents.codex.*` | Codex bridge 参数（binary/model/approval/sandbox/timeout/env） |
| `external_agents.kimi.*` | Kimi bridge 参数（binary/model/approval/sandbox/timeout/env） |
| `external_agents.teams[]` | 团队编排定义（`alex team run --template ...`） |

Team 配置示例：

```yaml
runtime:
  external_agents:
    codex:
      enabled: true
      binary: "codex"
      default_model: "gpt-5.2-codex"
      approval_policy: "never"
      sandbox: "danger-full-access"
      timeout: "30m"
      env:
        OPENAI_API_KEY: "${OPENAI_API_KEY}"
    claude_code:
      enabled: true
      binary: "claude"
      default_mode: "autonomous"
      timeout: "30m"
      env:
        ANTHROPIC_API_KEY: "${ANTHROPIC_API_KEY}"
    kimi:
      enabled: true
      binary: "kimi"
      default_model: "kimi-k2-0905-preview"
      approval_policy: "never"
      sandbox: "danger-full-access"
      timeout: "30m"
      env:
        KIMI_API_KEY: "${KIMI_API_KEY}"
    teams:
      - name: "execute_review_report"
        description: "Codex 执行、Kimi 复核、Claude 汇总"
        roles:
          - name: "executor"
            agent_type: "codex"
            execution_mode: "execute"
            autonomy_level: "full"
            workspace_mode: "worktree"
          - name: "reviewer"
            agent_type: "kimi"
            execution_mode: "execute"
            inherit_context: true
          - name: "reporter"
            agent_type: "claude_code"
            execution_mode: "execute"
            inherit_context: true
        stages:
          - name: "execution"
            roles: ["executor"]
          - name: "review"
            roles: ["reviewer"]
          - name: "reporting"
            roles: ["reporter"]
```

### 外部工具 Keys

| 字段 | 说明 |
|------|------|
| `tavily_api_key` | Tavily web search |
| `ark_api_key` | Ark |
| `moltbook_api_key` | Moltbook |
| `moltbook_base_url` | Moltbook base URL |

### Seedream（Ark）端点

`seedream_text_endpoint_id` / `seedream_image_endpoint_id` / `seedream_text_model` / `seedream_image_model` / `seedream_vision_model` / `seedream_video_model`

---

## Proactive 配置

### 记忆（proactive.memory）

| 字段 | 说明 | 默认 |
|------|------|------|
| `proactive.enabled` | 总开关 | `true` |
| `proactive.memory.enabled` | Markdown 记忆加载 | — |
| `proactive.memory.index.enabled` | 本地向量索引（SQLite + sqlite-vec） | — |
| `proactive.memory.index.db_path` | 索引数据库路径 | `~/.alex/memory/index.sqlite` |
| `proactive.memory.index.chunk_tokens` | 分块 token 上限 | `400` |
| `proactive.memory.index.chunk_overlap` | 分块重叠 token 数 | `80` |
| `proactive.memory.index.min_score` | 检索最小分数 | `0.35` |
| `proactive.memory.index.fusion_weight_vector` | 向量检索权重 | `0.7` |
| `proactive.memory.index.fusion_weight_bm25` | BM25 权重 | `0.3` |
| `proactive.memory.index.embedder_model` | Ollama embedding 模型 | `nomic-embed-text` |

### Prompt 组装（proactive.prompt）

| 字段 | 说明 | 默认 |
|------|------|------|
| `proactive.prompt.mode` | 系统提示词模式：`full` / `minimal` / `none` | `full` |
| `proactive.prompt.timezone` | 用户时区 | — |
| `proactive.prompt.bootstrap_max_chars` | Bootstrap 文件单文件最大字符数 | `20000` |
| `proactive.prompt.bootstrap_files` | 首轮注入文件列表 | `AGENTS.md`, `SOUL.md`, `TOOLS.md`, `IDENTITY.md`, `USER.md`, `HEARTBEAT.md`, `BOOTSTRAP.md` |
| `proactive.prompt.reply_tags_enabled` | Reply Tags 段落 | `false` |

### Heartbeat（proactive.scheduler.heartbeat / proactive.timer）

| 字段 | 说明 | 默认 |
|------|------|------|
| `proactive.scheduler.heartbeat.enabled` | 全局 heartbeat cron | — |
| `proactive.scheduler.heartbeat.schedule` | Cron 表达式 | `*/30 * * * *` |
| `proactive.scheduler.heartbeat.task` | Heartbeat 任务提示 | — |
| `proactive.scheduler.heartbeat.channel/user_id/chat_id` | 通知路由 | — |
| `proactive.scheduler.heartbeat.quiet_hours` | 安静时段 | `[23, 8]` |
| `proactive.scheduler.heartbeat.window_lookback_hours` | 安静超时后触达窗口 | `8` |
| `proactive.timer.heartbeat_enabled` | Timer 轨 heartbeat | — |
| `proactive.timer.heartbeat_minutes` | Timer heartbeat 周期（分钟） | `30` |

### Scheduler 通用

| 字段 | 说明 | 默认 |
|------|------|------|
| `proactive.scheduler.enabled` | 调度器总开关 | — |
| `proactive.scheduler.trigger_timeout_seconds` | 触发器超时 | — |
| `proactive.scheduler.concurrency_policy` | 并发策略 | — |
| `proactive.scheduler.leader_lock_enabled` | Leader 锁 | — |
| `proactive.scheduler.leader_lock_name` | 锁名称 | — |
| `proactive.scheduler.leader_lock_acquire_interval_seconds` | 锁获取间隔 | — |
| `proactive.scheduler.job_store_path` | Job 存储路径 | — |
| `proactive.scheduler.cooldown_seconds` | 冷却时间（秒） | — |
| `proactive.scheduler.max_concurrent` | 最大并发 | — |
| `proactive.scheduler.recovery_max_retries` | 恢复最大重试 | — |
| `proactive.scheduler.recovery_backoff_seconds` | 恢复退避时间 | — |

### Skills / OKR / Attention

| 字段 | 说明 |
|------|------|
| `proactive.skills.*` | Skills 自动激活配置 |
| `proactive.okr.*` | OKR 追踪配置 |
| `proactive.attention.*` | 注意力门控配置 |

---

## Server 配置段

由 `alex-server` 启动时读取，CLI 忽略。

### 基础

| 字段 | 说明 | 默认 |
|------|------|------|
| `port` | HTTP 端口 | `8080` |
| `debug_port` | Debug 端口 | — |
| `debug_bind_host` | Debug 绑定地址 | — |
| `max_task_body_bytes` | Task POST 请求体上限 | 20 MiB |
| `allowed_origins` | CORS 允许来源列表 | — |
| `leader_api_token` | Leader API token | — |
| `trusted_proxies` | 信任的代理列表 | — |

### 流式与速率

| 字段 | 说明 | 默认 |
|------|------|------|
| `stream_max_duration_seconds` | 流式请求最大时长 | 7200 (2h) |
| `stream_max_bytes` | 单连接最大输出字节 | 64 MiB |
| `stream_max_concurrent` | 同时流式连接数 | `128` |
| `rate_limit_requests_per_minute` | HTTP 速率限制 | `600` |
| `rate_limit_burst` | 速率限制突发配额 | `120` |
| `non_stream_timeout_seconds` | 非流式请求超时 | `30` |

### 任务执行

| 字段 | 说明 | 默认 |
|------|------|------|
| `task_execution_owner_id` | Claim/lease owner 标识 | `<hostname>:<pid>` |
| `task_execution_lease_ttl_seconds` | Lease TTL | `45` |
| `task_execution_lease_renew_interval_seconds` | Lease 续租间隔 | `15` |
| `task_execution_max_in_flight` | 全局并发上限（0 关闭限制） | `64` |
| `task_execution_resume_claim_batch_size` | 单次恢复 claim 上限 | `128` |

### 事件历史

| 字段 | 说明 | 默认 |
|------|------|------|
| `event_history_retention_days` | 保留天数（0 关闭清理） | `30` |
| `event_history_max_sessions` | 内存最大会话数（0 不限） | `100` |
| `event_history_session_ttl_seconds` | 空闲 TTL（0 不启用） | `3600` |
| `event_history_max_events` | 单会话最大事件数（0 不限） | `1000` |
| `event_history_async_batch_size` | 异步落盘批大小 | `200` |
| `event_history_async_flush_interval_ms` | 定时 flush 间隔 | `250` |
| `event_history_async_append_timeout_ms` | 队列满时等待超时 | `50` |
| `event_history_async_queue_capacity` | 异步队列容量 | `8192` |
| `event_history_async_flush_request_coalesce_window_ms` | Flush 合并窗口 | `8` |
| `event_history_async_backpressure_high_watermark` | 背压阈值 | `6553` |
| `event_history_degrade_debug_events_on_backpressure` | 背压下降级调试事件 | `true` |

---

## 其他配置段

### auth

`jwt_secret` / `access_token_ttl_minutes` / `refresh_token_ttl_days` / `state_ttl_minutes` / `redirect_base_url` / `database_url` / `database_pool_max_conns`（默认 4） / `bootstrap_email` / `bootstrap_password` / `bootstrap_display_name` / Google OAuth 字段。

### session

`database_url` / `dir` / `pool_max_conns` / `pool_min_conns` / `pool_max_conn_lifetime_seconds` / `pool_max_conn_idle_seconds` / `pool_health_check_seconds` / `pool_connect_timeout_seconds` / `cache_size`（默认 256，0 关闭）。

### analytics

`posthog_api_key` / `posthog_host`。

### attachments

| 字段 | 说明 |
|------|------|
| `provider` | `local` / `cloudflare` |
| `dir` | 本地存储目录（provider=local 时） |
| `cloudflare_*` | Cloudflare R2 配置 |
| `presign_ttl` | 预签名 TTL（建议 `4h`） |

### web

`api_url` — 仅供部署脚本读取（`NEXT_PUBLIC_API_URL`）。

### apps

自定义 app 插件列表，由 server 配置 API 管理。

### observability

由 `internal/infra/observability` 读取（日志/metrics/tracing）。

---

## Channels

### channels.lark

| 字段 | 说明 | 默认 |
|------|------|------|
| `enabled` | 启用 Lark 网关 | `false` |
| `app_id` / `app_secret` | Lark 应用凭证 | — |
| `base_domain` | Lark API 域名 | `https://open.larkoffice.com` |
| `session_prefix` | 会话 ID 前缀 | `lark` |
| `reply_prefix` | 回复前缀 | — |
| `allow_groups` / `allow_direct` | 是否响应群聊/私聊 | — |
| `agent_preset` / `tool_preset` / `tool_mode` | 通道级 preset | `tool_preset: full` |
| `workspace_dir` | 本地工具工作区根目录 | 进程 cwd |
| `tenant_calendar_id` | 共享日历 ID | — |
| `reply_timeout_seconds` | 单条消息执行超时 | — |
| `memory_enabled` | Markdown 记忆加载 | — |
| `show_tool_progress` | 显示工具执行进度 | — |
| `auto_chat_context` / `auto_chat_context_size` | 自动拉取近期聊天上下文 | — |

**Plan Review：**
`plan_review_enabled` / `plan_review_require_confirmation` / `plan_review_pending_ttl_minutes`

**Persistence：**
`persistence.mode`（`file`/`memory`，默认 `file`） / `persistence.dir`（默认 `~/.alex/lark`） / `persistence.retention_hours`（默认 168） / `persistence.max_tasks_per_chat`（默认 200）

**Auto Upload：**
`auto_upload_files`（默认 true） / `auto_upload_max_bytes`（默认 2MB） / `auto_upload_allow_ext`

**Browser：**
`browser.cdp_url` / `browser.chrome_path` / `browser.headless` / `browser.user_data_dir` / `browser.timeout_seconds`

**Reactions：**
`react_emoji`（随机表情池） / `injection_ack_react_emoji`（默认 `THINKING`）

> `allow_groups` 控制代码侧响应。平台是否投递群消息取决于应用权限。"获取群组中所有消息"需额外权限。

---

## 环境变量

### 配置文件定位

- `ALEX_CONFIG_PATH` — 主配置文件路径

### YAML 插值

在 `config.yaml` 中使用 `${ENV}`（`$$` 表示字面量 `$`）：

```yaml
runtime:
  api_key: "${LLM_API_KEY}"
```

### LLM Keys

| 变量 | 说明 |
|------|------|
| `LLM_API_KEY` | 统一 LLM key（兜底） |
| `OPENAI_API_KEY` | OpenAI key（优先于 `LLM_API_KEY`） |
| `ANTHROPIC_API_KEY` | Claude key |
| `CLAUDE_CODE_OAUTH_TOKEN` | Claude Code OAuth |
| `ANTHROPIC_AUTH_TOKEN` | Claude OAuth（备用） |
| `CODEX_API_KEY` | Codex key |
| `ANTIGRAVITY_API_KEY` | Antigravity key |
| `KIMI_API_KEY` | Kimi key |

### Base URL Overrides

`OPENAI_BASE_URL` / `ANTHROPIC_BASE_URL` / `CODEX_BASE_URL` / `ANTIGRAVITY_BASE_URL`

### 工具 Keys

`TAVILY_API_KEY`（web_search） / `ARK_API_KEY`（Seedream/Ark）

### 路径覆盖

| 变量 | 说明 | 默认 |
|------|------|------|
| `ALEX_PROFILE` | 运行 profile | — |
| `ALEX_CLI_AUTH_PATH` | CLI auth.json 路径 | — |
| `ALEX_LLM_SELECTION_PATH` | 订阅模型选择文件 | `~/.alex/llm_selection.json` |
| `ALEX_ONBOARDING_STATE_PATH` | Onboarding 状态文件 | `~/.alex/onboarding_state.json` |
| `ALEX_SKILLS_DIR` | Skills 根目录 | `~/.alex/skills` |

### 服务端

`AUTH_JWT_SECRET` / `AUTH_DATABASE_URL` / `AUTH_DATABASE_POOL_MAX_CONNS` / `ALEX_SESSION_DATABASE_URL` / `GOOGLE_CLIENT_SECRET` / `CLOUDFLARE_ACCOUNT_ID` / `CLOUDFLARE_ACCESS_KEY_ID` / `CLOUDFLARE_SECRET_ACCESS_KEY`

### 网络代理

ALEX 遵循标准代理变量：`HTTP_PROXY` / `HTTPS_PROXY` / `ALL_PROXY` / `NO_PROXY`。

`ALEX_PROXY_MODE` 控制代理行为：

| 模式 | 说明 |
|------|------|
| `auto`（默认） | 遵循代理 env；loopback 代理不可达时自动绕过 |
| `strict` | 严格遵循代理 env；不可用直接失败 |
| `direct` | 忽略代理 env，全部直连 |

---

## Skills 目录

- 默认使用 `~/.alex/skills`（可通过 `ALEX_SKILLS_DIR` 覆盖）。
- 首次启动时从仓库 `skills/` 同步到 `~/.alex/skills`（仓库优先补齐，写入 `.repo_backfill_version` marker）。
- 之后仅复制缺失 skill，已存在的不覆盖。

---

## Lark 自治脚本（`lark.sh`）

`lark.sh` 是本地自治入口（`up|down|restart|status|logs|doctor|cycle`）。

| 变量 | 说明 | 默认 |
|------|------|------|
| `LARK_SUPERVISOR_TICK_SECONDS` | Supervisor 轮询周期 | `5` |
| `LARK_RESTART_MAX_IN_WINDOW` | 窗口内最大重启次数 | `5` |
| `LARK_RESTART_WINDOW_SECONDS` | 重启窗口（秒） | `600` |
| `LARK_COOLDOWN_SECONDS` | 熔断冷却时长 | `300` |
| `LARK_SUPERVISOR_AUTOFIX_ENABLED` | 自动修复 | `1` |
| `LARK_SUPERVISOR_AUTOFIX_TRIGGER` | 修复触发策略 | `cooldown` |
| `LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS` | Codex 执行超时 | `1800` |
| `LARK_SUPERVISOR_AUTOFIX_MAX_IN_WINDOW` | 窗口内修复上限 | `3` |
| `LARK_SUPERVISOR_AUTOFIX_WINDOW_SECONDS` | 修复窗口（秒） | `3600` |
| `LARK_SUPERVISOR_AUTOFIX_COOLDOWN_SECONDS` | 修复限流冷却 | `900` |
| `LARK_SUPERVISOR_AUTOFIX_SCOPE` | 修复范围 | `repo` |
| `MAIN_CONFIG` | Main 进程配置路径 | `~/.alex/config.yaml` |
| `TEST_CONFIG` | Test 进程配置路径 | `~/.alex/test.yaml` |

状态文件：`.worktrees/test/tmp/lark-supervisor.status.json`

---

## Vision 配置

当用户消息携带图片附件时：
- 若配置了 `llm_vision_model`，本次请求切换到该模型。
- 否则使用 `llm_model`（可能不支持图片）。

---

## 最佳实践

- **分离 secrets**：生产用 env 注入敏感字段，`config.yaml` 用 `${ENV}` 引用。
- **排查配置不生效**：`alex config` 查看快照 → `alex config path` 打开文件 → 检查 overrides 是否覆盖了 runtime。
- **重启生效**：主配置文件修改需重启 `alex-server`。
- **base_url 带 `/v1`**：OpenAI `https://api.openai.com/v1`、OpenRouter `https://openrouter.ai/api/v1`。Responses API 不要把 `/responses` 写进 base_url。
- **Vision 模型要真支持图片**：明确配置 `llm_vision_model`。
- **Ollama 只接受 inline base64 图片**：确保提供 `data`（或 data URI）。
- **控制图片体积**：优先使用远程 URL 或在上传阶段压缩。
