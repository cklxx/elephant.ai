package formatter

import (
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/shared/utils"
)

// formatTodoResult shows FULL todo list - user needs complete task overview
func (tf *ToolFormatter) formatTodoResult(content string) string {
	lines := strings.Split(content, "\n")
	var output []string
	currentSection := ""

	// Add header
	output = append(output, "  Todo List:")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Detect section headers (both text and markdown format)
		switch {
		case strings.HasPrefix(line, "In Progress:") || strings.HasPrefix(line, "## In Progress"):
			currentSection = "progress"
			output = append(output, "")
			output = append(output, "  In Progress:")
			continue
		case strings.HasPrefix(line, "Pending:") || strings.HasPrefix(line, "## Pending"):
			currentSection = "pending"
			output = append(output, "")
			output = append(output, "  Pending:")
			continue
		case strings.Contains(line, "Completed:") || strings.HasPrefix(line, "## Completed"):
			currentSection = "completed"
			output = append(output, "")
			output = append(output, "  Completed:")
			continue
		case strings.HasPrefix(line, "Updated:") || strings.HasPrefix(line, "Todo List:"):
			// Skip summary/header lines
			continue
		}

		// Format task items - show ALL tasks (no limits)
		if strings.HasPrefix(line, "- [▶]") || strings.HasPrefix(line, "- [ ]") ||
			strings.HasPrefix(line, "- [✓]") || strings.HasPrefix(line, "- [x]") ||
			strings.HasPrefix(line, "- ") {
			// Extract task text
			task := line
			task = strings.TrimPrefix(task, "- [▶] ")
			task = strings.TrimPrefix(task, "- [ ] ")
			task = strings.TrimPrefix(task, "- [✓] ")
			task = strings.TrimPrefix(task, "- [x] ")
			task = strings.TrimPrefix(task, "- ")
			task = strings.TrimSpace(task)

			if task == "" {
				continue
			}

			// Add task with appropriate marker
			switch currentSection {
			case "progress":
				output = append(output, "    [▶] "+task)
			case "pending":
				output = append(output, "    [ ] "+task)
			case "completed":
				output = append(output, "    [✓] "+task)
			}
		}
	}

	if len(output) <= 1 { // Only header
		return "  → No tasks"
	}

	return strings.Join(output, "\n")
}

// formatListFilesResult shows file count and key files
func (tf *ToolFormatter) formatListFilesResult(content string) string {
	type listDirPayload struct {
		DirectoryCount int `json:"directory_count"`
		FileCount      int `json:"file_count"`
		Files          []struct {
			Name        string `json:"name"`
			IsDirectory bool   `json:"is_directory"`
		} `json:"files"`
	}

	var payload listDirPayload
	if err := json.Unmarshal([]byte(content), &payload); err == nil && len(payload.Files) > 0 {
		dirs := make([]string, 0, len(payload.Files))
		files := make([]string, 0, len(payload.Files))
		for _, entry := range payload.Files {
			if utils.IsBlank(entry.Name) {
				continue
			}
			if entry.IsDirectory {
				dirs = append(dirs, entry.Name)
			} else {
				files = append(files, entry.Name)
			}
		}
		var parts []string
		if payload.DirectoryCount > 0 {
			parts = append(parts, fmt.Sprintf("%d dirs", payload.DirectoryCount))
		}
		if payload.FileCount > 0 {
			parts = append(parts, fmt.Sprintf("%d files", payload.FileCount))
		}
		if len(files) > 0 {
			preview := files
			if len(preview) > 3 {
				preview = preview[:3]
			}
			parts = append(parts, fmt.Sprintf("sample: %s", strings.Join(preview, ", ")))
		} else if len(dirs) > 0 {
			preview := dirs
			if len(preview) > 3 {
				preview = preview[:3]
			}
			parts = append(parts, fmt.Sprintf("sample dirs: %s", strings.Join(preview, ", ")))
		}
		if len(parts) > 0 {
			return "  → " + strings.Join(parts, " | ")
		}
	}

	lines := strings.Split(content, "\n")
	var dirs, files []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "[DIR]") {
			dirName := strings.TrimPrefix(line, "[DIR]")
			dirName = strings.TrimSpace(dirName)
			if dirName != "" {
				dirs = append(dirs, dirName)
			}
		} else if strings.HasPrefix(line, "[FILE]") {
			fileName := strings.TrimPrefix(line, "[FILE]")
			// Extract just filename (before size info)
			parts := strings.Fields(fileName)
			if len(parts) > 0 {
				files = append(files, parts[0])
			}
		}
	}

	var parts []string
	if len(dirs) > 0 {
		if len(dirs) <= 3 {
			parts = append(parts, fmt.Sprintf("dirs: %s", strings.Join(dirs, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d dirs: %s...", len(dirs), strings.Join(dirs[:3], ", ")))
		}
	}
	if len(files) > 0 {
		if len(files) <= 3 {
			parts = append(parts, fmt.Sprintf("files: %s", strings.Join(files, ", ")))
		} else {
			parts = append(parts, fmt.Sprintf("%d files: %s...", len(files), strings.Join(files[:3], ", ")))
		}
	}

	if len(parts) > 0 {
		return "  → " + strings.Join(parts, " | ")
	}
	return "  → empty"
}
