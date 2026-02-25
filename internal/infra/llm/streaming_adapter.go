package llm

import portsllm "alex/internal/domain/agent/ports/llm"

// EnsureStreamingClient guarantees the returned client implements
// StreamingLLMClient by wrapping non-streaming implementations with a fallback
// adapter.
func EnsureStreamingClient(client portsllm.LLMClient) portsllm.LLMClient {
	return portsllm.EnsureStreamingClient(client)
}
