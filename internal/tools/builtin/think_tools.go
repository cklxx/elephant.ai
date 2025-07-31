package builtin

import (
	"context"
	"fmt"
	"time"
)

// ThinkTool implements simple thinking assistance
type ThinkTool struct{}

func NewThinkTool() *ThinkTool {
	return &ThinkTool{}
}

func (t *ThinkTool) Name() string {
	return "think"
}

func (t *ThinkTool) Description() string {
	return "Allows the model to think and reason. Tool parameters contain the content to think about, tool result returns the model's thinking process as-is."
}

func (t *ThinkTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "Content or problem for the model to think about",
			},
		},
		"required": []string{"content"},
	}
}

func (t *ThinkTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("content", "Content or problem for the model to think about")

	return validator.Validate(args)
}

func (t *ThinkTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	content, ok := args["content"].(string)
	if !ok {
		return nil, fmt.Errorf("content parameter is required and must be a string")
	}

	return &ToolResult{
		Content: content,
		Data: map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"content":   content,
		},
	}, nil
}
