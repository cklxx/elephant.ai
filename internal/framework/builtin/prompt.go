package builtin

import (
	"context"

	"alex/internal/core/hook"
)

// BuildPrompt implements hook.PromptBuilder.
// Constructs a basic Prompt from TurnState: sets a simple system prompt,
// appends TurnState.Messages, and provides an empty tools list.
// Will be wired to existing context/manager_prompt.go later.
func (p *Plugin) BuildPrompt(_ context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	prompt := &hook.Prompt{
		System:   "You are a helpful assistant.",
		Messages: make([]hook.Message, 0, len(state.Messages)+1),
		Tools:    nil,
	}

	// Copy existing messages.
	prompt.Messages = append(prompt.Messages, state.Messages...)

	// Append the current user input as a message if non-empty.
	if state.Input != "" {
		prompt.Messages = append(prompt.Messages, hook.Message{
			Role:    "user",
			Content: state.Input,
		})
	}

	return prompt, nil
}

var _ hook.PromptBuilder = (*Plugin)(nil)
