//go:build integration

package integration

import (
	"context"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/tools/builtin/orchestration"
)

func runTeamLikeTool(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	args := call.Arguments
	result, err := orchestration.NewTeamRunner().Run(ctx, orchestration.RunRequest{
		Dispatcher:      agent.GetBackgroundDispatcher(ctx),
		FilePath:        stringArg(args, "file"),
		TemplateName:    stringArg(args, "template"),
		Goal:            stringArg(args, "goal"),
		Wait:            boolArg(args, "wait"),
		Timeout:         time.Duration(intArg(args, "timeout_seconds")) * time.Second,
		Mode:            taskfile.ExecutionMode(stringArg(args, "mode")),
		TaskIDs:         stringSliceArg(args, "task_ids"),
		CausationID:     call.ID,
		SessionID:       call.SessionID,
		TeamDefinitions: agent.GetTeamDefinitions(ctx),
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return &ports.ToolResult{CallID: call.ID}, nil
	}
	return &ports.ToolResult{
		CallID:       call.ID,
		Content:      result.Content,
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
	}, nil
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if raw, ok := args[key]; ok {
		if value, ok := raw.(string); ok {
			return value
		}
	}
	return ""
}

func boolArg(args map[string]any, key string) bool {
	if args == nil {
		return false
	}
	if raw, ok := args[key]; ok {
		if value, ok := raw.(bool); ok {
			return value
		}
	}
	return false
}

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

func stringSliceArg(args map[string]any, key string) []string {
	if args == nil {
		return nil
	}
	raw, ok := args[key]
	if !ok {
		return nil
	}
	switch value := raw.(type) {
	case []string:
		return append([]string(nil), value...)
	case []any:
		out := make([]string, 0, len(value))
		for _, item := range value {
			if text, ok := item.(string); ok && text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}
