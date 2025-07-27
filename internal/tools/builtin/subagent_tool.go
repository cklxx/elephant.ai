package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"github.com/fatih/color"
)

var (
	purplePrefix = color.New(color.FgMagenta, color.Bold).SprintFunc()
)

// subAgentLog - sub-agent专用的紫色日志函数
func subAgentLog(level, format string, args ...interface{}) {
	prefix := purplePrefix("[SUB-AGENT]")
	message := fmt.Sprintf(format, args...)
	log.Printf("%s [%s] %s", prefix, level, message)
}

// SubAgentExecutor - Sub-agent执行器接口，避免循环依赖
type SubAgentExecutor interface {
	ExecuteSubAgentTask(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// SubAgentTool - Sub-agent工具实现
type SubAgentTool struct {
	executor SubAgentExecutor
}

// CreateSubAgentTool - 创建sub-agent工具
func CreateSubAgentTool(executor SubAgentExecutor) Tool {
	return &SubAgentTool{
		executor: executor,
	}
}

// Name - 实现Tool接口
func (t *SubAgentTool) Name() string {
	return "subagent"
}

// Description - 实现Tool接口
func (t *SubAgentTool) Description() string {
	return "Execute a complex task using a specialized sub-agent with its own context and session. Use this when you need to delegate a substantial, self-contained task that requires multiple steps or specialized focus."
}

// Parameters - 实现Tool接口
func (t *SubAgentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"task": map[string]interface{}{
				"type":        "string",
				"description": "The task description to be executed by the sub-agent. Be specific and clear about what needs to be accomplished.",
			},
		},
		"required": []string{"task"},
	}
}

// Execute - 实现Tool接口
func (t *SubAgentTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	if t.executor == nil {
		return nil, fmt.Errorf("sub-agent tool not properly initialized: missing executor")
	}

	// 设置工程内部预设的参数
	processedArgs := make(map[string]interface{})
	for k, v := range args {
		processedArgs[k] = v
	}
	
	// 设置默认的system_prompt（如果未提供）
	if _, exists := processedArgs["system_prompt"]; !exists {
		processedArgs["system_prompt"] = "你是一个专门处理特定任务的子代理。请专注于完成分配给你的任务，使用合适的工具来达成目标。"
	}
	
	// 设置默认的max_iterations
	if _, exists := processedArgs["max_iterations"]; !exists {
		processedArgs["max_iterations"] = 50
	}
	
	// 默认允许所有工具（不设置allowed_tools限制）

	// 调用executor的ExecuteSubAgentTask方法
	resultInterface, err := t.executor.ExecuteSubAgentTask(ctx, processedArgs)
	if err != nil {
		subAgentLog("ERROR", "Tool execution failed: %v", err)
		return &ToolResult{
			Content: fmt.Sprintf("Sub-agent execution failed: %v", err),
			Data: map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			},
		}, nil // 不返回error，而是在结果中标明失败
	}

	// 类型断言获取结果（需要定义一个通用结果结构）
	resultData, ok := resultInterface.(map[string]interface{})
	if !ok {
		// 尝试使用JSON序列化/反序列化作为后备方案
		resultBytes, err := json.Marshal(resultInterface)
		if err != nil {
			return nil, fmt.Errorf("failed to process sub-agent result: %v", err)
		}
		
		if err := json.Unmarshal(resultBytes, &resultData); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sub-agent result: %v", err)
		}
	}

	// 安全地获取结果字段
	success, _ := resultData["success"].(bool)
	taskCompleted, _ := resultData["task_completed"].(bool)
	result, _ := resultData["result"].(string)
	materialPath, _ := resultData["material_path"].(string)
	sessionID, _ := resultData["session_id"].(string)
	tokensUsed, _ := resultData["tokens_used"].(int)
	duration, _ := resultData["duration_ms"].(int64)
	errorMessage, _ := resultData["error_message"].(string)

	// 构建工具结果
	content := fmt.Sprintf("Sub-agent task completed %s\n\nResult: %s", 
		map[bool]string{true: "successfully", false: "with issues"}[success], 
		result)
	
	if materialPath != "" {
		content += fmt.Sprintf("\n\nMaterial Path: %s", materialPath)
	}
	
	if errorMessage != "" {
		content += fmt.Sprintf("\n\nError Details: %s", errorMessage)
	}

	// 构建数据部分
	data := map[string]interface{}{
		"success":        success,
		"task_completed": taskCompleted,
		"session_id":     sessionID,
		"tokens_used":    tokensUsed,
		"duration_ms":    duration,
	}
	
	if materialPath != "" {
		data["material_path"] = materialPath
	}
	
	if errorMessage != "" {
		data["error_message"] = errorMessage
	}

	return &ToolResult{
		Content: content,
		Data:    data,
		Metadata: map[string]interface{}{
			"tool_type":      "sub_agent",
			"execution_time": duration,
			"tokens_used":    tokensUsed,
		},
	}, nil
}

// Validate - 实现Tool接口
func (t *SubAgentTool) Validate(args map[string]interface{}) error {
	// 验证必需参数
	task, ok := args["task"]
	if !ok {
		return fmt.Errorf("missing required parameter: task")
	}
	
	if taskStr, ok := task.(string); !ok || taskStr == "" {
		return fmt.Errorf("task parameter must be a non-empty string")
	}

	// 验证可选参数
	if maxIter, exists := args["max_iterations"]; exists {
		switch v := maxIter.(type) {
		case int:
			if v < 1 || v > 200 {
				return fmt.Errorf("max_iterations must be between 1 and 200")
			}
		case float64:
			if v < 1 || v > 200 {
				return fmt.Errorf("max_iterations must be between 1 and 200")
			}
		default:
			return fmt.Errorf("max_iterations must be an integer")
		}
	}

	if systemPrompt, exists := args["system_prompt"]; exists {
		if _, ok := systemPrompt.(string); !ok {
			return fmt.Errorf("system_prompt parameter must be a string")
		}
	}

	if allowedTools, exists := args["allowed_tools"]; exists {
		switch v := allowedTools.(type) {
		case []interface{}:
			for i, tool := range v {
				if _, ok := tool.(string); !ok {
					return fmt.Errorf("allowed_tools[%d] must be a string", i)
				}
			}
		case []string:
			// OK
		default:
			return fmt.Errorf("allowed_tools must be an array of strings")
		}
	}

	return nil
}

