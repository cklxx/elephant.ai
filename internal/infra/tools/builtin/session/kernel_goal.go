package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

const (
	kernelGoalDefaultKernelID = "default"
	kernelGoalDefaultRoot     = "~/.alex/kernel"
	kernelGoalFileName        = "GOAL.md"
)

var kernelIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type kernelGoalTool struct {
	shared.BaseTool
}

// NewKernelGoal creates a tool for reading/updating kernel mission goals.
func NewKernelGoal() tools.ToolExecutor {
	return &kernelGoalTool{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "kernel_goal",
				Description: `Read or update kernel mission goals.

Use action=get to inspect current goal, action=set to update goal content.
Goal is stored in ~/.alex/kernel/{kernel_id}/GOAL.md.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"action": {
							Type:        "string",
							Description: "get|set",
							Enum:        []any{"get", "set"},
						},
						"kernel_id": {
							Type:        "string",
							Description: "Kernel ID (default: default).",
						},
						"goal": {
							Type:        "string",
							Description: "Goal markdown content for action=set.",
						},
					},
					Required: []string{"action"},
				},
			},
			ports.ToolMetadata{
				Name:     "kernel_goal",
				Version:  "1.0.0",
				Category: "meta",
				Tags:     []string{"kernel", "goal", "alignment", "system_prompt"},
			},
		),
	}
}

func (t *kernelGoalTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action, _ := call.Arguments["action"].(string)
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		return shared.ToolError(call.ID, "action is required")
	}

	kernelIDRaw, _ := call.Arguments["kernel_id"].(string)
	kernelID, err := normalizeKernelID(kernelIDRaw)
	if err != nil {
		return shared.ToolError(call.ID, "%s", err.Error())
	}
	path := kernelGoalPath(kernelID)

	switch action {
	case "get":
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return &ports.ToolResult{
					CallID:  call.ID,
					Content: "(empty)",
					Metadata: map[string]any{
						"kernel_id": kernelID,
						"path":      path,
						"exists":    false,
					},
				}, nil
			}
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: string(content),
			Metadata: map[string]any{
				"kernel_id": kernelID,
				"path":      path,
				"exists":    true,
			},
		}, nil

	case "set":
		goal, _ := call.Arguments["goal"].(string)
		goal = strings.TrimSpace(goal)
		if goal == "" {
			return shared.ToolError(call.ID, "goal is required for action=set")
		}
		if err := writeAtomic(path, goal+"\n"); err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "kernel goal updated",
			Metadata: map[string]any{
				"kernel_id": kernelID,
				"path":      path,
				"bytes":     len(goal) + 1,
			},
		}, nil

	default:
		return shared.ToolError(call.ID, "unsupported action %q", action)
	}
}

func normalizeKernelID(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return kernelGoalDefaultKernelID, nil
	}
	if !kernelIDPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid kernel_id %q: only letters, digits, dot, dash, underscore allowed", raw)
	}
	return trimmed, nil
}

func kernelGoalPath(kernelID string) string {
	root := resolveHome(kernelGoalDefaultRoot)
	return filepath.Join(root, kernelID, kernelGoalFileName)
}

func resolveHome(path string) string {
	trimmed := strings.TrimSpace(path)
	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}

func writeAtomic(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir kernel goal dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write kernel goal tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename kernel goal: %w", err)
	}
	return nil
}
