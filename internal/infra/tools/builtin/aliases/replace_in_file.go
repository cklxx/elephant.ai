package aliases

import (
	"context"
	"fmt"
	"os"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

type replaceInFile struct {
	shared.BaseTool
}

func NewReplaceInFile(cfg shared.FileToolConfig) tools.ToolExecutor {
	_ = cfg
	return &replaceInFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "replace_in_file",
				Description: "Apply an exact in-place patch/hotfix to an existing file (absolute paths only). Use only for surgical code/text edits when target text is already known (requires old_str and new_str). Do not use for read-only inspection/extraction (use read_file), search/investigation, creating new files, artifact deletion/cleanup, listing/inventory, or clarification questions.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path":    {Type: "string", Description: "Absolute file path"},
						"old_str": {Type: "string", Description: "Original string to replace"},
						"new_str": {Type: "string", Description: "Replacement string"},
						"sudo":    {Type: "boolean", Description: "Use sudo privileges"},
					},
					Required: []string{"path", "old_str", "new_str"},
				},
			},
			ports.ToolMetadata{
				Name:     "replace_in_file",
				Version:  "0.1.0",
				Category: "files",
				Tags:     []string{"file", "replace", "patch", "hotfix", "inplace", "edit_existing", "modify"},
			},
		),
	}
}

func (t *replaceInFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		return shared.ToolError(call.ID, "path is required")
	}
	oldStr := shared.StringArg(call.Arguments, "old_str")
	if oldStr == "" {
		return shared.ToolError(call.ID, "old_str is required")
	}
	newStr := shared.StringArg(call.Arguments, "new_str")

	resolved, err := pathutil.ResolveLocalPath(ctx, path)
	if err != nil {
		return shared.ToolError(call.ID, "%w", err)
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		return shared.ToolError(call.ID, "%w", err)
	}

	original := string(content)
	count := strings.Count(original, oldStr)
	if count == 0 {
		return shared.ToolError(call.ID, "old_str not found in file")
	}

	updated := strings.ReplaceAll(original, oldStr, newStr)
	if err := os.WriteFile(resolved, []byte(updated), 0o644); err != nil {
		return shared.ToolError(call.ID, "%w", err)
	}

	result := &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Replaced %d occurrence(s) in %s", count, resolved),
		Metadata: map[string]any{
			"path":           resolved,
			"replaced_count": count,
		},
	}

	attachments, errs := autoUploadFile(ctx, resolved)
	if len(attachments) > 0 {
		result.Attachments = attachments
	}
	if len(errs) > 0 {
		result.Metadata["attachment_errors"] = errs
	}

	return result, nil
}
