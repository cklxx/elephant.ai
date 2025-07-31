package message

import (
	"alex/internal/llm"
	"alex/internal/session"
	"regexp"
	"strings"
)

// TokenEstimator provides unified token estimation logic
type TokenEstimator struct {
	// Patterns for improved token estimation
	wordPattern *regexp.Regexp
	codePattern *regexp.Regexp
}

// NewTokenEstimator creates a new token estimator
func NewTokenEstimator() *TokenEstimator {
	return &TokenEstimator{
		wordPattern: regexp.MustCompile(`\b\w+\b`),
		codePattern: regexp.MustCompile("```[\\s\\S]*?```|`[^`]*`"),
	}
}

// EstimateSessionMessages estimates tokens for session messages
func (te *TokenEstimator) EstimateSessionMessages(messages []*session.Message) int {
	totalTokens := 0

	for _, msg := range messages {
		contentTokens := te.estimateContentTokens(msg.Content)
		totalTokens += contentTokens + 10 // Role and metadata overhead

		// Add tokens for tool calls
		for _, tc := range msg.ToolCalls {
			totalTokens += len(tc.Function.Name)/3 + len(tc.ID)/3 + 15 // Tool call overhead
		}
	}

	return totalTokens
}

// estimateContentTokens provides more accurate token estimation for content
func (te *TokenEstimator) estimateContentTokens(content string) int {
	if content == "" {
		return 0
	}

	// Handle code blocks separately
	codeBlocks := te.codePattern.FindAllString(content, -1)
	textWithoutCode := te.codePattern.ReplaceAllString(content, "")

	// Count tokens in regular text
	words := te.wordPattern.FindAllString(textWithoutCode, -1)
	regularTokens := len(words)

	// Add punctuation and spacing tokens (roughly 1 token per 4 characters)
	nonWordChars := len(textWithoutCode) - len(strings.Join(words, ""))
	regularTokens += nonWordChars / 4

	// Count tokens in code blocks (code is more token-dense)
	codeTokens := 0
	for _, codeBlock := range codeBlocks {
		// Code blocks: approximately 2 characters per token
		codeTokens += len(codeBlock) / 2
	}

	return regularTokens + codeTokens
}

// EstimateLLMMessages estimates tokens for LLM messages
func (te *TokenEstimator) EstimateLLMMessages(messages []llm.Message) int {
	totalTokens := 0

	for _, msg := range messages {
		contentTokens := te.estimateContentTokens(msg.Content)
		totalTokens += contentTokens + 10 // Role and metadata overhead

		// Add tokens for tool calls
		for _, tc := range msg.ToolCalls {
			totalTokens += len(tc.Function.Name)/3 + len(tc.ID)/3 + 15 // Tool call overhead
		}
	}

	return totalTokens
}

// EstimateString estimates tokens for a string
func (te *TokenEstimator) EstimateString(content string) int {
	return te.estimateContentTokens(content)
}

// EstimateMessages estimates tokens for mixed message types
func (te *TokenEstimator) EstimateMessages(sessionMessages []*session.Message, llmMessages []llm.Message) int {
	totalTokens := 0

	if len(sessionMessages) > 0 {
		totalTokens += te.EstimateSessionMessages(sessionMessages)
	}

	if len(llmMessages) > 0 {
		totalTokens += te.EstimateLLMMessages(llmMessages)
	}

	return totalTokens
}

// CheckTokenLimit checks if messages exceed token limit
func (te *TokenEstimator) CheckTokenLimit(messages []*session.Message, maxTokens int) bool {
	estimated := te.EstimateSessionMessages(messages)
	return estimated > maxTokens
}

// GetCompressionThreshold calculates compression threshold based on max tokens
func (te *TokenEstimator) GetCompressionThreshold(maxTokens int, threshold float64) int {
	return int(float64(maxTokens) * threshold)
}
