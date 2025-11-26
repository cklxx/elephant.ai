package context

import (
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestHistorySummarizerReturnsLastRawTurn(t *testing.T) {
	summarizer := NewHistorySummarizer()
	messages := []ports.Message{
		{Role: "system", Content: "policy", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "请帮我规划一次旅行", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "好的，我先确认需求。", Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "上一句是什么？", Source: ports.MessageSourceUserInput},
	}

	summary := summarizer.Summarize(messages)

	if summary.LastRawTurn.Content != "上一句是什么？" {
		t.Fatalf("expected last raw turn to be preserved verbatim, got %q", summary.LastRawTurn.Content)
	}
	if len(summary.Bullets) == 0 {
		t.Fatalf("expected summarizer to emit bullets for prior turns")
	}
	for _, bullet := range summary.Bullets {
		if len(bullet.Citations) == 0 {
			t.Fatalf("expected bullet to include at least one citation: %+v", bullet)
		}
	}
}

func TestSummarizerKeepsCitationsPerRole(t *testing.T) {
	summarizer := NewHistorySummarizer()
	messages := []ports.Message{
		{Role: "user", Content: "first question", Source: ports.MessageSourceUserHistory},
		{Role: "assistant", Content: "first answer", Source: ports.MessageSourceAssistantReply},
		{Role: "assistant", Content: "second answer", Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "latest", Source: ports.MessageSourceUserInput},
	}

	summary := summarizer.Summarize(messages)

	if len(summary.Bullets) != 2 {
		t.Fatalf("expected user and assistant bullets, got %d", len(summary.Bullets))
	}
	if got := summary.Bullets[0].Citations[0].Ref; got != "msg_0" {
		t.Fatalf("expected first citation to reference msg_0, got %q", got)
	}
	if got := summary.Bullets[1].Citations[len(summary.Bullets[1].Citations)-1].Ref; got != "msg_2" {
		t.Fatalf("expected last assistant citation to reference msg_2, got %q", got)
	}
	if summary.LastRawTurn.Content != "latest" {
		t.Fatalf("expected last raw turn to be the final message, got %q", summary.LastRawTurn.Content)
	}
}

func TestSummarizerUsesTemplatesFromYAML(t *testing.T) {
	yaml := []byte("user: 'User said {{.Snippet}}'\nassistant: 'Bot replied {{.Snippet}}'\ntool: 'Tool did {{.Snippet}}'\nlast_raw: 'Raw {{.Provenance}} -> {{.Snippet}}'\n")
	templates, err := LoadHistoryTemplates(yaml)
	if err != nil {
		t.Fatalf("failed to parse templates: %v", err)
	}
	summarizer := NewHistorySummarizerWithTemplates(templates)

	summary := summarizer.Summarize([]ports.Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}})

	if len(summary.Bullets) == 0 || !strings.Contains(summary.Bullets[0].Text, "User said hello") {
		t.Fatalf("expected custom user template to be applied, got %+v", summary.Bullets)
	}
}
