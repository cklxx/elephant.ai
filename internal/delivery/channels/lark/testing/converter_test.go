package larktesting

import (
	"strings"
	"testing"
)

func TestTraceToScenarioBasic(t *testing.T) {
	entries := []TraceEntry{
		{
			Direction: "inbound",
			Type:      "message",
			SenderID:  "ou_real_user_1",
			ChatID:    "oc_real_chat_1",
			ChatType:  "p2p",
			MessageID: "om_real_msg_1",
			Content:   `{"text":"你好"}`,
		},
		{
			Direction: "outbound",
			Type:      "reply",
			Method:    "ReplyMessage",
			Content:   `{"text":"你好！有什么可以帮你的吗？"}`,
		},
	}

	scenario := TraceToScenario("basic_test", "basic conversation", entries, true)

	if scenario.Name != "basic_test" {
		t.Fatalf("expected name 'basic_test', got %q", scenario.Name)
	}
	if scenario.Description != "basic conversation" {
		t.Fatalf("expected description 'basic conversation', got %q", scenario.Description)
	}
	if len(scenario.Tags) != 1 || scenario.Tags[0] != "auto-generated" {
		t.Fatalf("expected tags [auto-generated], got %v", scenario.Tags)
	}
	if scenario.Setup.LLMMode != "mock" {
		t.Fatalf("expected llm_mode 'mock', got %q", scenario.Setup.LLMMode)
	}

	if len(scenario.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(scenario.Turns))
	}

	turn := scenario.Turns[0]
	if turn.Content != "你好" {
		t.Fatalf("expected content '你好', got %q", turn.Content)
	}
	if turn.ChatType != "p2p" {
		t.Fatalf("expected chat_type 'p2p', got %q", turn.ChatType)
	}
}

func TestTraceToScenarioAnonymization(t *testing.T) {
	entries := []TraceEntry{
		{
			Direction: "inbound",
			SenderID:  "ou_abc123",
			ChatID:    "oc_xyz789",
			MessageID: "om_msg_001",
			Content:   "hello",
		},
		{Direction: "outbound", Method: "ReplyMessage", Content: "world"},
		{
			Direction: "inbound",
			SenderID:  "ou_abc123", // same user
			ChatID:    "oc_xyz789", // same chat
			MessageID: "om_msg_002",
			Content:   "second message",
		},
		{Direction: "outbound", Method: "ReplyMessage", Content: "second reply"},
	}

	scenario := TraceToScenario("anon_test", "", entries, true)

	if len(scenario.Turns) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(scenario.Turns))
	}

	// Same real IDs should map to same anonymous IDs.
	if scenario.Turns[0].SenderID != scenario.Turns[1].SenderID {
		t.Fatalf("expected same anonymous sender ID across turns, got %q and %q",
			scenario.Turns[0].SenderID, scenario.Turns[1].SenderID)
	}
	if scenario.Turns[0].ChatID != scenario.Turns[1].ChatID {
		t.Fatalf("expected same anonymous chat ID across turns, got %q and %q",
			scenario.Turns[0].ChatID, scenario.Turns[1].ChatID)
	}

	// Anonymized IDs should not contain original IDs.
	if strings.Contains(scenario.Turns[0].SenderID, "abc123") {
		t.Fatalf("sender ID should be anonymized, got %q", scenario.Turns[0].SenderID)
	}
	if strings.Contains(scenario.Turns[0].ChatID, "xyz789") {
		t.Fatalf("chat ID should be anonymized, got %q", scenario.Turns[0].ChatID)
	}

	// Anonymized IDs should have correct prefixes.
	if !strings.HasPrefix(scenario.Turns[0].SenderID, "ou_user_") {
		t.Fatalf("expected sender prefix 'ou_user_', got %q", scenario.Turns[0].SenderID)
	}
	if !strings.HasPrefix(scenario.Turns[0].ChatID, "oc_chat_") {
		t.Fatalf("expected chat prefix 'oc_chat_', got %q", scenario.Turns[0].ChatID)
	}
}

func TestTraceToScenarioNoAnonymization(t *testing.T) {
	entries := []TraceEntry{
		{
			Direction: "inbound",
			SenderID:  "ou_real_id",
			ChatID:    "oc_real_chat",
			MessageID: "om_real_msg",
			Content:   "hello",
		},
		{Direction: "outbound", Method: "ReplyMessage", Content: "reply"},
	}

	scenario := TraceToScenario("raw_test", "", entries, false)

	if scenario.Turns[0].SenderID != "ou_real_id" {
		t.Fatalf("expected raw sender ID, got %q", scenario.Turns[0].SenderID)
	}
	if scenario.Turns[0].ChatID != "oc_real_chat" {
		t.Fatalf("expected raw chat ID, got %q", scenario.Turns[0].ChatID)
	}
}

func TestTraceToScenarioMockResponse(t *testing.T) {
	entries := []TraceEntry{
		{Direction: "inbound", Content: "question"},
		{Direction: "outbound", Method: "ReplyMessage", Content: `{"text":"the answer is 42"}`},
	}

	scenario := TraceToScenario("mock_test", "", entries, false)

	if len(scenario.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(scenario.Turns))
	}
	turn := scenario.Turns[0]
	if turn.MockResponse == nil {
		t.Fatal("expected mock response to be set")
	}
	if turn.MockResponse.Answer != "the answer is 42" {
		t.Fatalf("expected answer 'the answer is 42', got %q", turn.MockResponse.Answer)
	}
}

func TestTraceToScenarioAssertions(t *testing.T) {
	entries := []TraceEntry{
		{Direction: "inbound", Content: "hello"},
		{Direction: "outbound", Method: "AddReaction", Emoji: "SMILE"},
		{Direction: "outbound", Method: "ReplyMessage", Content: `{"text":"hello world response"}`},
		{Direction: "outbound", Method: "UploadImage"},
	}

	scenario := TraceToScenario("assert_test", "", entries, false)

	turn := scenario.Turns[0]
	assertions := turn.Assertions.Messenger

	if len(assertions) != 3 {
		t.Fatalf("expected 3 assertions, got %d", len(assertions))
	}

	// AddReaction assertion with emoji.
	found := false
	for _, a := range assertions {
		if a.Method == "AddReaction" && a.EmojiType == "SMILE" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected AddReaction assertion with SMILE emoji")
	}

	// ReplyMessage assertion with content keywords.
	found = false
	for _, a := range assertions {
		if a.Method == "ReplyMessage" && len(a.ContentContains) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected ReplyMessage assertion with content keywords")
	}

	// UploadImage assertion.
	found = false
	for _, a := range assertions {
		if a.Method == "UploadImage" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected UploadImage assertion")
	}
}

func TestTraceToScenarioMultiTurn(t *testing.T) {
	entries := []TraceEntry{
		{Direction: "inbound", Content: "first question"},
		{Direction: "outbound", Method: "ReplyMessage", Content: "first answer"},
		{Direction: "inbound", Content: "second question"},
		{Direction: "outbound", Method: "ReplyMessage", Content: "second answer"},
		{Direction: "inbound", Content: "third question"},
		// No outbound for last turn.
	}

	scenario := TraceToScenario("multi_turn", "", entries, false)

	if len(scenario.Turns) != 3 {
		t.Fatalf("expected 3 turns, got %d", len(scenario.Turns))
	}
	if scenario.Turns[0].Content != "first question" {
		t.Fatalf("turn 0: expected 'first question', got %q", scenario.Turns[0].Content)
	}
	if scenario.Turns[1].Content != "second question" {
		t.Fatalf("turn 1: expected 'second question', got %q", scenario.Turns[1].Content)
	}
	if scenario.Turns[2].Content != "third question" {
		t.Fatalf("turn 2: expected 'third question', got %q", scenario.Turns[2].Content)
	}
	// Third turn has no mock response.
	if scenario.Turns[2].MockResponse != nil {
		t.Fatal("turn 2: expected no mock response")
	}
}

func TestTraceToScenarioOrphanedOutbound(t *testing.T) {
	// Outbound entries before any inbound should be discarded.
	entries := []TraceEntry{
		{Direction: "outbound", Method: "AddReaction", Emoji: "SMILE"},
		{Direction: "inbound", Content: "real question"},
		{Direction: "outbound", Method: "ReplyMessage", Content: "answer"},
	}

	scenario := TraceToScenario("orphan_test", "", entries, false)

	if len(scenario.Turns) != 1 {
		t.Fatalf("expected 1 turn, got %d", len(scenario.Turns))
	}
	if scenario.Turns[0].Content != "real question" {
		t.Fatalf("expected 'real question', got %q", scenario.Turns[0].Content)
	}
}

func TestScenarioToYAML(t *testing.T) {
	scenario := &Scenario{
		Name:        "yaml_test",
		Description: "test serialization",
		Tags:        []string{"test"},
		Turns: []Turn{
			{
				SenderID: "user_1",
				ChatID:   "chat_1",
				Content:  "hello",
			},
		},
	}

	data, err := ScenarioToYAML(scenario)
	if err != nil {
		t.Fatalf("ScenarioToYAML failed: %v", err)
	}

	yaml := string(data)
	if !strings.Contains(yaml, "yaml_test") {
		t.Fatalf("expected 'yaml_test' in YAML output, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "hello") {
		t.Fatalf("expected 'hello' in YAML output, got:\n%s", yaml)
	}
}

func TestAnonymizeID(t *testing.T) {
	id1 := AnonymizeID("ou_user_real_123", "ou_user")
	id2 := AnonymizeID("ou_user_real_123", "ou_user")
	id3 := AnonymizeID("ou_user_real_456", "ou_user")

	// Deterministic: same input → same output.
	if id1 != id2 {
		t.Fatalf("expected deterministic output, got %q and %q", id1, id2)
	}

	// Different inputs → different outputs.
	if id1 == id3 {
		t.Fatalf("expected different outputs for different inputs, got %q", id1)
	}

	// Should have correct prefix.
	if !strings.HasPrefix(id1, "ou_user_") {
		t.Fatalf("expected prefix 'ou_user_', got %q", id1)
	}
}

func TestExtractTextFromContent(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`{"text":"hello world"}`, "hello world"},
		{`{"text":""}`, `{"text":""}`}, // Empty text → fallback to raw content.
		{`plain text`, "plain text"},
		{`{"invalid json`, `{"invalid json`},
		{`  {"text":"trimmed"}  `, "trimmed"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		got := extractTextFromContent(tt.input)
		if got != tt.expected {
			t.Errorf("extractTextFromContent(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestExtractKeywords(t *testing.T) {
	tests := []struct {
		text     string
		max      int
		expected int
	}{
		{"hello world foo bar baz", 3, 3},
		{"hello world", 5, 2},
		{"a b c", 3, 0},           // All single-char words filtered.
		{"", 3, 0},                // Empty text.
		{"hello hello hello", 3, 1}, // Dedup.
		{"Hello HELLO hello", 3, 1}, // Case-insensitive dedup.
	}

	for _, tt := range tests {
		keywords := extractKeywords(tt.text, tt.max)
		if len(keywords) != tt.expected {
			t.Errorf("extractKeywords(%q, %d) = %v (len %d), want len %d",
				tt.text, tt.max, keywords, len(keywords), tt.expected)
		}
	}
}

func TestDeduplicateAssertions(t *testing.T) {
	assertions := []MessengerAssertion{
		{Method: "ReplyMessage", ContentContains: []string{"hello"}},
		{Method: "ReplyMessage", ContentContains: []string{"world"}},
		{Method: "AddReaction", EmojiType: "SMILE"},
		{Method: "AddReaction", EmojiType: "SMILE"},
		{Method: "AddReaction", EmojiType: "THUMBSUP"},
	}

	result := deduplicateAssertions(assertions)

	// ReplyMessage deduped to 1, SMILE deduped to 1, THUMBSUP kept = 3 total.
	if len(result) != 3 {
		t.Fatalf("expected 3 deduplicated assertions, got %d: %+v", len(result), result)
	}

	methods := make(map[string]int)
	for _, a := range result {
		methods[a.Method+":"+a.EmojiType]++
	}
	if methods["ReplyMessage:"] != 1 {
		t.Fatal("expected 1 ReplyMessage assertion")
	}
	if methods["AddReaction:SMILE"] != 1 {
		t.Fatal("expected 1 AddReaction SMILE assertion")
	}
	if methods["AddReaction:THUMBSUP"] != 1 {
		t.Fatal("expected 1 AddReaction THUMBSUP assertion")
	}
}
