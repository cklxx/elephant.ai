# Attachment Layering Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep attachments inline for LLM/agent/tool flows and only externalize to CDN at the HTTP/SSE boundary, avoiding unnecessary CDN fetches for HTML artifacts.

**Architecture:** Remove attachment migrator usage from the agent domain so internal attachment state remains inline. Preserve SSE boundary normalization (attachment store + CDN URLs) for client delivery. Ensure attachment placeholder replacement and LLM-facing catalog remain unchanged.

**Tech Stack:** Go (agent/domain, server/http), attachments store, SSE streaming.

**Notes:**
- Agent/tool/LLM flows keep attachments inline (`Data` or `data:` URIs); no CDN rewrites inside the agent state.
- CDN URLs are generated only at the HTTP/SSE boundary via attachment store normalization.
- `html_edit` prefers inline payloads when both `Data` and `URI` are present.

### Task 1: Add regression test for html_edit reading inline HTML without CDN fetch

**Files:**
- Modify: `internal/tools/builtin/html_edit_test.go`

**Step 1: Write the failing test**

```go
func TestHTMLEditUsesInlineAttachmentWithoutRemoteFetch(t *testing.T) {
	ctx := context.Background()
	tool := NewHTMLEdit(llm.NewMockClient())
	inlineHTML := "<!doctype html><html><body><h1>Hi</h1></body></html>"
	att := ports.Attachment{
		Name:      "demo.html",
		MediaType: "text/html",
		Data:      base64.StdEncoding.EncodeToString([]byte(inlineHTML)),
	}
	ctx = ports.WithAttachmentContext(ctx, map[string]ports.Attachment{att.Name: att}, nil)

	call := ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action": "view",
			"name":   "demo.html",
		},
	}

	res, err := tool.Execute(ctx, call)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Contains(t, res.Content, "<h1>Hi</h1>")
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/builtin -run TestHTMLEditUsesInlineAttachmentWithoutRemoteFetch -v`
Expected: FAIL if current flow forces CDN and loses inline payload.

**Step 3: Implement minimal code**

No code change yet (this test will pass once migration is removed from agent state).

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tools/builtin -run TestHTMLEditUsesInlineAttachmentWithoutRemoteFetch -v`
Expected: PASS after Task 2.

**Step 5: Commit**

```bash
git add internal/tools/builtin/html_edit_test.go
git commit -m "test: cover html_edit inline attachment usage"
```

### Task 2: Remove attachment migrator from agent internal state

**Files:**
- Modify: `internal/agent/domain/react_engine.go` (functions: `applyToolAttachmentMutations`, `normalizeMessageHistoryAttachments`)
- Modify: `internal/agent/app/coordinator.go` (comment updates if needed)
- Test: `internal/agent/domain/react_engine_internal_test.go` (update expectations if any rely on CDN URIs)

**Step 1: Write the failing test**

Add a test in `internal/agent/domain/react_engine_internal_test.go` that ensures attachments produced by tools retain inline `Data`/`data:` URI after mutations and are not rewritten to CDN when in agent state.

```go
func TestAttachmentMutationsKeepInlinePayloads(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{SessionID: "s1", TaskID: "t1"}
	call := ToolCall{Name: "html_edit", ID: "call-1"}
	inline := ports.Attachment{
		Name:      "demo.html",
		MediaType: "text/html",
		Data:      base64.StdEncoding.EncodeToString([]byte("<html></html>")),
	}
	attachments := map[string]ports.Attachment{inline.Name: inline}

	merged := engine.applyToolAttachmentMutations(context.Background(), state, call, attachments, nil, nil)
	got := merged[inline.Name]
	require.NotEmpty(t, got.Data)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/agent/domain -run TestAttachmentMutationsKeepInlinePayloads -v`
Expected: FAIL if migrator rewrites to CDN and clears Data.

**Step 3: Write minimal implementation**

- In `applyToolAttachmentMutations`, remove calls to `normalizeAttachmentsWithMigrator` for `normalized` and mutation maps so attachments remain inline in state.
- In `normalizeMessageHistoryAttachments`, skip migrator normalization so message attachments remain inline.
- Update any comments indicating CDN rewrites in coordinator to reflect boundary-only behavior.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/agent/domain -run TestAttachmentMutationsKeepInlinePayloads -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/agent/domain/react_engine.go internal/agent/domain/react_engine_internal_test.go internal/agent/app/coordinator.go
git commit -m "fix: keep attachments inline inside agent state"
```

### Task 3: Ensure SSE boundary still externalizes HTML attachments

**Files:**
- Modify: `internal/server/http/sse_handler_test.go`

**Step 1: Write the failing test**

Add a test ensuring SSE normalization converts inline HTML attachment to CDN URL and adds HTML preview asset.

```go
func TestSSEExternalizesHTMLAttachments(t *testing.T) {
	store := newTestAttachmentStore(t)
	cache := NewDataCache(1024 * 1024)
	att := ports.Attachment{
		Name:      "demo.html",
		MediaType: "text/html",
		Data:      base64.StdEncoding.EncodeToString([]byte("<html></html>")),
	}
	out := normalizeAttachmentPayload(att, cache, store)
	require.NotEmpty(t, out.URI)
	require.Empty(t, out.Data)
	require.NotEmpty(t, out.PreviewAssets)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/server/http -run TestSSEExternalizesHTMLAttachments -v`
Expected: FAIL if externalization is not happening.

**Step 3: Write minimal implementation**

No code change unless behavior regressed; adjust if needed to ensure `persistHTMLAttachment` and `ensureHTMLPreview` are invoked for inline HTML in SSE normalization.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/server/http -run TestSSEExternalizesHTMLAttachments -v`
Expected: PASS.

**Step 5: Commit**

```bash
git add internal/server/http/sse_handler_test.go
git commit -m "test: sse externalizes inline html attachments"
```

### Task 4: Update documentation about attachment layering

**Files:**
- Modify: `docs/plans/2026-01-19-attachment-layering-design.md` (if exists) or add note to `docs/plans/2026-01-19-attachment-layering.md`

**Step 1: Write the failing test**

Not applicable.

**Step 2: Update docs**

Add a short note documenting: internal attachments remain inline; CDN only at HTTP boundary; html_edit reads inline payloads first.

**Step 3: Commit**

```bash
git add docs/plans/2026-01-19-attachment-layering.md
git commit -m "docs: note attachment layering boundary"
```

### Task 5: Full verification

**Step 1: Run targeted tests**

Run:
- `go test ./internal/tools/builtin -run TestHTMLEditUsesInlineAttachmentWithoutRemoteFetch -v`
- `go test ./internal/agent/domain -run TestAttachmentMutationsKeepInlinePayloads -v`
- `go test ./internal/server/http -run TestSSEExternalizesHTMLAttachments -v`

Expected: PASS.

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 3: Commit any fixes (if required)**

```bash
git add <files>
git commit -m "fix: stabilize attachment layering"
```
