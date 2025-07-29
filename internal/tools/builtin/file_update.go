package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	
	"alex/internal/utils"
)

// FileUpdateTool implements file content updating functionality
type FileUpdateTool struct{}

func CreateFileUpdateTool() *FileUpdateTool {
	return &FileUpdateTool{}
}

func (t *FileUpdateTool) Name() string {
	return "file_edit"
}

func (t *FileUpdateTool) Description() string {
	return `Edit files by replacing specific text content with exact string matching.

Usage:
- For editing existing files: Provide unique old_string that exists in the file
- For creating new files: Use empty old_string ("") and provide new_string content
- Returns unified diff showing changes made to the file
- Automatically creates parent directories when creating new files

Parameters:
- file_path: Path to the file to modify (relative paths are resolved)
- old_string: Text to replace (must be unique in file, empty for new file)
- new_string: Replacement text content

File Creation (old_string = ""):
- Creates new file with new_string as content
- Fails if file already exists
- Creates parent directories automatically

File Editing (old_string not empty):
- Finds and replaces exact match of old_string
- Fails if old_string not found or appears multiple times
- Shows unified diff of changes

Security:
- Uses exact string matching (not regex)
- old_string must appear exactly once in the file for safety
- Preserves file permissions (644 for new files)

Example workflow:
1. Use file_read to examine current content
2. Copy exact text for old_string (including whitespace)  
3. Provide replacement text as new_string
4. Tool shows diff and updates file`
}

func (t *FileUpdateTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "The absolute path to the file to modify",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The text to replace (empty for new file)",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The text to replace with",
			},
		},
		"required": []string{"file_path", "old_string", "new_string"},
	}
}

func (t *FileUpdateTool) Validate(args map[string]any) error {
	validator := NewValidationFramework().
		AddStringField("file_path", "Path to the file").
		AddRequiredStringField("old_string", "Text to replace (empty for new file)").
		AddStringField("new_string", "Replacement text")

	return validator.Validate(args)
}

func (t *FileUpdateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	// 参数已通过Validate验证，可以安全访问
	filePath := args["file_path"].(string)
	newString := args["new_string"].(string)
	
	oldString := ""
	if os, ok := args["old_string"]; ok {
		oldString = os.(string)
	}

	// 解析路径（处理相对路径）
	resolver := GetPathResolverFromContext(ctx)
	resolvedPath := resolver.ResolvePath(filePath)

	// Handle new file creation case (empty old_string)
	if oldString == "" {
		// Create parent directories if needed
		dir := filepath.Dir(resolvedPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create directories: %w", err)
		}

		// Check if file already exists
		if _, err := os.Stat(resolvedPath); err == nil {
			return nil, fmt.Errorf("file already exists: %s", filePath)
		}

		// Write new file
		err := os.WriteFile(resolvedPath, []byte(newString), 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}

		fileInfo, _ := os.Stat(resolvedPath)
		
		// Generate diff data for CLI display
		diff := utils.GenerateUnifiedDiff("", newString, filePath, utils.DefaultDiffOptions)
		
		return &ToolResult{
			Content: fmt.Sprintf("Created %s (%d lines)", filePath, len(strings.Split(newString, "\n"))),
			Files:   []string{resolvedPath},
			Data: map[string]any{
				"file_path":     filePath,
				"resolved_path": resolvedPath,
				"operation":     "created", 
				"bytes_written": len(newString),
				"lines_total":   len(strings.Split(newString, "\n")),
				"modified":      fileInfo.ModTime().Unix(),
				"diff":          diff,
			},
		}, nil
	}

	// Check if file exists for editing
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	// Read file content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	originalContent := string(content)

	// Check for uniqueness of old_string
	occurrences := strings.Count(originalContent, oldString)
	if occurrences == 0 {
		return nil, fmt.Errorf("old_string not found in file")
	}
	if occurrences > 1 {
		return nil, fmt.Errorf("old_string appears %d times in file. Please include more context to make it unique", occurrences)
	}

	// Perform the replacement (only one occurrence)
	newContent := strings.Replace(originalContent, oldString, newString, 1)

	// Generate diff data for CLI display
	diff := utils.GenerateUnifiedDiff(originalContent, newContent, filePath, utils.DefaultDiffOptions)
	
	// Write the modified content
	err = os.WriteFile(resolvedPath, []byte(newContent), 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write file: %w", err)
	}

	// Get file info after writing
	fileInfo, _ := os.Stat(resolvedPath)
	newLineCount := len(strings.Split(newContent, "\n"))

	return &ToolResult{
		Content: fmt.Sprintf("Updated %s (%d lines)", filePath, newLineCount),
		Files:   []string{resolvedPath},
		Data: map[string]any{
			"file_path":         filePath,
			"resolved_path":     resolvedPath,
			"operation":         "edited",
			"lines_total":       newLineCount,
			"modified":          fileInfo.ModTime().Unix(),
			"diff":              diff,
		},
	}, nil
}