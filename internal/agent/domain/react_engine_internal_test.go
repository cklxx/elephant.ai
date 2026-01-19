package domain

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/agent/ports"
	materialports "alex/internal/materials/ports"
)

func TestCollectGeneratedAttachmentsIncludesAllGeneratedUpToIteration(t *testing.T) {
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"v1.png": {
				Name:      "v1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"v2.png": {
				Name:      "v2.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"user.png": {
				Name:      "user.png",
				MediaType: "image/png",
				Source:    "user_upload",
			},
		},
		AttachmentIterations: map[string]int{
			"v1.png": 1,
			"v2.png": 2,
		},
	}

	got := collectGeneratedAttachments(state, 2)
	if len(got) != 2 {
		t.Fatalf("expected two generated attachments, got %d", len(got))
	}
	if _, ok := got["v1.png"]; !ok {
		t.Fatalf("expected earlier iteration attachment to be present: %+v", got)
	}
	if _, ok := got["v2.png"]; !ok {
		t.Fatalf("expected latest iteration attachment to be present: %+v", got)
	}
	if _, ok := got["user.png"]; ok {
		t.Fatalf("did not expect user-uploaded attachment in result: %+v", got)
	}
}

func TestExpandPlaceholdersPrefersAttachmentURI(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
			URI:       "https://example.com/seed.png",
		},
	}
	args := map[string]any{"init_image": "[seed.png]"}
	state := &TaskState{Attachments: attachments}
	expanded := engine.expandPlaceholders(args, state)

	value, ok := expanded["init_image"].(string)
	if !ok {
		t.Fatalf("expected expanded init_image to be a string, got %T", expanded["init_image"])
	}
	if value != attachments["seed.png"].URI {
		t.Fatalf("expected URI reference, got %q", value)
	}
}

func TestExpandPlaceholdersResolvesGenericAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"seedream_0.png": {
				Name:      "seedream_0.png",
				MediaType: "image/png",
				Data:      "YmFzZTY0",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"seedream_0.png": 3,
		},
	}
	args := map[string]any{"init_image": "[image.png]"}
	expanded := engine.expandPlaceholders(args, state)

	value, ok := expanded["init_image"].(string)
	if !ok {
		t.Fatalf("expected expanded init_image to be a string, got %T", expanded["init_image"])
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI fallback, got %q", value)
	}
}

func TestEnsureAttachmentPlaceholdersUsesURIWhenAvailable(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
			URI:       "https://example.com/seed.png",
		},
	}

	result := ensureAttachmentPlaceholders("", attachments)
	if strings.Contains(result, "https://example.com/seed.png") {
		t.Fatalf("expected placeholders instead of direct URIs, got %q", result)
	}
	if !strings.Contains(result, "[seed.png]") {
		t.Fatalf("expected placeholder reference for attachment, got %q", result)
	}
}

func TestEnsureAttachmentPlaceholdersReplacesInlinePlaceholders(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			URI:       "https://example.com/seed.png",
		},
	}

	answer := "Latest render: [seed.png]"
	result := ensureAttachmentPlaceholders(answer, attachments)

	if !strings.Contains(result, "[seed.png]") {
		t.Fatalf("expected placeholder to remain for frontend replacement, got %q", result)
	}
	if strings.Contains(result, "Latest render: [seed.png]") {
		t.Fatalf("expected placeholder to move to the end, got %q", result)
	}
	if strings.Contains(result, "https://example.com/seed.png") {
		t.Fatalf("expected to avoid embedding attachment URI, got %q", result)
	}
}

func TestEnsureAttachmentPlaceholdersRemovesUnknownPlaceholders(t *testing.T) {
	answer := "Attachments: [missing.png]"
	result := ensureAttachmentPlaceholders(answer, nil)
	if strings.Contains(result, "[missing.png]") {
		t.Fatalf("expected missing placeholder to be removed, got %q", result)
	}
	if strings.TrimSpace(result) != "Attachments:" {
		t.Fatalf("expected surrounding text to remain, got %q", result)
	}
}

func TestLookupAttachmentByNameResolvesSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_0.png": {
				Name:      "doubao-seedream-3-0_nonce_0.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_nonce_0.png": 2,
		},
	}

	alias := "doubao-seedream-3-0_0.png"
	att, canonical, ok := engine.lookupAttachmentByName(alias, state)
	if !ok {
		t.Fatalf("expected alias %q to resolve", alias)
	}
	if canonical != "doubao-seedream-3-0_nonce_0.png" {
		t.Fatalf("expected canonical placeholder to include nonce, got %q", canonical)
	}
	if att.MediaType != "image/png" {
		t.Fatalf("expected attachment metadata to be preserved")
	}
}

func TestLookupAttachmentByNamePrefersLatestSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_old_1.png": {
				Name:      "doubao-seedream-3-0_old_1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"doubao-seedream-3-0_new_1.png": {
				Name:      "doubao-seedream-3-0_new_1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_old_1.png": 1,
			"doubao-seedream-3-0_new_1.png": 5,
		},
	}

	alias := "doubao-seedream-3-0_1.png"
	_, canonical, ok := engine.lookupAttachmentByName(alias, state)
	if !ok {
		t.Fatalf("expected alias %q to resolve", alias)
	}
	if canonical != "doubao-seedream-3-0_new_1.png" {
		t.Fatalf("expected latest iteration to win, got %q", canonical)
	}
}

func TestBuildContextTurnRecordClonesStructuredFields(t *testing.T) {
	ts := time.Date(2024, time.May, 10, 15, 30, 0, 0, time.UTC)
	plans := []ports.PlanNode{{
		ID:    "root",
		Title: "Root Plan",
		Children: []ports.PlanNode{{
			ID:    "child",
			Title: "Child",
		}},
	}}
	beliefs := []ports.Belief{{Statement: "Will finish", Confidence: 0.8, Source: "test"}}
	refs := []ports.KnowledgeReference{{
		ID:          "analysis",
		Description: "Auto",
		SOPRefs:     []string{"query"},
	}}
	state := &TaskState{
		SessionID:     "sess-123",
		Iterations:    4,
		Plans:         plans,
		Beliefs:       beliefs,
		KnowledgeRefs: refs,
		WorldState: map[string]any{
			"profile": map[string]any{"id": "local", "environment": "ci"},
		},
		WorldDiff: map[string]any{
			"iteration": 3,
		},
		FeedbackSignals: []ports.FeedbackSignal{{Kind: "tool_result", Message: "ok", Value: 1}},
	}
	messages := []ports.Message{{Role: "system", Content: "hello"}}
	record := buildContextTurnRecord(state, messages, ts, "summary")
	if record.SessionID != state.SessionID || record.TurnID != state.Iterations {
		t.Fatalf("expected identifiers to propagate: %+v", record)
	}
	if len(record.Plans) == 0 || len(record.Plans[0].Children) == 0 {
		t.Fatalf("expected nested plans in record: %+v", record.Plans)
	}
	if len(record.Beliefs) != len(beliefs) {
		t.Fatalf("expected belief copy, got %+v", record.Beliefs)
	}
	if len(record.KnowledgeRefs) == 0 || len(record.KnowledgeRefs[0].SOPRefs) == 0 {
		t.Fatalf("expected knowledge refs to copy nested slices: %+v", record.KnowledgeRefs)
	}
	profile := record.World["profile"].(map[string]any)
	if profile["id"] != "local" {
		t.Fatalf("expected world profile to propagate, got %+v", record.World)
	}
	if iteration, ok := record.Diff["iteration"].(int); !ok || iteration != 3 {
		t.Fatalf("expected diff iteration copy, got %+v", record.Diff)
	}
	if len(record.Feedback) != 1 || record.Feedback[0].Message != "ok" {
		t.Fatalf("expected feedback signals to copy, got %+v", record.Feedback)
	}
	state.Plans[0].Children[0].Title = "mutated"
	state.KnowledgeRefs[0].SOPRefs[0] = "mutated"
	state.WorldState["profile"].(map[string]any)["id"] = "mutated"
	state.WorldDiff["iteration"] = 99
	state.FeedbackSignals[0].Message = "changed"
	if record.Plans[0].Children[0].Title == "mutated" {
		t.Fatalf("expected plan deep copy to remain immutable")
	}
	if record.KnowledgeRefs[0].SOPRefs[0] == "mutated" {
		t.Fatalf("expected knowledge ref deep copy to remain immutable")
	}
	if record.World["profile"].(map[string]any)["id"] == "mutated" {
		t.Fatalf("expected world state copy to remain immutable")
	}
	if record.Diff["iteration"] == 99 {
		t.Fatalf("expected diff copy to remain immutable")
	}
	if record.Feedback[0].Message == "changed" {
		t.Fatalf("expected feedback copy to remain immutable")
	}
}

func TestSnapshotSummaryFromMessagesPrefersLatestContent(t *testing.T) {
	messages := []ports.Message{
		{Role: "system", Content: "boot"},
		{Role: "assistant", Content: "Plan ready."},
		{Role: "user", Content: "Need\nmultiline   help"},
	}
	got := snapshotSummaryFromMessages(messages)
	want := "User: Need multiline help"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSnapshotSummaryFromMessagesFallsBackToAssistant(t *testing.T) {
	messages := []ports.Message{
		{Role: "assistant", Content: "Latest reasoning"},
		{Role: "user", Content: "   "},
	}
	got := snapshotSummaryFromMessages(messages)
	if got != "Assistant: Latest reasoning" {
		t.Fatalf("expected assistant summary fallback, got %q", got)
	}
}

func TestSnapshotSummaryFromMessagesTruncatesLongContent(t *testing.T) {
	longContent := strings.Repeat("x", snapshotSummaryLimit+50)
	messages := []ports.Message{{Role: "user", Content: longContent}}
	got := snapshotSummaryFromMessages(messages)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) > snapshotSummaryLimit {
		t.Fatalf("expected summary within limit %d, got len=%d", snapshotSummaryLimit, len([]rune(got)))
	}
}

func TestResolveContentAttachmentsSupportsSeedreamAlias(t *testing.T) {
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_temp_2.png": {
				Name:      "doubao-seedream-3-0_temp_2.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_temp_2.png": 3,
		},
	}

	content := "Analyze [doubao-seedream-3-0_2.png] for details."
	resolved := resolveContentAttachments(content, state)
	if len(resolved) != 1 {
		t.Fatalf("expected one attachment, got %d", len(resolved))
	}
	if _, ok := resolved["doubao-seedream-3-0_2.png"]; !ok {
		t.Fatalf("expected resolved map to use alias key, got %+v", resolved)
	}
}

func TestExpandPlaceholdersResolvesPlainSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_0.png": {
				Name:      "doubao-seedream-3-0_nonce_0.png",
				MediaType: "image/png",
				Data:      "YmFzZTY0",
				Source:    "seedream",
			},
		},
	}
	args := map[string]any{"images": []any{"doubao-seedream-3-0_0.png"}}
	expanded := engine.expandPlaceholders(args, state)

	images, ok := expanded["images"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("expected expanded images slice, got %#v", expanded["images"])
	}
	value, ok := images[0].(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", images[0])
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI payload, got %q", value)
	}
	if !strings.HasSuffix(value, "YmFzZTY0") {
		t.Fatalf("expected inline base64 data, got %q", value)
	}
}

func TestObserveToolResultsPopulatesWorldDiffAndFeedback(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	results := []ToolResult{{
		CallID:  "web_fetch",
		Content: "Fetched updated OKR doc",
		Metadata: map[string]any{
			"reward": 0.75,
		},
	}}
	engine.observeToolResults(state, 2, results)
	if state.WorldDiff == nil {
		t.Fatalf("expected world diff to be populated")
	}
	if iter, ok := state.WorldDiff["iteration"].(int); !ok || iter != 2 {
		t.Fatalf("expected diff iteration 2, got %+v", state.WorldDiff)
	}
	entries, ok := state.WorldDiff["tool_results"].([]map[string]any)
	if !ok || len(entries) == 0 {
		t.Fatalf("expected tool result summary in diff, got %+v", state.WorldDiff)
	}
	if state.WorldState == nil || state.WorldState["last_updated_at"] == "" {
		t.Fatalf("expected world state last_updated_at set, got %+v", state.WorldState)
	}
	if len(state.FeedbackSignals) != 1 {
		t.Fatalf("expected feedback signal captured, got %+v", state.FeedbackSignals)
	}
	if state.FeedbackSignals[0].Value != 0.75 {
		t.Fatalf("expected reward to propagate, got %v", state.FeedbackSignals[0].Value)
	}
}

func TestExpandPlaceholdersResolvesCanonicalSeedreamNameWithoutBrackets(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_1.png": {
				Name:      "doubao-seedream-3-0_nonce_1.png",
				MediaType: "image/png",
				Data:      "ZGF0YTEyMw==",
				Source:    "seedream",
			},
		},
	}
	args := map[string]any{"images": []string{"doubao-seedream-3-0_nonce_1.png"}}
	expanded := engine.expandPlaceholders(args, state)

	images, ok := expanded["images"].([]string)
	if !ok || len(images) != 1 {
		t.Fatalf("expected expanded images slice, got %#v", expanded["images"])
	}
	if !strings.HasPrefix(images[0], "data:image/png;base64,") {
		t.Fatalf("expected canonical placeholder to convert to data URI, got %q", images[0])
	}
	if !strings.HasSuffix(images[0], "ZGF0YTEyMw==") {
		t.Fatalf("expected appended base64 payload, got %q", images[0])
	}
}

func TestExpandToolCallArgumentsSkipsArtifactNameExpansion(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"article.md": {
				Name:      "article.md",
				MediaType: "text/markdown",
				URI:       "https://example.com/article.md",
			},
			"note.md": {
				Name:      "note.md",
				MediaType: "text/markdown",
				URI:       "https://example.com/note.md",
			},
		},
	}

	expanded := engine.expandToolCallArguments("artifacts_list", map[string]any{"name": "article.md"}, state)
	if got, _ := expanded["name"].(string); got != "article.md" {
		t.Fatalf("expected artifacts_list name to remain literal, got %q", got)
	}

	expanded = engine.expandToolCallArguments("artifacts_list", map[string]any{"name": "[article.md]"}, state)
	if got, _ := expanded["name"].(string); got != "article.md" {
		t.Fatalf("expected artifacts_list placeholder to unwrap, got %q", got)
	}

	expanded = engine.expandToolCallArguments("artifacts_write", map[string]any{"name": "note.md"}, state)
	if got, _ := expanded["name"].(string); got != "note.md" {
		t.Fatalf("expected artifacts_write name to remain literal, got %q", got)
	}

	expanded = engine.expandToolCallArguments("artifacts_delete", map[string]any{"names": []string{"[note.md]", "article.md"}}, state)
	names, ok := expanded["names"].([]string)
	if !ok || len(names) != 2 {
		t.Fatalf("expected names slice, got %#v", expanded["names"])
	}
	if names[0] != "note.md" || names[1] != "article.md" {
		t.Fatalf("expected placeholder unwrapping, got %#v", names)
	}
}

func TestDecorateFinalResultOmitsUnreferencedAttachments(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"seed.png": {
				Name:      "seed.png",
				MediaType: "image/png",
				Source:    "seedream",
				Data:      "YmFzZTY0",
			},
		},
	}

	result := &TaskResult{
		Answer: "Task finished successfully.",
	}

	attachments := engine.decorateFinalResult(state, result)
	if attachments != nil {
		t.Fatalf("expected no attachments to be returned, got %+v", attachments)
	}
	if result.Answer != "Task finished successfully." {
		t.Fatalf("expected final answer to remain unchanged, got %q", result.Answer)
	}
}

func TestDecorateFinalResultIncludesReferencedAttachments(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"seed.png": {
				Name:      "seed.png",
				MediaType: "image/png",
				URI:       "https://example.com/seed.png",
			},
		},
	}

	result := &TaskResult{
		Answer: "Artifacts ready: [seed.png]",
	}

	attachments := engine.decorateFinalResult(state, result)
	if len(attachments) != 1 {
		t.Fatalf("expected single attachment to remain, got %d", len(attachments))
	}
	if _, ok := attachments["seed.png"]; !ok {
		t.Fatalf("expected placeholder to resolve to attachment, got %+v", attachments)
	}
	if !strings.Contains(result.Answer, "[seed.png]") {
		t.Fatalf("expected placeholder to remain for frontend rendering, got %q", result.Answer)
	}
	if strings.Contains(result.Answer, "https://example.com/seed.png") {
		t.Fatalf("expected to avoid embedding attachment URI, got %q", result.Answer)
	}
}

func TestRegisterMessageAttachmentsDetectsChanges(t *testing.T) {
	state := &TaskState{}
	msg := Message{
		Attachments: map[string]ports.Attachment{
			"diagram.png": {
				Name:        "diagram.png",
				Description: "Architecture plan",
			},
		},
	}

	if !registerMessageAttachments(state, msg) {
		t.Fatal("expected first registration to report changes")
	}
	if registerMessageAttachments(state, msg) {
		t.Fatal("expected duplicate registration to report no changes")
	}
}

type rewriteMigrator struct {
	called int32
}

func (m *rewriteMigrator) Normalize(ctx context.Context, req materialports.MigrationRequest) (map[string]ports.Attachment, error) {
	atomic.AddInt32(&m.called, 1)
	rewritten := make(map[string]ports.Attachment, len(req.Attachments))
	for key, att := range req.Attachments {
		att.Data = ""
		att.URI = "https://cdn.example.com/" + att.Name
		rewritten[key] = att
	}
	return rewritten, nil
}

func TestAttachmentMutationsKeepInlinePayloads(t *testing.T) {
	migrator := &rewriteMigrator{}
	engine := NewReactEngine(ReactEngineConfig{AttachmentMigrator: migrator})
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
	if got.Data == "" {
		t.Fatalf("expected inline payload to remain in agent state")
	}
	if atomic.LoadInt32(&migrator.called) != 0 {
		t.Fatalf("expected migrator to be skipped for agent state")
	}
}

func TestApplyToolAttachmentMutationsUpdatesState(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"old.txt": {Name: "old.txt", MediaType: "text/plain", Source: "user_upload"},
		},
		AttachmentIterations: map[string]int{"old.txt": 1},
		Iterations:           3,
	}

	call := ToolCall{ID: "call-1", Name: "artifacts_write"}
	metadata := map[string]any{
		"attachment_mutations": map[string]any{
			"replace": map[string]any{
				"note.md": map[string]any{
					"media_type": "text/markdown",
					"data":       "YmFzZSBjb250ZW50",
				},
			},
			"add": map[string]any{
				"thumb.png": map[string]any{
					"uri":        "https://cdn.example.com/thumb.png",
					"media_type": "image/png",
				},
			},
			"remove": []any{"old.txt"},
		},
	}

	attachments := map[string]ports.Attachment{
		"note.md": {MediaType: "text/markdown", Data: "YmFzZSBjb250ZW50"},
	}

	var mu sync.Mutex
	merged := engine.applyToolAttachmentMutations(context.Background(), state, call, attachments, metadata, &mu)

	if len(merged) != 2 {
		t.Fatalf("expected merged attachments to include replace and add, got %d", len(merged))
	}
	if _, ok := merged["thumb.png"]; !ok {
		t.Fatalf("expected merged attachments to include new thumbnail: %+v", merged)
	}
	if _, ok := state.Attachments["old.txt"]; ok {
		t.Fatalf("expected old attachment to be removed, got %+v", state.Attachments)
	}
	if iter, ok := state.AttachmentIterations["note.md"]; !ok || iter != 3 {
		t.Fatalf("expected iteration for note.md to be updated to 3, got %d", iter)
	}
	if source := state.Attachments["note.md"].Source; source != "artifacts_write" {
		t.Fatalf("expected default source to be tool name, got %q", source)
	}
}

func TestApplyToolAttachmentMutationsRemoveOnly(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"keep.txt": {Name: "keep.txt", URI: "https://cdn/keep.txt"},
			"drop.txt": {Name: "drop.txt", URI: "https://cdn/drop.txt"},
		},
		AttachmentIterations: map[string]int{"keep.txt": 1, "drop.txt": 2},
		Iterations:           3,
	}

	metadata := map[string]any{
		"attachment_mutations": map[string]any{
			"remove": []any{"drop.txt"},
		},
	}

	var mu sync.Mutex
	merged := engine.applyToolAttachmentMutations(context.Background(), state, ToolCall{Name: "artifacts_delete"}, nil, metadata, &mu)

	if len(merged) != 1 {
		t.Fatalf("expected merged attachments to keep remaining entries, got %+v", merged)
	}
	if _, ok := merged["drop.txt"]; ok {
		t.Fatalf("expected removed attachment to be pruned from merged set: %+v", merged)
	}
	if iter := state.AttachmentIterations["keep.txt"]; iter != 3 {
		t.Fatalf("expected surviving attachment to record current iteration, got %d", iter)
	}
}

func TestUpdateAttachmentCatalogMessageAppendsSystemNote(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		SessionID: "session-abc",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
		Attachments: map[string]ports.Attachment{
			"diagram.png": {
				Name:        "diagram.png",
				Description: "Architecture overview",
			},
		},
	}

	engine.updateAttachmentCatalogMessage(state)
	if len(state.Messages) != 2 {
		t.Fatalf("expected catalog message to be appended, got %d messages", len(state.Messages))
	}

	note := state.Messages[len(state.Messages)-1]
	if note.Role != "assistant" {
		t.Fatalf("expected catalog note to use assistant role, got %q", note.Role)
	}
	if note.Metadata == nil || note.Metadata[attachmentCatalogMetadataKey] != true {
		t.Fatalf("expected catalog metadata flag to be set, got %+v", note.Metadata)
	}
	if !strings.Contains(note.Content, "[diagram.png]") {
		t.Fatalf("expected catalog content to reference attachment placeholder, got %q", note.Content)
	}
	if note.Source != ports.MessageSourceAssistantReply {
		t.Fatalf("expected catalog note to use assistant reply source, got %q", note.Source)
	}
}

func TestUpdateAttachmentCatalogMessageRefreshesExistingNote(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"first.png": {
				Name: "first.png",
			},
		},
	}

	engine.updateAttachmentCatalogMessage(state)
	initialLen := len(state.Messages)
	state.Attachments["second.png"] = ports.Attachment{Name: "second.png", Description: "refined result"}

	engine.updateAttachmentCatalogMessage(state)
	if len(state.Messages) != initialLen+1 {
		t.Fatalf("expected catalog refresh to append a new note, got %d vs %d", len(state.Messages), initialLen)
	}

	note := state.Messages[len(state.Messages)-1]
	if !strings.Contains(note.Content, "[second.png]") {
		t.Fatalf("expected refreshed catalog to include new attachment, got %q", note.Content)
	}

	// Ensure catalog notes are append-only.
	count := 0
	for _, msg := range state.Messages {
		if msg.Metadata != nil && msg.Metadata[attachmentCatalogMetadataKey] == true {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected two catalog notes, found %d", count)
	}
}

func TestReactRuntimeAttachesReferencedTaskAttachmentsToUserMessage(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"diagram.png": {Name: "diagram.png", MediaType: "image/png", URI: "https://cdn/diagram.png"},
		},
	}

	task := "Analyze [diagram.png]"
	runtime := newReactRuntime(engine, context.Background(), task, state, Services{}, nil)
	runtime.prepareContext()

	var userMsg *Message
	for i := range state.Messages {
		msg := state.Messages[i]
		if msg.Source == ports.MessageSourceUserInput && msg.Content == task {
			userMsg = &state.Messages[i]
			break
		}
	}
	if userMsg == nil {
		t.Fatalf("expected user message to be present, got %d messages", len(state.Messages))
	}
	if userMsg.Attachments == nil {
		t.Fatalf("expected referenced attachments to be attached to user message")
	}
	if _, ok := userMsg.Attachments["diagram.png"]; !ok {
		t.Fatalf("expected user message to include diagram.png attachment, got %#v", userMsg.Attachments)
	}
}

func TestNormalizeMessageHistoryAttachmentsKeepsInlinePayloads(t *testing.T) {
	migrator := &captureMigrator{}
	engine := NewReactEngine(ReactEngineConfig{AttachmentMigrator: migrator})
	state := &TaskState{
		SessionID: "session-1",
		TaskID:    "task-1",
		Messages: []Message{
			{
				Role:   "user",
				Source: ports.MessageSourceUserHistory,
				Attachments: map[string]ports.Attachment{
					"[seed.png]": {Name: "seed.png", MediaType: "image/png", Data: "YmFzZTY0"},
				},
			},
			{
				Role:       "tool",
				ToolCallID: "call-123",
				Source:     ports.MessageSourceToolResult,
				Attachments: map[string]ports.Attachment{
					"[tool.png]": {Name: "tool.png", MediaType: "image/png", Data: "aW1hZ2U="},
				},
			},
		},
	}

	engine.normalizeMessageHistoryAttachments(context.Background(), state)

	if len(migrator.requests) != 0 {
		t.Fatalf("expected migrator to be skipped, got %d requests", len(migrator.requests))
	}

	for idx, msg := range state.Messages {
		for key, att := range msg.Attachments {
			if att.Data == "" {
				t.Fatalf("message %d attachment %s missing inline data", idx, key)
			}
			if att.URI != "" {
				t.Fatalf("message %d attachment %s should not be externalized", idx, key)
			}
		}
	}
}

func TestCompactToolCallHistoryReplacesLargeArguments(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	largeContent := strings.Repeat("a", toolArgHistoryInlineLimit+8)

	state := &TaskState{
		Messages: []Message{{
			Role: "assistant",
			ToolCalls: []ToolCall{{
				ID:   "call-1",
				Name: "file_write",
				Arguments: map[string]any{
					"path":    "note.txt",
					"content": largeContent,
				},
			}},
		}},
	}

	results := []ToolResult{{
		CallID: "call-1",
		Metadata: map[string]any{
			"path": "/workspace/note.txt",
		},
	}}

	engine.compactToolCallHistory(state, results)

	args := state.Messages[0].ToolCalls[0].Arguments
	contentRef, ok := args["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected content argument to be compacted, got %#v", args["content"])
	}
	if args["path"] != "note.txt" {
		t.Fatalf("expected path argument to remain, got %#v", args["path"])
	}
	if contentRef["content_ref"] != "/workspace/note.txt" {
		t.Fatalf("expected content_ref to point to resolved path, got %#v", contentRef["content_ref"])
	}
	if contentRef["content_len"] != len(largeContent) {
		t.Fatalf("expected content_len %d, got %#v", len(largeContent), contentRef["content_len"])
	}
	if hash, ok := contentRef["content_sha256"].(string); !ok || hash == "" {
		t.Fatalf("expected content_sha256 to be populated, got %#v", contentRef["content_sha256"])
	}
}

type captureMigrator struct {
	requests []materialports.MigrationRequest
}

func (m *captureMigrator) Normalize(ctx context.Context, req materialports.MigrationRequest) (map[string]ports.Attachment, error) {
	m.requests = append(m.requests, req)
	result := make(map[string]ports.Attachment, len(req.Attachments))
	for key, att := range req.Attachments {
		att.URI = fmt.Sprintf("https://cdn/%s", att.Name)
		att.Data = ""
		result[key] = att
	}
	return result, nil
}

var _ materialports.Migrator = (*captureMigrator)(nil)

func TestEnsureSystemPromptMessagePrependsWhenMissing(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		SystemPrompt: "Stay focused.",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	}

	engine.ensureSystemPromptMessage(state)

	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(state.Messages))
	}
	if state.Messages[0].Role != "system" {
		t.Fatalf("expected system prompt at index 0, got role %q", state.Messages[0].Role)
	}
	if state.Messages[0].Content != "Stay focused." {
		t.Fatalf("unexpected system prompt content: %q", state.Messages[0].Content)
	}
}

func TestEnsureSystemPromptMessageNoDuplicate(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		SystemPrompt: "Follow the plan.",
		Messages: []Message{
			{Role: "system", Content: "Follow the plan.", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "hello"},
		},
	}

	engine.ensureSystemPromptMessage(state)

	if len(state.Messages) != 2 {
		t.Fatalf("expected message count to remain 2, got %d", len(state.Messages))
	}
	if state.Messages[0].Content != "Follow the plan." {
		t.Fatalf("expected existing system prompt to remain first, got %q", state.Messages[0].Content)
	}
}

func TestEnsureSystemPromptMessageUpdatesExistingWhenContentDiffers(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		SystemPrompt: "Follow the updated framework.",
		Messages: []Message{
			{Role: "system", Content: "Follow the legacy framework.", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "hello"},
		},
	}

	engine.ensureSystemPromptMessage(state)

	if len(state.Messages) != 2 {
		t.Fatalf("expected message count to remain 2, got %d", len(state.Messages))
	}
	if got := state.Messages[0].Content; got != state.SystemPrompt {
		t.Fatalf("expected system prompt to update, got %q", got)
	}
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected system prompt source to be preserved, got %q", state.Messages[0].Source)
	}
}

func TestEnsureSystemPromptMessageHandlesLegacySystemRole(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		SystemPrompt: "Follow the structured context.",
		Messages: []Message{
			{Role: "system", Content: "Follow the legacy context."},
			{Role: "user", Content: "hello"},
		},
	}

	engine.ensureSystemPromptMessage(state)

	if len(state.Messages) != 2 {
		t.Fatalf("expected message count to remain 2, got %d", len(state.Messages))
	}
	if got := state.Messages[0].Content; got != state.SystemPrompt {
		t.Fatalf("expected legacy system role content to update, got %q", got)
	}
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected legacy system role to adopt system prompt source, got %q", state.Messages[0].Source)
	}
}

func TestEnsureSystemPromptMessageLeavesHistorySummariesIntact(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	historySummary := Message{Role: "system", Content: "Earlier rounds summarized", Source: ports.MessageSourceUserHistory}
	state := &TaskState{
		SystemPrompt: "Follow the structured context.",
		Messages: []Message{
			historySummary,
			{Role: "user", Content: "hello"},
		},
	}

	engine.ensureSystemPromptMessage(state)

	if len(state.Messages) != 3 {
		t.Fatalf("expected system prompt to be prepended without dropping history, got %d messages", len(state.Messages))
	}
	if state.Messages[0].Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected first message to be the system prompt, got source %q", state.Messages[0].Source)
	}
	if state.Messages[1].Content != historySummary.Content {
		t.Fatalf("expected history summary to remain after system prompt, got %q", state.Messages[1].Content)
	}
	if state.Messages[1].Source != ports.MessageSourceUserHistory {
		t.Fatalf("expected history summary to retain source, got %q", state.Messages[1].Source)
	}
}

func TestAttachmentReferenceValuePrefersURI(t *testing.T) {
	att := ports.Attachment{
		Name:      "diagram.png",
		MediaType: "image/png",
		Data:      "ZGF0YQo=",
		URI:       "https://example.com/diagram.png",
	}
	if value := attachmentReferenceValue(att); value != att.URI {
		t.Fatalf("expected URI to be preferred, got %q", value)
	}
}

func TestAttachmentReferenceValueFallsBackToData(t *testing.T) {
	att := ports.Attachment{
		Name:      "diagram.png",
		MediaType: "image/png",
		Data:      "ZGF0YQo=",
	}
	value := attachmentReferenceValue(att)
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI fallback, got %q", value)
	}
}

func TestAppendGoalPlanReminderWhenDistanceExceeded(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	planText := strings.Repeat("plan ", goalPlanPromptDistanceThreshold)
	calls := []ToolCall{{
		Name: "plan",
		Arguments: map[string]any{
			"overall_goal_ui": "ship it",
			"internal_plan":   planText,
		},
	}}
	results := []ToolResult{{
		CallID:  "call-1",
		Content: "ship it",
		Metadata: map[string]any{
			"internal_plan": planText,
		},
	}}
	engine.updateGoalPlanPrompts(state, calls, results)
	messages := engine.buildToolMessages(results)
	updated := engine.appendGoalPlanReminder(state, messages)
	if len(updated) != 1 {
		t.Fatalf("expected a single tool message, got %d", len(updated))
	}
	if !strings.Contains(updated[0].Content, "<system-reminder>") {
		t.Fatalf("expected reminder to be appended, got %q", updated[0].Content)
	}
	if !strings.Contains(updated[0].Content, "Goal: ship it") {
		t.Fatalf("expected goal to appear in reminder, got %q", updated[0].Content)
	}
	if !strings.Contains(updated[0].Content, "Plan:") {
		t.Fatalf("expected plan to appear in reminder, got %q", updated[0].Content)
	}
}

func TestAppendGoalPlanReminderSkippedWhenDistanceSmall(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	calls := []ToolCall{{
		Name: "plan",
		Arguments: map[string]any{
			"overall_goal_ui": "stay close",
			"internal_plan":   "stay close plan",
		},
	}}
	results := []ToolResult{{
		CallID:  "call-1",
		Content: "stay close",
		Metadata: map[string]any{
			"internal_plan": "stay close plan",
		},
	}}
	engine.updateGoalPlanPrompts(state, calls, results)
	messages := engine.buildToolMessages(results)
	updated := engine.appendGoalPlanReminder(state, messages)
	if len(updated) != 1 {
		t.Fatalf("expected a single tool message, got %d", len(updated))
	}
	if strings.Contains(updated[0].Content, "<system-reminder>") {
		t.Fatalf("did not expect reminder when distance small, got %q", updated[0].Content)
	}
}
