package distillation

import (
	"context"
	"testing"
	"time"

	core "alex/internal/domain/agent/ports"
)

type mockLLMClient struct {
	response string
	model    string
	err      error
}

func (m *mockLLMClient) Complete(_ context.Context, _ core.CompletionRequest) (*core.CompletionResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &core.CompletionResponse{
		Content: m.response,
		Usage:   core.TokenUsage{TotalTokens: 100},
	}, nil
}

func (m *mockLLMClient) Model() string { return m.model }

func fixedNow() time.Time { return time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC) }

func TestExtractDaily(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		response    string
		wantFacts   int
		wantErr     bool
	}{
		{
			name:    "single fact extraction",
			content: "We decided to use Go for the backend.",
			response: `[{"content":"Team chose Go for backend","category":"decision","confidence":0.95}]`,
			wantFacts: 1,
		},
		{
			name:    "multiple facts",
			content: "Meeting notes from today.",
			response: `[{"content":"fact1","category":"decision","confidence":0.9},{"content":"fact2","category":"preference","confidence":0.8}]`,
			wantFacts: 2,
		},
		{
			name:     "empty response",
			content:  "Nothing interesting.",
			response: `[]`,
			wantFacts: 0,
		},
		{
			name:    "invalid JSON response",
			content: "Some content.",
			response: `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &mockLLMClient{response: tt.response, model: "test"}
			ext := NewExtractor(client, 4096, fixedNow)

			result, err := ext.ExtractDaily(context.Background(), tt.content, "2026-03-18")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Facts) != tt.wantFacts {
				t.Errorf("got %d facts, want %d", len(result.Facts), tt.wantFacts)
			}
			if result.Date != "2026-03-18" {
				t.Errorf("got date %q, want %q", result.Date, "2026-03-18")
			}
		})
	}
}

func TestChunkContent(t *testing.T) {
	tests := []struct {
		name       string
		contentLen int
		wantChunks int
	}{
		{name: "small content no chunking", contentLen: 1000, wantChunks: 1},
		{name: "at threshold no chunking", contentLen: chunkingThreshold * charsPerToken, wantChunks: 1},
		{name: "large content needs chunking", contentLen: 50000, wantChunks: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := make([]byte, tt.contentLen)
			for i := range content {
				content[i] = 'a'
			}
			chunks := chunkContent(string(content))
			if len(chunks) != tt.wantChunks {
				t.Errorf("got %d chunks, want %d", len(chunks), tt.wantChunks)
			}
		})
	}
}

func TestChunkContentOverlap(t *testing.T) {
	// Verify chunks overlap by 200 tokens worth of chars
	content := make([]byte, 20000)
	for i := range content {
		content[i] = byte('a' + (i % 26))
	}
	chunks := chunkContent(string(content))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}

	overlapChars := overlapTokens * charsPerToken
	firstEnd := chunks[0][len(chunks[0])-overlapChars:]
	secondStart := chunks[1][:overlapChars]
	if firstEnd != secondStart {
		t.Error("chunks do not overlap correctly")
	}
}
