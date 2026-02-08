package toolregistry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/pathutil"
)

type legacyAliasExecutor struct {
	aliasName     string
	canonicalName string
	delegate      tools.ToolExecutor
	transform     func(context.Context, map[string]any) (map[string]any, error)
}

func (l *legacyAliasExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	args := call.Arguments
	if args == nil {
		args = map[string]any{}
	}
	translated, err := l.transform(ctx, args)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	forward := call
	forward.Name = l.canonicalName
	forward.Arguments = translated
	result, execErr := l.delegate.Execute(ctx, forward)
	if result != nil {
		if result.Metadata == nil {
			result.Metadata = map[string]any{}
		}
		result.Metadata["legacy_tool_alias"] = l.aliasName
		result.Metadata["canonical_tool"] = l.canonicalName
	}
	return result, execErr
}

func (l *legacyAliasExecutor) Definition() ports.ToolDefinition {
	def := l.delegate.Definition()
	def.Name = l.aliasName
	def.Description = fmt.Sprintf("Legacy compatibility alias for %s. Prefer %s.", l.canonicalName, l.canonicalName)
	return def
}

func (l *legacyAliasExecutor) Metadata() ports.ToolMetadata {
	meta := l.delegate.Metadata()
	meta.Name = l.aliasName
	return meta
}

type legacyFileEditAliasExecutor struct {
	replaceDelegate tools.ToolExecutor
	writeDelegate   tools.ToolExecutor
}

func (l *legacyFileEditAliasExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	path, err := pickPath(ctx, call.Arguments, true, "", "file_path", "path")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	newStr, ok := pickString(call.Arguments, "new_string", "new_str")
	if !ok {
		err := errors.New("new_string is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	oldStr, _ := pickString(call.Arguments, "old_string", "old_str")

	if oldStr == "" {
		forward := call
		forward.Name = "write_file"
		forward.Arguments = map[string]any{
			"path":    path,
			"content": newStr,
		}
		result, execErr := l.writeDelegate.Execute(ctx, forward)
		if result != nil {
			if result.Metadata == nil {
				result.Metadata = map[string]any{}
			}
			result.Metadata["legacy_tool_alias"] = "file_edit"
			result.Metadata["canonical_tool"] = "write_file"
		}
		return result, execErr
	}

	forward := call
	forward.Name = "replace_in_file"
	forward.Arguments = map[string]any{
		"path":    path,
		"old_str": oldStr,
		"new_str": newStr,
	}
	result, execErr := l.replaceDelegate.Execute(ctx, forward)
	if result != nil {
		if result.Metadata == nil {
			result.Metadata = map[string]any{}
		}
		result.Metadata["legacy_tool_alias"] = "file_edit"
		result.Metadata["canonical_tool"] = "replace_in_file"
	}
	return result, execErr
}

func (l *legacyFileEditAliasExecutor) Definition() ports.ToolDefinition {
	def := l.replaceDelegate.Definition()
	def.Name = "file_edit"
	def.Description = "Legacy compatibility alias for replace_in_file/write_file. Prefer replace_in_file or write_file."
	return def
}

func (l *legacyFileEditAliasExecutor) Metadata() ports.ToolMetadata {
	meta := l.replaceDelegate.Metadata()
	meta.Name = "file_edit"
	return meta
}

func (r *Registry) resolveLegacyAliasLocked(name string) tools.ToolExecutor {
	switch strings.TrimSpace(name) {
	case "file_read":
		return r.newLegacyAliasLocked("file_read", "read_file", transformLegacyFileReadArgs)
	case "file_write":
		return r.newLegacyAliasLocked("file_write", "write_file", transformLegacyFileWriteArgs)
	case "list_files":
		return r.newLegacyAliasLocked("list_files", "list_dir", transformLegacyListFilesArgs)
	case "bash":
		return r.newLegacyAliasLocked("bash", "shell_exec", transformLegacyBashArgs)
	case "code_execute":
		return r.newLegacyAliasLocked("code_execute", "execute_code", transformLegacyCodeExecuteArgs)
	case "file_edit":
		replaceTool, ok := r.getRawLocked("replace_in_file")
		if !ok {
			return nil
		}
		writeTool, ok := r.getRawLocked("write_file")
		if !ok {
			return nil
		}
		return &legacyFileEditAliasExecutor{
			replaceDelegate: replaceTool,
			writeDelegate:   writeTool,
		}
	default:
		return nil
	}
}

func (r *Registry) newLegacyAliasLocked(aliasName, canonicalName string, transform func(context.Context, map[string]any) (map[string]any, error)) tools.ToolExecutor {
	delegate, ok := r.getRawLocked(canonicalName)
	if !ok {
		return nil
	}
	return &legacyAliasExecutor{
		aliasName:     aliasName,
		canonicalName: canonicalName,
		delegate:      delegate,
		transform:     transform,
	}
}

func transformLegacyFileReadArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	path, err := pickPath(ctx, args, true, "", "path", "file_path")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"path": path}

	startLine, hasStart := pickInt(args, "start_line", "offset")
	endLine, hasEnd := pickInt(args, "end_line")
	if hasStart {
		out["start_line"] = startLine
	}
	if hasEnd {
		out["end_line"] = endLine
	}
	if !hasEnd {
		if limit, ok := pickInt(args, "limit"); ok && limit > 0 {
			if !hasStart {
				startLine = 0
			}
			out["end_line"] = startLine + limit
		}
	}
	if value, ok := pickBool(args, "sudo"); ok {
		out["sudo"] = value
	}
	return out, nil
}

func transformLegacyFileWriteArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	path, err := pickPath(ctx, args, true, "", "path", "file_path")
	if err != nil {
		return nil, err
	}
	content, ok := pickString(args, "content")
	if !ok {
		return nil, errors.New("content is required")
	}

	out := map[string]any{
		"path":    path,
		"content": content,
	}
	copyKeys(args, out, "encoding", "append", "leading_newline", "trailing_newline", "sudo")
	return out, nil
}

func transformLegacyListFilesArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	path, err := pickPath(ctx, args, false, ".", "path")
	if err != nil {
		return nil, err
	}
	out := map[string]any{"path": path}
	copyKeys(args, out,
		"recursive",
		"show_hidden",
		"file_types",
		"max_depth",
		"include_size",
		"include_permissions",
		"sort_by",
		"sort_desc",
	)
	return out, nil
}

func transformLegacyBashArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	command, ok := pickString(args, "command")
	if !ok {
		return nil, errors.New("command is required")
	}
	out := map[string]any{"command": command}

	if execDir, ok := pickString(args, "exec_dir", "working_dir", "cwd"); ok {
		out["exec_dir"] = ensureAbsolutePath(ctx, execDir)
	}
	copyKeys(args, out, "timeout", "async_mode", "session_id", "attachments", "output_files")
	return out, nil
}

func transformLegacyCodeExecuteArgs(ctx context.Context, args map[string]any) (map[string]any, error) {
	language, ok := pickString(args, "language")
	if !ok {
		return nil, errors.New("language is required")
	}
	out := map[string]any{"language": language}

	if code, ok := pickString(args, "code"); ok {
		out["code"] = code
	}
	if codePath, ok := pickString(args, "code_path"); ok {
		out["code_path"] = ensureAbsolutePath(ctx, codePath)
	}
	if execDir, ok := pickString(args, "exec_dir", "working_dir", "cwd"); ok {
		out["exec_dir"] = ensureAbsolutePath(ctx, execDir)
	}
	copyKeys(args, out, "timeout", "attachments", "output_files")
	return out, nil
}

func pickPath(ctx context.Context, args map[string]any, required bool, defaultPath string, keys ...string) (string, error) {
	value, ok := pickString(args, keys...)
	if !ok {
		value = defaultPath
	}
	if strings.TrimSpace(value) == "" {
		if required {
			return "", fmt.Errorf("%s is required", keys[0])
		}
		value = "."
	}
	return ensureAbsolutePath(ctx, value), nil
}

func ensureAbsolutePath(ctx context.Context, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	if filepath.IsAbs(trimmed) {
		return trimmed
	}
	return pathutil.GetPathResolverFromContext(ctx).ResolvePath(trimmed)
}

func pickString(args map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if args == nil {
			continue
		}
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case string:
			return typed, true
		}
	}
	return "", false
}

func pickInt(args map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if args == nil {
			continue
		}
		value, ok := args[key]
		if !ok || value == nil {
			continue
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
				continue
			}
			if parsed, err := strconv.Atoi(trimmed); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

func pickBool(args map[string]any, keys ...string) (bool, bool) {
	for _, key := range keys {
		if args == nil {
			continue
		}
		value, ok := args[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed, true
		case int:
			return typed != 0, true
		case int64:
			return typed != 0, true
		case float64:
			return typed != 0, true
		case string:
			trimmed := strings.ToLower(strings.TrimSpace(typed))
			if trimmed == "" {
				continue
			}
			if trimmed == "1" || trimmed == "true" || trimmed == "yes" {
				return true, true
			}
			if trimmed == "0" || trimmed == "false" || trimmed == "no" {
				return false, true
			}
		}
	}
	return false, false
}

func copyKeys(src, dst map[string]any, keys ...string) {
	for _, key := range keys {
		value, ok := src[key]
		if !ok {
			continue
		}
		dst[key] = value
	}
}
