package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileReadTool(t *testing.T) {
	tool := CreateFileReadTool()

	// Test tool metadata
	if tool.Name() != "file_read" {
		t.Errorf("expected name 'file_read', got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}

	// Test parameters schema
	params := tool.Parameters()
	if params == nil {
		t.Error("expected parameters schema")
	}

	// Check required parameters
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("expected properties in parameters schema")
	}

	if _, ok := properties["file_path"]; !ok {
		t.Error("expected 'file_path' parameter in schema")
	}

	// Test validation
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:        "missing file_path",
			args:        map[string]interface{}{},
			expectError: true,
		},
		{
			name: "empty file_path",
			args: map[string]interface{}{
				"file_path": "",
			},
			expectError: true,
		},
		{
			name: "invalid file_path type",
			args: map[string]interface{}{
				"file_path": 123,
			},
			expectError: true,
		},
		{
			name: "valid file_path",
			args: map[string]interface{}{
				"file_path": "/test/file.txt",
			},
			expectError: false,
		},
		{
			name: "valid with path parameter",
			args: map[string]interface{}{
				"path": "/test/file.txt",
			},
			expectError: false,
		},
		{
			name: "valid with line range",
			args: map[string]interface{}{
				"file_path":  "/test/file.txt",
				"start_line": 1.0,
				"end_line":   10.0,
			},
			expectError: false,
		},
		{
			name: "invalid start_line",
			args: map[string]interface{}{
				"file_path":  "/test/file.txt",
				"start_line": 0.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if tt.expectError && err == nil {
				t.Error("expected validation error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestFileUpdateTool(t *testing.T) {
	tool := CreateFileUpdateTool()

	// Test tool metadata
	if tool.Name() != "file_edit" {
		t.Errorf("expected name 'file_edit', got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}

	// Test parameters schema
	params := tool.Parameters()
	if params == nil {
		t.Error("expected parameters schema")
	}

	// Check required parameters
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("expected properties in parameters schema")
	}

	requiredParams := []string{"file_path", "old_string", "new_string"}
	for _, param := range requiredParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("expected '%s' parameter in schema", param)
		}
	}

	// Test validation
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:        "missing all parameters",
			args:        map[string]interface{}{},
			expectError: true,
		},
		{
			name: "missing old_string",
			args: map[string]interface{}{
				"file_path":  "/test/file.txt",
				"new_string": "new content",
			},
			expectError: true,
		},
		{
			name: "missing new_string",
			args: map[string]interface{}{
				"file_path":  "/test/file.txt",
				"old_string": "old content",
			},
			expectError: true,
		},
		{
			name: "invalid file_path type",
			args: map[string]interface{}{
				"file_path":  123,
				"old_string": "old",
				"new_string": "new",
			},
			expectError: true,
		},
		{
			name: "valid parameters",
			args: map[string]interface{}{
				"file_path":  "/test/file.txt",
				"old_string": "old content",
				"new_string": "new content",
			},
			expectError: false,
		},
		{
			name: "valid new file creation",
			args: map[string]interface{}{
				"file_path":  "/test/newfile.txt",
				"old_string": "",
				"new_string": "file content",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if tt.expectError && err == nil {
				t.Error("expected validation error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestFileReplaceTool(t *testing.T) {
	tool := CreateFileReplaceTool()

	// Test tool metadata
	if tool.Name() != "file_replace" {
		t.Errorf("expected name 'file_replace', got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}

	// Test parameters schema
	params := tool.Parameters()
	if params == nil {
		t.Error("expected parameters schema")
	}

	// Check required parameters
	properties, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Error("expected properties in parameters schema")
	}

	requiredParams := []string{"file_path", "content"}
	for _, param := range requiredParams {
		if _, ok := properties[param]; !ok {
			t.Errorf("expected '%s' parameter in schema", param)
		}
	}

	// Test validation
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:        "missing all parameters",
			args:        map[string]interface{}{},
			expectError: true,
		},
		{
			name: "missing content",
			args: map[string]interface{}{
				"file_path": "/test/file.txt",
			},
			expectError: true,
		},
		{
			name: "missing file_path",
			args: map[string]interface{}{
				"content": "file content",
			},
			expectError: true,
		},
		{
			name: "invalid file_path type",
			args: map[string]interface{}{
				"file_path": 123,
				"content":   "content",
			},
			expectError: true,
		},
		{
			name: "invalid content type",
			args: map[string]interface{}{
				"file_path": "/test/file.txt",
				"content":   123,
			},
			expectError: true,
		},
		{
			name: "valid parameters",
			args: map[string]interface{}{
				"file_path": "/test/file.txt",
				"content":   "file content",
			},
			expectError: false,
		},
		{
			name: "empty content is valid",
			args: map[string]interface{}{
				"file_path": "/test/file.txt",
				"content":   "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if tt.expectError && err == nil {
				t.Error("expected validation error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

func TestFileListTool(t *testing.T) {
	tool := CreateFileListTool()

	// Test tool metadata
	if tool.Name() != "file_list" {
		t.Errorf("expected name 'file_list', got %s", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("expected non-empty description")
	}

	// Test parameters schema
	params := tool.Parameters()
	if params == nil {
		t.Error("expected parameters schema")
	}

	// Test validation
	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
	}{
		{
			name:        "no parameters",
			args:        map[string]interface{}{},
			expectError: false,
		},
		{
			name: "valid path",
			args: map[string]interface{}{
				"path": "/test/dir",
			},
			expectError: false,
		},
		{
			name: "invalid path type",
			args: map[string]interface{}{
				"path": 123,
			},
			expectError: true,
		},
		{
			name: "valid recursive",
			args: map[string]interface{}{
				"path":      "/test",
				"recursive": true,
			},
			expectError: false,
		},
		{
			name: "invalid recursive type",
			args: map[string]interface{}{
				"recursive": "true",
			},
			expectError: true,
		},
		{
			name: "valid show_hidden",
			args: map[string]interface{}{
				"show_hidden": true,
			},
			expectError: false,
		},
		{
			name: "invalid show_hidden type",
			args: map[string]interface{}{
				"show_hidden": "true",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tool.Validate(tt.args)
			if tt.expectError && err == nil {
				t.Error("expected validation error")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}

// Integration tests with actual file operations
func TestFileOperationsIntegration(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "file_ops_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create context with working directory
	ctx := context.Background()
	ctx = WithWorkingDir(ctx, tempDir)

	t.Run("FileReplace_CreateAndRead", func(t *testing.T) {
		// Test file creation with FileReplaceTool
		replaceTool := CreateFileReplaceTool()
		testFile := filepath.Join(tempDir, "test.txt")
		testContent := "Hello, World!\nThis is a test file."

		args := map[string]interface{}{
			"file_path": testFile,
			"content":   testContent,
		}

		result, err := replaceTool.Execute(ctx, args)
		if err != nil {
			t.Fatalf("failed to create file: %v", err)
		}

		if result == nil || result.Content == "" {
			t.Error("expected non-empty result")
		}

		// Verify file was created
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("file was not created")
		}

		// Test reading the file
		readTool := CreateFileReadTool()
		readArgs := map[string]interface{}{
			"file_path": testFile,
		}

		readResult, err := readTool.Execute(ctx, readArgs)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}

		expectedContent := "    1→Hello, World!\n    2→This is a test file."
		if readResult.Content != expectedContent {
			t.Errorf("expected content %q, got %q", expectedContent, readResult.Content)
		}
	})

	t.Run("FileEdit_ReplaceContent", func(t *testing.T) {
		// Create initial file
		testFile := filepath.Join(tempDir, "edit_test.txt")
		initialContent := "Line 1\nLine 2\nLine 3"
		err := os.WriteFile(testFile, []byte(initialContent), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		// Test editing the file
		editTool := CreateFileUpdateTool()
		args := map[string]interface{}{
			"file_path":  testFile,
			"old_string": "Line 2",
			"new_string": "Modified Line 2",
		}

		result, err := editTool.Execute(ctx, args)
		if err != nil {
			t.Fatalf("failed to edit file: %v", err)
		}

		if result == nil {
			t.Error("expected non-nil result")
		}

		// Verify the edit
		content, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read edited file: %v", err)
		}

		expectedContent := "Line 1\nModified Line 2\nLine 3"
		if string(content) != expectedContent {
			t.Errorf("expected %q, got %q", expectedContent, string(content))
		}
	})

	t.Run("FileEdit_MultipleOccurrences", func(t *testing.T) {
		// Test handling of multiple occurrences
		testFile := filepath.Join(tempDir, "multi_test.txt")
		initialContent := "test\ntest\ntest"
		err := os.WriteFile(testFile, []byte(initialContent), 0644)
		if err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}

		editTool := CreateFileUpdateTool()
		args := map[string]interface{}{
			"file_path":  testFile,
			"old_string": "test",
			"new_string": "modified",
		}

		_, err = editTool.Execute(ctx, args)
		if err == nil {
			t.Error("expected error for multiple occurrences")
		}

		if !strings.Contains(err.Error(), "appears") || !strings.Contains(err.Error(), "times") {
			t.Errorf("expected multiple occurrences error, got: %v", err)
		}
	})

	t.Run("FileList_Directory", func(t *testing.T) {
		// Create some test files
		testFiles := []string{"file1.txt", "file2.go", ".hidden"}
		for _, fileName := range testFiles {
			filePath := filepath.Join(tempDir, fileName)
			err := os.WriteFile(filePath, []byte("content"), 0644)
			if err != nil {
				t.Fatalf("failed to create test file %s: %v", fileName, err)
			}
		}

		// Test directory listing
		listTool := CreateFileListTool()
		args := map[string]interface{}{
			"path": tempDir,
		}

		result, err := listTool.Execute(ctx, args)
		if err != nil {
			t.Fatalf("failed to list directory: %v", err)
		}

		if result == nil || result.Content == "" {
			t.Error("expected non-empty result")
		}

		// Check that visible files are listed
		content := result.Content
		if !strings.Contains(content, "file1.txt") {
			t.Error("expected file1.txt in listing")
		}
		if !strings.Contains(content, "file2.go") {
			t.Error("expected file2.go in listing")
		}

		// Hidden file should not be listed by default
		if strings.Contains(content, ".hidden") {
			t.Error("hidden file should not be listed by default")
		}

		// Test with show_hidden
		args["show_hidden"] = true
		result, err = listTool.Execute(ctx, args)
		if err != nil {
			t.Fatalf("failed to list directory with hidden files: %v", err)
		}

		if !strings.Contains(result.Content, ".hidden") {
			t.Error("expected hidden file when show_hidden is true")
		}
	})
}

func TestFileReplaceProjectRelativePath(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "file_replace_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Logf("Failed to remove temp dir: %v", err)
		}
	}()

	// Create context with working directory
	ctx := context.Background()
	ctx = WithWorkingDir(ctx, tempDir)

	t.Run("FileReplace_ProjectRelativePath", func(t *testing.T) {
		// Test file creation with project-relative path (leading slash)
		replaceTool := CreateFileReplaceTool()
		testContent := `import { ReactNode } from 'react';

export interface ContainerProps {
    children: ReactNode;
    className?: string;
}

export const Container: React.FC<ContainerProps> = ({ children, className }) => {
    return (
        <div className={className}>
            {children}
        </div>
    );
};`

		args := map[string]interface{}{
			"file_path": "/src/core/ContainerTypes.ts", // Project-relative path with leading slash
			"content":   testContent,
		}

		result, err := replaceTool.Execute(ctx, args)
		if err != nil {
			t.Fatalf("failed to create file with project-relative path: %v", err)
		}

		if result == nil || result.Content == "" {
			t.Error("expected non-empty result")
		}

		// Verify file was created in correct location (tempDir/src/core/ContainerTypes.ts)
		expectedPath := filepath.Join(tempDir, "src", "core", "ContainerTypes.ts")
		if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
			t.Errorf("file was not created at expected path: %s", expectedPath)
		}

		// Verify file content
		content, err := os.ReadFile(expectedPath)
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}

		if string(content) != testContent {
			t.Errorf("file content mismatch")
		}

		// Test that src directory was created
		srcDir := filepath.Join(tempDir, "src")
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			t.Error("src directory was not created")
		}

		// Test that core directory was created
		coreDir := filepath.Join(tempDir, "src", "core")
		if _, err := os.Stat(coreDir); os.IsNotExist(err) {
			t.Error("core directory was not created")
		}
	})
}
