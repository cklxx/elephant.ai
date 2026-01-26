package output

import (
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func ConstrainOutputWidth(text string, w io.Writer) string {
	return ConstrainWidth(text, detectOutputWidth(w))
}

func ConstrainWidth(text string, width int) string {
	if text == "" || width <= 0 {
		return text
	}

	parts := strings.SplitAfter(text, "\n")
	for i, part := range parts {
		line := part
		newline := ""
		if strings.HasSuffix(part, "\n") {
			line = strings.TrimSuffix(part, "\n")
			newline = "\n"
		}
		if line == "" {
			parts[i] = part
			continue
		}
		if ansi.StringWidth(line) > width {
			line = ansi.Truncate(line, width, "…")
		}
		parts[i] = line + newline
	}

	return strings.Join(parts, "")
}
