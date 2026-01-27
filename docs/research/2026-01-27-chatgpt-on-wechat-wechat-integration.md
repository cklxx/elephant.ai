# Research: Integrating elephant.ai with WeChat via chatgpt-on-wechat (2026-01-27)

## Goal
- Provide a copyable integration path to connect elephant.ai to WeChat using chatgpt-on-wechat as the gateway.

## Key findings from chatgpt-on-wechat
- chatgpt-on-wechat uses `config.json` (copied from `config-template.json`); `channel_type` selects the channel (terminal/wechatmp/wechatmp_service/wechatcom_app, etc.) and `open_ai_api_base` + `open_ai_api_key` point at an OpenAI-compatible backend.
- For OpenAI-compatible third-party services, `bot_type` should be `chatGPT`.
- WeChat official account config fields include `wechatmp_token`, `wechatmp_port`, `wechatmp_app_id`, `wechatmp_app_secret`, and `wechatmp_aes_key`. The `wechatmp_port` requires port forwarding to 80 or 443.
- WeCom self-built app config fields include `wechatcom_corp_id`, `wechatcomapp_token`, `wechatcomapp_port`, `wechatcomapp_secret`, `wechatcomapp_agent_id`, and `wechatcomapp_aes_key`. The `wechatcomapp_port` does not require port forwarding.
- Personal account channels are split between itchat (`wx`) and wechaty (`wxy`); itchat handles QR login + message dispatch directly, while wechaty uses a puppet service token for login and message events.

## Recommended integration pattern (copy the technique)
### 1) Use chatgpt-on-wechat as the WeChat gateway
Pick either:
- `wechatmp` / `wechatmp_service` for public account (公众号)
- `wechatcom_app` for WeCom self-built app (企业微信自建应用)

### 2) Expose elephant.ai via an OpenAI-compatible adapter
chatgpt-on-wechat expects OpenAI-style endpoints when `open_ai_api_base` is configured. Set `bot_type: "chatGPT"` for OpenAI-compatible third-party services. A thin adapter makes elephant.ai look like OpenAI `/v1/chat/completions`.

### 3) Map WeChat sessions to elephant.ai sessions
- Stable mapping (recommendation): `session_id = sha1(channel_type + conversation_id + user_id)`.
- Use the same `session_id` for follow-up messages to leverage elephant.ai session memory.

### 4) Convert OpenAI chat to elephant.ai task
- Convert `messages` (system + conversation) into a single `task` string or use the latest user message plus a stitched context.
- Call elephant.ai `POST /api/tasks` with `{task, session_id}`.
- Optionally stream from `GET /api/sse?session_id=...&replay=session` and map the final assistant output back to OpenAI response schema.

## Personal account channel notes (wx vs wxy)
### `wx` (itchat)
- Uses itchat QR login with ASCII QR rendering; supports hot reload to reuse cookies between restarts.
- Registers text and “note” message handlers and routes friend/group messages through a shared processor.
- Best for quick local experiments, but depends on WeChat Web login availability.

### `wxy` (wechaty)
- Uses Wechaty with `wechaty_puppet_service_token`; token-driven puppet services abstract the login + protocol.
- Provides event hooks for message/ready/logout and reuses the same chat processing pipeline.
- Better for long-running deployments if you have a stable puppet provider, but adds an external dependency.

## YAML config mapping (mirror chatgpt-on-wechat config.json)
> Upstream uses `config.json`. The YAML below is a 1:1 key map; keep it YAML here and translate when applying upstream.

### WeChat official account (公众号)
```yaml
channel_type: wechatmp
wechatmp_token: "TOKEN"
wechatmp_port: 80 # requires port forwarding to 80 or 443
wechatmp_app_id: "APPID"
wechatmp_app_secret: "APPSECRET"
wechatmp_aes_key: ""
model: "gpt-4o-mini"
bot_type: "chatGPT"
open_ai_api_key: "adapter-key"
open_ai_api_base: "http://adapter-host:8081/v1"
# optional: allow any message to trigger
single_chat_prefix: [""]
single_chat_reply_prefix: ""
```

### WeCom self-built app (企业微信自建应用)
```yaml
channel_type: wechatcom_app
wechatcom_corp_id: "CORPID"
wechatcomapp_token: "TOKEN"
wechatcomapp_port: 9898 # no port forwarding required
wechatcomapp_secret: "SECRET"
wechatcomapp_agent_id: "AGENTID"
wechatcomapp_aes_key: "AESKEY"
model: "gpt-4o-mini"
bot_type: "chatGPT"
open_ai_api_key: "adapter-key"
open_ai_api_base: "http://adapter-host:8081/v1"
```

## Minimal adapter contract (OpenAI-compatible)
- **Input**: `POST /v1/chat/completions` with `messages` and `model`.
- **Output**: OpenAI-style completion JSON (or streaming chunks if you implement streaming).
- **Internal calls**:
  - `POST http://elephant-host:8080/api/tasks` with `task` + `session_id`
  - `GET http://elephant-host:8080/api/sse?session_id=...&replay=session` (optional streaming)

## Elephant.ai server references (local)
- `internal/server/README.md` documents `POST /api/tasks` and `GET /api/sse`.

## Sources
- chatgpt-on-wechat `README.md` (configuration section).
- chatgpt-on-wechat `config.py` (available_setting fields).
