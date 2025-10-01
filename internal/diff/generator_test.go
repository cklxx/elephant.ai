package diff

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerator_GenerateUnified_IdenticalContent(t *testing.T) {
	gen := NewGenerator(3, false)
	content := "line1\nline2\nline3\n"

	result, err := gen.GenerateUnified(content, content, "test.txt")
	require.NoError(t, err)
	assert.Empty(t, result.UnifiedDiff)
	assert.Equal(t, 0, result.AddedLines)
	assert.Equal(t, 0, result.DeletedLines)
	assert.Equal(t, 0, result.ChangedFiles)
	assert.False(t, result.IsBinary)
}

func TestGenerator_GenerateUnified_SimpleAddition(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := "line1\nline2\nline3\n"
	newContent := "line1\nline2\nline3\nline4\n"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	assert.Greater(t, result.AddedLines, 0)
	assert.Equal(t, 0, result.DeletedLines)
	assert.Equal(t, 1, result.ChangedFiles)
	assert.False(t, result.IsBinary)

	// Check for file headers
	assert.Contains(t, result.UnifiedDiff, "--- a/test.txt")
	assert.Contains(t, result.UnifiedDiff, "+++ b/test.txt")
}

func TestGenerator_GenerateUnified_SimpleDeletion(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := "line1\nline2\nline3\nline4\n"
	newContent := "line1\nline2\nline3\n"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	assert.Equal(t, 0, result.AddedLines)
	assert.Greater(t, result.DeletedLines, 0)
	assert.Equal(t, 1, result.ChangedFiles)
	assert.False(t, result.IsBinary)
}

func TestGenerator_GenerateUnified_Modification(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := "line1\nline2\nline3\n"
	newContent := "line1\nmodified line2\nline3\n"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	// At least one line should have changed (added or deleted or both)
	assert.True(t, result.AddedLines > 0 || result.DeletedLines > 0, "Expected at least some lines to be added or deleted")
	assert.Equal(t, 1, result.ChangedFiles)
	assert.False(t, result.IsBinary)
}

func TestGenerator_GenerateUnified_NewFile(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := ""
	newContent := "line1\nline2\nline3\n"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	assert.Greater(t, result.AddedLines, 0)
	assert.Equal(t, 0, result.DeletedLines)
}

func TestGenerator_GenerateUnified_DeletedFile(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := "line1\nline2\nline3\n"
	newContent := ""

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	assert.Equal(t, 0, result.AddedLines)
	assert.Greater(t, result.DeletedLines, 0)
}

func TestGenerator_GenerateUnified_BinaryFile(t *testing.T) {
	gen := NewGenerator(3, false)
	// Binary content with null bytes
	oldContent := "some text\x00binary data"
	newContent := "different text\x00binary data"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.bin")
	require.NoError(t, err)
	assert.True(t, result.IsBinary)
	assert.Contains(t, result.UnifiedDiff, "Binary file")
}

func TestGenerator_GenerateUnified_LargeFile(t *testing.T) {
	gen := NewGenerator(3, false)
	// Create content larger than 10MB
	largeContent := strings.Repeat("a", 11*1024*1024)
	modifiedContent := strings.Repeat("b", 11*1024*1024)

	result, err := gen.GenerateUnified(largeContent, modifiedContent, "large.txt")
	require.NoError(t, err)
	assert.Contains(t, result.UnifiedDiff, "Large file")
	assert.Contains(t, result.UnifiedDiff, "diff skipped")
}

func TestGenerator_GenerateUnified_WithColors(t *testing.T) {
	gen := NewGenerator(3, true)
	oldContent := "line1\nline2\nline3\n"
	newContent := "line1\nmodified line2\nline3\n"

	result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	// Color codes should be present when color is enabled
	// Note: color codes are ANSI escape sequences
}

func TestGenerator_GenerateUnified_MultipleChanges(t *testing.T) {
	gen := NewGenerator(3, false)
	oldContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	newContent := `package main

import (
	"fmt"
	"log"
)

func main() {
	log.Println("Hello, World!")
}
`

	result, err := gen.GenerateUnified(oldContent, newContent, "main.go")
	require.NoError(t, err)
	assert.NotEmpty(t, result.UnifiedDiff)
	assert.Greater(t, result.AddedLines, 0)
	assert.Greater(t, result.DeletedLines, 0)
	assert.Equal(t, 1, result.ChangedFiles)
	assert.False(t, result.IsBinary)

	// Check for proper diff format
	assert.Contains(t, result.UnifiedDiff, "--- a/main.go")
	assert.Contains(t, result.UnifiedDiff, "+++ b/main.go")
}

func TestDiffResult_FormatSummary_NoChanges(t *testing.T) {
	result := &DiffResult{
		AddedLines:   0,
		DeletedLines: 0,
		ChangedFiles: 0,
		IsBinary:     false,
	}

	summary := result.FormatSummary()
	assert.Equal(t, "No changes", summary)
}

func TestDiffResult_FormatSummary_OnlyAdditions(t *testing.T) {
	result := &DiffResult{
		AddedLines:   5,
		DeletedLines: 0,
		ChangedFiles: 1,
		IsBinary:     false,
	}

	summary := result.FormatSummary()
	assert.Equal(t, "+5 lines", summary)
}

func TestDiffResult_FormatSummary_OnlyDeletions(t *testing.T) {
	result := &DiffResult{
		AddedLines:   0,
		DeletedLines: 3,
		ChangedFiles: 1,
		IsBinary:     false,
	}

	summary := result.FormatSummary()
	assert.Equal(t, "-3 lines", summary)
}

func TestDiffResult_FormatSummary_Mixed(t *testing.T) {
	result := &DiffResult{
		AddedLines:   5,
		DeletedLines: 3,
		ChangedFiles: 1,
		IsBinary:     false,
	}

	summary := result.FormatSummary()
	assert.Contains(t, summary, "+5 lines")
	assert.Contains(t, summary, "-3 lines")
}

func TestDiffResult_FormatSummary_Binary(t *testing.T) {
	result := &DiffResult{
		AddedLines:   0,
		DeletedLines: 0,
		ChangedFiles: 1,
		IsBinary:     true,
	}

	summary := result.FormatSummary()
	assert.Equal(t, "Binary file changed", summary)
}

func TestIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "plain text",
			content:  "Hello, World!\nThis is plain text.",
			expected: false,
		},
		{
			name:     "binary with null byte",
			content:  "Hello\x00World",
			expected: true,
		},
		{
			name:     "empty content",
			content:  "",
			expected: false,
		},
		{
			name:     "unicode text",
			content:  "Hello, 世界! 🌍",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinary(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerator_GenerateUnified_EdgeCases(t *testing.T) {
	gen := NewGenerator(3, false)

	t.Run("empty to empty", func(t *testing.T) {
		result, err := gen.GenerateUnified("", "", "test.txt")
		require.NoError(t, err)
		assert.Empty(t, result.UnifiedDiff)
	})

	t.Run("single line change", func(t *testing.T) {
		result, err := gen.GenerateUnified("old", "new", "test.txt")
		require.NoError(t, err)
		assert.NotEmpty(t, result.UnifiedDiff)
		assert.Greater(t, result.AddedLines, 0)
		assert.Greater(t, result.DeletedLines, 0)
	})

	t.Run("whitespace only change", func(t *testing.T) {
		oldContent := "line1\nline2\nline3"
		newContent := "line1\n line2\nline3" // Added space before line2
		result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
		require.NoError(t, err)
		assert.NotEmpty(t, result.UnifiedDiff)
	})

	t.Run("newline differences", func(t *testing.T) {
		oldContent := "line1\nline2\nline3"
		newContent := "line1\nline2\nline3\n" // Added trailing newline
		result, err := gen.GenerateUnified(oldContent, newContent, "test.txt")
		require.NoError(t, err)
		assert.NotEmpty(t, result.UnifiedDiff)
	})
}

func TestGenerator_ContextLines(t *testing.T) {
	tests := []struct {
		name         string
		contextLines int
		oldContent   string
		newContent   string
	}{
		{
			name:         "3 context lines",
			contextLines: 3,
			oldContent:   "line1\nline2\nline3\nline4\nline5\n",
			newContent:   "line1\nline2\nmodified\nline4\nline5\n",
		},
		{
			name:         "1 context line",
			contextLines: 1,
			oldContent:   "line1\nline2\nline3\nline4\nline5\n",
			newContent:   "line1\nline2\nmodified\nline4\nline5\n",
		},
		{
			name:         "0 context lines",
			contextLines: 0,
			oldContent:   "line1\nline2\nline3\nline4\nline5\n",
			newContent:   "line1\nline2\nmodified\nline4\nline5\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := NewGenerator(tt.contextLines, false)
			result, err := gen.GenerateUnified(tt.oldContent, tt.newContent, "test.txt")
			require.NoError(t, err)
			assert.NotEmpty(t, result.UnifiedDiff)
		})
	}
}
