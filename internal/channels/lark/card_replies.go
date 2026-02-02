package lark

import (
	"fmt"
	"strings"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/jsonx"
	larkcards "alex/internal/lark/cards"
)

const maxCardReplyChars = 1200

func (g *Gateway) buildPlanReviewCard(marker planReviewMarker) (string, error) {
	planMarkdown := ""
	if marker.InternalPlan != nil {
		if data, err := jsonx.MarshalIndent(marker.InternalPlan, "", "  "); err == nil {
			planMarkdown = "```json\n" + string(data) + "\n```"
		} else {
			planMarkdown = fmt.Sprintf("%v", marker.InternalPlan)
		}
	}
	planMarkdown = truncateCardText(planMarkdown, maxCardReplyChars)
	return larkcards.PlanReviewCard(larkcards.PlanReviewParams{
		Title:                "计划确认",
		Goal:                 marker.OverallGoalUI,
		PlanMarkdown:         planMarkdown,
		RunID:                marker.RunID,
		RequireConfirmation:  g.cfg.PlanReviewRequireConfirmation,
		IncludeFeedbackInput: true,
	})
}

func (g *Gateway) buildCardReply(reply string, result *agent.TaskResult, execErr error) (string, error) {
	attachments := buildCardAttachmentMarkdown(result)
	if execErr != nil {
		message := strings.TrimSpace(reply)
		if message == "" {
			message = strings.TrimSpace(execErr.Error())
		}
		if len(message) > maxCardReplyChars {
			return "", fmt.Errorf("reply too long for card")
		}
		message = truncateCardText(message, maxCardReplyChars)
		if attachments != "" {
			message = message + "\n\n" + attachments
		}
		return larkcards.ErrorCard("执行失败", message)
	}

	summary := strings.TrimSpace(reply)
	if summary == "" {
		return "", fmt.Errorf("empty reply")
	}
	if len(summary) > maxCardReplyChars {
		return "", fmt.Errorf("reply too long for card")
	}
	summary = truncateCardText(summary, maxCardReplyChars)
	if attachments != "" {
		summary = summary + "\n\n" + attachments
	}
	return larkcards.ResultCard(larkcards.ResultParams{
		Title:         "任务完成",
		Summary:       summary,
		EnableForward: true,
	})
}

func truncateCardText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || limit <= 0 {
		return trimmed
	}
	if len(trimmed) <= limit {
		return trimmed
	}
	if limit <= 3 {
		return trimmed[:limit]
	}
	return trimmed[:limit-3] + "..."
}

func buildCardAttachmentMarkdown(result *agent.TaskResult) string {
	if result == nil || len(result.Attachments) == 0 {
		return ""
	}
	var lines []string
	for _, name := range sortedAttachmentNames(result.Attachments) {
		att := result.Attachments[name]
		if isA2UIAttachment(att) {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			lines = append(lines, fmt.Sprintf("- %s", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, uri))
	}
	if len(lines) == 0 {
		return ""
	}
	return "**附件**\n" + strings.Join(lines, "\n")
}
