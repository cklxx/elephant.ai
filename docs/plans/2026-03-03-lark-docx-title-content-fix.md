# Lark Docx Title/Content Mapping Fix (2026-03-03)

## Goal
- Fix Feishu doc writing path where body content is written into title.
- Align payload construction with official Feishu doc APIs.
- Verify end-to-end tool behavior for create/update/write actions.

## Scope
- `internal/infra/tools/builtin/larktools/docx_manage.go`
- `internal/infra/lark/docx.go`
- Related tests for docx tool and Lark client wrappers.

## Steps
- [x] Trace current docx create/update payload mapping.
- [x] Check official Feishu docs for title/content field semantics.
- [x] Implement minimal mapping fix.
- [x] Add/adjust tests for title/content separation.
- [x] Run targeted tests and required package tests.

## Validation
- `go test ./internal/infra/tools/builtin/larktools -run Docx`
- `go test ./internal/infra/lark -run Docx`

## Progress
- 2026-03-03: plan created; docx path investigation started.
- 2026-03-03: implemented create_doc initial-content write path and task summary/body normalization; passed `go test ./internal/infra/tools/builtin/larktools` and `go test ./internal/infra/lark`.
