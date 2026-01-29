package sandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/sandbox"
	"alex/internal/tools/builtin/shared"
)

type sandboxFileReadTool struct {
	client *sandbox.Client
}

type sandboxFileWriteTool struct {
	client *sandbox.Client
}

type sandboxFileListTool struct {
	client *sandbox.Client
}

type sandboxFileSearchTool struct {
	client *sandbox.Client
}

type sandboxFileReplaceTool struct {
	client *sandbox.Client
}

func NewSandboxFileRead(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxFileReadTool{client: newSandboxClient(cfg)}
}

func NewSandboxFileWrite(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxFileWriteTool{client: newSandboxClient(cfg)}
}

func NewSandboxFileList(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxFileListTool{client: newSandboxClient(cfg)}
}

func NewSandboxFileSearch(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxFileSearchTool{client: newSandboxClient(cfg)}
}

func NewSandboxFileReplace(cfg SandboxConfig) tools.ToolExecutor {
	return &sandboxFileReplaceTool{client: newSandboxClient(cfg)}
}

func (t *sandboxFileReadTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "read_file",
		Version:  "0.1.0",
		Category: "files",
		Tags:     []string{"file", "read"},
	}
}

func (t *sandboxFileReadTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "read_file",
		Description: "Read file contents from the local filesystem (absolute paths only).",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":       {Type: "string", Description: "Absolute file path"},
				"start_line": {Type: "integer", Description: "Optional start line (0-based)"},
				"end_line":   {Type: "integer", Description: "Optional end line (exclusive)"},
				"sudo":       {Type: "boolean", Description: "Use sudo privileges"},
			},
			Required: []string{"path"},
		},
	}
}

func (t *sandboxFileReadTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_file_read")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{"file": path}
	if value, ok := intArgOptional(call.Arguments, "start_line"); ok {
		payload["start_line"] = value
	}
	if value, ok := intArgOptional(call.Arguments, "end_line"); ok {
		payload["end_line"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "sudo"); ok {
		payload["sudo"] = value
	}

	var response sandbox.Response[sandbox.FileReadResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/read", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox file read failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox file read returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  response.Data.Content,
		Metadata: map[string]any{"path": response.Data.File},
	}, nil
}

func (t *sandboxFileWriteTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "write_file",
		Version:  "0.1.0",
		Category: "files",
		Tags:     []string{"file", "write"},
	}
}

func (t *sandboxFileWriteTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "write_file",
		Description: "Write content to a file (absolute paths only). Use encoding=base64 for binary data.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":             {Type: "string", Description: "Absolute file path"},
				"content":          {Type: "string", Description: "Text content or base64 payload"},
				"encoding":         {Type: "string", Description: "Content encoding: utf-8 or base64"},
				"append":           {Type: "boolean", Description: "Append to the file instead of overwriting"},
				"leading_newline":  {Type: "boolean", Description: "Add a leading newline (text only)"},
				"trailing_newline": {Type: "boolean", Description: "Add a trailing newline (text only)"},
				"sudo":             {Type: "boolean", Description: "Use sudo privileges"},
			},
			Required: []string{"path", "content"},
		},
	}
}

func (t *sandboxFileWriteTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_file_write")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	content := shared.StringArg(call.Arguments, "content")
	if content == "" {
		err := errors.New("content is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{
		"file":    path,
		"content": content,
	}
	if encoding := strings.TrimSpace(shared.StringArg(call.Arguments, "encoding")); encoding != "" {
		payload["encoding"] = encoding
	}
	if value, ok := boolArgOptional(call.Arguments, "append"); ok {
		payload["append"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "leading_newline"); ok {
		payload["leading_newline"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "trailing_newline"); ok {
		payload["trailing_newline"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "sudo"); ok {
		payload["sudo"] = value
	}

	var response sandbox.Response[sandbox.FileWriteResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/write", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox file write failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox file write returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	bytesWritten := len(content)
	if response.Data.BytesWritten != nil {
		bytesWritten = *response.Data.BytesWritten
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Wrote %d bytes to %s", bytesWritten, response.Data.File),
		Metadata: map[string]any{
			"path":          response.Data.File,
			"bytes_written": bytesWritten,
		},
	}, nil
}

func (t *sandboxFileListTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "list_dir",
		Version:  "0.1.0",
		Category: "files",
		Tags:     []string{"file", "list", "directory"},
	}
}

func (t *sandboxFileListTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "list_dir",
		Description: "List files in a directory (absolute paths only).",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":                {Type: "string", Description: "Absolute directory path"},
				"recursive":           {Type: "boolean", Description: "List recursively"},
				"show_hidden":         {Type: "boolean", Description: "Include hidden files"},
				"file_types":          {Type: "array", Description: "Filter by file extensions", Items: &ports.Property{Type: "string"}},
				"max_depth":           {Type: "integer", Description: "Maximum depth for recursive listing"},
				"include_size":        {Type: "boolean", Description: "Include file size"},
				"include_permissions": {Type: "boolean", Description: "Include permissions"},
				"sort_by":             {Type: "string", Description: "Sort by: name, size, modified, type"},
				"sort_desc":           {Type: "boolean", Description: "Sort in descending order"},
			},
			Required: []string{"path"},
		},
	}
}

func (t *sandboxFileListTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_file_list")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{"path": path}
	if value, ok := boolArgOptional(call.Arguments, "recursive"); ok {
		payload["recursive"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "show_hidden"); ok {
		payload["show_hidden"] = value
	}
	if types := shared.StringSliceArg(call.Arguments, "file_types"); len(types) > 0 {
		payload["file_types"] = types
	}
	if value, ok := intArgOptional(call.Arguments, "max_depth"); ok {
		payload["max_depth"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "include_size"); ok {
		payload["include_size"] = value
	}
	if value, ok := boolArgOptional(call.Arguments, "include_permissions"); ok {
		payload["include_permissions"] = value
	}
	if sortBy := strings.TrimSpace(shared.StringArg(call.Arguments, "sort_by")); sortBy != "" {
		payload["sort_by"] = sortBy
	}
	if value, ok := boolArgOptional(call.Arguments, "sort_desc"); ok {
		payload["sort_desc"] = value
	}

	var response sandbox.Response[sandbox.FileListResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/list", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox file list failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox file list returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payloadJSON, err := json.MarshalIndent(response.Data, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payloadJSON),
		Metadata: map[string]any{"path": response.Data.Path},
	}, nil
}

func (t *sandboxFileSearchTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "search_file",
		Version:  "0.1.0",
		Category: "files",
		Tags:     []string{"file", "search"},
	}
}

func (t *sandboxFileSearchTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "search_file",
		Description: "Search for a regex pattern in a file (absolute paths only).",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"path":  {Type: "string", Description: "Absolute file path"},
				"regex": {Type: "string", Description: "Regex pattern to search"},
				"sudo":  {Type: "boolean", Description: "Use sudo privileges"},
			},
			Required: []string{"path", "regex"},
		},
	}
}

func (t *sandboxFileSearchTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_file_search")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	regex := strings.TrimSpace(shared.StringArg(call.Arguments, "regex"))
	if regex == "" {
		err := errors.New("regex is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payload := map[string]any{
		"file":  path,
		"regex": regex,
	}
	if value, ok := boolArgOptional(call.Arguments, "sudo"); ok {
		payload["sudo"] = value
	}

	var response sandbox.Response[sandbox.FileSearchResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/search", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox file search failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox file search returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	payloadJSON, err := json.MarshalIndent(response.Data, "", "  ")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  string(payloadJSON),
		Metadata: map[string]any{"path": response.Data.File},
	}, nil
}

func (t *sandboxFileReplaceTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "replace_in_file",
		Version:  "0.1.0",
		Category: "files",
		Tags:     []string{"file", "replace", "edit"},
	}
}

func (t *sandboxFileReplaceTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "replace_in_file",
		Description: "Replace exact text in a file (absolute paths only).",
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
	}
}

func (t *sandboxFileReplaceTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path := strings.TrimSpace(shared.StringArg(call.Arguments, "path"))
	if path == "" {
		err := errors.New("path is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !strings.HasPrefix(path, "/") {
		err := errors.New("path must be absolute in sandbox_file_replace")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	oldStr := shared.StringArg(call.Arguments, "old_str")
	if oldStr == "" {
		err := errors.New("old_str is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	newStr := shared.StringArg(call.Arguments, "new_str")

	payload := map[string]any{
		"file":    path,
		"old_str": oldStr,
		"new_str": newStr,
	}
	if value, ok := boolArgOptional(call.Arguments, "sudo"); ok {
		payload["sudo"] = value
	}

	var response sandbox.Response[sandbox.FileReplaceResult]
	if err := t.client.DoJSON(ctx, httpMethodPost, "/v1/file/replace", payload, call.SessionID, &response); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if !response.Success {
		err := fmt.Errorf("sandbox file replace failed: %s", response.Message)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	if response.Data == nil {
		err := errors.New("sandbox file replace returned empty payload")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: fmt.Sprintf("Replaced %d occurrence(s) in %s", response.Data.ReplacedCount, response.Data.File),
		Metadata: map[string]any{
			"path":           response.Data.File,
			"replaced_count": response.Data.ReplacedCount,
		},
	}, nil
}

func boolArgOptional(args map[string]any, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		if trimmed == "" {
			return false, false
		}
		return trimmed == "true" || trimmed == "1" || trimmed == "yes", true
	case int:
		return typed != 0, true
	case int64:
		return typed != 0, true
	case float64:
		return typed != 0, true
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return parsed != 0, true
		}
	}
	return false, false
}

func intArgOptional(args map[string]any, key string) (int, bool) {
	if args == nil {
		return 0, false
	}
	value, ok := args[key]
	if !ok || value == nil {
		return 0, false
	}
	switch typed := value.(type) {
	case int:
		return typed, true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed), true
		}
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, false
		}
		if parsed, err := strconv.Atoi(trimmed); err == nil {
			return parsed, true
		}
	}
	return 0, false
}
