# WeChat Gateway (QR login)

This gateway runs inside `alex-server` and bridges WeChat messages directly into the agent runtime.

## Quick start

1. Enable the gateway in `config.yaml`:

```yaml
channels:
  wechat:
    enabled: true
    login_mode: "desktop"
    hot_login: true
    hot_login_storage_path: "~/.alex/wechat/storage.json"
    mention_only: true
    reply_with_mention: true
    allow_groups: true
    allow_direct: true
    agent_preset: "default"
    tool_preset: "full"
    tool_mode: "cli"
    reply_timeout_seconds: 180
```

2. Start the server:

```bash
./dev.sh
```

3. Scan the QR code printed by the server process. Once logged in, the gateway will start replying to WeChat messages.

## Computer mode (local bash + filesystem)

To run directly on the host machine, set:

```yaml
channels:
  wechat:
    tool_mode: "cli"
    tool_preset: "full"
```

If the agent server is remote but you want it to drive a local computer, set `tool_mode: "web"` and run `alex acp serve` on the local machine, then point `runtime.acp_executor_addr` at it.

## Allowlisting conversations

If you need to limit which chats can trigger the agent, populate `channels.wechat.allowed_conversation_ids` with the stable `User.ID()` values reported by openwechat. You can log those IDs by temporarily enabling debug logging.
