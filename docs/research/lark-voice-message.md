# Lark Voice/Audio Message Support — Evidence & Code Touchpoints

> Goal: enable sending voice messages (audio) via Lark from this system.
> Source focus: `/Users/bytedance/code/elephant.ai/docs`
> Evidence comes from repo code inspection + public Lark docs (URLs below).

## 1) Lark API Endpoints (evidence)

**Send message**
- Endpoint: `POST /open-apis/im/v1/messages`
- Supports `msg_type=audio` (along with text, post, image, file, media, etc.).
- Evidence: Lark “Send message” API docs list `audio` in message types.
  - https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
  - https://open.feishu.cn/document/server-docs/im-v1/message/create

**Upload file (audio)**
- Endpoint: `POST /open-apis/im/v1/files` (documented as “im-v1/file/create”).
- Supports uploading audio files.
- Evidence: Lark “Upload file” API docs.
  - https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create

**Message content format (audio)**
- For `msg_type=audio`, content uses `file_key`:
  ```json
  {"file_key":"file_v2_xxx"}
  ```
- Evidence: Lark “Sent message content” doc shows audio example with `file_key`.
  - https://open.larksuite.com/document/server-docs/im-v1/message-content-description/create_json

> Note: This environment cannot fetch full doc text; URLs above contain the authoritative scope requirements & request samples.

## 2) Required Scopes (how to verify)

The Lark docs for **Send message** and **Upload file** each include a **“Scope requirements”** section. Check those pages in your Lark app console:
- Send message scope requirements: https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
- Upload file scope requirements: https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create

**Actionable check**: ensure the app has the IM send + file upload scope(s) shown on those pages. (Exact scope strings are not embedded in this repo.)

## 3) Current Code Touchpoints (repo evidence)

**Message sending (text)**
- `internal/infra/tools/builtin/larktools/send_message.go`
  - Uses `client.Im.Message.Create` with `MsgType("text")` and JSON content.
  - No support for `msg_type=audio`.

**File upload + message send (file)**
- `internal/infra/tools/builtin/larktools/upload_file.go`
  - Uploads via `client.Im.V1.File.Create(...)`
  - Sends a **file** message: `MsgType("file")` with `content={"file_key":...}`.
  - Supported file types list includes `opus`, `mp4`, `pdf`, `doc`, `xls`, `ppt`, `stream`.
  - No `audio` message type send.

**Unified tool**
- `internal/infra/tools/builtin/larktools/channel.go`
  - Supports actions: `send_message`, `upload_file`, `history`, calendar, tasks.
  - No `send_audio` or `send_voice` action.

**Chat history parsing**
- `internal/infra/tools/builtin/larktools/chat_history.go`
  - Recognizes `msg_type=audio` and renders `[audio]`.

## 4) Gap Analysis

**What exists**
- File upload works and already supports audio-like file types (e.g., `opus`).
- Chat history can display audio message types.

**What’s missing**
- A sender path for **audio messages**: `msg_type=audio` with `content={"file_key":...}`.
- Optional: support for `msg_type=media` (audio+cover image), if desired.

## 5) Minimal Implementation Plan (repo-local)

1) Extend upload tool or add a new tool action:
   - Add `action=upload_audio` or `send_audio` in `channel`.
   - In `upload_file.go`, allow an argument like `message_type` (`file` vs `audio`).

2) When `message_type=audio`:
   - Upload file as today to get `file_key`.
   - Send message with `MsgType("audio")` and `Content({"file_key": "..."})`.

3) Ensure scopes are enabled in Lark app console:
   - Confirm exact scope strings on the Send/Upload docs above.

---

## Evidence index
- Lark Send message API doc (msg_type includes audio):
  - https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/create
- Lark Upload file API doc:
  - https://open.larksuite.com/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create
- Lark Sent message content (audio uses file_key):
  - https://open.larksuite.com/document/server-docs/im-v1/message-content-description/create_json
- Feishu equivalent (same schema, optional cross-check):
  - https://open.feishu.cn/document/server-docs/im-v1/message/create

