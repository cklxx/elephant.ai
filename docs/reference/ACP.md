# ACP (Agent Client Protocol)

This document defines the ACP surface **as implemented by elephant.ai**. It is a precise, unambiguous contract for
clients that connect to `alex acp` (stdio) or `alex acp serve` (HTTP/SSE).

## 1. Transport & Framing

ACP uses JSON-RPC 2.0 over two transports:

1. **stdio transport** (`alex acp`): JSON-RPC messages are framed using either:
   - **Line-delimited JSON**: each JSON-RPC message is a single line ending with `\n`.
   - **Content-Length framing**: `Content-Length: <bytes>\r\n\r\n<json>` (LSP/MCP style).

   If the **first** client message uses `Content-Length`, the server will respond using `Content-Length` framing.
   Otherwise the server uses line-delimited JSON.

2. **HTTP/SSE transport** (`alex acp serve --host <host> --port <port>`):
   - Client → server: `POST /acp/rpc?client_id=<id>` with a JSON-RPC payload.
   - Server → client: `GET /acp/sse?client_id=<id>` streams JSON-RPC payloads as SSE `data:` lines.

   Responses to client requests are delivered over SSE (the HTTP POST returns only a status code).

## 2. JSON-RPC Basics

All requests and responses follow JSON-RPC 2.0:

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}
```

Errors follow standard JSON-RPC error objects:

```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32602,"message":"Invalid params","data":"..."}}
```

## 3. Protocol Version

- Server protocol version: **1**
- `initialize.params.protocolVersion` is required from the client.
- The server responds with `protocolVersion: 1`.

## 4. Lifecycle

1. `initialize`
2. (optional) `authenticate`
3. `session/new` **or** `session/load`
4. `session/prompt` (per user turn)
5. `session/cancel` (notification, optional)
6. `session/set_mode` (optional)

Only **one** `session/prompt` may run per session at a time. A second prompt request for the same session while a
prompt is active returns `InvalidRequest`.

## 5. Methods (Agent side)

### 5.1 initialize

**Request**

| field | type | required | meaning |
|---|---|---|---|
| protocolVersion | integer | yes | Client protocol version |
| clientInfo | object | no | Client name/version |
| clientCapabilities | object | no | Client capabilities |

**Response**

| field | type | required | meaning |
|---|---|---|---|
| protocolVersion | integer | yes | Always `1` |
| agentInfo | object | no | Agent name/version/title |
| agentCapabilities | object | no | Capabilities (see below) |
| authMethods | array | no | Always empty |

**agentCapabilities** (current implementation)

| field | value |
|---|---|
| loadSession | `true` |
| promptCapabilities.audio | `true` |
| promptCapabilities.image | `true` |
| promptCapabilities.embeddedContext | `true` |
| mcpCapabilities.http | `false` |
| mcpCapabilities.sse | `false` |
| sessionCapabilities | `{}` |

### 5.2 authenticate

Accepted but **no-op**.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| methodId | string | yes | Authentication method id |

**Response**: empty object.

### 5.3 session/new

Creates a new session.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| cwd | string | yes | Absolute path for the session working directory |
| mcpServers | array | yes | MCP server list (stdio only) |

**Response**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Newly created session id |
| modes | object | no | Session modes state |

### 5.4 session/load

Loads an existing session.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Existing session id |
| cwd | string | yes | Absolute path |
| mcpServers | array | yes | MCP server list (stdio only) |

**Response**

| field | type | required | meaning |
|---|---|---|---|
| modes | object | no | Session modes state |

### 5.5 session/prompt

Executes a prompt turn.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Session id |
| prompt | array | yes | Array of ContentBlock |

**Response**

| field | type | required | meaning |
|---|---|---|---|
| stopReason | string | yes | `end_turn` \| `max_tokens` \| `max_turn_requests` \| `refusal` \| `cancelled` |

### 5.6 session/cancel (notification)

Cancels the active prompt.

**Params**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Session to cancel |

### 5.7 session/set_mode

Switches the session tool mode.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Session id |
| modeId | string | yes | `full` \| `read-only` \| `safe` \| `sandbox` |

**Response**: empty object.

The server also emits a `session/update` notification of type `current_mode_update`.

## 6. Methods (Client side)

### 6.1 session/request_permission

Sent by the agent when user approval is required for a dangerous tool.

**Request**

| field | type | required | meaning |
|---|---|---|---|
| sessionId | string | yes | Session id |
| toolCall | object | yes | ToolCallUpdate (see below) |
| options | array | yes | PermissionOption[] |

The server currently sends **two** options:

| optionId | name | kind |
|---|---|---|
| allow_once | Allow | allow_once |
| reject_once | Reject | reject_once |

**Response**

`outcome` must be one of:

```json
{"outcome":{"outcome":"selected","optionId":"allow_once"}}
```

```json
{"outcome":{"outcome":"selected","optionId":"reject_once"}}
```

```json
{"outcome":{"outcome":"cancelled"}}
```

If the client sends `session/cancel` while a permission request is pending, it **must** respond with `cancelled`.

## 7. Content Blocks

Supported ContentBlock variants in `session/prompt`:

### text

| field | type | required |
|---|---|---|
| type | string | yes (must be `"text"`) |
| text | string | yes |

### resource_link

| field | type | required |
|---|---|---|
| type | string | yes (`"resource_link"`) |
| uri | string | yes |
| name | string | no |
| mimeType | string | no |
| description | string | no |

### resource (embedded)

`resource.resource` accepts **either** text or blob:

| field | type | required |
|---|---|---|
| type | string | yes (`"resource"`) |
| resource.uri | string | yes |
| resource.text | string | yes (text content) |
| resource.blob | string | yes (base64 content) |
| resource.mimeType | string | no |

If `resource.text` is provided, the server **base64-encodes** it internally as an attachment.

### image / audio

| field | type | required |
|---|---|---|
| type | string | yes (`"image"` or `"audio"`) |
| data | string | yes (base64) |
| mimeType | string | no |

## 8. Session Updates (session/update)

The agent sends notifications:

```json
{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"...","update":{...}}}
```

### 8.1 user_message_chunk

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"user_message_chunk"`) |
| content | ContentBlock | yes |

### 8.2 agent_message_chunk

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"agent_message_chunk"`) |
| content | ContentBlock | yes |

### 8.3 tool_call

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"tool_call"`) |
| toolCallId | string | yes |
| title | string | yes |
| status | string | yes (`"in_progress"`) |
| kind | string | no |
| locations | array | no |
| rawInput | object | no |

### 8.4 tool_call_update

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"tool_call_update"`) |
| toolCallId | string | yes |
| status | string | no (`"in_progress" \| "completed" \| "failed"`) |
| content | array | no (ToolCallContent blocks) |
| rawOutput | any | no |
| kind | string | no |
| locations | array | no |

### 8.5 plan

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"plan"`) |
| entries | array | yes |

Each entry:

| field | type | required |
|---|---|---|
| content | string | yes |
| priority | string | yes (`high` \| `medium` \| `low`) |
| status | string | yes (`pending` \| `in_progress` \| `completed`) |

### 8.6 current_mode_update

| field | type | required |
|---|---|---|
| sessionUpdate | string | yes (`"current_mode_update"`) |
| currentModeId | string | yes |

### Not emitted by current implementation

- `agent_thought_chunk`
- `available_commands_update`

## 9. Session Modes

`modes` returned in `session/new`/`session/load`:

```json
{
  "currentModeId": "full",
  "availableModes": [
    {"id":"full","name":"Full Access","description":"All tools available"},
    {"id":"read-only","name":"Read-Only","description":"No local writes or execution"},
    {"id":"safe","name":"Safe Mode","description":"Excludes potentially dangerous tools"},
    {"id":"sandbox","name":"Sandbox Mode","description":"Disable local file/shell tools; use sandbox_* tools instead"}
  ]
}
```

Mode mapping:

| modeId | tool preset |
|---|---|
| full | full |
| read-only | read-only |
| safe | safe |
| sandbox | sandbox |

## 10. MCP Servers

`mcpServers` is required in `session/new` and `session/load`. Only **stdio** is supported.

**Stdio MCP server**

| field | type | required | meaning |
|---|---|---|---|
| type | string | yes (`"stdio"`) |
| name | string | yes | Server name |
| command | string | yes | Executable path |
| args | array | yes | Arguments |
| env | array | yes | Environment variables |

Each env entry:

| field | type | required |
|---|---|---|
| name | string | yes |
| value | string | yes |

`http` and `sse` types are **rejected**.

## 11. Stop Reasons

The server maps internal results to ACP stop reasons:

| stopReason | meaning |
|---|---|
| end_turn | normal completion |
| max_tokens | token limit |
| max_turn_requests | max iterations |
| refusal | refusal |
| cancelled | `session/cancel` or context cancellation |

## 12. Examples

### Initialize

```json
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":1}}
```

### New session

```json
{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/abs/path","mcpServers":[]}}
```

### Prompt

```json
{
  "jsonrpc":"2.0",
  "id":3,
  "method":"session/prompt",
  "params":{
    "sessionId":"session-...",
    "prompt":[{"type":"text","text":"Hello ACP"}]
  }
}
```

### Cancel

```json
{"jsonrpc":"2.0","method":"session/cancel","params":{"sessionId":"session-..."}}
```
