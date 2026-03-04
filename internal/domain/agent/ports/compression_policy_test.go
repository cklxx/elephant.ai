package ports

import "testing"

func TestBuildCompressionPlan_PreservesSystemAndKeepsRecentTurn(t *testing.T) {
	messages := []Message{
		{Role: "system", Content: "rules", Source: MessageSourceSystemPrompt},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
	}

	plan := BuildCompressionPlan(messages, CompressionPlanOptions{KeepRecentTurns: 1})
	if len(plan.CompressibleIndexes) != 2 {
		t.Fatalf("compressible count = %d, want 2", len(plan.CompressibleIndexes))
	}
	if _, ok := plan.CompressibleIndexes[1]; !ok {
		t.Fatalf("index 1 should be compressible")
	}
	if _, ok := plan.CompressibleIndexes[2]; !ok {
		t.Fatalf("index 2 should be compressible")
	}
	if len(plan.SummarySource) != 2 {
		t.Fatalf("summary source count = %d, want 2", len(plan.SummarySource))
	}
	if plan.SummarySource[0].Content != "u1" || plan.SummarySource[1].Content != "a1" {
		t.Fatalf("summary source order mismatch: %+v", plan.SummarySource)
	}
}

func TestBuildCompressionPlan_SkipsSyntheticMessagesInSummary(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: CompressionSummaryPrefix + " old", Source: MessageSourceUserHistory},
		{Role: "assistant", Content: "assistant old"},
		{Role: "user", Content: "latest user"},
	}

	plan := BuildCompressionPlan(messages, CompressionPlanOptions{
		KeepRecentTurns: 1,
		IsSynthetic: func(msg Message) bool {
			return IsSyntheticSummary(msg.Content)
		},
	})
	if len(plan.CompressibleIndexes) != 2 {
		t.Fatalf("compressible count = %d, want 2", len(plan.CompressibleIndexes))
	}
	if len(plan.SummarySource) != 1 {
		t.Fatalf("summary source count = %d, want 1", len(plan.SummarySource))
	}
	if got := plan.SummarySource[0].Content; got != "assistant old" {
		t.Fatalf("summary source content = %q, want %q", got, "assistant old")
	}
}

func TestBuildCompressionPlan_CustomPreserveSource(t *testing.T) {
	messages := []Message{
		{Role: "user", Content: "u1", Source: MessageSourceUserHistory},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}

	plan := BuildCompressionPlan(messages, CompressionPlanOptions{
		KeepRecentTurns: 1,
		PreserveSource: func(source MessageSource) bool {
			return source == MessageSourceUserHistory
		},
	})
	if len(plan.CompressibleIndexes) != 1 {
		t.Fatalf("compressible count = %d, want 1", len(plan.CompressibleIndexes))
	}
	if _, ok := plan.CompressibleIndexes[1]; !ok {
		t.Fatalf("index 1 should be compressible")
	}
	if len(plan.SummarySource) != 1 || plan.SummarySource[0].Content != "a1" {
		t.Fatalf("unexpected summary source: %+v", plan.SummarySource)
	}
}
