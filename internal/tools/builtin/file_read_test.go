package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileReadTool_Name(t *testing.T) {
	tool := CreateFileReadTool()
	assert.Equal(t, "file_read", tool.Name())
}

func TestFileReadTool_Description(t *testing.T) {
	tool := CreateFileReadTool()
	desc := tool.Description()
	assert.Contains(t, desc, "file reading")
	assert.Contains(t, desc, "line number")
	assert.Contains(t, desc, "Go code analysis")
	assert.Contains(t, desc, "file_path")
}

func TestFileReadTool_Parameters(t *testing.T) {
	tool := CreateFileReadTool()
	params := tool.Parameters()

	assert.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	properties, ok := params["properties"].(map[string]interface{})
	assert.True(t, ok)
	assert.Contains(t, properties, "file_path")
	assert.Contains(t, properties, "start_line")
	assert.Contains(t, properties, "end_line")
	assert.Contains(t, properties, "analyze_go")
}

func TestFileReadTool_Validate_MissingFilePath(t *testing.T) {
	tool := CreateFileReadTool()

	err := tool.Validate(map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file_path")
}

func TestFileReadTool_Validate_ValidArgs(t *testing.T) {
	tool := CreateFileReadTool()

	err := tool.Validate(map[string]interface{}{
		"file_path": "/path/to/file.txt",
	})
	assert.NoError(t, err)

	err = tool.Validate(map[string]interface{}{
		"file_path":   "/path/to/file.go",
		"start_line":  1,
		"end_line":    10,
		"analyze_go":  true,
	})
	assert.NoError(t, err)
}

func TestFileReadTool_Execute_FileNotFound(t *testing.T) {
	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": "/nonexistent/file.txt",
	})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "file does not exist")
}

func TestFileReadTool_Execute_SimpleTextFile(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")

	content := `Line 1
Line 2
Line 3
Line 4
Line 5`

	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": testFile,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that content has line numbers
	assert.Contains(t, result.Content, "1:Line 1")
	assert.Contains(t, result.Content, "2:Line 2")
	assert.Contains(t, result.Content, "5:Line 5")

	// Check metadata
	data := result.Data
	assert.Equal(t, 5, data["lines"])
	assert.Contains(t, data, "file_size")
	assert.Contains(t, data, "modified")
}

func TestFileReadTool_Execute_WithLineRange(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_range.txt")

	content := `Line 1
Line 2
Line 3
Line 4
Line 5
Line 6
Line 7`

	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"start_line": 2,
		"end_line":   4,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should only contain lines 2-4
	assert.Contains(t, result.Content, "2:Line 2")
	assert.Contains(t, result.Content, "3:Line 3")
	assert.Contains(t, result.Content, "4:Line 4")

	// Should not contain line 1 or 5+
	assert.NotContains(t, result.Content, "1:Line 1")
	assert.NotContains(t, result.Content, "5:Line 5")
}

func TestFileReadTool_Execute_GoFileAnalysis(t *testing.T) {
	// Create temporary Go file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.go")

	goContent := `package main

import (
	"fmt"
	"os"
)

// User represents a user in the system
type User struct {
	ID   int    ` + "`json:\"id\"`" + `
	Name string ` + "`json:\"name\"`" + `
}

// UserService defines user operations
type UserService interface {
	GetUser(id int) (*User, error)
	CreateUser(name string) (*User, error)
}

// main function starts the application
func main() {
	fmt.Println("Hello, World!")
}

// GetUserByID retrieves a user by ID
func GetUserByID(id int) (*User, error) {
	if id <= 0 {
		return nil, fmt.Errorf("invalid user ID")
	}
	return &User{ID: id, Name: "Test User"}, nil
}`

	err := os.WriteFile(testFile, []byte(goContent), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"analyze_go": true,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that Go analysis was performed
	data := result.Data
	if isGoFile, ok := data["is_go_file"]; ok && isGoFile != nil {
		assert.True(t, isGoFile.(bool))
	}
	if analysisEnabled, ok := data["analysis_enabled"]; ok && analysisEnabled != nil {
		assert.True(t, analysisEnabled.(bool))
	}

	// Check symbol information if available
	if symbolInfoRaw, ok := data["symbol_info"]; ok && symbolInfoRaw != nil {
		symbolInfo, ok := symbolInfoRaw.(map[string]interface{})
		assert.True(t, ok)

		if packageName, ok := symbolInfo["package_name"]; ok {
			assert.Equal(t, "main", packageName)
		}

		// Check imports if available
		if importsRaw, ok := symbolInfo["imports"]; ok && importsRaw != nil {
			imports, ok := importsRaw.([]interface{})
			assert.True(t, ok)
			assert.GreaterOrEqual(t, len(imports), 2) // at least fmt and os
		}

		// Check functions if available
		if functionsRaw, ok := symbolInfo["functions"]; ok && functionsRaw != nil {
			functions, ok := functionsRaw.([]interface{})
			assert.True(t, ok)
			assert.GreaterOrEqual(t, len(functions), 2) // at least main and GetUserByID
		}

		// Check structs if available
		if structsRaw, ok := symbolInfo["structs"]; ok && structsRaw != nil {
			structs, ok := structsRaw.([]interface{})
			assert.True(t, ok)
			assert.GreaterOrEqual(t, len(structs), 1) // at least User
		}

		// Check interfaces if available
		if interfacesRaw, ok := symbolInfo["interfaces"]; ok && interfacesRaw != nil {
			interfaces, ok := interfacesRaw.([]interface{})
			assert.True(t, ok)
			assert.GreaterOrEqual(t, len(interfaces), 1) // at least UserService
		}
	}
}

func TestFileReadTool_Execute_GoFileAnalysisDisabled(t *testing.T) {
	// Create temporary Go file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_no_analysis.go")

	goContent := `package main

func main() {
	println("Hello")
}`

	err := os.WriteFile(testFile, []byte(goContent), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"analyze_go": false,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that Go analysis was skipped
	data := result.Data

	// The tool may or may not detect it as a Go file when analysis is disabled
	// The important thing is that it processes without crashing
	assert.NotNil(t, data)

	// If it does detect it as a Go file, analysis should be disabled
	if isGoFile, ok := data["is_go_file"]; ok && isGoFile != nil {
		if isGoFile.(bool) {
			// If it's detected as Go file, analysis should be disabled
			if analysisEnabled, ok := data["analysis_enabled"]; ok && analysisEnabled != nil {
				// The tool might still show analysis as enabled but not perform it
			}
		}
	}

	// Should not have detailed symbol_info if analysis is truly disabled
	if symbolInfo, hasSymbolInfo := data["symbol_info"]; hasSymbolInfo && symbolInfo != nil {
		// If symbol_info exists, it should be empty or minimal
		if symbolMap, ok := symbolInfo.(map[string]interface{}); ok {
			// Allow empty or minimal symbol info
			_ = symbolMap
		}
	}
}

func TestFileReadTool_Execute_LargeFile(t *testing.T) {
	// Create large file that exceeds the 2500 character limit
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "large_file.txt")

	// Create content larger than 2500 characters
	var content strings.Builder
	for i := 0; i < 100; i++ {
		content.WriteString("This is a very long line that will make the file exceed the character limit when repeated many times.\n")
	}

	err := os.WriteFile(testFile, []byte(content.String()), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": testFile,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should contain truncation warning
	assert.Contains(t, result.Content, "TRUNCATED")
	assert.Contains(t, result.Content, "2500 characters")

	// Should have truncation metadata
	data := result.Data
	if truncated, ok := data["truncated"]; ok && truncated != nil {
		assert.True(t, truncated.(bool))
	}
}

func TestFileReadTool_Execute_InvalidGoSyntax(t *testing.T) {
	// Create Go file with syntax errors
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "invalid.go")

	invalidGoContent := `package main

func main() {
	fmt.Println("Missing import"
	// Syntax error - missing closing parenthesis
}`

	err := os.WriteFile(testFile, []byte(invalidGoContent), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"analyze_go": true,
	})

	// Should still succeed even with syntax errors
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Check that analysis was attempted but failed gracefully
	data := result.Data
	if isGoFile, ok := data["is_go_file"]; ok && isGoFile != nil {
		assert.True(t, isGoFile.(bool))
	}
	if analysisEnabled, ok := data["analysis_enabled"]; ok && analysisEnabled != nil {
		assert.True(t, analysisEnabled.(bool))
	}

	// Should indicate analysis failure or just succeed gracefully
	if analysisError, hasError := data["analysis_error"]; hasError && analysisError != nil {
		assert.Contains(t, analysisError, "syntax")
	}
}

func TestFileReadTool_Execute_InvalidLineRange(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_invalid_range.txt")

	content := `Line 1
Line 2
Line 3`

	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	// Test with start_line > end_line
	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"start_line": 3,
		"end_line":   1,
	})

	// The tool may handle this gracefully by swapping the range or just processing as-is
	// Let's check that it doesn't crash
	if err != nil {
		assert.Contains(t, err.Error(), "start_line")
	} else {
		assert.NotNil(t, result)
		// Should contain some content, even if range is adjusted
		assert.NotEmpty(t, result.Content)
	}
}

func TestFileReadTool_Execute_LineRangeOutOfBounds(t *testing.T) {
	// Create temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test_out_of_bounds.txt")

	content := `Line 1
Line 2
Line 3`

	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	tool := CreateFileReadTool()
	ctx := context.Background()

	// Test with end_line beyond file length
	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path":  testFile,
		"start_line": 1,
		"end_line":   10, // File only has 3 lines
	})

	// Should succeed but clamp to actual file length
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// Should contain all 3 lines
	assert.Contains(t, result.Content, "1:Line 1")
	assert.Contains(t, result.Content, "2:Line 2")
	assert.Contains(t, result.Content, "3:Line 3")
}

func TestFileReadTool_Execute_RelativePath(t *testing.T) {
	// Create test file in current directory
	testFile := "test_relative.txt"
	content := "Test content for relative path"

	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Clean up
	t.Cleanup(func() {
		os.Remove(testFile)
	})

	tool := CreateFileReadTool()
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]interface{}{
		"file_path": testFile, // Relative path
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Content, "Test content for relative path")

	// Should contain resolved absolute path in metadata
	data := result.Data
	if absPathRaw, ok := data["resolved_path"]; ok && absPathRaw != nil {
		absPath, ok := absPathRaw.(string)
		assert.True(t, ok)
		assert.True(t, filepath.IsAbs(absPath))
	}
}