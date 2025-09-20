package llm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextKeyType(t *testing.T) {
	// Test ContextKeyType custom type
	key1 := ContextKeyType("test-key-1")
	key2 := ContextKeyType("test-key-2")

	assert.Equal(t, "test-key-1", string(key1))
	assert.Equal(t, "test-key-2", string(key2))
	assert.NotEqual(t, key1, key2)
}

func TestMessage(t *testing.T) {
	// Test Message struct with basic fields
	message := Message{
		Role:    "user",
		Content: "Hello, how are you?",
		Name:    "test-user",
	}

	assert.Equal(t, "user", message.Role)
	assert.Equal(t, "Hello, how are you?", message.Content)
	assert.Equal(t, "test-user", message.Name)
	assert.Empty(t, message.ToolCalls)
	assert.Empty(t, message.ToolCallId)
}

func TestMessageWithToolCalls(t *testing.T) {
	// Test Message with tool calls
	toolCalls := []ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: Function{
				Name:      "get_weather",
				Arguments: `{"location": "New York"}`,
			},
		},
	}

	message := Message{
		Role:      "assistant",
		Content:   "",
		ToolCalls: toolCalls,
	}

	assert.Equal(t, "assistant", message.Role)
	assert.Empty(t, message.Content)
	assert.Len(t, message.ToolCalls, 1)
	assert.Equal(t, "call_123", message.ToolCalls[0].ID)
	assert.Equal(t, "get_weather", message.ToolCalls[0].Function.Name)
}

func TestMessageWithReasoning(t *testing.T) {
	// Test Message with OpenAI reasoning fields
	message := Message{
		Role:             "assistant",
		Content:          "The answer is 42",
		Reasoning:        "First, I analyzed the question...",
		ReasoningSummary: "Used mathematical analysis",
		Think:            "I need to think about this carefully",
	}

	assert.Equal(t, "assistant", message.Role)
	assert.Equal(t, "The answer is 42", message.Content)
	assert.Equal(t, "First, I analyzed the question...", message.Reasoning)
	assert.Equal(t, "Used mathematical analysis", message.ReasoningSummary)
	assert.Equal(t, "I need to think about this carefully", message.Think)
}

func TestMessageWithMetadata(t *testing.T) {
	// Test Message with metadata and compression tracking
	sourceMessages := []Message{
		{Role: "user", Content: "First message"},
		{Role: "assistant", Content: "First response"},
	}

	message := Message{
		Role:    "assistant",
		Content: "Compressed response",
		Metadata: map[string]interface{}{
			"compression_ratio": 0.5,
			"original_length":   100,
		},
		SourceMessages: sourceMessages,
		IsCompressed:   true,
	}

	assert.Equal(t, "assistant", message.Role)
	assert.Equal(t, "Compressed response", message.Content)
	assert.True(t, message.IsCompressed)
	assert.Len(t, message.SourceMessages, 2)
	assert.Equal(t, 0.5, message.Metadata["compression_ratio"])
	assert.Equal(t, 100, message.Metadata["original_length"])
}

func TestChatRequest(t *testing.T) {
	// Test ChatRequest struct
	messages := []Message{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}

	request := ChatRequest{
		Messages:    messages,
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   1000,
		Stream:      true,
		ModelType:   ReasoningModel,
	}

	assert.Len(t, request.Messages, 2)
	assert.Equal(t, "gpt-4", request.Model)
	assert.Equal(t, 0.7, request.Temperature)
	assert.Equal(t, 1000, request.MaxTokens)
	assert.True(t, request.Stream)
	assert.Equal(t, ReasoningModel, request.ModelType)
}

func TestChatRequestWithTools(t *testing.T) {
	// Test ChatRequest with tools
	tools := []Tool{
		{
			Type: "function",
			Function: Function{
				Name:        "get_weather",
				Description: "Get current weather",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"location": map[string]interface{}{
							"type":        "string",
							"description": "The city and state",
						},
					},
				},
			},
		},
	}

	request := ChatRequest{
		Messages:   []Message{{Role: "user", Content: "What's the weather?"}},
		Tools:      tools,
		ToolChoice: "auto",
	}

	assert.Len(t, request.Tools, 1)
	assert.Equal(t, "auto", request.ToolChoice)
	assert.Equal(t, "get_weather", request.Tools[0].Function.Name)
}

func TestChatResponse(t *testing.T) {
	// Test ChatResponse struct
	choices := []Choice{
		{
			Index:   0,
			Message: Message{Role: "assistant", Content: "Hello!"},
		},
	}

	usage := Usage{
		PromptTokens:     50,
		CompletionTokens: 20,
		TotalTokens:      70,
	}

	response := ChatResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: choices,
		Usage:   usage,
	}

	assert.Equal(t, "chatcmpl-123", response.ID)
	assert.Equal(t, "chat.completion", response.Object)
	assert.Equal(t, "gpt-4", response.Model)
	assert.Len(t, response.Choices, 1)
	assert.Equal(t, "Hello!", response.Choices[0].Message.Content)
}

func TestChatResponseGetUsage(t *testing.T) {
	// Test ChatResponse.GetUsage() method with OpenAI format
	response := ChatResponse{
		Usage: Usage{
			PromptTokens:     30,
			CompletionTokens: 15,
			TotalTokens:      45,
		},
	}

	usage := response.GetUsage()
	assert.Equal(t, 30, usage.GetPromptTokens())
	assert.Equal(t, 15, usage.GetCompletionTokens())
	assert.Equal(t, 45, usage.GetTotalTokens())
}

func TestChatResponseGetUsageGeminiFormat(t *testing.T) {
	// Test ChatResponse.GetUsage() method with Gemini format fallback
	response := ChatResponse{
		UsageMetadata: Usage{
			PromptTokenCount:     25,
			CandidatesTokenCount: 10,
			TotalTokenCount:      35,
		},
	}

	usage := response.GetUsage()
	assert.Equal(t, 25, usage.GetPromptTokens())
	assert.Equal(t, 10, usage.GetCompletionTokens())
	assert.Equal(t, 35, usage.GetTotalTokens())
}

func TestChoice(t *testing.T) {
	// Test Choice struct
	choice := Choice{
		Index: 0,
		Message: Message{
			Role:    "assistant",
			Content: "Test response",
		},
		FinishReason: "stop",
	}

	assert.Equal(t, 0, choice.Index)
	assert.Equal(t, "assistant", choice.Message.Role)
	assert.Equal(t, "Test response", choice.Message.Content)
	assert.Equal(t, "stop", choice.FinishReason)
}

func TestChoiceWithDelta(t *testing.T) {
	// Test Choice with delta (streaming)
	choice := Choice{
		Index: 0,
		Delta: Message{
			Role:    "assistant",
			Content: "Streaming content",
		},
		FinishReason: "",
	}

	assert.Equal(t, 0, choice.Index)
	assert.Equal(t, "assistant", choice.Delta.Role)
	assert.Equal(t, "Streaming content", choice.Delta.Content)
	assert.Empty(t, choice.FinishReason)
}

func TestUsageOpenAIFormat(t *testing.T) {
	// Test Usage with OpenAI format
	usage := Usage{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	assert.Equal(t, 100, usage.GetPromptTokens())
	assert.Equal(t, 50, usage.GetCompletionTokens())
	assert.Equal(t, 150, usage.GetTotalTokens())
}

func TestUsageGeminiFormat(t *testing.T) {
	// Test Usage with Gemini format
	usage := Usage{
		PromptTokenCount:     80,
		CandidatesTokenCount: 40,
		TotalTokenCount:      120,
	}

	assert.Equal(t, 80, usage.GetPromptTokens())
	assert.Equal(t, 40, usage.GetCompletionTokens())
	assert.Equal(t, 120, usage.GetTotalTokens())
}

func TestUsageMixedFormat(t *testing.T) {
	// Test Usage with mixed format (OpenAI takes precedence)
	usage := Usage{
		PromptTokens:         100, // OpenAI format
		CompletionTokens:     50,  // OpenAI format
		TotalTokens:          150, // OpenAI format
		PromptTokenCount:     200, // Gemini format (should be ignored)
		CandidatesTokenCount: 100, // Gemini format (should be ignored)
		TotalTokenCount:      300, // Gemini format (should be ignored)
	}

	assert.Equal(t, 100, usage.GetPromptTokens())
	assert.Equal(t, 50, usage.GetCompletionTokens())
	assert.Equal(t, 150, usage.GetTotalTokens())
}

func TestStreamDelta(t *testing.T) {
	// Test StreamDelta struct
	delta := StreamDelta{
		ID:      "chatcmpl-stream-123",
		Object:  "chat.completion.chunk",
		Created: time.Now().Unix(),
		Model:   "gpt-4",
		Choices: []Choice{
			{
				Index: 0,
				Delta: Message{Content: "Hello"},
			},
		},
		Provider: "openai",
	}

	assert.Equal(t, "chatcmpl-stream-123", delta.ID)
	assert.Equal(t, "chat.completion.chunk", delta.Object)
	assert.Equal(t, "gpt-4", delta.Model)
	assert.Equal(t, "openai", delta.Provider)
	assert.Len(t, delta.Choices, 1)
	assert.Equal(t, "Hello", delta.Choices[0].Delta.Content)
}

func TestStreamDeltaGetUsage(t *testing.T) {
	// Test StreamDelta.GetUsage() method
	usage := Usage{
		PromptTokens:     10,
		CompletionTokens: 5,
		TotalTokens:      15,
	}

	delta := StreamDelta{
		Usage: usage,
	}

	deltaUsage := delta.GetUsage()
	assert.Equal(t, 10, deltaUsage.GetPromptTokens())
	assert.Equal(t, 5, deltaUsage.GetCompletionTokens())
	assert.Equal(t, 15, deltaUsage.GetTotalTokens())
}

func TestModelType(t *testing.T) {
	// Test ModelType constants
	assert.Equal(t, ModelType("basic"), BasicModel)
	assert.Equal(t, ModelType("reasoning"), ReasoningModel)

	// Test usage in variables
	var modelType = BasicModel
	assert.Equal(t, BasicModel, modelType)

	modelType = ReasoningModel
	assert.Equal(t, ReasoningModel, modelType)
}

func TestModelConfig(t *testing.T) {
	// Test ModelConfig struct
	config := ModelConfig{
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-4",
		APIKey:      "sk-test-key",
		Temperature: 0.7,
		MaxTokens:   2000,
	}

	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, 0.7, config.Temperature)
	assert.Equal(t, 2000, config.MaxTokens)
}

func TestConfig(t *testing.T) {
	// Test Config struct with single model configuration
	config := Config{
		APIKey:      "sk-test-key",
		BaseURL:     "https://api.openai.com/v1",
		Model:       "gpt-4",
		Temperature: 0.8,
		MaxTokens:   1500,
		Timeout:     30 * time.Second,
	}

	assert.Equal(t, "sk-test-key", config.APIKey)
	assert.Equal(t, "https://api.openai.com/v1", config.BaseURL)
	assert.Equal(t, "gpt-4", config.Model)
	assert.Equal(t, 0.8, config.Temperature)
	assert.Equal(t, 1500, config.MaxTokens)
	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestConfigMultiModel(t *testing.T) {
	// Test Config struct with multi-model configuration
	config := Config{
		DefaultModelType: BasicModel,
		Models: map[ModelType]*ModelConfig{
			BasicModel: {
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-3.5-turbo",
				APIKey:  "sk-basic-key",
			},
			ReasoningModel: {
				BaseURL: "https://api.openai.com/v1",
				Model:   "gpt-4",
				APIKey:  "sk-reasoning-key",
			},
		},
	}

	assert.Equal(t, BasicModel, config.DefaultModelType)
	assert.Len(t, config.Models, 2)

	basicConfig := config.Models[BasicModel]
	require.NotNil(t, basicConfig)
	assert.Equal(t, "gpt-3.5-turbo", basicConfig.Model)
	assert.Equal(t, "sk-basic-key", basicConfig.APIKey)

	reasoningConfig := config.Models[ReasoningModel]
	require.NotNil(t, reasoningConfig)
	assert.Equal(t, "gpt-4", reasoningConfig.Model)
	assert.Equal(t, "sk-reasoning-key", reasoningConfig.APIKey)
}

// Benchmark tests for performance
func BenchmarkUsageGetPromptTokens(b *testing.B) {
	usage := Usage{
		PromptTokens: 100,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		usage.GetPromptTokens()
	}
}

func BenchmarkChatResponseGetUsage(b *testing.B) {
	response := ChatResponse{
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 25,
			TotalTokens:      75,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		response.GetUsage()
	}
}
