package ports

// ContextManager handles token limits and compression
type ContextManager interface {
	// EstimateTokens estimates token count for messages
	EstimateTokens(messages []Message) int

	// Compress reduces message size when limit approached
	Compress(messages []Message, targetTokens int) ([]Message, error)

	// ShouldCompress checks if compression needed
	ShouldCompress(messages []Message, limit int) bool
}

// SessionContextKey is the shared context key for storing session IDs across packages.
// This ensures consistent session ID propagation from server layer to agent layer.
type SessionContextKey struct{}
