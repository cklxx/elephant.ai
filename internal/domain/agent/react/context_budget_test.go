package react

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestModelContextWindowTokens(t *testing.T) {
	cases := []struct {
		name  string
		model string
		want  int
	}{
		{name: "gpt5 codex", model: "gpt-5.2-codex", want: gpt5ContextWindowTokens},
		{name: "gpt5 spark", model: "gpt-5.3-codex-spark", want: gpt5ContextWindowTokens},
		{name: "claude", model: "claude-sonnet-4-20250514", want: claudeContextWindowTokens},
		{name: "gpt4o", model: "gpt-4o-mini", want: defaultModelContextWindowTokens},
		{name: "unknown", model: "my-custom-model", want: defaultModelContextWindowTokens},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := modelContextWindowTokens(tc.model)
			if got != tc.want {
				t.Fatalf("modelContextWindowTokens(%q) = %d, want %d", tc.model, got, tc.want)
			}
		})
	}
}

func TestDeriveContextTokenLimitByModel(t *testing.T) {
	gotCodex := deriveContextTokenLimit("gpt-5.2-codex", 12000)
	gotDefault := deriveContextTokenLimit("gpt-4o-mini", 12000)

	if gotCodex <= gotDefault {
		t.Fatalf("expected codex budget > default budget, got codex=%d default=%d", gotCodex, gotDefault)
	}
	if gotCodex <= 200000 {
		t.Fatalf("expected codex budget to be large enough for 200k+ contexts, got %d", gotCodex)
	}
	if gotDefault <= 100000 {
		t.Fatalf("expected default budget to stay above 100k for 128k models, got %d", gotDefault)
	}
}

func TestSplitContextBudgetSubtractsToolTokens(t *testing.T) {
	tools := []ports.ToolDefinition{
		{
			Name:        "search_web",
			Description: strings.Repeat("search the web for current info. ", 80),
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"q": {Type: "string", Description: strings.Repeat("query ", 60)},
				},
			},
		},
	}

	split := splitContextBudget(120000, tools)
	if split.ToolTokens <= 0 {
		t.Fatalf("expected positive tool token estimate, got %d", split.ToolTokens)
	}
	if split.MessageLimit >= split.TotalLimit {
		t.Fatalf("expected message limit to be reduced by tool budget, got message_limit=%d total_limit=%d", split.MessageLimit, split.TotalLimit)
	}
	want := split.TotalLimit - split.ToolTokens - contextBudgetRequestSafetyTokens
	if want < minMessageBudgetTokens {
		want = minMessageBudgetTokens
	}
	if split.MessageLimit != want {
		t.Fatalf("splitContextBudget message limit = %d, want %d", split.MessageLimit, want)
	}
}

func TestSplitContextBudgetKeepsAtLeastOneMessageToken(t *testing.T) {
	huge := strings.Repeat("x", 80000)
	tools := []ports.ToolDefinition{
		{
			Name:        "massive_tool",
			Description: huge,
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"payload": {Type: "string", Description: huge},
				},
			},
		},
	}

	split := splitContextBudget(1000, tools)
	if split.MessageLimit != minMessageBudgetTokens {
		t.Fatalf("expected floor message limit %d, got %d", minMessageBudgetTokens, split.MessageLimit)
	}
}
