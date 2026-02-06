package agent

import (
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestExtractAwaitUserInputPrompt(t *testing.T) {
	t.Run("extracts question and string options", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Content: "fallback question",
				Metadata: map[string]any{
					"needs_user_input": true,
					"question_to_user": "Which env?",
					"options":          []string{"dev", "staging", "prod"},
				},
			}},
		}}

		prompt, ok := ExtractAwaitUserInputPrompt(messages)
		if !ok {
			t.Fatal("expected prompt to be found")
		}
		if prompt.Question != "Which env?" {
			t.Fatalf("expected question_to_user, got %q", prompt.Question)
		}
		if len(prompt.Options) != 3 {
			t.Fatalf("expected 3 options, got %#v", prompt.Options)
		}
		if prompt.Options[0] != "dev" || prompt.Options[1] != "staging" || prompt.Options[2] != "prod" {
			t.Fatalf("unexpected options: %#v", prompt.Options)
		}
	})

	t.Run("normalizes mixed options", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Metadata: map[string]any{
					"needs_user_input": true,
					"message":          "Choose one",
					"options": []any{
						"  A  ",
						map[string]any{"label": "B"},
						map[string]any{"text": "C"},
						map[string]any{"value": "D"},
						"A",
						"",
					},
				},
			}},
		}}

		prompt, ok := ExtractAwaitUserInputPrompt(messages)
		if !ok {
			t.Fatal("expected prompt")
		}
		if prompt.Question != "Choose one" {
			t.Fatalf("expected message fallback, got %q", prompt.Question)
		}
		if len(prompt.Options) != 4 {
			t.Fatalf("expected 4 options after dedupe, got %#v", prompt.Options)
		}
		if prompt.Options[0] != "A" || prompt.Options[1] != "B" || prompt.Options[2] != "C" || prompt.Options[3] != "D" {
			t.Fatalf("unexpected normalized options: %#v", prompt.Options)
		}
	})

	t.Run("returns false without question text", func(t *testing.T) {
		messages := []core.Message{{
			ToolResults: []core.ToolResult{{
				Metadata: map[string]any{
					"needs_user_input": true,
					"options":          []string{"A"},
				},
			}},
		}}

		prompt, ok := ExtractAwaitUserInputPrompt(messages)
		if ok {
			t.Fatalf("expected not found, got %#v", prompt)
		}
	})
}

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
