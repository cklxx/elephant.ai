package agent

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"alex/internal/llm"
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

