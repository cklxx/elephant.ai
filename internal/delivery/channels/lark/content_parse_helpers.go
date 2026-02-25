package lark

import (
	"encoding/json"
	"strings"
)

type larkTextPayload struct {
	Text string `json:"text"`
}

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
	var parsed larkTextPayload
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

func flattenLarkPostPayload(parsed larkPostPayload, renderText func(larkPostElement) string, renderAt func(larkPostElement) string) string {
	var sb strings.Builder
	if title := strings.TrimSpace(parsed.Title); title != "" {
		sb.WriteString(title)
	}
	for _, line := range parsed.Content {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		for _, el := range line {
			switch el.Tag {
			case "text":
				sb.WriteString(renderText(el))
			case "at":
				sb.WriteString(renderAt(el))
			default:
				if el.Text != "" {
					sb.WriteString(el.Text)
				}
			}
		}
	}
	return strings.TrimSpace(sb.String())
}
