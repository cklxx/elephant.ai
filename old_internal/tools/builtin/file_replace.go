package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileReplaceTool implements file content replacement functionality
type FileReplaceTool struct{}

func CreateFileReplaceTool() *FileReplaceTool {
	return &FileReplaceTool{}
}

func (t *FileReplaceTool) Name() string {
	return "file_replace"
}

func (t *FileReplaceTool) Description() string {
	return "Write a file to the local filesystem. Overwrites the existing file if there is one."
}

func (t *FileReplaceTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"file_path": map[string]interface{}{
				"type":        "string",
				"description": "The absolute path to the file to write",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the file",
			},
		},
		"required": []string{"file_path", "content"},
	}
}

func (t *FileReplaceTool) Validate(args map[string]interface{}) error {
	validator := NewValidationFramework().
		AddStringField("file_path", "Path to the file").
		AddRequiredStringField("content", "Content to write (can be empty)")

	return validator.Validate(args)
}

func (t *FileReplaceTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// 参数已通过Validate验证，可以安全访问
	filePath := args["file_path"].(string)
	content := args["content"].(string)

	// 解析路径（处理相对路径）
	resolver := GetPathResolverFromContext(ctx)
	resolvedPath := resolver.ResolvePath(filePath)

	// Create parent directories if needed
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	// Check if file exists to determine operation type
	var operation string
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		operation = "created"
	} else {
		operation = "overwritten"
	}

	// Write the content to file (overwrites if exists)
	err := os.WriteFile(resolvedPath, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info after writing
	fileInfo, _ := os.Stat(resolvedPath)
	newLineCount := len(strings.Split(content, "\n"))

	var message string
	if operation == "created" {
		message = fmt.Sprintf("Created %s with %d lines", filePath, newLineCount)
	} else {
		message = fmt.Sprintf("Updated %s with %d lines", filePath, newLineCount)
	}

	return &ToolResult{
		Content: message,
		Files:   []string{resolvedPath},
		Data: map[string]interface{}{
			"file_path":     filePath,
			"resolved_path": resolvedPath,
			"operation":     operation,
			"bytes_written": len(content),
			"lines_total":   newLineCount,
			"modified":      fileInfo.ModTime().Unix(),
			"content":       content,
		},
	}, nil
}
