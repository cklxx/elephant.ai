package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/pkg/types"
)

// ToolExecutionConfig configures tool execution behavior
type ToolExecutionConfig struct {
	EnablePanicRecovery bool
	TimeoutDuration     time.Duration
	MaxRetries          int
	ComponentName       string
}

// StreamCallback represents a callback function for streaming updates
// Note: This should match the definition in agent package
type StreamCallback func(chunk StreamChunk)

// StreamChunk represents a chunk of streaming data - matches agent package
type StreamChunk struct {
	Type             string                 `json:"type"`
	Content          string                 `json:"content"`
	Complete         bool                   `json:"complete,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	TokensUsed       int                    `json:"tokens_used,omitempty"`
	TotalTokensUsed  int                    `json:"total_tokens_used,omitempty"`
	PromptTokens     int                    `json:"prompt_tokens,omitempty"`
	CompletionTokens int                    `json:"completion_tokens,omitempty"`
}

// ToolExecutor provides safe tool execution with error handling and recovery
type ToolExecutor struct {
	logger *ComponentLogger
}

// NewToolExecutor creates a new tool executor with the specified configuration
func NewToolExecutor(componentName string) *ToolExecutor {
	return &ToolExecutor{
		logger: Logger.GetLogger(componentName),
	}
}

// ExecuteToolWithRecovery executes a tool with panic recovery and comprehensive error handling
func (te *ToolExecutor) ExecuteToolWithRecovery(
	ctx context.Context,
	toolCall *types.ReactToolCall,
	toolExecutor func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error),
	callback StreamCallback,
) *types.ReactToolResult {
	
	var finalResult *types.ReactToolResult

	// Execute with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				te.logger.Error("Tool call panicked: %v", r)
				finalResult = &types.ReactToolResult{
					Success:  false,
					Error:    fmt.Sprintf("tool execution panicked: %v", r),
					ToolName: toolCall.Name,
					ToolArgs: toolCall.Arguments,
					CallID:   toolCall.CallID,
				}
				if callback != nil {
					callback(StreamChunk{
						Type:    "tool_error",
						Content: fmt.Sprintf("%s: panic occurred", toolCall.Name),
					})
				}
			}
		}()

		// Execute the tool
		result, err := toolExecutor(ctx, toolCall.Name, toolCall.Arguments, toolCall.CallID)

		if err != nil {
			te.logger.Debug("Tool call failed with error: %v", err)
			if callback != nil {
				callback(StreamChunk{
					Type:    "tool_error",
					Content: fmt.Sprintf("%s: %v", toolCall.Name, err),
				})
			}
			finalResult = &types.ReactToolResult{
				Success:  false,
				Error:    err.Error(),
				ToolName: toolCall.Name,
				ToolArgs: toolCall.Arguments,
				CallID:   toolCall.CallID,
			}
		} else if result != nil {
			te.logger.Info("Tool call succeeded")
			finalResult = result
			// Send tool result signal with smart content formatting
			if callback != nil {
				formattedContent := te.formatToolResultContent(toolCall.Name, result.Content)
				callback(StreamChunk{Type: "tool_result", Content: formattedContent})
			}
			if !result.Success && callback != nil {
				callback(StreamChunk{
					Type:    "tool_error",
					Content: fmt.Sprintf("%s: %s", toolCall.Name, result.Error),
				})
			}
		} else {
			te.logger.Error("Tool call returned nil result without error")
			finalResult = &types.ReactToolResult{
				Success:  false,
				Error:    "tool execution returned nil result",
				ToolName: toolCall.Name,
				ToolArgs: toolCall.Arguments,
				CallID:   toolCall.CallID,
			}
			if callback != nil {
				callback(StreamChunk{
					Type:    "tool_error",
					Content: fmt.Sprintf("%s: nil result", toolCall.Name),
				})
			}
		}
	}()

	// Validate and fix the final result
	return te.ValidateAndFixResult(finalResult, toolCall)
}

// ValidateAndFixResult performs comprehensive validation and fixing of tool results
func (te *ToolExecutor) ValidateAndFixResult(result *types.ReactToolResult, originalCall *types.ReactToolCall) *types.ReactToolResult {
	// Final safety check: ensure result is not nil
	if result == nil {
		te.logger.Error("finalResult is nil, creating emergency fallback")
		result = &types.ReactToolResult{
			Success:  false,
			Error:    "unknown error: finalResult was nil",
			ToolName: originalCall.Name,
			ToolArgs: originalCall.Arguments,
			CallID:   originalCall.CallID,
		}
	}

	// Ensure CallID consistency
	if result.CallID != originalCall.CallID {
		te.logger.Warn("CallID mismatch detected, correcting from '%s' to '%s'", result.CallID, originalCall.CallID)
		result.CallID = originalCall.CallID
	}

	// Ensure ToolName is set
	if result.ToolName == "" {
		result.ToolName = originalCall.Name
	}

	// Ensure ToolArgs is set
	if result.ToolArgs == nil {
		result.ToolArgs = originalCall.Arguments
	}

	te.logger.Debug("Added result for tool call with CallID: '%s'", result.CallID)
	return result
}

// ExecuteSerialToolsWithRecovery executes multiple tools in series with comprehensive error handling
func (te *ToolExecutor) ExecuteSerialToolsWithRecovery(
	ctx context.Context,
	toolCalls []*types.ReactToolCall,
	toolExecutor func(ctx context.Context, toolName string, args map[string]interface{}, callID string) (*types.ReactToolResult, error),
	callback StreamCallback,
	displayFormatter func(toolName string, args map[string]interface{}) string,
) []*types.ReactToolResult {
	
	if len(toolCalls) == 0 {
		return []*types.ReactToolResult{
			{
				Success: false,
				Error:   "no tool calls provided",
			},
		}
	}

	te.logger.Debug("Starting execution of %d tool calls", len(toolCalls))
	for i, tc := range toolCalls {
		te.logger.Debug("Tool call %d - Name: '%s', CallID: '%s'", i, tc.Name, tc.CallID)
	}

	// Execute tools in series
	combinedResult := make([]*types.ReactToolResult, 0, len(toolCalls))

	for i, toolCall := range toolCalls {
		te.logger.Info("Processing tool call %d/%d - %s", i+1, len(toolCalls), toolCall.Name)

		// Send tool start signal
		if callback != nil && displayFormatter != nil {
			toolCallStr := displayFormatter(toolCall.Name, toolCall.Arguments)
			callback(StreamChunk{Type: "tool_start", Content: toolCallStr})
		}

		// Execute with recovery
		result := te.ExecuteToolWithRecovery(ctx, toolCall, toolExecutor, callback)
		combinedResult = append(combinedResult, result)
	}

	te.logger.Debug("Completed execution, returning %d results", len(combinedResult))
	return combinedResult
}

// formatToolResultContent formats tool result content with smart truncation
func (te *ToolExecutor) formatToolResultContent(toolName string, content string) string {
	// First, clean up leading/trailing whitespace and normalize line endings
	content = strings.TrimSpace(content)
	if content == "" {
		return content
	}

	// Split content into lines for analysis
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	// Tools that typically return long content and should be truncated to 3 lines
	longContentTools := map[string]bool{
		"file_read":   true,
		"file_list":   true,
		"grep":        true,
		"ripgrep":     true,
		"bash":        true,
		"find":        true,
		"web_search":  true,
	}

	// For tools that typically return long content, show only first 3 lines + summary
	if longContentTools[toolName] && totalLines > 3 {
		// Take first 3 non-empty lines with consistent alignment
		var displayLines []string
		firstLineProcessed := false
		
		for _, line := range lines {
			if len(displayLines) >= 3 {
				break
			}
			
			// For the first non-empty line, start processing
			if !firstLineProcessed && strings.TrimSpace(line) != "" {
				firstLineProcessed = true
				displayLines = append(displayLines, strings.TrimSpace(line))
			} else if firstLineProcessed {
				// For subsequent lines, maintain consistent alignment (no leading spaces)
				cleanLine := strings.TrimSpace(line)
				if cleanLine != "" {
					displayLines = append(displayLines, cleanLine)
				}
			}
		}
		
		result := strings.Join(displayLines, "\n")
		if totalLines > 3 {
			result += fmt.Sprintf("\n... (%d total lines)", totalLines)
		}
		return result
	}

	// For other tools, apply general length limits with line preservation
	if totalLines > 10 {
		// Show first 5 lines + summary for moderately long content with alignment
		var displayLines []string
		firstLineProcessed := false
		
		for i := 0; i < 5 && i < len(lines); i++ {
			line := lines[i]
			if !firstLineProcessed && strings.TrimSpace(line) != "" {
				firstLineProcessed = true
				displayLines = append(displayLines, strings.TrimSpace(line))
			} else if firstLineProcessed {
				cleanLine := strings.TrimSpace(line)
				displayLines = append(displayLines, cleanLine)
			}
		}
		
		result := strings.Join(displayLines, "\n")
		result += fmt.Sprintf("\n... (%d total lines)", totalLines)
		return result
	}

	// For short content, align all lines consistently
	var cleanLines []string
	firstLineProcessed := false
	
	for _, line := range lines {
		if !firstLineProcessed && strings.TrimSpace(line) != "" {
			firstLineProcessed = true
			cleanLines = append(cleanLines, strings.TrimSpace(line))
		} else if firstLineProcessed {
			cleanLine := strings.TrimSpace(line)
			cleanLines = append(cleanLines, cleanLine)
		} else {
			// Preserve empty lines before first content
			cleanLines = append(cleanLines, line)
		}
	}

	return strings.Join(cleanLines, "\n")
}

// GenerateCallID generates a fallback call ID when one is missing
func GenerateCallID(toolName string) string {
	return fmt.Sprintf("fallback_%s_%d", toolName, time.Now().UnixNano())
}

// FormatToolCallForDisplay provides a standard way to format tool calls for display
type ToolDisplayFormatter struct {
	colorDot func(...interface{}) string
}

// NewToolDisplayFormatter creates a new formatter with the specified color
func NewToolDisplayFormatter(colorAttribute ...interface{}) *ToolDisplayFormatter {
	var colorFunc func(...interface{}) string
	if len(colorAttribute) > 0 {
		colorFunc = func(args ...interface{}) string {
			return fmt.Sprintf("\033[%vm⏺\033[0m", colorAttribute[0])
		}
	} else {
		colorFunc = func(args ...interface{}) string {
			return "\033[32m⏺\033[0m" // Default green
		}
	}
	
	return &ToolDisplayFormatter{
		colorDot: colorFunc,
	}
}

// Format formats a tool call for display
func (tdf *ToolDisplayFormatter) Format(toolName string, args map[string]interface{}) string {
	if len(args) == 0 {
		return fmt.Sprintf("%s %s()", tdf.colorDot(), toolName)
	}

	var argsStr []string
	for k, v := range args {
		argsStr = append(argsStr, fmt.Sprintf("%s=%v", k, v))
	}

	return fmt.Sprintf("%s %s(%s)", tdf.colorDot(), toolName, fmt.Sprintf("%v", argsStr))
}