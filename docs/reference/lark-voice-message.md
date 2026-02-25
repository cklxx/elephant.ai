# Lark Voice Message Support (Audio) — Evidence + Integration Notes

**Goal:** enable sending voice messages (audio) via Lark from this system.

## 1) What Lark APIs support audio messages

Lark/Feishu IM API supports sending **audio** messages via `/open-apis/im/v1/messages` with `msg_type: "audio"` and uploading audio via `/open-apis/im/v1/files`.

**Evidence (official docs):**
- Send message (msg_type supports audio):
  - https://open.feishu.cn/document/server-docs/im-v1/message/create
  - “Optional values: text, post, image, file, **audio**, media…”
- Upload file (supports audio, returns file_key):
  - https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create
  - “Upload files, with videos, **audios**, and common file types supported.”
- Message content spec (audio section):
  - https://open.feishu.cn/document/server-docs/im-v1/message-content-description/create_json (Audio section)
- Reply message doc notes audio/files must be uploaded first and then file_key used:
  - https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/reply

**Implication:** to send voice:
1) upload audio file via `im/v1/files` → get `file_key`
2) send message with `msg_type = "audio"` and `content` referencing `file_key`

## 2) Required scopes / permissions

Minimum permissions for bot messaging + file upload:
- `im:message` (read/send messages in chats)
- `im:resource` (read/upload images or other files)

**Evidence:**
- Bot external group guide lists scopes:
  - https://open.feishu.cn/document/develop-robots/add-bot-to-external-group
  - “im:message (Read and send messages), im:resource (Read and upload images or other files)”

> If using “send as bot” policies, ensure `im:message:send_as_bot` is approved (commonly required by app setups).

## 3) Current code paths in this repo

### Upload file tool (already exists)
- **File:** `internal/infra/tools/builtin/larktools/upload_file.go`
- Uses `client.Im.V1.File.Create` to upload a file → returns `file_key`
- Then **sends a message with `msg_type="file"`** in `sendFileMessage()`

Key code:
```go
uploadReq := larkim.NewCreateFileReqBuilder().
  Body(larkim.NewCreateFileReqBodyBuilder().
    FileType(candidate.fileType).
    FileName(candidate.fileName).
    File(candidate.reader).Build()).Build()

// sends message with msg_type="file"
req := larkim.NewCreateMessageReqBuilder().
  ReceiveIdType("chat_id").
  Body(larkim.NewCreateMessageReqBodyBuilder().
    ReceiveId(chatID).
    MsgType("file").
    Content(content).Build()).Build()
```

### Lark gateway file type mapping
- **File:** `internal/delivery/channels/lark/gateway.go`
- `larkSupportedFileTypes` includes **"opus"**, “mp4”, “pdf”, “doc”, “xls”, “ppt”, “stream”
  - **No explicit "audio" send support** here; it maps extensions to file upload types.

Key snippet:
```go
var larkSupportedFileTypes = map[string]bool{
  "opus": true, "mp4": true, "pdf": true,
  "doc": true, "xls": true, "ppt": true,
  "stream": true,
}
```

### Current capability gap
- We **can upload audio files**, but we **always send `msg_type=file`**.
- **No code path uses `msg_type=audio`** with audio content structure.
- There is no dedicated `lark_send_audio` tool or `channel action` for audio.

## 4) What needs to be added

1) **Add an audio send path** (new tool or channel action) that:
   - Uploads audio via `im/v1/files` (already in upload_file tool)
   - Sends `msg_type="audio"` with proper content structure referencing `file_key`

2) **Define audio content JSON** per message content spec (Audio section):
   - (Doc: https://open.feishu.cn/document/server-docs/im-v1/message-content-description/create_json#7768ebc7)
   - The content structure is a JSON string; **audio requires file_key**.

3) **Expose a new tool** (example)
   - `lark_send_audio` or extend `channel` with `action: send_audio`
   - Params: `path` or `attachment_name`, optional `file_name`, `duration`, `file_type` (opus/mp3/m4a)

## 5) Suggested implementation sketch

- **Option A: extend existing lark_upload_file tool**
  - If media type is audio (e.g., `audio/*` or extension `.m4a`, `.mp3`, `.opus`) then use `msg_type="audio"` and audio content JSON
  - Otherwise keep `msg_type="file"`

- **Option B: add new tool** `lark_send_audio`
  - Reuse upload logic from `lark_upload_file` but send `msg_type="audio"`

## 6) Validation checklist

- ✅ `im/v1/files` returns `file_key`
- ✅ `im/v1/messages` accepts `msg_type=audio`
- ✅ Required scopes are granted (`im:message`, `im:resource`)
- ✅ Audio file format supported (opus/mp3/m4a; if unsupported, fallback to `file`)

## 7) Open questions / risks

- Audio content format specifics (fields) must match Lark doc (Audio section). Confirm exact JSON fields.
- App permission may require `im:message:send_as_bot` depending on tenant configuration.
- Large audio files may exceed size limits; need to confirm file size caps for `im/v1/files`.

---

### Quick reference links
- Message create: https://open.feishu.cn/document/server-docs/im-v1/message/create
- Message content spec: https://open.feishu.cn/document/server-docs/im-v1/message-content-description/create_json
- Upload file: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/file/create
- Reply message: https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/im-v1/message/reply
- Bot permissions (im:message, im:resource): https://open.feishu.cn/document/develop-robots/add-bot-to-external-group

