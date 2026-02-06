package react

import (
	"alex/internal/domain/agent/ports"
)

// injectUserInput drains the user input channel and appends each message as a
// user-role message to the conversation state. Called once per iteration before
// the think step so the LLM sees new follow-up messages from the chat.
func (r *reactRuntime) injectUserInput() {
	if r.userInputCh == nil {
		return
	}

	for {
		select {
		case input, ok := <-r.userInputCh:
			if !ok {
				r.userInputCh = nil
				return
			}
			r.state.Messages = append(r.state.Messages, ports.Message{
				Role:    "user",
				Content: input.Content,
				Source:  ports.MessageSourceUserInput,
			})
			r.engine.logger.Info("Injected user input from sender %s (msg_id=%s)", input.SenderID, input.MessageID)
		default:
			return
		}
	}
}
