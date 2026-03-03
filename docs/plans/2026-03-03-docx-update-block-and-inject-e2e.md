# 2026-03-03 Docx Block Update + Inject E2E Enhancement

## Context

User feedback indicates Feishu doc editing is still not working. Current `channel`/`docx` implementation supports create/read/read_content/list_blocks only, so editing paths fail by capability gap.

## Goals

1. Add docx block text update capability aligned with Feishu `PATCH /docx/v1/documents/:document_id/blocks/:block_id`.
2. Expose the capability through unified `channel` action routing.
3. Add/adjust tests so docx edit path is covered and passing.
4. Enhance inject E2E script to include docx edit verification as part of Feishu feature coverage.

## Non-Goals

1. Full coverage of every docx patch sub-request type (table/grid/image/task).
2. New UX flows outside existing `channel` tool.

## Design

### Option A (chosen)
- Implement focused text update API:
  - input: `document_id`, `block_id`, `content`, optional `document_revision_id`, `client_token`, `user_id_type`.
  - request body uses `update_text.elements[].text_run.content`.
- Keep API small, deterministic, and easy for LLM planning.

### Option B
- Expose generic arbitrary patch payload passthrough.
- Rejected now: too broad, unsafe surface, weak schema guidance.

## Work Plan

1. [ ] Extend `internal/infra/lark/docx.go` with typed `UpdateDocumentBlockText` method.
2. [ ] Extend `internal/infra/tools/builtin/larktools/docx_manage.go` with `update_block_text` action handler.
3. [ ] Extend `internal/infra/tools/builtin/larktools/channel.go` action enum/description/routing/safety mapping for `update_doc_block`.
4. [ ] Add/extend unit tests in:
   - `internal/infra/lark/docx_test.go`
   - `internal/infra/tools/builtin/larktools/docx_manage_test.go`
5. [ ] Enhance `scripts/e2e/lark_doc_tools_e2e.sh` with docx edit verification step.
6. [ ] Run targeted tests + script static check.
7. [ ] Run mandatory code review skill and fix P0/P1 if any.
8. [ ] Commit with clear message.

## Verification

- `go test ./internal/infra/lark ./internal/infra/tools/builtin/larktools -count=1`
- `bash -n scripts/e2e/lark_doc_tools_e2e.sh`

## Risks & Mitigations

1. SDK field mismatch for docx patch body.
   - Mitigation: use generated builders/types directly and add unit test with request-path assertion.
2. Action safety classification drift.
   - Mitigation: map new action to high-impact write level and cover via channel tests.
