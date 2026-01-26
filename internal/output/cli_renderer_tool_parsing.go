package output

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type listFilesSummary struct {
	Total      int
	Files      int
	Dirs       int
	TotalBytes int64
}

func parseListFilesSummary(lines []string) listFilesSummary {
	var summary listFilesSummary
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		summary.Total++
		if strings.HasPrefix(trimmed, "[DIR]") {
			summary.Dirs++
			continue
		}
		if strings.HasPrefix(trimmed, "[FILE]") {
			summary.Files++
			if size, ok := parseFileSize(trimmed); ok {
				summary.TotalBytes += size
			}
		}
	}
	return summary
}

func parseFileSize(line string) (int64, bool) {
	open := strings.LastIndex(line, "(")
	close := strings.LastIndex(line, ")")
	if open == -1 || close == -1 || close <= open {
		return 0, false
	}
	inner := strings.TrimSpace(line[open+1 : close])
	fields := strings.Fields(inner)
	if len(fields) == 0 {
		return 0, false
	}
	size, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, false
	}
	return size, true
}

type searchSummary struct {
	Total     int
	Matches   []string
	Truncated bool
	Warning   string
	NoMatches bool
}

func parseSearchSummary(content string) searchSummary {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return searchSummary{NoMatches: true}
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return searchSummary{NoMatches: true}
	}
	first := strings.TrimSpace(lines[0])
	summary := searchSummary{}
	if strings.HasPrefix(first, "No matches found") {
		summary.NoMatches = true
		return summary
	}
	if strings.HasPrefix(first, "Found ") {
		if total, ok := parseFoundMatches(first); ok {
			summary.Total = total
		}
		lines = lines[1:]
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[TRUNCATED]") {
			summary.Truncated = true
			summary.Warning = line
			continue
		}
		summary.Matches = append(summary.Matches, line)
	}
	if summary.Total == 0 {
		summary.Total = len(summary.Matches)
	}
	return summary
}

func parseFoundMatches(line string) (int, bool) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "Found "))
	if rest == "" {
		return 0, false
	}
	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return 0, false
	}
	value, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return value, true
}

func summarizeFileOperation(content string) (string, bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "Wrote ") && strings.Contains(trimmed, " bytes to ") {
		parts := strings.SplitN(trimmed, " bytes to ", 2)
		if len(parts) != 2 {
			return "", false
		}
		bytesText := strings.TrimSpace(strings.TrimPrefix(parts[0], "Wrote "))
		path := strings.TrimSpace(parts[1])
		if path == "" {
			return "", false
		}
		if bytesValue, err := strconv.ParseInt(bytesText, 10, 64); err == nil {
			return fmt.Sprintf("wrote %s (%s)", path, formatBytes(bytesValue)), true
		}
		return fmt.Sprintf("wrote %s", path), true
	}
	if strings.HasPrefix(trimmed, "Created ") {
		return parseFileLineSummary("created", strings.TrimPrefix(trimmed, "Created "))
	}
	if strings.HasPrefix(trimmed, "Updated ") {
		return parseFileLineSummary("updated", strings.TrimPrefix(trimmed, "Updated "))
	}
	if strings.HasPrefix(trimmed, "Replaced ") && strings.Contains(trimmed, " in ") {
		parts := strings.SplitN(strings.TrimPrefix(trimmed, "Replaced "), " in ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("replaced %s in %s", strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])), true
		}
	}
	return "", false
}

func parseFileLineSummary(action, remainder string) (string, bool) {
	value := strings.TrimSpace(remainder)
	if value == "" {
		return "", false
	}
	idx := strings.LastIndex(value, " (")
	if idx == -1 || !strings.HasSuffix(value, ")") {
		return fmt.Sprintf("%s %s", action, value), true
	}
	path := strings.TrimSpace(value[:idx])
	suffix := strings.TrimSuffix(value[idx+2:], ")")
	fields := strings.Fields(suffix)
	if len(fields) > 0 {
		return fmt.Sprintf("%s %s (%s lines)", action, path, fields[0]), true
	}
	return fmt.Sprintf("%s %s", action, path), true
}

type webSearchItem struct {
	Title string
	URL   string
}

type webSearchSummary struct {
	Query       string
	Summary     string
	ResultCount int
	Results     []webSearchItem
}

func parseWebSearchContent(content string) webSearchSummary {
	var summary webSearchSummary
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "Search:"):
			summary.Query = strings.TrimSpace(strings.TrimPrefix(trimmed, "Search:"))
		case strings.HasPrefix(trimmed, "Summary:"):
			summary.Summary = strings.TrimSpace(strings.TrimPrefix(trimmed, "Summary:"))
		case strings.HasSuffix(trimmed, "Results:"):
			fields := strings.Fields(trimmed)
			if len(fields) > 0 {
				if count, err := strconv.Atoi(fields[0]); err == nil {
					summary.ResultCount = count
				}
			}
		case strings.HasPrefix(trimmed, "URL:"):
			if len(summary.Results) > 0 {
				summary.Results[len(summary.Results)-1].URL = strings.TrimSpace(strings.TrimPrefix(trimmed, "URL:"))
			}
		default:
			if title, ok := parseNumberedTitle(trimmed); ok {
				summary.Results = append(summary.Results, webSearchItem{Title: title})
			}
		}
	}
	if summary.ResultCount == 0 && len(summary.Results) > 0 {
		summary.ResultCount = len(summary.Results)
	}
	return summary
}

func parseNumberedTitle(line string) (string, bool) {
	idx := strings.Index(line, ".")
	if idx <= 0 {
		return "", false
	}
	if _, err := strconv.Atoi(strings.TrimSpace(line[:idx])); err != nil {
		return "", false
	}
	title := strings.TrimSpace(line[idx+1:])
	if title == "" {
		return "", false
	}
	return title, true
}

type webFetchSummary struct {
	URL      string
	Cached   bool
	Question string
	Analysis string
	Content  string
}

func parseWebFetchContent(content string) webFetchSummary {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return webFetchSummary{}
	}
	lines := strings.Split(trimmed, "\n")
	summary := webFetchSummary{}
	index := 0
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "Source:") {
		source := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "Source:"))
		if strings.HasSuffix(source, "(cached)") {
			summary.Cached = true
			source = strings.TrimSpace(strings.TrimSuffix(source, "(cached)"))
		}
		summary.URL = source
		index = 1
	}
	for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
		index++
	}
	if index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "Question:") {
		summary.Question = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[index]), "Question:"))
		index++
		for index < len(lines) && strings.TrimSpace(lines[index]) == "" {
			index++
		}
		if index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "Analysis:") {
			index++
			summary.Analysis = strings.TrimSpace(strings.Join(lines[index:], "\n"))
			return summary
		}
		summary.Content = strings.TrimSpace(strings.Join(lines[index:], "\n"))
		return summary
	}
	summary.Content = strings.TrimSpace(strings.Join(lines[index:], "\n"))
	return summary
}

func hostFromURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.Host != "" {
		return parsed.Host
	}
	return trimmed
}

func takePreviewLines(content string, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

type sandboxFileListSummary struct {
	Path       string
	Total      int
	Files      int
	Dirs       int
	TotalBytes int64
	Entries    []sandboxFileEntry
}

type sandboxFileEntry struct {
	Path  string
	IsDir bool
	Size  *int64
}

func parseSandboxFileListSummary(content string) (sandboxFileListSummary, bool) {
	var payload struct {
		Path  string `json:"path"`
		Files []struct {
			Path        string  `json:"path"`
			Name        string  `json:"name"`
			IsDirectory bool    `json:"is_directory"`
			Size        *int64  `json:"size"`
			Permissions *string `json:"permissions"`
		} `json:"files"`
		TotalCount     int `json:"total_count"`
		DirectoryCount int `json:"directory_count"`
		FileCount      int `json:"file_count"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return sandboxFileListSummary{}, false
	}
	summary := sandboxFileListSummary{
		Path:  payload.Path,
		Total: payload.TotalCount,
		Files: payload.FileCount,
		Dirs:  payload.DirectoryCount,
	}
	derivedFiles := 0
	derivedDirs := 0
	for _, entry := range payload.Files {
		if entry.IsDirectory {
			derivedDirs++
		} else {
			derivedFiles++
		}
	}
	if summary.Files == 0 && summary.Dirs == 0 && (derivedFiles > 0 || derivedDirs > 0) {
		summary.Files = derivedFiles
		summary.Dirs = derivedDirs
	}
	if summary.Total == 0 && len(payload.Files) > 0 {
		summary.Total = len(payload.Files)
	}
	for _, entry := range payload.Files {
		summary.Entries = append(summary.Entries, sandboxFileEntry{
			Path:  entry.Path,
			IsDir: entry.IsDirectory,
			Size:  entry.Size,
		})
		if entry.Size != nil {
			summary.TotalBytes += *entry.Size
		}
	}
	return summary, true
}

type sandboxFileSearchSummary struct {
	File    string
	Matches []string
	Lines   []int
}

func parseSandboxFileSearchSummary(content string) (sandboxFileSearchSummary, bool) {
	var payload struct {
		File        string   `json:"file"`
		Matches     []string `json:"matches"`
		LineNumbers []int    `json:"line_numbers"`
	}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		return sandboxFileSearchSummary{}, false
	}
	return sandboxFileSearchSummary{
		File:    payload.File,
		Matches: payload.Matches,
		Lines:   payload.LineNumbers,
	}, true
}
