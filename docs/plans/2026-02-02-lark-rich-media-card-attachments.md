# Plan: Lark Rich Media Card with Attachments (2026-02-02)

## Goal
After a conversation completes, upload images/files and send a single rich media card to Lark that previews the result and exposes the uploaded assets.

## Scope
- Add a post-run packaging step that:
  - uploads images/files via Lark APIs,
  - renders an interactive card containing: summary + asset list + download buttons.
- Keep existing plain text replies as fallback.

## Architecture Notes
- Upload path: `internal/channels/lark/gateway.go` → `sendAttachments` → `uploadImage` / `uploadFile`.
- Card rendering: `internal/lark/cards/*` with `msgType="interactive"` (via `sdkMessenger`).
- Rich text builder: `internal/channels/lark/richcontent/*` (for optional preview blocks).

## Design
1) **Asset collection**
   - Use `TaskResult.Attachments` (already assembled during finalization).
   - Filter by size/type with existing Lark config (`auto_upload_max_bytes`, `auto_upload_allow_ext`).

2) **Upload**
   - Reuse `Gateway.sendAttachments` or a new helper that returns:
     - `image_key` for images,
     - `file_key` for documents.
   - Keep all Lark-specific file typing in `larkFileType`.
   - Apply `auto_upload_max_bytes` / `auto_upload_allow_ext` before uploading.

3) **Card payload**
   - Add a new card template: `internal/lark/cards/templates.go`.
   - Fields:
     - Title: session title / task goal.
     - Summary: trimmed answer or extracted highlights.
     - Assets: list of buttons (click to send file/image) with labels.
     - Preview: inline only first 3 images (remaining image assets are buttons only).
   - Card action: `attachment_send` → bot replies with image/file message by key.

4) **Dispatch**
   - In `Gateway.dispatchResult`, when attachments exist:
     - build the card JSON,
     - send `interactive` card after uploads,
     - fall back to text + existing attachment sending if card creation fails.
    - When the attachment card succeeds: do not send separate image/file messages.

## Tests
- Upload helper returns keys and handles unsupported extensions.
- Card payload renders with 0/1/many assets.
- Integration: mock messenger to assert `interactive` payload includes asset buttons.

## Success Criteria
- One Lark message contains a rich card with preview + attachment buttons.
- Attachments are uploaded once and referenced by keys.
- Fallback to text-only reply works when card rendering fails.

## Progress
- [x] 更新方案决策（单卡片 + 按钮发送 + 预览上限）。
- [x] 卡片模板与回调处理实现。
- [x] Gateway 发送逻辑与附件上传策略实现。
- [x] 测试 + 文档更新 + 草稿清理。
