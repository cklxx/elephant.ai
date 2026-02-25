package lark

import (
	"context"
	"fmt"
	"strings"

	ports "alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
	jsonx "alex/internal/shared/json"
)

const (
	planReviewMarkerStart = "<plan_review_pending>"
	planReviewMarkerEnd   = "</plan_review_pending>"
)

type planReviewMarker struct {
	RunID         string
	OverallGoalUI string
	InternalPlan  any
}

func (g *Gateway) loadPlanReviewPending(ctx context.Context, session *storage.Session, userID, chatID string) (PlanReviewPending, bool) {
	if g == nil || userID == "" || chatID == "" {
		return PlanReviewPending{}, false
	}
	if g.planReviewStore != nil {
		pending, ok, err := g.planReviewStore.GetPending(ctx, userID, chatID)
		if err != nil {
			g.logger.Warn("Lark plan review pending load failed: %v", err)
			return PlanReviewPending{}, false
		}
		if ok {
			return pending, true
		}
		return PlanReviewPending{}, false
	}
	if session == nil || len(session.Messages) == 0 {
		return PlanReviewPending{}, false
	}
	if marker, ok := extractPlanReviewMarker(session.Messages); ok {
		return PlanReviewPending{
			UserID:        userID,
			ChatID:        chatID,
			RunID:         marker.RunID,
			OverallGoalUI: marker.OverallGoalUI,
			InternalPlan:  marker.InternalPlan,
		}, true
	}
	return PlanReviewPending{}, false
}

func extractPlanReviewMarker(messages []ports.Message) (planReviewMarker, bool) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		if strings.ToLower(strings.TrimSpace(msg.Role)) != "system" {
			continue
		}
		if marker, ok := parsePlanReviewMarker(msg.Content); ok {
			return marker, true
		}
	}
	return planReviewMarker{}, false
}

func parsePlanReviewMarker(content string) (planReviewMarker, bool) {
	start := strings.Index(content, planReviewMarkerStart)
	end := strings.Index(content, planReviewMarkerEnd)
	if start == -1 || end == -1 || end <= start {
		return planReviewMarker{}, false
	}
	body := strings.TrimSpace(content[start+len(planReviewMarkerStart) : end])
	if body == "" {
		return planReviewMarker{}, false
	}
	marker := planReviewMarker{}
	lines := strings.Split(body, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "run_id:"):
			marker.RunID = strings.TrimSpace(strings.TrimPrefix(line, "run_id:"))
		case strings.HasPrefix(line, "overall_goal_ui:"):
			marker.OverallGoalUI = strings.TrimSpace(strings.TrimPrefix(line, "overall_goal_ui:"))
		case strings.HasPrefix(line, "internal_plan:"):
			raw := strings.TrimSpace(strings.TrimPrefix(line, "internal_plan:"))
			if raw != "" {
				var plan any
				if err := jsonx.Unmarshal([]byte(raw), &plan); err == nil {
					marker.InternalPlan = plan
				} else {
					marker.InternalPlan = raw
				}
			}
		}
	}
	if strings.TrimSpace(marker.OverallGoalUI) == "" {
		return planReviewMarker{}, false
	}
	return marker, true
}

func buildPlanReviewReply(marker planReviewMarker, requireConfirmation bool) string {
	var sb strings.Builder
	sb.WriteString("计划确认\n")
	if marker.OverallGoalUI != "" {
		sb.WriteString("目标: ")
		sb.WriteString(marker.OverallGoalUI)
		sb.WriteString("\n")
	}
	if marker.InternalPlan != nil {
		sb.WriteString("\n计划:\n")
		if data, err := jsonx.MarshalIndent(marker.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", marker.InternalPlan))
		}
		sb.WriteString("\n")
	}
	if requireConfirmation {
		sb.WriteString("\n请回复 OK 继续，或直接回复修改意见。")
	}
	return strings.TrimSpace(sb.String())
}

func buildPlanFeedbackBlock(pending PlanReviewPending, userFeedback string) string {
	var sb strings.Builder
	sb.WriteString("<plan_feedback>\n")
	sb.WriteString("plan:\n")
	if pending.OverallGoalUI != "" {
		sb.WriteString("goal: ")
		sb.WriteString(strings.TrimSpace(pending.OverallGoalUI))
		sb.WriteString("\n")
	}
	if pending.InternalPlan != nil {
		sb.WriteString("internal_plan: ")
		if data, err := jsonx.MarshalIndent(pending.InternalPlan, "", "  "); err == nil {
			sb.WriteString(string(data))
		} else {
			sb.WriteString(fmt.Sprintf("%v", pending.InternalPlan))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nuser_feedback:\n")
	sb.WriteString(strings.TrimSpace(userFeedback))
	sb.WriteString("\n\ninstruction: If the feedback changes the plan, call plan() again; otherwise continue with the next step.\n")
	sb.WriteString("</plan_feedback>")
	return strings.TrimSpace(sb.String())
}
