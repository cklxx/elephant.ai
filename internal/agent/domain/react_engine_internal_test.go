package domain

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	"alex/internal/materials/legacy"
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
	if !strings.Contains(result, "https://example.com/seed.png") {
		t.Fatalf("expected markdown to reference attachment URI, got %q", result)
	}
	if strings.Contains(result, "data:image/png") {
		t.Fatalf("expected markdown to avoid embedding base64 data, got %q", result)
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

	if strings.Contains(result, " [seed.png]") {
		t.Fatalf("expected placeholder brackets to be removed from plain text, got %q", result)
	}
	if !strings.Contains(result, "![seed.png](https://example.com/seed.png)") {
		t.Fatalf("expected inline image markdown, got %q", result)
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
			"profile": map[string]any{"id": "sandbox", "environment": "ci"},
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
	if profile["id"] != "sandbox" {
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
	if !strings.Contains(result.Answer, "![seed.png](https://example.com/seed.png)") {
		t.Fatalf("expected placeholder to be converted to markdown reference, got %q", result.Answer)
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
	if note.Role != "system" {
		t.Fatalf("expected catalog note to use system role, got %q", note.Role)
	}
	if note.Metadata == nil || note.Metadata[attachmentCatalogMetadataKey] != true {
		t.Fatalf("expected catalog metadata flag to be set, got %+v", note.Metadata)
	}
	if !strings.Contains(note.Content, "[diagram.png]") {
		t.Fatalf("expected catalog content to reference attachment placeholder, got %q", note.Content)
	}
	if !strings.Contains(note.Content, "/workspace/.alex/sessions/session-abc/attachments") {
		t.Fatalf("expected catalog content to mention sandbox path, got %q", note.Content)
	}
	if note.Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected catalog note to use system prompt source, got %q", note.Source)
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
	if len(state.Messages) != initialLen {
		t.Fatalf("expected catalog refresh to keep message count stable, got %d vs %d", len(state.Messages), initialLen)
	}

	note := state.Messages[len(state.Messages)-1]
	if !strings.Contains(note.Content, "[second.png]") {
		t.Fatalf("expected refreshed catalog to include new attachment, got %q", note.Content)
	}

	// Ensure only one catalog message exists.
	count := 0
	for _, msg := range state.Messages {
		if msg.Metadata != nil && msg.Metadata[attachmentCatalogMetadataKey] == true {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected a single catalog note, found %d", count)
	}
}

func TestNormalizeMessageHistoryAttachmentsMigratesInlinePayloads(t *testing.T) {
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

	if len(migrator.requests) != 2 {
		t.Fatalf("expected two migration requests, got %d", len(migrator.requests))
	}
	if got := migrator.requests[0].Status; got != materialapi.MaterialStatusInput {
		t.Fatalf("expected user message to use input status, got %v", got)
	}
	if got := migrator.requests[1].Context.ToolCallID; got != "call-123" {
		t.Fatalf("expected tool message to preserve tool call id, got %q", got)
	}

	for idx, msg := range state.Messages {
		for key, att := range msg.Attachments {
			if att.Data != "" {
				t.Fatalf("message %d attachment %s still has inline data", idx, key)
			}
			if att.URI == "" {
				t.Fatalf("message %d attachment %s missing CDN URI", idx, key)
			}
		}
	}
}

type captureMigrator struct {
	requests []legacy.MigrationRequest
}

func (m *captureMigrator) Normalize(ctx context.Context, req legacy.MigrationRequest) (map[string]ports.Attachment, error) {
	m.requests = append(m.requests, req)
	result := make(map[string]ports.Attachment, len(req.Attachments))
	for key, att := range req.Attachments {
		att.URI = fmt.Sprintf("https://cdn/%s", att.Name)
		att.Data = ""
		result[key] = att
	}
	return result, nil
}

var _ legacy.Migrator = (*captureMigrator)(nil)

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
