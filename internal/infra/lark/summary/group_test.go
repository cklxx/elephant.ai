package summary

import (
	"strings"
	"testing"
	"time"
)

// base is a fixed reference time used across tests.
var base = time.Date(2026, 2, 2, 10, 0, 0, 0, time.UTC)

// helpers to build test messages quickly.

func msg(sender, content string, offset time.Duration) GroupMessage {
	return GroupMessage{
		SenderID:   "id_" + sender,
		SenderName: sender,
		Content:    content,
		Timestamp:  base.Add(offset),
		MsgType:    "text",
	}
}

func msgTyped(sender, content, msgType string, offset time.Duration) GroupMessage {
	m := msg(sender, content, offset)
	m.MsgType = msgType
	return m
}

// ---------- Tests ----------

func TestSummarize_BasicMixedMessages(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Hello everyone", 0),
		msg("Bob", "Hey Alice", 1*time.Minute),
		msg("Charlie", "Good morning", 2*time.Minute),
		msg("Alice", "Let's discuss the backend design", 3*time.Minute),
		msg("Bob", "Sounds good", 4*time.Minute),
		msg("Charlie", "Agreed, let's go with Go", 5*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	if result.MessageCount != 6 {
		t.Errorf("MessageCount = %d; want 6", result.MessageCount)
	}
	if result.ActiveSpeakers != 3 {
		t.Errorf("ActiveSpeakers = %d; want 3", result.ActiveSpeakers)
	}
}

func TestSummarize_HighlightDetection_Decisions(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "We need to pick a language", 0),
		msg("Bob", "I vote for Go", 1*time.Minute),
		msg("Alice", "Agreed, let's go with Go for the backend", 2*time.Minute),
		msg("Charlie", "Confirmed — Go it is", 3*time.Minute),
		msg("Bob", "Great, approved by all", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	decisions := 0
	for _, h := range result.Highlights {
		if h.Type == "decision" {
			decisions++
		}
	}
	if decisions < 2 {
		t.Errorf("expected at least 2 decision highlights; got %d", decisions)
	}
}

func TestSummarize_HighlightDetection_Actions(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "We need CI", 0),
		msg("Bob", "TODO: set up CI pipeline by Friday", 1*time.Minute),
		msg("Charlie", "I'll handle the Dockerfile", 2*time.Minute),
		msg("Alice", "Great, action item noted", 3*time.Minute),
		msg("Bob", "Deadline is next Monday", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	actions := 0
	for _, h := range result.Highlights {
		if h.Type == "action" {
			actions++
		}
	}
	if actions < 3 {
		t.Errorf("expected at least 3 action highlights; got %d", actions)
	}
}

func TestSummarize_HighlightDetection_Questions(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "When does the sprint end?", 0),
		msg("Bob", "Next Friday", 1*time.Minute),
		msg("Charlie", "How do we deploy this?", 2*time.Minute),
		msg("Alice", "What about monitoring?", 3*time.Minute),
		msg("Bob", "Good point", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	questions := 0
	for _, h := range result.Highlights {
		if h.Type == "question" {
			questions++
		}
	}
	if questions < 3 {
		t.Errorf("expected at least 3 question highlights; got %d", questions)
	}
}

func TestSummarize_ParticipantExtraction(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Hello", 0),
		msg("Bob", "Hi", 1*time.Minute),
		msg("Alice", "Again", 2*time.Minute),
		msg("Charlie", "Hey", 3*time.Minute),
		msg("Bob", "Bye", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	want := []string{"Alice", "Bob", "Charlie"}
	if len(result.Participants) != len(want) {
		t.Fatalf("Participants = %v; want %v", result.Participants, want)
	}
	for i, p := range result.Participants {
		if p != want[i] {
			t.Errorf("Participants[%d] = %q; want %q", i, p, want[i])
		}
	}
}

func TestSummarize_ParticipantFallbackToSenderID(t *testing.T) {
	messages := []GroupMessage{
		{SenderID: "u1", Content: "hello", Timestamp: base, MsgType: "text"},
		{SenderID: "u2", Content: "world", Timestamp: base.Add(time.Minute), MsgType: "text"},
		{SenderID: "u1", Content: "again", Timestamp: base.Add(2 * time.Minute), MsgType: "text"},
		{SenderID: "u3", Content: "hi", Timestamp: base.Add(3 * time.Minute), MsgType: "text"},
		{SenderID: "u2", Content: "bye", Timestamp: base.Add(4 * time.Minute), MsgType: "text"},
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	if len(result.Participants) != 3 {
		t.Errorf("expected 3 participants; got %d: %v", len(result.Participants), result.Participants)
	}
}

func TestSummarize_TimeWindowFiltering(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "old message", -2*time.Hour),
		msg("Bob", "also old", -90*time.Minute),
		msg("Alice", "recent 1", 0),
		msg("Bob", "recent 2", 1*time.Minute),
		msg("Charlie", "recent 3", 2*time.Minute),
		msg("Alice", "recent 4", 3*time.Minute),
		msg("Bob", "recent 5", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig() // 1h window
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	// Only the 5 recent messages should be included.
	if result.MessageCount != 5 {
		t.Errorf("MessageCount = %d; want 5 (after time-window filter)", result.MessageCount)
	}
}

func TestSummarize_MinMessagesThreshold(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Hello", 0),
		msg("Bob", "Hi", 1*time.Minute),
	}
	cfg := DefaultSummaryConfig() // MinMessages=5
	result := Summarize(messages, cfg)
	if result != nil {
		t.Error("expected nil summary when below MinMessages threshold")
	}
}

func TestSummarize_MinParticipantsThreshold(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Hello", 0),
		msg("Alice", "Anyone there?", 1*time.Minute),
		msg("Alice", "Guess not", 2*time.Minute),
		msg("Alice", "Still here", 3*time.Minute),
		msg("Alice", "Alone", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig() // MinParticipants=2
	result := Summarize(messages, cfg)
	if result != nil {
		t.Error("expected nil summary when below MinParticipants threshold")
	}
}

func TestSummarize_EmptyMessages(t *testing.T) {
	cfg := DefaultSummaryConfig()
	result := Summarize(nil, cfg)
	if result != nil {
		t.Error("expected nil summary for nil input")
	}
	result = Summarize([]GroupMessage{}, cfg)
	if result != nil {
		t.Error("expected nil summary for empty input")
	}
}

func TestSummarize_TextSummaryContainsAllSections(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Let's decide on the framework", 0),
		msg("Bob", "I prefer Gin", 1*time.Minute),
		msg("Charlie", "Which version should we use?", 2*time.Minute),
		msg("Alice", "Agreed, let's go with Gin", 3*time.Minute),
		msg("Bob", "TODO: set up the project", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	ts := result.TextSummary
	required := []string{
		"**Group Discussion Summary**",
		"**Participants**",
		"Alice",
		"Bob",
		"Charlie",
		"**Key Highlights**",
		"[Decision]",
		"[Action]",
		"[Question]",
		"**Activity**",
	}
	for _, s := range required {
		if !strings.Contains(ts, s) {
			t.Errorf("TextSummary missing %q", s)
		}
	}
}

func TestShouldAutoSummarize_HighVolume(t *testing.T) {
	var messages []GroupMessage
	for i := 0; i < 25; i++ {
		sender := "Alice"
		if i%2 == 0 {
			sender = "Bob"
		}
		messages = append(messages, msg(sender, "message", time.Duration(i)*time.Minute))
	}
	cfg := DefaultSummaryConfig()
	if !ShouldAutoSummarize(messages, cfg) {
		t.Error("expected ShouldAutoSummarize to return true for 25 messages")
	}
}

func TestShouldAutoSummarize_ManyParticipants(t *testing.T) {
	senders := []string{"Alice", "Bob", "Charlie", "Dave", "Eve", "Frank"}
	var messages []GroupMessage
	for i, s := range senders {
		messages = append(messages, msg(s, "hello", time.Duration(i)*time.Minute))
	}
	cfg := DefaultSummaryConfig()
	if !ShouldAutoSummarize(messages, cfg) {
		t.Error("expected ShouldAutoSummarize to return true for 6 participants")
	}
}

func TestShouldAutoSummarize_LowVolume(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Hi", 0),
		msg("Bob", "Hello", 1*time.Minute),
		msg("Charlie", "Hey", 2*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	if ShouldAutoSummarize(messages, cfg) {
		t.Error("expected ShouldAutoSummarize to return false for 3 messages and 3 participants")
	}
}

func TestShouldAutoSummarize_Empty(t *testing.T) {
	cfg := DefaultSummaryConfig()
	if ShouldAutoSummarize(nil, cfg) {
		t.Error("expected ShouldAutoSummarize to return false for nil input")
	}
}

func TestSummarize_NonTextMessagesExcludedFromHighlights(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Let's plan", 0),
		msgTyped("Bob", "Agreed on the design", "image", 1*time.Minute),
		msg("Charlie", "Confirmed — looks good", 2*time.Minute),
		msg("Alice", "What about tests?", 3*time.Minute),
		msg("Bob", "Good point", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	// Bob's image message should not produce a highlight even though it
	// contains "Agreed".
	for _, h := range result.Highlights {
		if h.Author == "Bob" && h.Type == "decision" {
			t.Error("image message should not produce a decision highlight")
		}
	}
}

func TestSummarize_HighlightLimit(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Decided item 1", 0),
		msg("Bob", "Agreed on item 2", 1*time.Minute),
		msg("Charlie", "Confirmed item 3", 2*time.Minute),
		msg("Alice", "Approved item 4", 3*time.Minute),
		msg("Bob", "Let's go with item 5", 4*time.Minute),
		msg("Charlie", "Decided item 6", 5*time.Minute),
		msg("Alice", "Agreed item 7", 6*time.Minute),
	}
	cfg := DefaultSummaryConfig() // HighlightLimit = 5
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	if len(result.Highlights) > 5 {
		t.Errorf("expected at most 5 highlights; got %d", len(result.Highlights))
	}
}

func TestSummarize_DurationCalculation(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "Start", 0),
		msg("Bob", "Middle", 15*time.Minute),
		msg("Charlie", "End", 30*time.Minute),
		msg("Alice", "More", 31*time.Minute),
		msg("Bob", "Done", 45*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	if result.Duration != 45*time.Minute {
		t.Errorf("Duration = %v; want 45m", result.Duration)
	}
}

func TestSummarize_QuestionByPrefix(t *testing.T) {
	messages := []GroupMessage{
		msg("Alice", "How do we handle auth", 0),
		msg("Bob", "What about caching", 1*time.Minute),
		msg("Charlie", "Why not use Redis", 2*time.Minute),
		msg("Alice", "When can we start", 3*time.Minute),
		msg("Bob", "Where should we deploy", 4*time.Minute),
	}
	cfg := DefaultSummaryConfig()
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	questions := 0
	for _, h := range result.Highlights {
		if h.Type == "question" {
			questions++
		}
	}
	if questions != 5 {
		t.Errorf("expected 5 question highlights from prefix detection; got %d", questions)
	}
}

func TestDefaultSummaryConfig(t *testing.T) {
	cfg := DefaultSummaryConfig()
	if cfg.MaxOutputChars != 1000 {
		t.Errorf("MaxOutputChars = %d; want 1000", cfg.MaxOutputChars)
	}
	if cfg.MinMessages != 5 {
		t.Errorf("MinMessages = %d; want 5", cfg.MinMessages)
	}
	if cfg.MinParticipants != 2 {
		t.Errorf("MinParticipants = %d; want 2", cfg.MinParticipants)
	}
	if cfg.HighlightLimit != 5 {
		t.Errorf("HighlightLimit = %d; want 5", cfg.HighlightLimit)
	}
	if cfg.TimeWindow != time.Hour {
		t.Errorf("TimeWindow = %v; want 1h", cfg.TimeWindow)
	}
}

func TestSummarize_MaxOutputCharsTruncation(t *testing.T) {
	var messages []GroupMessage
	for i := 0; i < 20; i++ {
		sender := "Alice"
		if i%3 == 1 {
			sender = "Bob"
		} else if i%3 == 2 {
			sender = "Charlie"
		}
		messages = append(messages, msg(sender, "Decided to implement a very long feature description for item number "+strings.Repeat("x", 50), time.Duration(i)*time.Minute))
	}
	cfg := DefaultSummaryConfig()
	cfg.MaxOutputChars = 200
	result := Summarize(messages, cfg)
	if result == nil {
		t.Fatal("expected non-nil summary")
	}
	if len(result.TextSummary) > 200 {
		t.Errorf("TextSummary length = %d; want <= 200", len(result.TextSummary))
	}
	if !strings.HasSuffix(result.TextSummary, "...") {
		t.Error("expected truncated TextSummary to end with '...'")
	}
}
