package agent

import (
	"context"
	"testing"
	"time"

	"alex/internal/config"
	"alex/internal/session"
	"alex/pkg/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponse(t *testing.T) {
	// Test Response struct creation and fields
	message := &session.Message{
		Role:      "assistant",
		Content:   "Test response",
		Timestamp: time.Now(),
	}

	response := Response{
		Message:   message,
		SessionID: "session_123",
		Complete:  true,
	}

	assert.Equal(t, message, response.Message)
	assert.Equal(t, "session_123", response.SessionID)
	assert.True(t, response.Complete)
	assert.Empty(t, response.ToolResults)
}

func TestStreamChunk(t *testing.T) {
	// Test StreamChunk struct creation and fields
	chunk := StreamChunk{
		Type:             "text",
		Content:          "Test content",
		Complete:         false,
		TokensUsed:       10,
		TotalTokensUsed:  100,
		PromptTokens:     50,
		CompletionTokens: 50,
		Metadata: map[string]any{
			"model": "test-model",
		},
	}

	assert.Equal(t, "text", chunk.Type)
	assert.Equal(t, "Test content", chunk.Content)
	assert.False(t, chunk.Complete)
	assert.Equal(t, 10, chunk.TokensUsed)
	assert.Equal(t, 100, chunk.TotalTokensUsed)
	assert.Equal(t, 50, chunk.PromptTokens)
	assert.Equal(t, 50, chunk.CompletionTokens)
	assert.Equal(t, "test-model", chunk.Metadata["model"])
}

func TestStreamChunkComplete(t *testing.T) {
	// Test StreamChunk with complete = true
	chunk := StreamChunk{
		Type:     "completion",
		Content:  "Final content",
		Complete: true,
	}

	assert.Equal(t, "completion", chunk.Type)
	assert.Equal(t, "Final content", chunk.Content)
	assert.True(t, chunk.Complete)
}

func TestMessageQueueItem(t *testing.T) {
	// Test MessageQueueItem struct creation and fields
	timestamp := time.Now()
	ctx := context.Background()
	config := &config.Config{}

	var callback StreamCallback = func(chunk StreamChunk) {}

	item := MessageQueueItem{
		Message:   "Test message",
		Timestamp: timestamp,
		Callback:  callback,
		Context:   ctx,
		Config:    config,
		Metadata: map[string]any{
			"priority": "high",
		},
	}

	assert.Equal(t, "Test message", item.Message)
	assert.Equal(t, timestamp, item.Timestamp)
	assert.NotNil(t, item.Callback)
	assert.Equal(t, ctx, item.Context)
	assert.Equal(t, config, item.Config)
	assert.Equal(t, "high", item.Metadata["priority"])
}

func TestMessageQueue(t *testing.T) {
	// Test MessageQueue creation
	queue := &MessageQueue{
		items: make([]MessageQueueItem, 0),
	}

	assert.Empty(t, queue.items)
	assert.NotNil(t, queue.items)
}

func TestMessageQueueOperations(t *testing.T) {
	// Test basic MessageQueue operations
	queue := &MessageQueue{
		items: make([]MessageQueueItem, 0),
	}

	// Add items to queue
	item1 := MessageQueueItem{
		Message:   "First message",
		Timestamp: time.Now(),
	}
	item2 := MessageQueueItem{
		Message:   "Second message",
		Timestamp: time.Now().Add(time.Second),
	}

	queue.items = append(queue.items, item1, item2)

	assert.Len(t, queue.items, 2)
	assert.Equal(t, "First message", queue.items[0].Message)
	assert.Equal(t, "Second message", queue.items[1].Message)
}


func TestStreamCallbackInvocation(t *testing.T) {
	// Test that StreamCallback can be called and receives data
	var receivedChunks []StreamChunk

	callback := func(chunk StreamChunk) {
		receivedChunks = append(receivedChunks, chunk)
	}

	// Test multiple chunks
	chunks := []StreamChunk{
		{Type: "start", Content: "Starting", Complete: false},
		{Type: "text", Content: "Processing", Complete: false},
		{Type: "end", Content: "Complete", Complete: true},
	}

	for _, chunk := range chunks {
		callback(chunk)
	}

	require.Len(t, receivedChunks, 3)
	assert.Equal(t, "start", receivedChunks[0].Type)
	assert.Equal(t, "text", receivedChunks[1].Type)
	assert.Equal(t, "end", receivedChunks[2].Type)
	assert.True(t, receivedChunks[2].Complete)
}

func TestMessageQueueItemWithNilValues(t *testing.T) {
	// Test MessageQueueItem with nil values (should be handled gracefully)
	item := MessageQueueItem{
		Message:   "Test with nil values",
		Timestamp: time.Now(),
		Callback:  nil,
		Context:   nil,
		Config:    nil,
		Metadata:  nil,
	}

	assert.Equal(t, "Test with nil values", item.Message)
	assert.Nil(t, item.Callback)
	assert.Nil(t, item.Context)
	assert.Nil(t, item.Config)
	assert.Nil(t, item.Metadata)
}

func TestResponseWithToolResults(t *testing.T) {
	// Test Response with tool results
	message := &session.Message{
		Role:      "assistant",
		Content:   "Task completed with tools",
		Timestamp: time.Now(),
	}

	response := Response{
		Message:   message,
		SessionID: "session_123",
		Complete:  true,
		ToolResults: []types.ReactToolResult{
			{
				Success: true,
				Content: "Tool 1 result",
			},
			{
				Success: false,
				Content: "Tool 2 failed",
			},
		},
	}

	assert.Len(t, response.ToolResults, 2)
	assert.True(t, response.ToolResults[0].Success)
	assert.False(t, response.ToolResults[1].Success)
	assert.Equal(t, "Tool 1 result", response.ToolResults[0].Content)
	assert.Equal(t, "Tool 2 failed", response.ToolResults[1].Content)
}