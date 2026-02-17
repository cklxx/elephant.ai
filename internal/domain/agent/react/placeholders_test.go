package react

import (
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// --- extractPlaceholderName ---

func TestExtractPlaceholderName_Valid(t *testing.T) {
	name, ok := extractPlaceholderName("[seed.png]")
	if !ok || name != "seed.png" {
		t.Fatalf("expected seed.png/true, got %q/%v", name, ok)
	}
}

func TestExtractPlaceholderName_WithWhitespace(t *testing.T) {
	name, ok := extractPlaceholderName("  [seed.png]  ")
	if !ok || name != "seed.png" {
		t.Fatalf("expected seed.png/true, got %q/%v", name, ok)
	}
}

func TestExtractPlaceholderName_EmptyBrackets(t *testing.T) {
	_, ok := extractPlaceholderName("[]")
	if ok {
		t.Fatal("expected false for empty brackets")
	}
}

func TestExtractPlaceholderName_NoBrackets(t *testing.T) {
	_, ok := extractPlaceholderName("seed.png")
	if ok {
		t.Fatal("expected false without brackets")
	}
}

func TestExtractPlaceholderName_TooShort(t *testing.T) {
	_, ok := extractPlaceholderName("ab")
	if ok {
		t.Fatal("expected false for string shorter than 3")
	}
}

func TestExtractPlaceholderName_WhitespaceInside(t *testing.T) {
	_, ok := extractPlaceholderName("[  ]")
	if ok {
		t.Fatal("expected false for whitespace-only content")
	}
}

// --- normalizeImportantNotes ---

type staticClock struct {
	t time.Time
}

func (c staticClock) Now() time.Time { return c.t }

func TestNormalizeImportantNotes_SliceOfNotes(t *testing.T) {
	input := []ports.ImportantNote{
		{ID: "n1", Content: "remember this"},
	}
	notes := normalizeImportantNotes(input, nil)
	if len(notes) != 1 || notes[0].ID != "n1" {
		t.Fatalf("expected note copied, got %+v", notes)
	}
}

func TestNormalizeImportantNotes_SliceOfMaps(t *testing.T) {
	clock := staticClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	input := []any{
		map[string]any{
			"id":      "n2",
			"content": "important fact",
			"source":  "web_search",
			"tags":    []any{"tag1", "tag2"},
		},
	}
	notes := normalizeImportantNotes(input, clock)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	n := notes[0]
	if n.ID != "n2" || n.Content != "important fact" || n.Source != "web_search" {
		t.Fatalf("unexpected note fields: %+v", n)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "tag1" {
		t.Fatalf("expected 2 tags, got %+v", n.Tags)
	}
	if n.CreatedAt.IsZero() {
		t.Fatal("expected clock time to be set")
	}
}

func TestNormalizeImportantNotes_SingleMap(t *testing.T) {
	input := map[string]any{
		"content": "a single note",
	}
	notes := normalizeImportantNotes(input, nil)
	if len(notes) != 1 || notes[0].Content != "a single note" {
		t.Fatalf("expected single note, got %+v", notes)
	}
}

func TestNormalizeImportantNotes_EmptyContentSkipped(t *testing.T) {
	input := map[string]any{
		"content": "",
	}
	notes := normalizeImportantNotes(input, nil)
	if len(notes) != 0 {
		t.Fatalf("expected empty content to be skipped, got %+v", notes)
	}
}

func TestNormalizeImportantNotes_NilInput(t *testing.T) {
	notes := normalizeImportantNotes(nil, nil)
	if notes != nil {
		t.Fatalf("expected nil for nil input, got %+v", notes)
	}
}

func TestNormalizeImportantNotes_MixedSlice(t *testing.T) {
	input := []any{
		ports.ImportantNote{ID: "n1", Content: "typed note"},
		map[string]any{"content": "map note"},
		42, // should be ignored
	}
	notes := normalizeImportantNotes(input, nil)
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d", len(notes))
	}
}

// --- parseImportantNoteMap ---

func TestParseImportantNoteMap_FullFields(t *testing.T) {
	ts := time.Date(2026, 2, 1, 12, 0, 0, 0, time.UTC)
	raw := map[string]any{
		"id":         "note-1",
		"content":    "critical info",
		"source":     "tool_x",
		"tags":       []any{"urgent", "security"},
		"created_at": ts,
	}
	note := parseImportantNoteMap(raw, nil)
	if note.ID != "note-1" || note.Content != "critical info" || note.Source != "tool_x" {
		t.Fatalf("unexpected note: %+v", note)
	}
	if len(note.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %+v", note.Tags)
	}
	if !note.CreatedAt.Equal(ts) {
		t.Fatalf("expected timestamp %v, got %v", ts, note.CreatedAt)
	}
}

func TestParseImportantNoteMap_CreatedAtString(t *testing.T) {
	raw := map[string]any{
		"content":    "test",
		"created_at": "2026-02-01T12:00:00Z",
	}
	note := parseImportantNoteMap(raw, nil)
	if note.CreatedAt.IsZero() {
		t.Fatal("expected timestamp parsed from string")
	}
}

func TestParseImportantNoteMap_InvalidCreatedAtUsesClockFallback(t *testing.T) {
	clock := staticClock{t: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)}
	raw := map[string]any{
		"content":    "test",
		"created_at": "not-a-date",
	}
	note := parseImportantNoteMap(raw, clock)
	if !note.CreatedAt.Equal(clock.t) {
		t.Fatalf("expected clock fallback, got %v", note.CreatedAt)
	}
}

func TestParseImportantNoteMap_TagsWithWhitespace(t *testing.T) {
	raw := map[string]any{
		"content": "test",
		"tags":    []any{"  valid  ", " ", ""},
	}
	note := parseImportantNoteMap(raw, nil)
	if len(note.Tags) != 1 || note.Tags[0] != "valid" {
		t.Fatalf("expected only trimmed non-empty tags, got %+v", note.Tags)
	}
}

// --- mergeImportantNotes (via ReactEngine) ---

func TestMergeImportantNotes_AddsToEmptyState(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	notes := []ports.ImportantNote{
		{ID: "n1", Content: "first"},
	}
	engine.mergeImportantNotes(state, notes)
	if len(state.Important) != 1 {
		t.Fatalf("expected 1 note, got %d", len(state.Important))
	}
}

func TestMergeImportantNotes_OverwritesByID(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Important: map[string]ports.ImportantNote{
			"n1": {ID: "n1", Content: "old"},
		},
	}
	notes := []ports.ImportantNote{
		{ID: "n1", Content: "updated"},
	}
	engine.mergeImportantNotes(state, notes)
	if state.Important["n1"].Content != "updated" {
		t.Fatalf("expected overwrite, got %q", state.Important["n1"].Content)
	}
}

func TestMergeImportantNotes_NilState(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	engine.mergeImportantNotes(nil, []ports.ImportantNote{{ID: "n1"}}) // should not panic
}

func TestMergeImportantNotes_EmptyNotes(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	engine.mergeImportantNotes(state, nil) // should not panic
	if state.Important != nil {
		t.Fatalf("expected nil important map, got %+v", state.Important)
	}
}

// --- extractImportantNotes (via ReactEngine) ---

func TestExtractImportantNotes_NoMetadata(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	notes := engine.extractImportantNotes(ToolCall{Name: "web_search"}, nil)
	if notes != nil {
		t.Fatalf("expected nil, got %+v", notes)
	}
}

func TestExtractImportantNotes_NoImportantNotesKey(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	notes := engine.extractImportantNotes(ToolCall{Name: "web_search"}, map[string]any{"other": "data"})
	if notes != nil {
		t.Fatalf("expected nil, got %+v", notes)
	}
}

func TestExtractImportantNotes_EnrichesNotes(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	metadata := map[string]any{
		"important_notes": []any{
			map[string]any{"content": "remember this"},
		},
	}
	notes := engine.extractImportantNotes(ToolCall{Name: "web_search"}, metadata)
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Source != "web_search" {
		t.Fatalf("expected source from tool call, got %q", notes[0].Source)
	}
	if notes[0].ID == "" {
		t.Fatal("expected ID to be generated")
	}
}

func TestExtractImportantNotes_SkipsEmptyContent(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	metadata := map[string]any{
		"important_notes": []any{
			map[string]any{"content": "  "},
		},
	}
	notes := engine.extractImportantNotes(ToolCall{Name: "tool"}, metadata)
	if len(notes) != 0 {
		t.Fatalf("expected empty content skipped, got %+v", notes)
	}
}

// --- unwrapAttachmentPlaceholderValue ---

func TestUnwrapAttachmentPlaceholderValue_String(t *testing.T) {
	got := unwrapAttachmentPlaceholderValue("[seed.png]")
	if got != "seed.png" {
		t.Fatalf("expected unwrapped name, got %v", got)
	}
}

func TestUnwrapAttachmentPlaceholderValue_PlainString(t *testing.T) {
	got := unwrapAttachmentPlaceholderValue("no_brackets.png")
	if got != "no_brackets.png" {
		t.Fatalf("expected unchanged string, got %v", got)
	}
}

func TestUnwrapAttachmentPlaceholderValue_SliceAny(t *testing.T) {
	input := []any{"[a.png]", "b.png"}
	got := unwrapAttachmentPlaceholderValue(input)
	result, ok := got.([]any)
	if !ok || len(result) != 2 {
		t.Fatalf("expected slice of 2, got %v", got)
	}
	if result[0] != "a.png" {
		t.Fatalf("expected unwrapped first element, got %v", result[0])
	}
	if result[1] != "b.png" {
		t.Fatalf("expected unchanged second element, got %v", result[1])
	}
}

func TestUnwrapAttachmentPlaceholderValue_SliceString(t *testing.T) {
	input := []string{"[a.png]", "b.png"}
	got := unwrapAttachmentPlaceholderValue(input)
	result, ok := got.([]string)
	if !ok || len(result) != 2 {
		t.Fatalf("expected string slice of 2, got %v", got)
	}
	if result[0] != "a.png" {
		t.Fatalf("expected unwrapped first element, got %v", result[0])
	}
}

func TestUnwrapAttachmentPlaceholderValue_NonStringPassthrough(t *testing.T) {
	got := unwrapAttachmentPlaceholderValue(42)
	if got != 42 {
		t.Fatalf("expected passthrough, got %v", got)
	}
}

// --- splitMessagesForLLM ---

func TestSplitMessagesForLLM_Empty(t *testing.T) {
	filtered, excluded := splitMessagesForLLM(nil)
	if filtered != nil || excluded != nil {
		t.Fatalf("expected nil/nil, got %v/%v", filtered, excluded)
	}
}

func TestSplitMessagesForLLM_FiltersDebugAndEval(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "boot", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "hello", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "debug info", Source: ports.MessageSourceDebug},
		{Role: "assistant", Content: "eval check", Source: ports.MessageSourceEvaluation},
		{Role: "assistant", Content: "real reply", Source: ports.MessageSourceAssistantReply},
	}

	filtered, excluded := splitMessagesForLLM(messages)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered messages, got %d", len(filtered))
	}
	if len(excluded) != 2 {
		t.Fatalf("expected 2 excluded messages, got %d", len(excluded))
	}
	// Verify filtered contains the right messages
	for _, msg := range filtered {
		if msg.Source == ports.MessageSourceDebug || msg.Source == ports.MessageSourceEvaluation {
			t.Fatalf("filtered should not contain debug/eval messages, found %q", msg.Source)
		}
	}
}

func TestSplitMessagesForLLM_ClonesToolCalls(t *testing.T) {
	messages := []Message{{
		Role: "assistant",
		ToolCalls: []ToolCall{{
			ID:   "c1",
			Name: "web_search",
		}},
	}}

	filtered, _ := splitMessagesForLLM(messages)
	if len(filtered) != 1 || len(filtered[0].ToolCalls) != 1 {
		t.Fatal("expected tool calls cloned")
	}
	// Mutate original
	messages[0].ToolCalls[0].Name = "mutated"
	if filtered[0].ToolCalls[0].Name == "mutated" {
		t.Fatal("expected deep clone - mutation should not propagate")
	}
}

// --- appendFeedbackSignals cap ---

func TestAppendFeedbackSignals_CapsAtMax(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Cognitive: &agent.CognitiveExtension{
			FeedbackSignals: make([]agent.FeedbackSignal, maxFeedbackSignals),
		},
	}
	results := []ToolResult{{CallID: "call-new", Content: "new result"}}
	engine.appendFeedbackSignals(state, results)
	if len(state.Cognitive.FeedbackSignals) != maxFeedbackSignals {
		t.Fatalf("expected capped at %d, got %d", maxFeedbackSignals, len(state.Cognitive.FeedbackSignals))
	}
	// Latest signal should be the new one
	last := state.Cognitive.FeedbackSignals[len(state.Cognitive.FeedbackSignals)-1]
	if last.Kind != "tool_result" {
		t.Fatalf("expected latest signal to be tool_result, got %q", last.Kind)
	}
}

func TestAppendFeedbackSignals_NilState(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	engine.appendFeedbackSignals(nil, []ToolResult{{CallID: "c1"}}) // should not panic
}

func TestAppendFeedbackSignals_EmptyResults(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{}
	engine.appendFeedbackSignals(state, nil) // should not panic
}
