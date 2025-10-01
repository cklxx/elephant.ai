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
