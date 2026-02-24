package lark

import (
	"encoding/json"
	"strings"
)

type larkPostElement struct {
	Tag      string `json:"tag"`
	Text     string `json:"text"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
}

type larkPostPayload struct {
	Title   string              `json:"title"`
	Content [][]larkPostElement `json:"content"`
}

func parseLarkTextPayload(raw string) (string, bool) {
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return "", false
	}
	return strings.TrimSpace(parsed.Text), true
}

func parseLarkPostPayload(raw string) (larkPostPayload, bool) {
	var parsed larkPostPayload
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return larkPostPayload{}, false
	}
	return parsed, true
}

func flattenLarkPostPayload(
	payload larkPostPayload,
	renderText func(string) string,
	renderMention func(larkPostElement) string,
) string {
	var builder strings.Builder
	if title := strings.TrimSpace(payload.Title); title != "" {
		builder.WriteString(title)
	}

	for _, line := range payload.Content {
		if builder.Len() > 0 {
			builder.WriteByte('\n')
		}
		for _, element := range line {
			switch element.Tag {
			case "text":
				if renderText == nil {
					builder.WriteString(element.Text)
					continue
				}
				builder.WriteString(renderText(element.Text))
			case "at":
				if renderMention == nil {
					continue
				}
				builder.WriteString(renderMention(element))
			default:
				if element.Text != "" {
					builder.WriteString(element.Text)
				}
			}
		}
	}

	return strings.TrimSpace(builder.String())
}
