package output

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

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

