package agent

import (
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestExtractAwaitUserInputQuestion(t *testing.T) {
	t.Run("question_to_user takes priority", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Content: "content fallback",
				Metadata: map[string]any{
					"needs_user_input": true,
					"question_to_user": "Which env?",
				},
			}},
		}}
		question, ok := ExtractAwaitUserInputQuestion(messages)
		if !ok {
			t.Fatal("expected question to be found")
		}
		if question != "Which env?" {
			t.Fatalf("expected question_to_user, got %q", question)
		}
	})

	t.Run("message fallback", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Metadata: map[string]any{
					"needs_user_input": true,
					"message":          "Please log in",
				},
			}},
		}}
		question, ok := ExtractAwaitUserInputQuestion(messages)
		if !ok {
			t.Fatal("expected message to be found")
		}
		if question != "Please log in" {
			t.Fatalf("expected message fallback, got %q", question)
		}
	})

	t.Run("content fallback", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Content: "Need your input",
				Metadata: map[string]any{
					"needs_user_input": true,
				},
			}},
		}}
		question, ok := ExtractAwaitUserInputQuestion(messages)
		if !ok {
			t.Fatal("expected content fallback")
		}
		if question != "Need your input" {
			t.Fatalf("expected content fallback, got %q", question)
		}
	})

	t.Run("ignores missing flag", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Content: "ignored",
			}},
		}}
		if question, ok := ExtractAwaitUserInputQuestion(messages); ok || question != "" {
			t.Fatalf("expected no question, got %q", question)
		}
	})
}
