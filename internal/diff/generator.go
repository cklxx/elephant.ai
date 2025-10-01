package diff

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// Generator handles unified diff generation
type Generator struct {
	contextLines int
	colorEnabled bool
}

// NewGenerator creates a new diff generator
func NewGenerator(contextLines int, colorEnabled bool) *Generator {
	return &Generator{
		contextLines: contextLines,
		colorEnabled: colorEnabled,
	}
}

// DiffResult contains the generated diff and statistics
type DiffResult struct {
	UnifiedDiff  string
	AddedLines   int
	DeletedLines int
	ChangedFiles int
	IsBinary     bool
}

// GenerateUnified creates a unified diff between old and new content
func (g *Generator) GenerateUnified(oldContent, newContent, filename string) (*DiffResult, error) {
	// Quick check: if contents are identical, return empty diff
	if oldContent == newContent {
		return &DiffResult{
			UnifiedDiff:  "",
			AddedLines:   0,
			DeletedLines: 0,
			ChangedFiles: 0,
			IsBinary:     false,
		}, nil
	}

	// Check if content appears to be binary
	if isBinary(oldContent) || isBinary(newContent) {
		return &DiffResult{
			UnifiedDiff:  fmt.Sprintf("Binary file %s has changed", filename),
			AddedLines:   0,
			DeletedLines: 0,
			ChangedFiles: 1,
			IsBinary:     true,
		}, nil
	}

	// Performance check: skip diff for very large files (>10MB)
	maxSize := 10 * 1024 * 1024
	if len(oldContent) > maxSize || len(newContent) > maxSize {
		return &DiffResult{
			UnifiedDiff: fmt.Sprintf("--- a/%s\n+++ b/%s\n@@ Large file (>10MB), diff skipped for performance @@",
				filename, filename),
			AddedLines:   0,
			DeletedLines: 0,
			ChangedFiles: 1,
			IsBinary:     false,
		}, nil
	}

	// Use diffmatchpatch library for accurate diff
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)

	// Cleanup diffs for better readability
	diffs = dmp.DiffCleanupSemantic(diffs)

	// Convert to unified diff format
	patches := dmp.PatchMake(oldContent, diffs)
	unifiedDiff := dmp.PatchToText(patches)

	// If we got no patches, generate line-based diff
	if len(patches) == 0 || unifiedDiff == "" {
		return g.generateLineDiff(oldContent, newContent, filename)
	}

	// Format the unified diff with proper headers
	formattedDiff := g.formatUnifiedDiff(unifiedDiff, filename)

	// Count statistics
	addedLines, deletedLines := g.countChanges(diffs)

	result := &DiffResult{
		UnifiedDiff:  formattedDiff,
		AddedLines:   addedLines,
		DeletedLines: deletedLines,
		ChangedFiles: 1,
		IsBinary:     false,
	}

	return result, nil
}

// generateLineDiff creates a line-based unified diff
func (g *Generator) generateLineDiff(oldContent, newContent, filename string) (*DiffResult, error) {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	var result strings.Builder

	// Diff header
	result.WriteString(g.colorize("--- a/"+filename+"\n", color.FgRed))
	result.WriteString(g.colorize("+++ b/"+filename+"\n", color.FgGreen))

	// Find changes
	addedLines := 0
	deletedLines := 0

	// Simple line-based comparison
	oldIdx, newIdx := 0, 0

	for oldIdx < len(oldLines) || newIdx < len(newLines) {
		if oldIdx >= len(oldLines) {
			// Only new lines remain
			for ; newIdx < len(newLines); newIdx++ {
				result.WriteString(g.colorize(fmt.Sprintf("+%s\n", newLines[newIdx]), color.FgGreen))
				addedLines++
			}
			break
		}

		if newIdx >= len(newLines) {
			// Only old lines remain
			for ; oldIdx < len(oldLines); oldIdx++ {
				result.WriteString(g.colorize(fmt.Sprintf("-%s\n", oldLines[oldIdx]), color.FgRed))
				deletedLines++
			}
			break
		}

		// Compare lines
		if oldLines[oldIdx] == newLines[newIdx] {
			// Context line
			result.WriteString(fmt.Sprintf(" %s\n", oldLines[oldIdx]))
			oldIdx++
			newIdx++
		} else {
			// Lines differ - show both
			result.WriteString(g.colorize(fmt.Sprintf("-%s\n", oldLines[oldIdx]), color.FgRed))
			result.WriteString(g.colorize(fmt.Sprintf("+%s\n", newLines[newIdx]), color.FgGreen))
			deletedLines++
			addedLines++
			oldIdx++
			newIdx++
		}
	}

	// Add hunk header at the beginning (after file headers)
	hunkHeader := fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 1, len(oldLines), 1, len(newLines))
	finalDiff := g.colorize("--- a/"+filename+"\n", color.FgRed) +
		g.colorize("+++ b/"+filename+"\n", color.FgGreen) +
		g.colorize(hunkHeader, color.FgCyan) +
		strings.TrimPrefix(result.String(), g.colorize("--- a/"+filename+"\n", color.FgRed)+g.colorize("+++ b/"+filename+"\n", color.FgGreen))

	return &DiffResult{
		UnifiedDiff:  finalDiff,
		AddedLines:   addedLines,
		DeletedLines: deletedLines,
		ChangedFiles: 1,
		IsBinary:     false,
	}, nil
}

// formatUnifiedDiff formats the patch text with proper headers and colors
func (g *Generator) formatUnifiedDiff(patchText, filename string) string {
	var result strings.Builder

	// Add file headers
	result.WriteString(g.colorize("--- a/"+filename+"\n", color.FgRed))
	result.WriteString(g.colorize("+++ b/"+filename+"\n", color.FgGreen))

	// Process patch lines
	lines := strings.Split(patchText, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			// Hunk header
			result.WriteString(g.colorize(line+"\n", color.FgCyan))
		} else if strings.HasPrefix(line, "+") {
			// Added line
			result.WriteString(g.colorize(line+"\n", color.FgGreen))
		} else if strings.HasPrefix(line, "-") {
			// Deleted line
			result.WriteString(g.colorize(line+"\n", color.FgRed))
		} else if line != "" {
			// Context line
			result.WriteString(line + "\n")
		}
	}

	return result.String()
}

// countChanges counts added and deleted lines from diffs
func (g *Generator) countChanges(diffs []diffmatchpatch.Diff) (added, deleted int) {
	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			added += strings.Count(diff.Text, "\n")
			if !strings.HasSuffix(diff.Text, "\n") {
				added++ // Count last line if no trailing newline
			}
		case diffmatchpatch.DiffDelete:
			deleted += strings.Count(diff.Text, "\n")
			if !strings.HasSuffix(diff.Text, "\n") {
				deleted++ // Count last line if no trailing newline
			}
		}
	}
	return
}

// colorize applies color to text if color is enabled
func (g *Generator) colorize(text string, colorAttr color.Attribute) string {
	if !g.colorEnabled {
		return text
	}
	c := color.New(colorAttr)
	return c.Sprint(text)
}

// isBinary checks if content appears to be binary
func isBinary(content string) bool {
	// Check for null bytes in first 8000 bytes
	checkLen := min(len(content), 8000)
	for i := 0; i < checkLen; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}

// FormatSummary returns a human-readable summary of changes
func (dr *DiffResult) FormatSummary() string {
	if dr.IsBinary {
		return "Binary file changed"
	}

	if dr.AddedLines == 0 && dr.DeletedLines == 0 {
		return "No changes"
	}

	parts := []string{}
	if dr.AddedLines > 0 {
		parts = append(parts, fmt.Sprintf("+%d lines", dr.AddedLines))
	}
	if dr.DeletedLines > 0 {
		parts = append(parts, fmt.Sprintf("-%d lines", dr.DeletedLines))
	}

	return strings.Join(parts, ", ")
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
