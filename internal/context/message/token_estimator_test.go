package message

import (
	"regexp"
	"strings"
	"testing"

	"alex/internal/llm"
	"alex/internal/session"

	"github.com/stretchr/testify/assert"
)

func TestNewTokenEstimator(t *testing.T) {
	// Test TokenEstimator creation
	estimator := NewTokenEstimator()

	assert.NotNil(t, estimator)
	assert.NotNil(t, estimator.wordPattern)
	assert.NotNil(t, estimator.codePattern)
}

func TestTokenEstimatorPatterns(t *testing.T) {
	// Test regex patterns initialization
	estimator := NewTokenEstimator()

	// Test word pattern
	wordMatches := estimator.wordPattern.FindAllString("Hello world test", -1)
	assert.Len(t, wordMatches, 3)
	assert.Contains(t, wordMatches, "Hello")
	assert.Contains(t, wordMatches, "world")
	assert.Contains(t, wordMatches, "test")

	// Test code pattern
	codeText := "Here is some `inline code` and ```\nmulti-line\ncode block\n```"
	codeMatches := estimator.codePattern.FindAllString(codeText, -1)
	assert.Len(t, codeMatches, 2)
	assert.Contains(t, codeMatches, "`inline code`")
}

func TestEstimateContentTokensEmpty(t *testing.T) {
	// Test empty content token estimation
	estimator := NewTokenEstimator()

	tokens := estimator.estimateContentTokens("")
	assert.Equal(t, 0, tokens)
}

func TestEstimateContentTokensSimpleText(t *testing.T) {
	// Test simple text token estimation
	estimator := NewTokenEstimator()

	content := "Hello world this is a test"
	tokens := estimator.estimateContentTokens(content)

	// Should be approximately 6 words, so around 6-8 tokens
	assert.Greater(t, tokens, 4)
	assert.Less(t, tokens, 12)
}

func TestEstimateContentTokensWithCode(t *testing.T) {
	// Test content with code blocks
	estimator := NewTokenEstimator()

	content := "Here is some text with `inline code` and:\n```python\nprint('hello')\n```"
	tokens := estimator.estimateContentTokens(content)

	// Should account for both text and code
	assert.Greater(t, tokens, 10)
	assert.Less(t, tokens, 50)
}

func TestEstimateContentTokensCodeOnly(t *testing.T) {
	// Test content that is only code
	estimator := NewTokenEstimator()

	content := "```python\ndef hello():\n    print('Hello, World!')\n    return True\n```"
	tokens := estimator.estimateContentTokens(content)

	// Code blocks typically have more tokens due to syntax
	assert.Greater(t, tokens, 5)
	assert.Less(t, tokens, 50)
}

func TestEstimateSessionMessages(t *testing.T) {
	// Test session messages token estimation
	estimator := NewTokenEstimator()

	messages := []*session.Message{
		{
			Role:    "user",
			Content: "Hello, how are you?",
		},
		{
			Role:    "assistant",
			Content: "I'm doing well, thank you for asking!",
		},
	}

	tokens := estimator.EstimateSessionMessages(messages)

	// Should include content tokens plus overhead for role/metadata
	assert.Greater(t, tokens, 10) // Minimum expected
	assert.Less(t, tokens, 50)    // Maximum reasonable
}

func TestEstimateSessionMessagesWithToolCalls(t *testing.T) {
	// Test session messages with tool calls
	estimator := NewTokenEstimator()

	messages := []*session.Message{
		{
			Role:    "assistant",
			Content: "I'll help you with that.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: llm.Function{
						Name:      "get_weather",
						Arguments: `{"location": "New York"}`,
					},
				},
			},
		},
	}

	tokens := estimator.EstimateSessionMessages(messages)

	// Should include content tokens, role overhead, and tool call overhead
	assert.Greater(t, tokens, 15) // Should be more than just content
	assert.Less(t, tokens, 100)
}

func TestEstimateSessionMessagesMultipleToolCalls(t *testing.T) {
	// Test session messages with multiple tool calls
	estimator := NewTokenEstimator()

	messages := []*session.Message{
		{
			Role:    "assistant",
			Content: "Let me check multiple things for you.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_1",
					Type: "function",
					Function: llm.Function{
						Name:      "get_weather",
						Arguments: `{"location": "New York"}`,
					},
				},
				{
					ID:   "call_2",
					Type: "function",
					Function: llm.Function{
						Name:      "get_time",
						Arguments: `{"timezone": "EST"}`,
					},
				},
			},
		},
	}

	tokens := estimator.EstimateSessionMessages(messages)

	// Should include overhead for both tool calls
	assert.Greater(t, tokens, 25)
	assert.Less(t, tokens, 120)
}

func TestEstimateSessionMessagesEmpty(t *testing.T) {
	// Test empty session messages
	estimator := NewTokenEstimator()

	tokens := estimator.EstimateSessionMessages([]*session.Message{})
	assert.Equal(t, 0, tokens)

	tokens = estimator.EstimateSessionMessages(nil)
	assert.Equal(t, 0, tokens)
}

func TestEstimateContentTokensLongText(t *testing.T) {
	// Test long text token estimation
	estimator := NewTokenEstimator()

	// Create a long text
	longText := strings.Repeat("This is a test sentence with multiple words. ", 20)
	tokens := estimator.estimateContentTokens(longText)

	// Should scale appropriately with content length
	assert.Greater(t, tokens, 100)
	assert.Less(t, tokens, 300)
}

func TestEstimateContentTokensSpecialCharacters(t *testing.T) {
	// Test content with special characters
	estimator := NewTokenEstimator()

	content := "Hello! @#$%^&*() Testing with symbols: +=[]{}|\\:;\"'<>?,./"
	tokens := estimator.estimateContentTokens(content)

	// Should handle special characters reasonably
	assert.Greater(t, tokens, 5)
	assert.Less(t, tokens, 25)
}

func TestEstimateContentTokensUnicode(t *testing.T) {
	// Test content with unicode characters
	estimator := NewTokenEstimator()

	content := "Hello 世界 🌍 Testing unicode characters"
	tokens := estimator.estimateContentTokens(content)

	// Should handle unicode reasonably
	assert.Greater(t, tokens, 5)
	assert.Less(t, tokens, 20)
}

func TestTokenEstimatorRegexPatterns(t *testing.T) {
	// Test regex patterns more thoroughly
	estimator := NewTokenEstimator()

	testCases := []struct {
		name     string
		content  string
		pattern  *regexp.Regexp
		expected int
	}{
		{
			name:     "simple words",
			content:  "hello world test",
			pattern:  estimator.wordPattern,
			expected: 3,
		},
		{
			name:     "words with numbers",
			content:  "test123 hello world2",
			pattern:  estimator.wordPattern,
			expected: 3,
		},
		{
			name:     "inline code",
			content:  "text `code` more text",
			pattern:  estimator.codePattern,
			expected: 1,
		},
		{
			name:     "code block",
			content:  "```\ncode here\n```",
			pattern:  estimator.codePattern,
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := tc.pattern.FindAllString(tc.content, -1)
			assert.Len(t, matches, tc.expected)
		})
	}
}

// Benchmark tests for performance
func BenchmarkNewTokenEstimator(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewTokenEstimator()
	}
}

func BenchmarkEstimateContentTokens(b *testing.B) {
	estimator := NewTokenEstimator()
	content := "This is a test sentence with multiple words and some code: `print('hello')`"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimator.estimateContentTokens(content)
	}
}

func BenchmarkEstimateSessionMessages(b *testing.B) {
	estimator := NewTokenEstimator()
	messages := []*session.Message{
		{Role: "user", Content: "Hello, how are you?"},
		{Role: "assistant", Content: "I'm doing well, thank you!"},
		{Role: "user", Content: "Can you help me with coding?"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		estimator.EstimateSessionMessages(messages)
	}
}