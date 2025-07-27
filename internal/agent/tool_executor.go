package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"alex/internal/llm"
	"alex/internal/utils"
	"alex/pkg/types"

	"github.com/kaptinlin/jsonrepair"
)

// ToolExecutor - 工具执行器
type ToolExecutor struct {
	registry *ToolRegistry
}

// NewToolExecutor - 创建工具执行器
func NewToolExecutor(registry *ToolRegistry) *ToolExecutor {
	return &ToolExecutor{registry: registry}
}

// parseToolCalls - 解析 OpenAI 标准工具调用格式和文本格式工具调用
func (te *ToolExecutor) parseToolCalls(message *llm.Message) []*types.ReactToolCall {
	var toolCalls []*types.ReactToolCall

	// 首先尝试解析标准 tool_calls 格式
	log.Printf("[DEBUG] parseToolCalls: Processing %d tool calls from LLM", len(message.ToolCalls))
	for i, tc := range message.ToolCalls {
		log.Printf("[DEBUG] parseToolCalls: Tool call %d - ID: '%s', Name: '%s'", i, tc.ID, tc.Function.Name)

		var args map[string]interface{}
		if tc.Function.Arguments != "" {
			// 首先尝试直接解析JSON
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				log.Printf("[ERROR] Failed to parse tool arguments: %v", err)
				log.Printf("[ERROR] Raw JSON content: %q", tc.Function.Arguments)
				log.Printf("[ERROR] JSON length: %d", len(tc.Function.Arguments))

				// 尝试使用jsonrepair库修复JSON
				log.Printf("[INFO] Attempting to repair JSON using jsonrepair library")
				fixedJSON, repairErr := jsonrepair.JSONRepair(tc.Function.Arguments)
				if repairErr != nil {
					log.Printf("[ERROR] JSON repair failed: %v", repairErr)

					// 如果jsonrepair失败，尝试简单的备用修复
					log.Printf("[INFO] Attempting fallback repair method")
					fallbackJSON := simpleFallbackRepair(tc.Function.Arguments)
					if fallbackJSON != tc.Function.Arguments {
						log.Printf("[INFO] Fallback repair applied, new length: %d", len(fallbackJSON))
						if err := json.Unmarshal([]byte(fallbackJSON), &args); err != nil {
							log.Printf("[ERROR] Fallback repair also failed: %v", err)
							continue
						}
						log.Printf("[INFO] Successfully parsed fallback repaired JSON")
					} else {
						log.Printf("[ERROR] No repair method succeeded, skipping tool call")
						continue
					}
				} else {
					log.Printf("[INFO] JSON repaired successfully, new length: %d", len(fixedJSON))
					log.Printf("[DEBUG] Repaired JSON: %q", fixedJSON)

					// 尝试解析修复后的JSON
					if err := json.Unmarshal([]byte(fixedJSON), &args); err != nil {
						log.Printf("[ERROR] Failed to parse even after JSON repair: %v", err)
						continue
					}

					log.Printf("[INFO] Successfully parsed repaired JSON")
				}
			}
		}

		// 确保CallID不为空 - 如果缺少则生成一个，但要确保一致性
		callID := tc.ID
		if callID == "" {
			callID = fmt.Sprintf("call_%d", time.Now().UnixNano())
			log.Printf("[WARN] parseToolCalls: Missing ID for tool %s, generated: %s", tc.Function.Name, callID)
			// 重要：更新原始工具调用的ID以保持一致性
			tc.ID = callID
		}

		toolCall := &types.ReactToolCall{
			Name:      tc.Function.Name,
			Arguments: args,
			CallID:    callID,
		}

		log.Printf("[DEBUG] parseToolCalls: Created ReactToolCall - Name: '%s', CallID: '%s'", toolCall.Name, toolCall.CallID)
		toolCalls = append(toolCalls, toolCall)
	}

	// 如果没有标准工具调用，尝试解析文本格式的工具调用
	if len(toolCalls) == 0 && message.Content != "" {
		textToolCalls := te.parseTextToolCalls(message.Content)
		toolCalls = append(toolCalls, textToolCalls...)
	}

	return toolCalls
}

// parseTextToolCalls - 解析文本格式的工具调用
func (te *ToolExecutor) parseTextToolCalls(content string) []*types.ReactToolCall {
	var toolCalls []*types.ReactToolCall

	// 处理 <｜tool▁calls▁begin｜> 格式
	if strings.Contains(content, "<｜tool▁calls▁begin｜>") {
		// 提取工具调用部分
		startIdx := strings.Index(content, "<｜tool▁calls▁begin｜>")
		endIdx := strings.Index(content, "<｜tool▁calls▁end｜>")

		if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
			toolSection := content[startIdx : endIdx+len("<｜tool▁calls▁end｜>")]

			// 解析每个工具调用
			calls := strings.Split(toolSection, "<｜tool▁call▁begin｜>")
			for i, call := range calls {
				if i == 0 || call == "" {
					continue // 跳过第一个空部分
				}

				// 查找工具调用结束标记
				endCallIdx := strings.Index(call, "<｜tool▁call▁end｜>")
				if endCallIdx == -1 {
					continue
				}

				callContent := call[:endCallIdx]
				if toolCall := te.parseIndividualTextToolCall(callContent); toolCall != nil {
					toolCalls = append(toolCalls, toolCall)
				}
			}
		}
	}

	return toolCalls
}

// parseIndividualTextToolCall - 解析单个文本工具调用
func (te *ToolExecutor) parseIndividualTextToolCall(callContent string) *types.ReactToolCall {
	// 格式: function<｜tool▁sep｜>tool_name\n```json\n{args}\n```
	parts := strings.Split(callContent, "<｜tool▁sep｜>")
	if len(parts) < 2 {
		return nil
	}

	if strings.TrimSpace(parts[0]) != "function" {
		return nil
	}

	remainder := parts[1]
	lines := strings.Split(remainder, "\n")
	if len(lines) == 0 {
		return nil
	}

	toolName := strings.TrimSpace(lines[0])
	if toolName == "" {
		return nil
	}

	// 寻找JSON参数
	jsonStart, jsonEnd := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "```json" {
			jsonStart = i + 1
		} else if trimmed == "```" && jsonStart != -1 {
			jsonEnd = i
			break
		}
	}

	var args map[string]interface{}
	if jsonStart != -1 && jsonEnd != -1 && jsonEnd > jsonStart {
		jsonContent := strings.Join(lines[jsonStart:jsonEnd], "\n")
		if err := json.Unmarshal([]byte(jsonContent), &args); err != nil {
			log.Printf("[WARN] Failed to parse JSON args for tool %s: %v", toolName, err)
			// 继续执行，使用空参数
			args = make(map[string]interface{})
		}
	} else {
		args = make(map[string]interface{})
	}

	return &types.ReactToolCall{
		Name:      toolName,
		Arguments: args,
		CallID:    fmt.Sprintf("text_%d", time.Now().UnixNano()),
	}
}

// executeSerialToolsStream - 串行执行工具调用（流式版本）
func (te *ToolExecutor) executeSerialToolsStream(ctx context.Context, toolCalls []*types.ReactToolCall, callback StreamCallback) []*types.ReactToolResult {
	if len(toolCalls) == 0 {
		return []*types.ReactToolResult{
			{
				Success: false,
				Error:   "no tool calls provided",
			},
		}
	}

	log.Printf("[DEBUG] executeSerialToolsStream: Starting execution of %d tool calls", len(toolCalls))
	for i, tc := range toolCalls {
		log.Printf("[DEBUG] executeSerialToolsStream: Tool call %d - Name: '%s', CallID: '%s'", i, tc.Name, tc.CallID)
	}

	// 串行执行工具调用，按顺序一个接一个执行
	// 确保为每个输入的工具调用都产生一个对应的结果
	combinedResult := make([]*types.ReactToolResult, 0, len(toolCalls))

	for i, toolCall := range toolCalls {
		log.Printf("[DEBUG] executeSerialToolsStream: Processing tool call %d/%d - Name: '%s', CallID: '%s'", i+1, len(toolCalls), toolCall.Name, toolCall.CallID)

		// 发送工具开始信号
		toolCallStr := te.formatToolCallForDisplay(toolCall.Name, toolCall.Arguments)
		callback(StreamChunk{Type: "tool_start", Content: toolCallStr})

		// 确保每个工具调用都产生一个结果，无论什么情况
		var finalResult *types.ReactToolResult

		// 使用defer确保即使panic也能恢复并生成错误结果
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERROR] executeSerialToolsStream: Tool call %d panicked: %v", i+1, r)
					finalResult = &types.ReactToolResult{
						Success:  false,
						Error:    fmt.Sprintf("tool execution panicked: %v", r),
						ToolName: toolCall.Name,
						ToolArgs: toolCall.Arguments,
						CallID:   toolCall.CallID,
					}
					callback(StreamChunk{Type: "tool_error", Content: fmt.Sprintf("%s: panic occurred", toolCall.Name)})
				}
			}()

			// 执行工具
			result, err := te.executeTool(ctx, toolCall.Name, toolCall.Arguments, toolCall.CallID)

			if err != nil {
				log.Printf("[DEBUG] executeSerialToolsStream: Tool call %d failed with error: %v", i+1, err)
				callback(StreamChunk{Type: "tool_error", Content: fmt.Sprintf("%s: %v", toolCall.Name, err)})
				finalResult = &types.ReactToolResult{
					Success:  false,
					Error:    err.Error(),
					ToolName: toolCall.Name,
					ToolArgs: toolCall.Arguments,
					CallID:   toolCall.CallID,
				}
			} else if result != nil {
				log.Printf("[DEBUG] executeSerialToolsStream: Tool call %d succeeded", i+1)
				// 发送工具结果信号
				var contentStr = result.Content

				// Show diff for file modifications with clean formatting
				if result.Data != nil {
					if diffStr, hasDiff := result.Data["diff"].(string); hasDiff && diffStr != "" {
						cleanDiff := formatDiffForDisplay(diffStr)
						if cleanDiff != "" {
							contentStr = result.Content + "\n" + cleanDiff
						}
					}
				}

				// Standard display limit for tool output
				var displayLimit = 200

				// Use rune-based slicing to properly handle UTF-8 characters like Chinese text
				runes := []rune(contentStr)
				if len(runes) > displayLimit {
					contentStr = string(runes[:displayLimit]) + "..."
				}
				callback(StreamChunk{Type: "tool_result", Content: contentStr})

				// 确保关键字段都正确设置
				if result.ToolName == "" {
					result.ToolName = toolCall.Name
				}
				if result.CallID == "" {
					result.CallID = toolCall.CallID
				}
				// 确保工具参数也被保存
				if result.ToolArgs == nil {
					result.ToolArgs = toolCall.Arguments
				}

				finalResult = result

				if !result.Success {
					callback(StreamChunk{Type: "tool_error", Content: fmt.Sprintf("%s: %s", toolCall.Name, result.Error)})
				}
			} else {
				// 这种情况不应该发生：err == nil 但 result == nil
				log.Printf("[ERROR] executeSerialToolsStream: Tool call %d returned nil result without error", i+1)
				finalResult = &types.ReactToolResult{
					Success:  false,
					Error:    "tool execution returned nil result",
					ToolName: toolCall.Name,
					ToolArgs: toolCall.Arguments,
					CallID:   toolCall.CallID,
				}
				callback(StreamChunk{Type: "tool_error", Content: fmt.Sprintf("%s: nil result", toolCall.Name)})
			}
		}()

		// 最后的安全检查：确保finalResult不为nil且CallID正确
		if finalResult == nil {
			log.Printf("[ERROR] executeSerialToolsStream: finalResult is nil for tool call %d, creating emergency fallback", i+1)
			finalResult = &types.ReactToolResult{
				Success:  false,
				Error:    "unknown error: finalResult was nil",
				ToolName: toolCall.Name,
				ToolArgs: toolCall.Arguments,
				CallID:   toolCall.CallID,
			}
		}

		// 确保CallID一致性的最终检查
		if finalResult.CallID != toolCall.CallID {
			log.Printf("[WARN] executeSerialToolsStream: CallID mismatch detected, correcting from '%s' to '%s'", finalResult.CallID, toolCall.CallID)
			finalResult.CallID = toolCall.CallID
		}

		// 确保每个工具调用都有对应的结果
		combinedResult = append(combinedResult, finalResult)
		log.Printf("[DEBUG] executeSerialToolsStream: Added result for tool call %d with CallID: '%s'", i+1, finalResult.CallID)
	}

	log.Printf("[DEBUG] executeSerialToolsStream: Completed execution, returning %d results", len(combinedResult))
	return combinedResult
}

// formatToolCallForDisplay - 格式化工具调用显示
func (te *ToolExecutor) formatToolCallForDisplay(toolName string, args map[string]interface{}) string {
	// Green color for the dot
	greenDot := "\033[32m⏺\033[0m"

	if len(args) == 0 {
		return fmt.Sprintf("%s %s()", greenDot, toolName)
	}

	// Build arguments string
	var argParts []string
	for key, value := range args {
		var valueStr string
		switch v := value.(type) {
		case string:
			// Special handling for todo_update content parameter - show only first line
			if toolName == "todo_update" && key == "content" {
				lines := strings.Split(v, "\n")
				if len(lines) > 0 {
					firstLine := strings.TrimSpace(lines[0])
					if firstLine != "" {
						// Remove markdown header prefix if present
						firstLine = strings.TrimPrefix(firstLine, "# ")
						runes := []rune(firstLine)
						if len(runes) > 30 {
							valueStr = fmt.Sprintf(`"%s..."`, string(runes[:27]))
						} else {
							valueStr = fmt.Sprintf(`"%s"`, firstLine)
						}
					} else {
						valueStr = `"[multi-line content]"`
					}
				} else {
					valueStr = `""`
				}
			} else {
				// Default string handling
				// Use rune-based slicing to properly handle UTF-8 characters like Chinese text
				runes := []rune(v)
				if len(runes) > 50 {
					valueStr = fmt.Sprintf(`"%s..."`, string(runes[:47]))
				} else {
					valueStr = fmt.Sprintf(`"%s"`, v)
				}
			}
		case int, int64, float64, bool:
			valueStr = fmt.Sprintf("%v", v)
		default:
			// For complex types, convert to string and truncate
			str := fmt.Sprintf("%v", v)
			// Use rune-based slicing to properly handle UTF-8 characters like Chinese text
			runes := []rune(str)
			if len(runes) > 30 {
				valueStr = string(runes[:27]) + "..."
			} else {
				valueStr = str
			}
		}
		argParts = append(argParts, fmt.Sprintf("%s=%s", key, valueStr))
	}

	argsStr := strings.Join(argParts, ", ")
	// Use rune-based slicing to properly handle UTF-8 characters like Chinese text
	runes := []rune(argsStr)
	if len(runes) > 100 {
		argsStr = string(runes[:97]) + "..."
	}

	return fmt.Sprintf("%s %s(%s)", greenDot, toolName, argsStr)
}

// executeTool - 执行工具
func (te *ToolExecutor) executeTool(ctx context.Context, toolName string, args map[string]interface{}, callId string) (*types.ReactToolResult, error) {
	log.Printf("[DEBUG] executeTool: Starting execution - Tool: '%s', CallID: '%s'", toolName, callId)

	// 使用统一的工具注册器获取工具
	tool, err := te.registry.GetTool(ctx, toolName)
	if err != nil {
		log.Printf("[ERROR] executeTool: %v", err)
		return nil, err
	}

	// 确保args不为nil，避免工具panic
	if args == nil {
		args = make(map[string]interface{})
	}

	// 验证参数，防止panic
	if err := tool.Validate(args); err != nil {
		log.Printf("[ERROR] executeTool: Tool %s validation failed: %v", toolName, err)
		return nil, fmt.Errorf("tool validation failed: %w", err)
	}

	// Session ID injection removed - tools now get session ID directly from manager
	
	// 直接执行工具
	start := time.Now()
	result, err := tool.Execute(ctx, args)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[ERROR] ToolExecutor: Tool %s execution failed: %v", toolName, err)
		resultObj := &types.ReactToolResult{
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}
		log.Printf("[DEBUG] executeTool: Error result - CallID: '%s', Success: %v", resultObj.CallID, resultObj.Success)
		return resultObj, nil
	}

	resultObj := &types.ReactToolResult{
		Success:  true,
		Content:  strings.TrimLeft(result.Content, " \t"),
		Data:     result.Data,
		Duration: duration,
		ToolName: toolName,
		ToolArgs: args,
		CallID:   callId,
	}

	log.Printf("[DEBUG] executeTool: Success result - CallID: '%s', Success: %v", resultObj.CallID, resultObj.Success)
	return resultObj, nil
}

// Session-related helper functions removed - tools now access session manager directly

// simpleFallbackRepair - 简单的备用JSON修复方法
// 当jsonrepair库失败时使用这个更保守的方法
func simpleFallbackRepair(jsonStr string) string {
	jsonStr = strings.TrimSpace(jsonStr)

	// 如果不是以{开始，无法修复
	if !strings.HasPrefix(jsonStr, "{") {
		return jsonStr
	}

	// 基本的补全：确保有结束大括号
	if !strings.HasSuffix(jsonStr, "}") {
		// 如果在字符串中间截断，尝试找到最后一个完整的键值对
		lastCommaIndex := strings.LastIndex(jsonStr, ",")
		lastColonIndex := strings.LastIndex(jsonStr, ":")

		// 如果最后一个字符是逗号，说明可能是在键值对之间截断
		if strings.HasSuffix(jsonStr, ",") {
			jsonStr = jsonStr[:len(jsonStr)-1] // 移除最后的逗号
		} else if lastCommaIndex > lastColonIndex {
			// 如果最后一个逗号在最后一个冒号之后，说明可能是值没有完成
			// 截断到最后一个逗号
			jsonStr = jsonStr[:lastCommaIndex]
		} else if lastColonIndex > 0 {
			// 如果正在键值对中间，尝试找到键的开始
			beforeColon := jsonStr[:lastColonIndex]
			// 寻找键的开始引号
			for i := len(beforeColon) - 1; i >= 0; i-- {
				if beforeColon[i] == '"' {
					// 找到了键的开始，截断到这里
					jsonStr = jsonStr[:i]
					break
				}
			}
		}

		// 添加结束大括号
		jsonStr += "}"
	}

	return jsonStr
}

// formatDiffForDisplay applies clean formatting to diff for CLI display
func formatDiffForDisplay(diffStr string) string {
	if diffStr == "" {
		return ""
	}

	// Filter out unnecessary diff headers and show only meaningful changes
	return utils.FilterAndFormatDiff(diffStr)
}
