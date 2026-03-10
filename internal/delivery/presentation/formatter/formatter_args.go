package formatter

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (tf *ToolFormatter) bashArgs(args map[string]any) map[string]string {
	command := tf.getStringArg(args, "command", "")
	if command == "" {
		return map[string]string{}
	}
	return map[string]string{
		"command": tf.summarizeString(command, 160),
	}
}

func (tf *ToolFormatter) fileReadArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	} else if path := tf.getStringArg(args, "path", ""); path != "" {
		result["path"] = path
	}
	if offset := tf.getIntArg(args, "offset", 0); offset > 0 {
		result["offset"] = strconv.Itoa(offset)
	} else if offset := tf.getIntArg(args, "start_line", 0); offset > 0 {
		result["offset"] = strconv.Itoa(offset)
	}
	if limit := tf.getIntArg(args, "limit", 0); limit > 0 {
		result["limit"] = strconv.Itoa(limit)
	} else if end := tf.getIntArg(args, "end_line", 0); end > 0 {
		start := tf.getIntArg(args, "start_line", 0)
		if end > start {
			result["limit"] = strconv.Itoa(end - start)
		}
	}
	return result
}

func (tf *ToolFormatter) fileEditArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	} else if path := tf.getStringArg(args, "path", ""); path != "" {
		result["path"] = path
	}
	oldStr := tf.getStringArg(args, "old_string", "")
	if oldStr == "" {
		oldStr = tf.getStringArg(args, "old_str", "")
	}
	if oldStr != "" {
		result["old_lines"] = strconv.Itoa(countLines(oldStr))
	}
	newStr := tf.getStringArg(args, "new_string", "")
	if newStr == "" {
		newStr = tf.getStringArg(args, "new_str", "")
	}
	if newStr != "" {
		result["new_lines"] = strconv.Itoa(countLines(newStr))
	}
	if oldStr != "" && newStr != "" {
		delta := utf8.RuneCountInString(newStr) - utf8.RuneCountInString(oldStr)
		result["delta_chars"] = strconv.Itoa(delta)
	}
	return result
}

func (tf *ToolFormatter) fileWriteArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if path := tf.getStringArg(args, "file_path", ""); path != "" {
		result["path"] = path
	} else if path := tf.getStringArg(args, "path", ""); path != "" {
		result["path"] = path
	}
	if content := tf.getStringArg(args, "content", ""); content != "" {
		result["lines"] = strconv.Itoa(countLines(content))
		result["chars"] = strconv.Itoa(utf8.RuneCountInString(content))
	}
	return result
}

func (tf *ToolFormatter) searchArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if pattern := tf.getStringArg(args, "pattern", ""); pattern != "" {
		result["pattern"] = tf.summarizeString(pattern, 80)
	}
	if path := tf.getStringArg(args, "path", ""); path != "" && path != "." {
		result["path"] = path
	}
	return result
}

func (tf *ToolFormatter) findArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if pattern := tf.getStringArg(args, "pattern", ""); pattern != "" {
		result["pattern"] = tf.summarizeString(pattern, 60)
	}
	if path := tf.getStringArg(args, "path", ""); path != "" && path != "." {
		result["path"] = path
	}
	return result
}

func (tf *ToolFormatter) webSearchArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if query := tf.getStringArg(args, "query", ""); query != "" {
		result["query"] = tf.summarizeString(query, 80)
	}
	if maxResults := tf.getIntArg(args, "max_results", 0); maxResults > 0 {
		result["max_results"] = strconv.Itoa(maxResults)
	}
	return result
}

func (tf *ToolFormatter) webFetchArgs(args map[string]any) map[string]string {
	if url := tf.getStringArg(args, "url", ""); url != "" {
		return map[string]string{"url": tf.summarizeString(url, 120)}
	}
	return map[string]string{}
}

func (tf *ToolFormatter) simplePathArgs(args map[string]any) map[string]string {
	if path := tf.getStringArg(args, "path", ""); path != "" {
		return map[string]string{"path": path}
	}
	return map[string]string{}
}

func (tf *ToolFormatter) finalArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	if answer := tf.getStringArg(args, "answer", ""); answer != "" {
		result["answer"] = tf.summarizeString(answer, 120)
	}
	if highlights := tf.getStringSliceArg(args, "highlights"); len(highlights) > 0 {
		summary := strings.Join(highlights, " | ")
		result["highlights"] = tf.summarizeString(summary, 120)
	}
	return result
}

func (tf *ToolFormatter) genericArgs(args map[string]any) map[string]string {
	result := make(map[string]string)
	for key, value := range args {
		result[key] = tf.summarizeString(tf.formatValue(value), 80)
	}
	return result
}

func (tf *ToolFormatter) summarizeString(value string, limit int) string {
	cleaned := strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if cleaned == "" {
		return ""
	}

	runes := []rune(cleaned)
	if len(runes) <= limit {
		return cleaned
	}

	truncated := string(runes[:limit])
	remaining := len(runes) - limit
	if remaining <= 0 {
		return truncated
	}

	return fmt.Sprintf("%s… (+%d chars)", truncated, remaining)
}
