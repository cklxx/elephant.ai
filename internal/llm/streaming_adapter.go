package llm

import "alex/internal/agent/ports"

// EnsureStreamingClient guarantees the returned client implements
// StreamingLLMClient by wrapping non-streaming implementations with a fallback
// adapter.
func EnsureStreamingClient(client ports.LLMClient) ports.LLMClient {
	return ports.EnsureStreamingClient(client)
}
