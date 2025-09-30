package utils

import (
	"fmt"
	"strings"
)

// DiffOptions controls diff generation behavior
type DiffOptions struct {
	MaxLines     int  // Maximum lines to include in diff (0 = no limit)
	ContextLines int  // Number of context lines around changes
	ShowStats    bool // Show statistics summary
}

// DefaultDiffOptions provides sensible defaults for diff generation
var DefaultDiffOptions = DiffOptions{
	MaxLines:     20,   // Limit diff to 20 lines for concise display
	ContextLines: 2,    // 2 lines of context (reduced for brevity)
	ShowStats:    true, // Show change statistics
}

// GenerateUnifiedDiff creates a unified diff between old and new content
func GenerateUnifiedDiff(oldContent, newContent, filename string, options DiffOptions) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Quick check: if contents are identical, return empty diff
	if oldContent == newContent {
		return ""
	}

	// Performance check: skip diff for very large files
	if len(oldContent) > 10*1024*1024 || len(newContent) > 10*1024*1024 {
		return fmt.Sprintf("diff --git a/%s b/%s\n--- a/%s\n+++ b/%s\n@@ Large file, diff skipped for performance @@",
			filename, filename, filename, filename)
	}

	// Generate diff using simple line-by-line comparison
	diff := generateSimpleDiff(oldLines, newLines, filename, options)

	// Apply line limit if specified
	if options.MaxLines > 0 {
		diff = limitDiffLines(diff, options.MaxLines)
	}

	return diff
}

// generateSimpleDiff creates a basic unified diff format with line numbers
func generateSimpleDiff(oldLines, newLines []string, filename string, options DiffOptions) string {
	var result strings.Builder

	// Simplified diff header - no timestamps for cleaner display
	result.WriteString(fmt.Sprintf("--- a/%s\n", filename))
	result.WriteString(fmt.Sprintf("+++ b/%s\n", filename))

	// Simple implementation: treat as complete replacement for now
	// This could be enhanced with proper LCS algorithm for better diffs
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

	// Track line numbers for both old and new files
	oldLineNum := 1
	newLineNum := 1

	// Show context before changes with line numbers
	contextStart := max(0, commonPrefix-options.ContextLines)
	for i := contextStart; i < commonPrefix; i++ {
		if i < len(oldLines) {
			result.WriteString(fmt.Sprintf("%4d        %s\n", oldLineNum+i, oldLines[i]))
		}
	}

	// Update line numbers to current position
	oldLineNum += commonPrefix
	newLineNum += commonPrefix

	// Show removed lines with line numbers
	for i := commonPrefix; i < oldEndIdx; i++ {
		if i < len(oldLines) {
			result.WriteString(fmt.Sprintf("%4d -      %s\n", oldLineNum, oldLines[i]))
			oldLineNum++
		}
	}

	// Show added lines with line numbers
	for i := commonPrefix; i < newEndIdx; i++ {
		if i < len(newLines) {
			result.WriteString(fmt.Sprintf("%4d +      %s\n", newLineNum, newLines[i]))
			newLineNum++
		}
	}

	// Show context after changes with line numbers
	currentLineNum := max(oldLineNum, newLineNum)

	// Show common suffix context
	for i := 0; i < commonSuffix && i < options.ContextLines; i++ {
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

	return result.String()
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

// limitDiffLines limits the diff output to specified number of lines
func limitDiffLines(diff string, maxLines int) string {
	lines := strings.Split(diff, "\n")
	if len(lines) <= maxLines {
		return diff
	}

	truncated := lines[:maxLines]
	truncated = append(truncated, "... (truncated)")
	return strings.Join(truncated, "\n")
}

// GenerateDiffStats generates statistics about the changes
func GenerateDiffStats(oldContent, newContent string) string {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	oldCount := len(oldLines)
	newCount := len(newLines)

	if oldContent == "" {
		oldCount = 0
	}
	if newContent == "" {
		newCount = 0
	}

	added := 0
	removed := 0

	if oldCount > newCount {
		removed = oldCount - newCount
	} else if newCount > oldCount {
		added = newCount - oldCount
	}

	// Simple heuristic: if lengths are similar, assume it's modifications
	if abs(oldCount-newCount) < min(oldCount, newCount)/10 {
		// Treat as modifications rather than pure add/remove
		added = max(0, newCount-oldCount)
		removed = max(0, oldCount-newCount)
	}

	var stats strings.Builder
	if added > 0 {
		stats.WriteString(fmt.Sprintf("+%d", added))
	}
	if removed > 0 {
		if stats.Len() > 0 {
			stats.WriteString(" ")
		}
		stats.WriteString(fmt.Sprintf("-%d", removed))
	}

	if stats.Len() == 0 {
		return "modified"
	}

	return stats.String()
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

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
