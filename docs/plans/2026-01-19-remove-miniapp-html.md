# Remove MiniAppHTML Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Delete the MiniAppHTML tool and document HTML generation via artifacts_write.

**Architecture:** Remove the tool implementation/registration/tests and update artifacts tool definitions to explicitly mention HTML. Clean up docs that referenced miniapp_html.

**Tech Stack:** Go (toolregistry, builtin tools), tests, docs.

### Task 1: Add failing test that artifacts_write definition mentions HTML

**Files:**
- Modify: `internal/tools/builtin/artifacts_test.go`

**Step 1: Write the failing test**

```go
func TestArtifactsWriteDefinitionMentionsHTML(t *testing.T) {
	def := NewArtifactsWrite().Definition()
	if !strings.Contains(strings.ToLower(def.Description), "html") {
		t.Fatalf("expected artifacts_write description to mention html")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/builtin -run TestArtifactsWriteDefinitionMentionsHTML -v`
Expected: FAIL (description does not yet mention HTML).

**Step 3: Commit test**

```bash
git add internal/tools/builtin/artifacts_test.go
git commit -m "test: assert artifacts_write mentions html"
```

### Task 2: Update artifacts tool definition to mention HTML support

**Files:**
- Modify: `internal/tools/builtin/artifacts.go`

**Step 1: Update description**

Change artifacts_write description and/or property descriptions to mention `text/html` and `format: html`.

**Step 2: Run test to verify it passes**

Run: `go test ./internal/tools/builtin -run TestArtifactsWriteDefinitionMentionsHTML -v`
Expected: PASS.

**Step 3: Commit**

```bash
git add internal/tools/builtin/artifacts.go
git commit -m "docs: mention html generation in artifacts_write"
```

### Task 3: Remove MiniAppHTML tool implementation and registry wiring

**Files:**
- Delete: `internal/tools/builtin/miniapp_html.go`
- Modify: `internal/toolregistry/registry.go`

**Step 1: Remove registry wiring**

Delete the `miniapp_html` block from the tool registry.

**Step 2: Delete tool implementation**

Remove `internal/tools/builtin/miniapp_html.go`.

**Step 3: Commit**

```bash
git add internal/toolregistry/registry.go internal/tools/builtin/miniapp_html.go
git commit -m "remove: drop miniapp_html tool"
```

### Task 4: Remove MiniAppHTML tests and doc references

**Files:**
- Modify: `internal/toolregistry/registry_test.go`
- Modify: `docs/plans/2026-01-19-attachment-layering.md`

**Step 1: Remove MiniAppHTML test + unused helpers**

Delete `TestMiniAppHTMLUsesConfiguredLLM` and unused helper types if they become dead code.

**Step 2: Update docs**

Replace the miniapp_html mention in the attachment layering plan with a generic HTML artifact note.

**Step 3: Commit**

```bash
git add internal/toolregistry/registry_test.go docs/plans/2026-01-19-attachment-layering.md
git commit -m "docs: remove miniapp_html references"
```

### Task 5: Full verification

**Step 1: Run targeted tests**

Run:
- `go test ./internal/tools/builtin -run TestArtifactsWriteDefinitionMentionsHTML -v`
- `go test ./internal/toolregistry -run TestToolDefinitionsArrayItems -v`

Expected: PASS.

**Step 2: Run full test suite**

Run: `go test ./...`
Expected: PASS.

**Step 3: Commit any fixes (if required)**

```bash
git add <files>
git commit -m "fix: stabilize miniapp_html removal"
```
