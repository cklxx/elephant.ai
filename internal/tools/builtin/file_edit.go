package builtin

import (
	"alex/internal/agent/ports"
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type fileEdit struct {
}

func NewFileEdit(cfg FileToolConfig) ports.ToolExecutor {
	_ = cfg
	return &fileEdit{}
}

func (t *fileEdit) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Extract parameters
	filePath, ok := call.Arguments["file_path"].(string)
	if !ok || filePath == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing or invalid 'file_path'")}, nil
	}

	newString, ok := call.Arguments["new_string"].(string)
	if !ok {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("missing 'new_string'")}, nil
	}

	oldString := ""
	if os, ok := call.Arguments["old_string"]; ok {
		oldString, _ = os.(string)
	}

	resolvedPath, err := resolveLocalPath(ctx, filePath)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	// Handle new file creation case (empty old_string)
	if oldString == "" {
		return t.createNewFile(call.ID, filePath, resolvedPath, newString)
	}

	// Handle file editing case
	return t.editExistingFile(call.ID, filePath, resolvedPath, oldString, newString)
}

// createNewFile handles creating a new file with the provided content
func (t *fileEdit) createNewFile(callID, filePath, resolvedPath, content string) (*ports.ToolResult, error) {
	// Create parent directories if needed
	dir := filepath.Dir(resolvedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("failed to create directories: %w", err)}, nil
	}

	// Check if file already exists
	if _, err := os.Stat(resolvedPath); err == nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("file already exists: %s", filePath)}, nil
	}

	// Write new file
	if err := os.WriteFile(resolvedPath, []byte(content), 0644); err != nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("failed to create file: %w", err)}, nil
	}

	fileInfo, _ := os.Stat(resolvedPath)
	lineCount := len(strings.Split(content, "\n"))

	// Generate diff for new file
	diff := generateUnifiedDiff("", content, filePath)
	sum := sha256.Sum256([]byte(content))

	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Created %s (%d lines)", filePath, lineCount),
		Metadata: map[string]any{
			"file_path":      filePath,
			"resolved_path":  resolvedPath,
			"operation":      "created",
			"bytes_written":  len(content),
			"lines_total":    lineCount,
			"modified":       fileInfo.ModTime().Unix(),
			"diff":           diff,
			"content_len":    len(content),
			"content_sha256": fmt.Sprintf("%x", sum),
		},
	}, nil
}

// editExistingFile handles editing an existing file with string replacement
func (t *fileEdit) editExistingFile(callID, filePath, resolvedPath, oldString, newString string) (*ports.ToolResult, error) {
	// Check if file exists
	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("file does not exist: %s", filePath)}, nil
	}

	// Read file content
	content, err := os.ReadFile(resolvedPath)
	if err != nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("failed to read file: %w", err)}, nil
	}

	originalContent := string(content)

	// Check for uniqueness of old_string
	occurrences := strings.Count(originalContent, oldString)
	if occurrences == 0 {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("old_string not found in file")}, nil
	}
	if occurrences > 1 {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("old_string appears %d times in file. Please include more context to make it unique", occurrences)}, nil
	}

	// Perform the replacement (only one occurrence)
	newContent := strings.Replace(originalContent, oldString, newString, 1)

	// Generate diff
	diff := generateUnifiedDiff(originalContent, newContent, filePath)

	// Write the modified content
	if err := os.WriteFile(resolvedPath, []byte(newContent), 0644); err != nil {
		return &ports.ToolResult{CallID: callID, Error: fmt.Errorf("failed to write file: %w", err)}, nil
	}

	// Get file info after writing
	fileInfo, _ := os.Stat(resolvedPath)
	newLineCount := len(strings.Split(newContent, "\n"))
	sum := sha256.Sum256([]byte(newContent))

	return &ports.ToolResult{
		CallID:  callID,
		Content: fmt.Sprintf("Updated %s (%d lines)", filePath, newLineCount),
		Metadata: map[string]any{
			"file_path":      filePath,
			"resolved_path":  resolvedPath,
			"operation":      "edited",
			"lines_total":    newLineCount,
			"modified":       fileInfo.ModTime().Unix(),
			"diff":           diff,
			"content_len":    len(newContent),
			"content_sha256": fmt.Sprintf("%x", sum),
		},
	}, nil
}

func (t *fileEdit) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "file_edit",
		Description: `Edit files by replacing specific text content with exact string matching.

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
4. Tool shows diff and updates file`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"file_path": {
					Type:        "string",
					Description: "The absolute path to the file to modify",
				},
				"old_string": {
					Type:        "string",
					Description: "The text to replace (empty for new file)",
				},
				"new_string": {
					Type:        "string",
					Description: "The text to replace with",
				},
			},
			Required: []string{"file_path", "old_string", "new_string"},
		},
	}
}

func (t *fileEdit) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:      "file_edit",
		Version:   "1.0.0",
		Category:  "file_operations",
		Tags:      []string{"file", "edit", "replace", "diff"},
		Dangerous: true,
	}
}

// generateUnifiedDiff creates a simple unified diff between old and new content
func generateUnifiedDiff(oldContent, newContent, filename string) string {
	// Quick check: if contents are identical, return empty diff
	if oldContent == newContent {
		return ""
	}

	// Performance check: skip diff for very large files
	if len(oldContent) > 10*1024*1024 || len(newContent) > 10*1024*1024 {
		return fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ Large file, diff skipped for performance @@",
			filename, filename, filename, filename)
	}

	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var result strings.Builder

	// Simplified diff header
	result.WriteString(fmt.Sprintf("--- a/%s\n", filename))
	result.WriteString(fmt.Sprintf("+++ b/%s\n", filename))

	oldLen := len(oldLines)
	newLen := len(newLines)

	// Hunk header
	result.WriteString(fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 1, oldLen, 1, newLen))

	// Find common prefix and suffix to minimize diff size
	commonPrefix := findCommonPrefix(oldLines, newLines)
	commonSuffix := findCommonSuffix(oldLines[commonPrefix:], newLines[commonPrefix:])

	// Adjust for common suffix
	oldEndIdx := oldLen - commonSuffix
	newEndIdx := newLen - commonSuffix

	// Show context before changes (2 lines)
	contextLines := 2
	contextStart := max(0, commonPrefix-contextLines)
	for i := contextStart; i < commonPrefix; i++ {
		if i < len(oldLines) {
			result.WriteString(fmt.Sprintf("%4d        %s\n", i+1, oldLines[i]))
		}
	}

	// Show removed lines
	oldLineNum := commonPrefix + 1
	for i := commonPrefix; i < oldEndIdx; i++ {
		if i < len(oldLines) {
			result.WriteString(fmt.Sprintf("%4d -      %s\n", oldLineNum, oldLines[i]))
			oldLineNum++
		}
	}

	// Show added lines
	newLineNum := commonPrefix + 1
	for i := commonPrefix; i < newEndIdx; i++ {
		if i < len(newLines) {
			result.WriteString(fmt.Sprintf("%4d +      %s\n", newLineNum, newLines[i]))
			newLineNum++
		}
	}

	// Show context after changes (2 lines)
	currentLineNum := max(oldLineNum, newLineNum)
	for i := 0; i < commonSuffix && i < contextLines; i++ {
		idx := max(oldLen, newLen) - commonSuffix + i
		if idx >= 0 {
			line := ""
			if idx < len(oldLines) {
				line = oldLines[idx]
			} else if idx-oldLen+newLen < len(newLines) {
				line = newLines[idx-oldLen+newLen]
			}
			if line != "" {
				result.WriteString(fmt.Sprintf("%4d        %s\n", currentLineNum+i, line))
			}
		}
	}

	// Limit output to 20 lines
	diffOutput := result.String()
	lines := strings.Split(diffOutput, "\n")
	if len(lines) > 20 {
		lines = lines[:20]
		lines = append(lines, "... (truncated)")
		diffOutput = strings.Join(lines, "\n")
	}

	return diffOutput
}

// findCommonPrefix finds the number of common lines at the beginning
func findCommonPrefix(oldLines, newLines []string) int {
	minLen := min(len(oldLines), len(newLines))
	common := 0
	for i := 0; i < minLen; i++ {
		if oldLines[i] == newLines[i] {
			common++
		} else {
			break
		}
	}
	return common
}

// findCommonSuffix finds the number of common lines at the end
func findCommonSuffix(oldLines, newLines []string) int {
	oldLen, newLen := len(oldLines), len(newLines)
	minLen := min(oldLen, newLen)
	common := 0

	for i := 1; i <= minLen; i++ {
		if oldLines[oldLen-i] == newLines[newLen-i] {
			common++
		} else {
			break
		}
	}
	return common
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
