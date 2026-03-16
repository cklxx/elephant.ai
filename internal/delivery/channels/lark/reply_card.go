package lark

import (
	"strings"
)

// buildResultCard creates a Lark interactive card for structured task results.
// It extracts the first line/sentence as a bold conclusion, shows the rest as
// markdown body content, keeping replies scannable in chat.
func buildResultCard(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return buildLarkCard("", "blue", []any{
			map[string]any{"tag": "markdown", "content": "完成。"},
		})
	}

	conclusion, body := splitConclusion(text)
	var elements []any

	// Conclusion as bold header text.
	if conclusion != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**" + conclusion + "**",
		})
	}

	// Body content (may contain tables, code blocks, lists).
	if body != "" {
		// Check for tables in body — lift them into card table components.
		bodyElements := buildCardElementsFromMarkdown(body)
		elements = append(elements, bodyElements...)
	}

	if len(elements) == 0 {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": text,
		})
	}

	return buildLarkCard("", "blue", elements)
}

// buildErrorCard creates a Lark interactive card for error results with
// actionable guidance for the user.
func buildErrorCard(errorMsg string) string {
	errorMsg = strings.TrimSpace(errorMsg)
	if errorMsg == "" {
		errorMsg = "执行过程中遇到了问题"
	}

	elements := []any{
		map[string]any{
			"tag":     "markdown",
			"content": errorMsg,
		},
		map[string]any{
			"tag": "hr",
		},
		map[string]any{
			"tag":     "markdown",
			"content": "你可以：\n- 直接回复**重试**让我再试一次\n- 换个方式描述你的需求\n- 回复**诊断**查看详细信息",
		},
	}

	return buildLarkCard("执行失败", "red", elements)
}

// buildPlanReviewCard creates a Lark interactive card for plan review with
// structured plan display and clear action guidance.
func buildPlanReviewCard(goal string, planText string, requireConfirmation bool) string {
	var elements []any

	if goal != "" {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**目标**\n" + goal,
		})
	}

	if planText != "" {
		elements = append(elements, map[string]any{
			"tag": "hr",
		})
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "**计划**\n" + planText,
		})
	}

	if requireConfirmation {
		elements = append(elements, map[string]any{
			"tag": "hr",
		})
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "回复 **OK** 确认执行，或直接回复修改意见。",
		})
	}

	if len(elements) == 0 {
		elements = append(elements, map[string]any{
			"tag":     "markdown",
			"content": "计划已生成，请确认。",
		})
	}

	return buildLarkCard("计划确认", "blue", elements)
}

// smartResultContent chooses the best message format for a task result reply.
// Errors use an error card with guidance. Await prompts and short replies use
// smartContent (text/post). Longer successful replies use a result card.
func smartResultContent(reply string, execErr error, isAwait bool) (msgType string, content string) {
	reply = strings.TrimSpace(reply)

	// Error with reply text — use error card.
	if execErr != nil && reply != "" {
		return "interactive", buildErrorCard(reply)
	}

	// Await prompts and very short replies — use existing smartContent logic.
	if isAwait || len([]rune(reply)) < 80 {
		return smartContent(reply)
	}

	// Longer successful replies — use result card for structure.
	return "interactive", buildResultCard(reply)
}

// splitConclusion extracts the first meaningful line as a conclusion and
// returns (conclusion, remainder). The conclusion is the first non-empty line
// up to the first blank line or paragraph break.
func splitConclusion(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}

	lines := strings.Split(text, "\n")
	conclusionLine := ""
	bodyStart := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if conclusionLine != "" {
				bodyStart = i + 1
				break
			}
			continue
		}
		if conclusionLine == "" {
			// Skip headings for conclusion — use content after them.
			if strings.HasPrefix(trimmed, "#") {
				trimmed = strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
			}
			conclusionLine = trimmed
			bodyStart = i + 1
			continue
		}
		// If second line exists without a blank line separator, it's part
		// of the same paragraph — use entire first paragraph as conclusion.
		bodyStart = i
		break
	}

	body := ""
	if bodyStart < len(lines) {
		body = strings.TrimSpace(strings.Join(lines[bodyStart:], "\n"))
	}

	return conclusionLine, body
}
