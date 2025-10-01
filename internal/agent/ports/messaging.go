package ports

import "context"

// MessageQueue handles async message processing
type MessageQueue interface {
	// Enqueue adds a message to process
	Enqueue(msg UserMessage) error

	// Dequeue retrieves next message (blocks if empty)
	Dequeue(ctx context.Context) (UserMessage, error)

	// Len returns queue length
	Len() int

	// Close stops the queue
	Close() error
}

// UserMessage represents a user request
type UserMessage struct {
	Content   string `json:"content"`
	SessionID string `json:"session_id"`
}

// MessageConverter converts between formats
type MessageConverter interface {
	// ToLLMFormat converts session messages to LLM format
	ToLLMFormat(messages []Message) ([]Message, error)

	// FromLLMFormat converts LLM response to session message
	FromLLMFormat(msg Message) (Message, error)
}
