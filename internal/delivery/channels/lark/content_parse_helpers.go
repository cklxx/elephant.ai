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
	ZhCN    struct {
		Title   string              `json:"title"`
		Content [][]larkPostElement `json:"content"`
	} `json:"zh_cn"`
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
	if len(parsed.Content) == 0 && len(parsed.ZhCN.Content) > 0 {
		parsed.Content = parsed.ZhCN.Content
	}
	if strings.TrimSpace(parsed.Title) == "" && strings.TrimSpace(parsed.ZhCN.Title) != "" {
		parsed.Title = parsed.ZhCN.Title
	}
	return parsed, len(parsed.Content) > 0 || strings.TrimSpace(parsed.Title) != ""
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
