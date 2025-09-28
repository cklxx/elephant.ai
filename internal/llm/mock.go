package llm

import (
	"context"
	"fmt"
	"log"
	"time"
)

// MockLLMClient implements the Client interface for testing
type MockLLMClient struct {
	model   string
	baseURL string
}

// Chat returns a mock response for testing
func (m *MockLLMClient) Chat(ctx context.Context, req *ChatRequest, sessionID string) (*ChatResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	// Simulate a small delay to mimic real API behavior
	time.Sleep(10 * time.Millisecond)

	log.Printf("[MockLLMClient] Returning mock response for model %s", m.model)

	return &ChatResponse{
		ID:      "mock-response-id",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   m.model,
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "This is a mock response for testing. No actual API calls were made.",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
	}, nil
}

// ChatStream returns a mock streaming response for testing
func (m *MockLLMClient) ChatStream(ctx context.Context, req *ChatRequest, sessionID string) (<-chan StreamDelta, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}

	ch := make(chan StreamDelta, 10)

	go func() {
		defer close(ch)

		// Simulate streaming chunks
		chunks := []string{"Mock ", "streaming ", "response ", "for ", "testing"}

		for i, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(10 * time.Millisecond) // Simulate network delay

				delta := StreamDelta{
					ID:      fmt.Sprintf("mock-stream-%d", i),
					Object:  "chat.completion.chunk",
					Created: time.Now().Unix(),
					Model:   m.model,
					Choices: []Choice{
						{
							Index: 0,
							Delta: Message{
								Role:    "assistant",
								Content: chunk,
							},
						},
					},
				}

				// Send the final chunk with finish reason
				if i == len(chunks)-1 {
					delta.Choices[0].FinishReason = "stop"
					delta.Usage = Usage{
						PromptTokens:     100,
						CompletionTokens: 50,
						TotalTokens:      150,
					}
				}

				ch <- delta
			}
		}

		log.Printf("[MockLLMClient] Completed mock streaming response for model %s", m.model)
	}()

	return ch, nil
}

// Close is a no-op for the mock client
func (m *MockLLMClient) Close() error {
	log.Printf("[MockLLMClient] Mock client closed (model: %s)", m.model)
	return nil
}