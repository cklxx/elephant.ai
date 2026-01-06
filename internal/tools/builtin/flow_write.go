package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/llm"
)

type flowWrite struct {
	llm ports.LLMClient
}

func NewFlowWrite(client ports.LLMClient) ports.ToolExecutor {
	if client == nil {
		client = llm.NewMockClient()
	}
	return &flowWrite{llm: client}
}

func (t *flowWrite) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "flow_write",
		Version:  "1.0.0",
		Category: "writing",
		Tags:     []string{"flow", "writing", "draft"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Produces: []string{"text/plain"},
		},
	}
}

func (t *flowWrite) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "flow_write",
		Description: "Apply a flow-mode writing action (continue, polish, outline, tighten, examples) to a draft and return updated text plus attachments.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"action": {
					Type:        "string",
					Description: "Action to perform (continue, polish, bullets, outline, tighten, examples)",
				},
				"draft": {
					Type:        "string",
					Description: "Draft text to transform",
				},
				"notes": {
					Type:        "string",
					Description: "Optional extra guidance for the action",
				},
				"attachment_name": {
					Type:        "string",
					Description: "Optional attachment filename for the transformed draft (default: flow_write.txt)",
				},
			},
			Required: []string{"action", "draft"},
		},
	}
}

func (t *flowWrite) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	action := strings.ToLower(strings.TrimSpace(stringArg(call.Arguments, "action")))
	draft := strings.TrimSpace(stringArg(call.Arguments, "draft"))
	if action == "" || draft == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "action and draft are required for flow_write",
			Error:   fmt.Errorf("missing action or draft"),
		}, nil
	}

	pattern := writingPatterns()[action]
	if pattern == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("unsupported action: %s", action),
			Error:   fmt.Errorf("unsupported action"),
		}, nil
	}

	notes := strings.TrimSpace(stringArg(call.Arguments, "notes"))
	attachmentName := strings.TrimSpace(stringArg(call.Arguments, "attachment_name"))
	if attachmentName == "" {
		attachmentName = "flow_write.txt"
	}

	userPrompt := buildWritePrompt(action, pattern, notes, draft)
	resp, err := t.llm.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "你是写作搭档，专注快速改写和补全草稿。"},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.4,
		MaxTokens:   800,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("flow_write failed: %v", err),
			Error:   err,
		}, nil
	}

	output := strings.TrimSpace(resp.Content)
	attachments := map[string]ports.Attachment{
		attachmentName: {
			Name:        attachmentName,
			MediaType:   "text/plain",
			Data:        base64.StdEncoding.EncodeToString([]byte(output)),
			Source:      "flow_write",
			Description: fmt.Sprintf("Result of action %s", action),
			Kind:        "artifact",
			Format:      "txt",
		},
		"draft.txt": {
			Name:        "draft.txt",
			MediaType:   "text/plain",
			Data:        base64.StdEncoding.EncodeToString([]byte(draft)),
			Source:      "flow_write",
			Description: "Original draft provided to flow_write",
			Kind:        "artifact",
			Format:      "txt",
		},
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     output,
		Attachments: attachments,
		Metadata: map[string]any{
			"action": action,
			"notes":  notes,
		},
	}, nil
}

func buildWritePrompt(action, pattern, notes, draft string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("动作: %s", action))
	parts = append(parts, fmt.Sprintf("操作说明: %s", pattern))
	if notes != "" {
		parts = append(parts, fmt.Sprintf("补充要求: %s", notes))
	}
	parts = append(parts, "请直接输出处理后的正文，不要解释。")
	parts = append(parts, "草稿如下：")
	parts = append(parts, draft)
	return strings.Join(parts, "\n")
}

func writingPatterns() map[string]string {
	return map[string]string{
		"continue": "延续当前语气续写 1 段，保持结构与论点连贯。",
		"polish":   "润色压缩表达，去除冗余，强化节奏与逻辑。",
		"bullets":  "提炼 4-6 条要点或行动项，便于继续写作。",
		"outline":  "生成分层提纲，标明关键论点、佐证与过渡。",
		"tighten":  "在保留主线的前提下，将文字压缩到约一半字数。",
		"examples": "为核心论点补充 2-3 个可引用的案例或数据，并注明出处或来源。",
	}
}
