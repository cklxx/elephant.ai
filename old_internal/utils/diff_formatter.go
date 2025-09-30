package utils

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Diff background color scheme using lipgloss
	addedLineBgColor   = lipgloss.Color("#2d5016") // Dark green background for added lines
	removedLineBgColor = lipgloss.Color("#5d1a1d") // Dark red background for deleted lines
	contextLineBgColor = lipgloss.Color("#1a1a1a") // Dark background for context lines
	headerLineColor    = lipgloss.Color("#8b5cf6") // Purple for diff headers

	// Styles for different diff line types - using background colors
	addedLineStyle   = lipgloss.NewStyle().Background(addedLineBgColor)
	removedLineStyle = lipgloss.NewStyle().Background(removedLineBgColor)
	contextLineStyle = lipgloss.NewStyle().Background(contextLineBgColor)
	headerLineStyle  = lipgloss.NewStyle().Foreground(headerLineColor).Bold(true)
)

// FormatDiffOutput applies color formatting to git diff output
func FormatDiffOutput(diffOutput string) string {
	lines := strings.Split(diffOutput, "\n")
	var formattedLines []string

	for _, line := range lines {
		if len(line) == 0 {
			formattedLines = append(formattedLines, line)
			continue
		}

		// Check for our new line number format: "  123 +      content" or "  123 -      content"
		if len(line) > 7 && line[0] >= '0' && line[0] <= '9' {
			// Look for the +/- indicator after the line number
			if strings.Contains(line, " +      ") {
				// Added lines (light green)
				formattedLines = append(formattedLines, addedLineStyle.Render(line))
				continue
			} else if strings.Contains(line, " -      ") {
				// Removed lines (light red)
				formattedLines = append(formattedLines, removedLineStyle.Render(line))
				continue
			} else if len(line) > 8 && line[4:12] == "        " {
				// Context lines with line numbers (slight gray tint)
				formattedLines = append(formattedLines, contextLineStyle.Render(line))
				continue
			}
		}

		// Fall back to original logic for traditional diff format
		switch line[0] {
		case '+':
			// Added lines (light green)
			formattedLines = append(formattedLines, addedLineStyle.Render(line))
		case '-':
			// Removed lines (light red)
			formattedLines = append(formattedLines, removedLineStyle.Render(line))
		case '@':
			// Diff headers with line numbers (purple)
			if strings.HasPrefix(line, "@@") {
				formattedLines = append(formattedLines, headerLineStyle.Render(line))
			} else {
				formattedLines = append(formattedLines, line)
			}
		case 'd', 'i', 'n':
			// Diff command headers like "diff --git", "index", "new file mode"
			if strings.HasPrefix(line, "diff ") ||
				strings.HasPrefix(line, "index ") ||
				strings.HasPrefix(line, "new file mode") ||
				strings.HasPrefix(line, "deleted file mode") ||
				strings.HasPrefix(line, "--- ") ||
				strings.HasPrefix(line, "+++ ") {
				formattedLines = append(formattedLines, headerLineStyle.Render(line))
			} else {
				// Context lines (no color change)
				formattedLines = append(formattedLines, line)
			}
		case ' ':
			// Context lines (slight gray tint)
			formattedLines = append(formattedLines, contextLineStyle.Render(line))
		default:
			// All other lines (no color change)
			formattedLines = append(formattedLines, line)
		}
	}

	return strings.Join(formattedLines, "\n")
}

// IsDiffOutput checks if the given output appears to be from git diff
func IsDiffOutput(output string) bool {
	diffIndicators := []string{
		"diff --git",
		"index ",
		"--- a/",
		"+++ b/",
		"@@",
	}

	for _, indicator := range diffIndicators {
		if strings.Contains(output, indicator) {
			return true
		}
	}

	return false
}

// FilterAndFormatDiff filters out unnecessary diff headers and formats only meaningful changes
func FilterAndFormatDiff(diffOutput string) string {
	if diffOutput == "" {
		return ""
	}

	lines := strings.Split(diffOutput, "\n")
	var filteredLines []string
	var lastContextLines []string
	var hasChanges bool

	for i, line := range lines {
		if len(line) == 0 {
			continue
		}

		// Skip diff headers
		if strings.HasPrefix(line, "--- a/") ||
			strings.HasPrefix(line, "+++ b/") ||
			strings.HasPrefix(line, "@@") ||
			strings.HasPrefix(line, "diff --git") ||
			strings.HasPrefix(line, "index ") {
			continue
		}

		// Check if this is a change line
		isAddition := false
		isDeletion := false
		isContext := false

		// Check for our line number format first
		if len(line) > 7 && line[0] >= '0' && line[0] <= '9' {
			if strings.Contains(line, " +      ") {
				isAddition = true
				hasChanges = true
			} else if strings.Contains(line, " -      ") {
				isDeletion = true
				hasChanges = true
			} else if len(line) > 8 && line[4:12] == "        " {
				isContext = true
			}
		} else {
			// Fall back to traditional diff format
			switch line[0] {
			case '+':
				isAddition = true
				hasChanges = true
			case '-':
				isDeletion = true
				hasChanges = true
			case ' ':
				isContext = true
			}
		}

		if isAddition || isDeletion {
			// Add any pending context lines (max 1 before changes)
			if len(lastContextLines) > 0 {
				contextToAdd := lastContextLines
				if len(contextToAdd) > 1 {
					contextToAdd = contextToAdd[len(contextToAdd)-1:] // Only last context line
				}
				filteredLines = append(filteredLines, contextToAdd...)
				lastContextLines = nil
			}

			// Add the change line
			filteredLines = append(filteredLines, line)

			// Look ahead for 1 line of context after changes
			if i+1 < len(lines) {
				nextLine := lines[i+1]
				if len(nextLine) > 0 {
					nextIsContext := false
					if len(nextLine) > 8 && nextLine[0] >= '0' && nextLine[0] <= '9' && nextLine[4:12] == "        " {
						nextIsContext = true
					} else if len(nextLine) > 0 && nextLine[0] == ' ' {
						nextIsContext = true
					}

					if nextIsContext {
						filteredLines = append(filteredLines, nextLine)
						// Note: we can't skip the next line in range loop, but it will be processed as context
					}
				}
			}
		} else if isContext {
			// Store context lines, we'll add them only if followed by changes
			lastContextLines = append(lastContextLines, line)
			if len(lastContextLines) > 1 {
				lastContextLines = lastContextLines[1:] // Keep only 1 context line
			}
		}
	}

	if !hasChanges || len(filteredLines) == 0 {
		return ""
	}

	// Apply color formatting to the filtered lines
	var formattedLines []string
	for _, line := range filteredLines {
		if len(line) == 0 {
			formattedLines = append(formattedLines, line)
			continue
		}

		// Apply formatting based on line type
		if len(line) > 7 && line[0] >= '0' && line[0] <= '9' {
			if strings.Contains(line, " +      ") {
				formattedLines = append(formattedLines, addedLineStyle.Render(line))
			} else if strings.Contains(line, " -      ") {
				formattedLines = append(formattedLines, removedLineStyle.Render(line))
			} else if len(line) > 8 && line[4:12] == "        " {
				formattedLines = append(formattedLines, contextLineStyle.Render(line))
			} else {
				formattedLines = append(formattedLines, line)
			}
		} else {
			switch line[0] {
			case '+':
				formattedLines = append(formattedLines, addedLineStyle.Render(line))
			case '-':
				formattedLines = append(formattedLines, removedLineStyle.Render(line))
			case ' ':
				formattedLines = append(formattedLines, contextLineStyle.Render(line))
			default:
				formattedLines = append(formattedLines, line)
			}
		}
	}

	return strings.Join(formattedLines, "\n")
}
