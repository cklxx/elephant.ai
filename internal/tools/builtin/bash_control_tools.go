package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// BashStatusTool provides status information for background bash commands
type BashStatusTool struct{}

func CreateBashStatusTool() *BashStatusTool {
	return &BashStatusTool{}
}

func (t *BashStatusTool) Name() string {
	return "bash_status"
}

func (t *BashStatusTool) Description() string {
	return `Check the status and progress of background bash commands.

This tool allows you to monitor background commands started with the bash tool in background mode.

Usage:
- Query status of specific background command by execution_id
- List all active background commands
- View recent output and execution statistics
- Monitor command progress and resource usage

Parameters:
- execution_id: ID of the background command to check (optional - if not provided, lists all commands)
- show_output: Whether to include recent output in the result (default: true)
- output_lines: Number of recent output lines to show (default: 10, max: 50)

Output Information:
- Command status (running, completed, failed, timed_out, killed)
- Execution time and start time
- Output line count and size statistics
- Recent command output (configurable)
- Last activity timestamp

Status Types:
- running: Command is currently executing
- completed: Command finished successfully
- failed: Command terminated with error
- timed_out: Command exceeded timeout limit
- killed: Command was manually terminated

Example Usage:
- bash_status {"execution_id": "bg_1234_5678"} - Check specific command
- bash_status {} - List all background commands
- bash_status {"execution_id": "bg_1234_5678", "output_lines": 20} - Show more output

This tool is essential for monitoring long-running background processes and understanding their current state.`
}

func (t *BashStatusTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"execution_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the background command to check (optional)",
			},
			"show_output": map[string]interface{}{
				"type":        "boolean",
				"description": "Whether to include recent output in the result",
				"default":     true,
			},
			"output_lines": map[string]interface{}{
				"type":        "integer", 
				"description": "Number of recent output lines to show",
				"default":     10,
				"minimum":     1,
				"maximum":     50,
			},
		},
		"required": []string{},
	}
}

func (t *BashStatusTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddOptionalStringField("execution_id", "ID of the background command to check").
		AddOptionalBooleanField("show_output", "Whether to include recent output in the result").
		AddOptionalIntField("output_lines", "Number of recent output lines to show", 1, 50)

	return validator.Validate(args)
}

func (t *BashStatusTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	mgr := GetBackgroundCommandManager()
	
	// Check if specific execution_id is provided
	if execIDValue, ok := args["execution_id"]; ok && execIDValue != nil {
		execID, ok := execIDValue.(string)
		if !ok {
			return nil, fmt.Errorf("execution_id must be a string")
		}
		
		if execID == "" {
			return nil, fmt.Errorf("execution_id cannot be empty")
		}
		
		return t.getCommandStatus(mgr, execID, args)
	}
	
	// List all background commands
	return t.listAllCommands(mgr, args)
}

func (t *BashStatusTool) getCommandStatus(mgr *BackgroundCommandManager, execID string, args map[string]interface{}) (*ToolResult, error) {
	bgCmd := mgr.Get(execID)
	if bgCmd == nil {
		return &ToolResult{
			Content: fmt.Sprintf("❌ 未找到执行ID: %s", execID),
			Data: map[string]interface{}{
				"found": false,
			},
		}, nil
	}
	
	// Get parameters
	showOutput := true
	if showOutputArg, ok := args["show_output"]; ok {
		showOutput, _ = showOutputArg.(bool)
	}
	
	outputLines := 10
	if outputLinesArg, ok := args["output_lines"]; ok {
		if outputLinesFloat, ok := outputLinesArg.(float64); ok {
			outputLines = int(outputLinesFloat)
		}
	}
	
	stats := bgCmd.GetStats()
	var content strings.Builder
	
	// Status header
	statusEmoji := t.getStatusEmoji(bgCmd.Status)
	content.WriteString(fmt.Sprintf("%s 命令状态: %s\n", statusEmoji, bgCmd.Status))
	content.WriteString(fmt.Sprintf("🆔 执行ID: %s\n", bgCmd.ID))
	content.WriteString(fmt.Sprintf("📝 命令: %s\n", bgCmd.Command))
	
	// Timing information
	content.WriteString(fmt.Sprintf("⏰ 开始时间: %s\n", bgCmd.StartTime.Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("⏱️ 运行时间: %v\n", stats.ExecutionTime.Truncate(time.Second)))
	
	// Output statistics
	content.WriteString(fmt.Sprintf("📊 输出行数: %d\n", stats.OutputLines))
	content.WriteString(fmt.Sprintf("💾 输出大小: %d bytes\n", stats.OutputSize))
	
	if !stats.LastActivity.IsZero() {
		content.WriteString(fmt.Sprintf("🔄 最后活动: %s\n", stats.LastActivity.Format("15:04:05")))
	}
	
	// Working directory
	if bgCmd.WorkingDir != "" {
		content.WriteString(fmt.Sprintf("📂 工作目录: %s\n", bgCmd.WorkingDir))
	}
	
	// Recent output
	if showOutput && stats.OutputLines > 0 {
		recentLines := bgCmd.progressDisplay.outputBuffer.GetRecentLines(outputLines)
		if len(recentLines) > 0 {
			content.WriteString(fmt.Sprintf("\n📄 最近 %d 行输出:\n", len(recentLines)))
			for i, line := range recentLines {
				content.WriteString(fmt.Sprintf("%3d: %s\n", i+1, line))
			}
		}
	}
	
	// Timeout decision information
	if pending, message := bgCmd.GetTimeoutDecision(); pending {
		content.WriteString(fmt.Sprintf("\n🚨 超时决策等待中:\n%s\n", message))
	}
	
	return &ToolResult{
		Content: content.String(),
		Data: map[string]interface{}{
			"found":          true,
			"execution_id":   bgCmd.ID,
			"command":        bgCmd.Command,
			"status":         string(bgCmd.Status),
			"start_time":     bgCmd.StartTime.Format(time.RFC3339),
			"execution_time": stats.ExecutionTime.String(),
			"output_lines":   stats.OutputLines,
			"output_size":    stats.OutputSize,
			"last_activity":  stats.LastActivity.Format(time.RFC3339),
			"working_dir":    bgCmd.WorkingDir,
		},
	}, nil
}

func (t *BashStatusTool) listAllCommands(mgr *BackgroundCommandManager, args map[string]interface{}) (*ToolResult, error) {
	commands := mgr.List()
	
	if len(commands) == 0 {
		return &ToolResult{
			Content: "📭 当前没有后台命令在运行或记录中",
			Data: map[string]interface{}{
				"count": 0,
			},
		}, nil
	}
	
	var content strings.Builder
	content.WriteString(fmt.Sprintf("📋 后台命令列表 (%d 个):\n\n", len(commands)))
	
	runningCount := 0
	for _, cmd := range commands {
		stats := cmd.GetStats()
		statusEmoji := t.getStatusEmoji(cmd.Status)
		
		if cmd.Status == StatusRunning {
			runningCount++
		}
		
		content.WriteString(fmt.Sprintf("%s [%s] %s\n", statusEmoji, cmd.ID[:8], cmd.Status))
		content.WriteString(fmt.Sprintf("   📝 %s\n", cmd.Command))
		content.WriteString(fmt.Sprintf("   ⏱️ 运行时间: %v | 输出行数: %d\n", 
			stats.ExecutionTime.Truncate(time.Second), stats.OutputLines))
		content.WriteString("\n")
	}
	
	if runningCount > 0 {
		content.WriteString(fmt.Sprintf("🔄 %d 个命令正在运行中\n", runningCount))
		content.WriteString("💡 使用 bash_status {\"execution_id\": \"具体ID\"} 查看详细信息\n")
	}
	
	return &ToolResult{
		Content: content.String(),
		Data: map[string]interface{}{
			"count":         len(commands),
			"running_count": runningCount,
			"commands":      t.buildCommandsData(commands),
		},
	}, nil
}

func (t *BashStatusTool) buildCommandsData(commands []*BackgroundCommand) []map[string]interface{} {
	var result []map[string]interface{}
	
	for _, cmd := range commands {
		stats := cmd.GetStats()
		result = append(result, map[string]interface{}{
			"execution_id":   cmd.ID,
			"command":        cmd.Command,
			"status":         string(cmd.Status),
			"start_time":     cmd.StartTime.Format(time.RFC3339),
			"execution_time": stats.ExecutionTime.String(),
			"output_lines":   stats.OutputLines,
		})
	}
	
	return result
}

func (t *BashStatusTool) getStatusEmoji(status CommandStatus) string {
	switch status {
	case StatusRunning:
		return "🔄"
	case StatusCompleted:
		return "✅"
	case StatusFailed:
		return "❌"
	case StatusTimedOut:
		return "⏰"
	case StatusKilled:
		return "🛑"
	default:
		return "❓"
	}
}

// BashControlTool provides control operations for background bash commands
type BashControlTool struct{}

func CreateBashControlTool() *BashControlTool {
	return &BashControlTool{}
}

func (t *BashControlTool) Name() string {
	return "bash_control"
}

func (t *BashControlTool) Description() string {
	return `Control background bash commands (terminate, extend timeout, get output).

This tool provides control operations for background commands started with the bash tool in background mode.

Available Actions:
- terminate: Forcefully stop a running background command
- extend_timeout: Extend the timeout for a running command by specified seconds
- get_full_output: Retrieve the complete output from a command (running or finished)

Usage:
- Use this tool when you need to control long-running background processes
- Essential for managing timeouts and getting complete output
- Provides clean termination of runaway processes

Parameters:
- execution_id: ID of the background command to control (required)
- action: Control action to perform (required: "terminate", "extend_timeout", "get_full_output")
- seconds: Number of seconds to extend timeout (required for "extend_timeout" action)

Action Details:
- terminate: Sends interrupt signal first, then force-kills if needed
- extend_timeout: Adds specified seconds to the current timeout
- get_full_output: Returns complete output without length limits

Examples:
- bash_control {"execution_id": "bg_1234_5678", "action": "terminate"}
- bash_control {"execution_id": "bg_1234_5678", "action": "extend_timeout", "seconds": 300}
- bash_control {"execution_id": "bg_1234_5678", "action": "get_full_output"}

Safety Features:
- Graceful termination attempts before force-kill
- Validates command existence before operations
- Prevents invalid timeout extensions

This tool is crucial for managing background processes and recovering from timeout situations.`
}

func (t *BashControlTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"execution_id": map[string]interface{}{
				"type":        "string",
				"description": "ID of the background command to control",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Control action to perform",
				"enum":        []string{"terminate", "extend_timeout", "get_full_output"},
			},
			"seconds": map[string]interface{}{
				"type":        "integer",
				"description": "Number of seconds to extend timeout (required for extend_timeout)",
				"minimum":     1,
				"maximum":     3600, // Max 1 hour extension
			},
		},
		"required": []string{"execution_id", "action"},
	}
}

func (t *BashControlTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("execution_id", "ID of the background command to control").
		AddCustomValidator("action", "Control action to perform", true, func(value interface{}) error {
			action, ok := value.(string)
			if !ok {
				return fmt.Errorf("action must be a string")
			}
			validActions := []string{"terminate", "extend_timeout", "get_full_output"}
			for _, validAction := range validActions {
				if action == validAction {
					return nil
				}
			}
			return fmt.Errorf("invalid action: %s", action)
		}).
		AddOptionalIntField("seconds", "Number of seconds to extend timeout", 1, 3600)

	// First run standard validation
	if err := validator.Validate(args); err != nil {
		return err
	}

	// Check if seconds is required for extend_timeout
	action := args["action"].(string)
	if action == "extend_timeout" {
		if _, exists := args["seconds"]; !exists {
			return fmt.Errorf("seconds parameter is required for extend_timeout action")
		}
	}

	return nil
}

func (t *BashControlTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	execID := args["execution_id"].(string)
	action := args["action"].(string)
	
	mgr := GetBackgroundCommandManager()
	bgCmd := mgr.Get(execID)
	
	if bgCmd == nil {
		return &ToolResult{
			Content: fmt.Sprintf("❌ 未找到执行ID: %s", execID),
			Data: map[string]interface{}{
				"success": false,
				"error":   "command not found",
			},
		}, nil
	}
	
	switch action {
	case "terminate":
		return t.terminateCommand(bgCmd)
	case "extend_timeout":
		var seconds int
		switch v := args["seconds"].(type) {
		case float64:
			seconds = int(v)
		case int:
			seconds = v
		default:
			return nil, fmt.Errorf("seconds must be a number, got %T", v)
		}
		return t.extendTimeout(bgCmd, seconds)
	case "get_full_output":
		return t.getFullOutput(bgCmd)
	default:
		return nil, fmt.Errorf("unsupported action: %s", action)
	}
}

func (t *BashControlTool) terminateCommand(bgCmd *BackgroundCommand) (*ToolResult, error) {
	if bgCmd.Status != StatusRunning {
		return &ToolResult{
			Content: fmt.Sprintf("⚠️ 命令 [%s] 当前状态为 %s，无需终止", bgCmd.ID[:8], bgCmd.Status),
			Data: map[string]interface{}{
				"success": false,
				"reason":  "command not running",
				"status":  string(bgCmd.Status),
			},
		}, nil
	}
	
	err := bgCmd.Terminate()
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("❌ 终止命令失败: %v", err),
			Data: map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			},
		}, nil
	}
	
	// Clear timeout decision state since user has made a decision
	bgCmd.ClearTimeoutDecision()
	
	return &ToolResult{
		Content: fmt.Sprintf("🛑 命令 [%s] 已被终止\n✅ 超时决策已处理", bgCmd.ID[:8]),
		Data: map[string]interface{}{
			"success":      true,
			"execution_id": bgCmd.ID,
			"action":       "terminated",
		},
	}, nil
}

func (t *BashControlTool) extendTimeout(bgCmd *BackgroundCommand, seconds int) (*ToolResult, error) {
	if bgCmd.Status != StatusRunning {
		return &ToolResult{
			Content: fmt.Sprintf("⚠️ 命令 [%s] 当前状态为 %s，无法延长超时", bgCmd.ID[:8], bgCmd.Status),
			Data: map[string]interface{}{
				"success": false,
				"reason":  "command not running",
				"status":  string(bgCmd.Status),
			},
		}, nil
	}
	
	duration := time.Duration(seconds) * time.Second
	bgCmd.ExtendTimeout(duration)
	
	// Clear timeout decision state since user has made a decision
	bgCmd.ClearTimeoutDecision()
	
	return &ToolResult{
		Content: fmt.Sprintf("⏱️ 命令 [%s] 超时已延长 %v\n✅ 超时决策已处理，命令继续执行", bgCmd.ID[:8], duration),
		Data: map[string]interface{}{
			"success":        true,
			"execution_id":   bgCmd.ID,
			"action":         "timeout_extended",
			"extended_by":    duration.String(),
			"extended_seconds": seconds,
		},
	}, nil
}

func (t *BashControlTool) getFullOutput(bgCmd *BackgroundCommand) (*ToolResult, error) {
	fullOutput := bgCmd.GetOutput()
	stats := bgCmd.GetStats()
	
	if fullOutput == "" {
		return &ToolResult{
			Content: fmt.Sprintf("📭 命令 [%s] 暂无输出", bgCmd.ID[:8]),
			Data: map[string]interface{}{
				"execution_id": bgCmd.ID,
				"output":       "",
				"output_lines": 0,
			},
		}, nil
	}
	
	// Prepare content with header
	var content strings.Builder
	content.WriteString(fmt.Sprintf("📄 命令 [%s] 完整输出:\n", bgCmd.ID[:8]))
	content.WriteString(fmt.Sprintf("📊 状态: %s | 总行数: %d | 大小: %d bytes\n", 
		bgCmd.Status, stats.OutputLines, stats.OutputSize))
	content.WriteString("=" + strings.Repeat("=", 50) + "\n")
	content.WriteString(fullOutput)
	
	if bgCmd.Status == StatusRunning {
		content.WriteString("\n" + strings.Repeat("=", 50))
		content.WriteString("\n🔄 命令仍在运行中，以上为当前输出")
	}
	
	return &ToolResult{
		Content: content.String(),
		Data: map[string]interface{}{
			"execution_id":   bgCmd.ID,
			"status":         string(bgCmd.Status),
			"full_output":    fullOutput,
			"output_lines":   stats.OutputLines,
			"output_size":    stats.OutputSize,
			"execution_time": stats.ExecutionTime.String(),
		},
	}, nil
}