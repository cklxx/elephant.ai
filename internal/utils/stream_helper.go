package utils

import (
	"fmt"
)

// StreamChunkType defines different types of stream chunks
type StreamChunkType string

const (
	// Tool-related chunks
	ToolStart  StreamChunkType = "tool_start"
	ToolResult StreamChunkType = "tool_result"
	ToolError  StreamChunkType = "tool_error"
	
	// Processing chunks
	Status         StreamChunkType = "status"
	Iteration      StreamChunkType = "iteration"
	ThinkingResult StreamChunkType = "thinking_result"
	FinalAnswer    StreamChunkType = "final_answer"
	
	// Token usage chunks
	TokenUsage     StreamChunkType = "token_usage"
	
	// Error chunks
	Error          StreamChunkType = "error"
	MaxIterations  StreamChunkType = "max_iterations"
)

// StreamHelper provides utilities for creating and managing stream callbacks
type StreamHelper struct {
	componentName string
	logger        *ComponentLogger
}

// NewStreamHelper creates a new stream helper for a component
func NewStreamHelper(componentName string) *StreamHelper {
	return &StreamHelper{
		componentName: componentName,
		logger:        Logger.GetLogger(componentName),
	}
}

// CreateChunk creates a standardized stream chunk
func (sh *StreamHelper) CreateChunk(chunkType StreamChunkType, content string, metadata ...map[string]interface{}) StreamChunk {
	chunk := StreamChunk{
		Type:    string(chunkType),
		Content: content,
	}
	
	if len(metadata) > 0 {
		chunk.Metadata = metadata[0]
	}
	
	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}
	
	// Add component information
	chunk.Metadata["component"] = sh.componentName
	
	return chunk
}

// SendChunk safely sends a chunk through the callback if it exists
func (sh *StreamHelper) SendChunk(callback StreamCallback, chunkType StreamChunkType, content string, metadata ...map[string]interface{}) {
	if callback == nil {
		return
	}
	
	chunk := sh.CreateChunk(chunkType, content, metadata...)
	callback(chunk)
	sh.logger.Debug("Sent %s chunk: %s", string(chunkType), content[:minInt(50, len(content))])
}

// SendToolStart sends a tool start chunk
func (sh *StreamHelper) SendToolStart(callback StreamCallback, toolName string, toolDisplay string) {
	sh.SendChunk(callback, ToolStart, toolDisplay, map[string]interface{}{
		"tool_name": toolName,
		"phase":     "tool_start",
	})
}

// SendToolResult sends a tool result chunk
func (sh *StreamHelper) SendToolResult(callback StreamCallback, toolName string, content string, success bool) {
	chunkType := ToolResult
	if !success {
		chunkType = ToolError
	}
	
	sh.SendChunk(callback, chunkType, content, map[string]interface{}{
		"tool_name": toolName,
		"success":   success,
		"phase":     "tool_result",
	})
}

// SendToolError sends a tool error chunk
func (sh *StreamHelper) SendToolError(callback StreamCallback, toolName string, errorMsg string) {
	sh.SendChunk(callback, ToolError, fmt.Sprintf("%s: %s", toolName, errorMsg), map[string]interface{}{
		"tool_name": toolName,
		"phase":     "tool_error",
	})
}

// SendStatus sends a status update chunk
func (sh *StreamHelper) SendStatus(callback StreamCallback, message string, phase string) {
	sh.SendChunk(callback, Status, message, map[string]interface{}{
		"phase": phase,
	})
}

// SendIteration sends an iteration progress chunk
func (sh *StreamHelper) SendIteration(callback StreamCallback, iteration int, maxIterations int, message string) {
	sh.SendChunk(callback, Iteration, message, map[string]interface{}{
		"iteration":      iteration,
		"max_iterations": maxIterations,
		"phase":          "iteration",
	})
}

// SendTokenUsage sends a token usage chunk
func (sh *StreamHelper) SendTokenUsage(callback StreamCallback, tokensUsed, totalTokens, promptTokens, completionTokens int, iteration int) {
	content := fmt.Sprintf("Tokens used: %d (prompt: %d, completion: %d)", tokensUsed, promptTokens, completionTokens)
	
	chunk := sh.CreateChunk(TokenUsage, content, map[string]interface{}{
		"iteration": iteration,
		"phase":     "token_accounting",
	})
	
	chunk.TokensUsed = tokensUsed
	chunk.TotalTokensUsed = totalTokens
	chunk.PromptTokens = promptTokens
	chunk.CompletionTokens = completionTokens
	
	if callback != nil {
		callback(chunk)
	}
}

// SendThinkingResult sends a thinking result chunk
func (sh *StreamHelper) SendThinkingResult(callback StreamCallback, content string, iteration int) {
	sh.SendChunk(callback, ThinkingResult, content, map[string]interface{}{
		"iteration": iteration,
		"phase":     "thinking",
	})
}

// SendFinalAnswer sends a final answer chunk
func (sh *StreamHelper) SendFinalAnswer(callback StreamCallback, answer string, iteration int) {
	sh.SendChunk(callback, FinalAnswer, answer, map[string]interface{}{
		"iteration": iteration,
		"phase":     "final_answer",
	})
}

// SendError sends an error chunk
func (sh *StreamHelper) SendError(callback StreamCallback, errorMsg string) {
	sh.SendChunk(callback, Error, fmt.Sprintf("❌ %s", errorMsg), map[string]interface{}{
		"phase": "error",
	})
}

// SendMaxIterations sends a max iterations reached chunk
func (sh *StreamHelper) SendMaxIterations(callback StreamCallback, maxIterations int) {
	content := fmt.Sprintf("⚠️ Reached maximum iterations (%d)", maxIterations)
	sh.SendChunk(callback, MaxIterations, content, map[string]interface{}{
		"max_iterations": maxIterations,
		"phase":          "max_iterations",
	})
}

// ConditionalCallback wraps a callback to only execute if the callback is not nil
type ConditionalCallback struct {
	callback StreamCallback
	helper   *StreamHelper
}

// NewConditionalCallback creates a new conditional callback wrapper
func NewConditionalCallback(callback StreamCallback, componentName string) *ConditionalCallback {
	return &ConditionalCallback{
		callback: callback,
		helper:   NewStreamHelper(componentName),
	}
}

// Send sends a chunk if the callback exists
func (cc *ConditionalCallback) Send(chunkType StreamChunkType, content string, metadata ...map[string]interface{}) {
	cc.helper.SendChunk(cc.callback, chunkType, content, metadata...)
}

// ToolStart sends a tool start chunk
func (cc *ConditionalCallback) ToolStart(toolName string, toolDisplay string) {
	cc.helper.SendToolStart(cc.callback, toolName, toolDisplay)
}

// ToolResult sends a tool result chunk
func (cc *ConditionalCallback) ToolResult(toolName string, content string, success bool) {
	cc.helper.SendToolResult(cc.callback, toolName, content, success)
}

// ToolError sends a tool error chunk
func (cc *ConditionalCallback) ToolError(toolName string, errorMsg string) {
	cc.helper.SendToolError(cc.callback, toolName, errorMsg)
}

// Status sends a status chunk
func (cc *ConditionalCallback) Status(message string, phase string) {
	cc.helper.SendStatus(cc.callback, message, phase)
}

// Error sends an error chunk
func (cc *ConditionalCallback) Error(errorMsg string) {
	cc.helper.SendError(cc.callback, errorMsg)
}

// minInt returns the minimum of two integers (renamed to avoid conflicts)
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Global stream helpers for common components
var (
	CoreStreamHelper     *StreamHelper
	ReactStreamHelper    *StreamHelper
	SubAgentStreamHelper *StreamHelper
	ToolStreamHelper     *StreamHelper
)

func init() {
	CoreStreamHelper = NewStreamHelper("REACT-CORE")
	ReactStreamHelper = NewStreamHelper("REACT-AGENT")
	SubAgentStreamHelper = NewStreamHelper("SUB-AGENT")
	ToolStreamHelper = NewStreamHelper("TOOL-EXEC")
}