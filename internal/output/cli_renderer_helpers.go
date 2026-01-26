package output

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const nonVerbosePreviewLimit = 80

var toolDisplayNames = map[string]string{
	"a2ui_emit":                  "a2ui.emit",
	"artifact_manifest":          "artifact.manifest",
	"artifacts_delete":           "artifact.delete",
	"artifacts_list":             "artifact.list",
	"artifacts_write":            "artifact.write",
	"douyin_hot":                 "douyin.hot",
	"file_edit":                  "file.edit",
	"file_read":                  "file.read",
	"file_write":                 "file.write",
	"image_to_image":             "image.i2i",
	"list_files":                 "file.list",
	"pptx_from_images":           "pptx.images",
	"sandbox_browser":            "sandbox.browser",
	"sandbox_browser_dom":        "browser.dom",
	"sandbox_browser_info":       "browser.info",
	"sandbox_browser_screenshot": "browser.shot",
	"sandbox_code_execute":       "sandbox.exec",
	"sandbox_file_list":          "sandbox.list",
	"sandbox_file_read":          "sandbox.read",
	"sandbox_file_replace":       "sandbox.replace",
	"sandbox_file_search":        "sandbox.search",
	"sandbox_file_write":         "sandbox.write",
	"sandbox_shell_exec":         "sandbox.shell",
	"sandbox_write_attachment":   "sandbox.attach",
	"text_to_image":              "image.t2i",
	"video_generate":             "video.gen",
	"vision_analyze":             "vision.analyze",
	"web_fetch":                  "web.fetch",
	"web_search":                 "web.search",
}

func truncateInlinePreview(preview string, limit int) string {
	if limit <= 0 {
		return preview
	}

	if utf8.RuneCountInString(preview) <= limit {
		return preview
	}

	runes := []rune(preview)
	if len(runes) <= limit {
		return preview
	}

	if limit == 1 {
		return string(runes[0])
	}

	return string(runes[:limit-1]) + "…"
}

func nextSpinnerFrame() string {
	frames := []string{"|", "/", "-", "\\"}
	idx := time.Now().UnixNano() % int64(len(frames))
	return frames[idx]
}

func isConversationalTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "plan", "clearify", "claify", "request_user":
		return true
	default:
		return false
	}
}

func displayToolName(toolName string) string {
	normalized := strings.ToLower(strings.TrimSpace(toolName))
	if normalized == "" {
		return toolName
	}
	if display, ok := toolDisplayNames[normalized]; ok {
		return display
	}
	return toolName
}

func appendDurationSuffix(rendered string, duration time.Duration) string {
	if rendered == "" || duration <= 0 {
		return rendered
	}
	formatted := formatDurationShort(duration)
	if formatted == "" {
		return rendered
	}
	suffix := fmt.Sprintf(" (%s)", formatted)
	newline := strings.Index(rendered, "\n")
	if newline == -1 {
		return rendered + suffix
	}
	return rendered[:newline] + suffix + rendered[newline:]
}

func formatDurationShort(duration time.Duration) string {
	if duration <= 0 {
		return ""
	}
	if duration < time.Second {
		return fmt.Sprintf("%dms", duration.Milliseconds())
	}
	if duration < time.Minute {
		seconds := duration.Seconds()
		if seconds < 10 {
			return fmt.Sprintf("%.2fs", seconds)
		}
		if seconds < 100 {
			return fmt.Sprintf("%.1fs", seconds)
		}
		return fmt.Sprintf("%.0fs", seconds)
	}
	if duration < time.Hour {
		minutes := int(duration.Minutes())
		seconds := int(duration.Seconds()) % 60
		return fmt.Sprintf("%dm%02ds", minutes, seconds)
	}
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	return fmt.Sprintf("%dh%02dm", hours, minutes)
}

func truncateWithEllipsis(preview string, limit int) string {
	if limit <= 0 {
		return preview
	}

	runes := []rune(preview)
	if len(runes) <= limit {
		return preview
	}

	ellipsis := "..."
	if limit <= len(ellipsis) {
		return string(runes[:limit])
	}

	return string(runes[:limit-len(ellipsis)]) + ellipsis
}

func countLines(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

func formatBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	value := float64(bytes)
	for _, unit := range units {
		value /= 1024
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%.1f PB", value)
}

// filterSystemReminders removes <system-reminder> tags from output
func filterSystemReminders(content string) string {
	lines := strings.Split(content, "\n")
	var filtered []string
	inReminder := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "<system-reminder>") {
			inReminder = true
			if strings.HasSuffix(trimmed, "</system-reminder>") {
				inReminder = false
			}
			continue
		}
		if strings.HasSuffix(trimmed, "</system-reminder>") {
			inReminder = false
			continue
		}
		if !inReminder {
			filtered = append(filtered, line)
		}
	}

	return strings.TrimSpace(strings.Join(filtered, "\n"))
}
