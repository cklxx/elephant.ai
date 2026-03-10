package lark

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/utils"
)

// CostTrackerReader is a narrow read-only port for the /usage command.
// Satisfied by storage.CostTracker from the domain layer.
type CostTrackerReader interface {
	GetDailyCost(ctx context.Context, date time.Time) (*agentstorage.CostSummary, error)
	GetDateRangeCost(ctx context.Context, start, end time.Time) (*agentstorage.CostSummary, error)
}

// isUsageCommand checks whether the message is a /usage or /stats command.
func (g *Gateway) isUsageCommand(trimmed string) bool {
	lower := utils.TrimLower(trimmed)
	return lower == "/usage" || lower == "/stats" ||
		strings.HasPrefix(lower, "/usage ") || strings.HasPrefix(lower, "/stats ")
}

// handleUsageCommand processes /usage and /stats commands, returning a
// formatted summary of token usage, cost, and top tasks.
func (g *Gateway) handleUsageCommand(msg *incomingMessage) {
	if g == nil || msg == nil {
		return
	}
	execCtx := g.buildTaskCommandContext(msg)
	reply := g.buildUsageReply(execCtx, msg)
	g.dispatch(execCtx, msg.chatID, replyTarget(msg.messageID, true), "text", textContent(reply))
}

// buildUsageReply assembles the usage dashboard text from available data sources.
func (g *Gateway) buildUsageReply(ctx context.Context, msg *incomingMessage) string {
	now := g.currentTime()
	var sb strings.Builder

	sb.WriteString("== AI \u7528\u91cf\u7edf\u8ba1 ==\n") // "AI 用量统计"

	// Section 1: Current model info
	sb.WriteString(g.formatCurrentModel(ctx, msg))

	// Section 2: Cost tracker data (today + this week)
	sb.WriteString(g.formatCostSummary(ctx, now))

	// Section 3: Top 3 tasks by token usage (from TaskStore)
	sb.WriteString(g.formatTopTasks(ctx, msg.chatID))

	// Section 4: Active task count
	sb.WriteString(g.formatActiveTaskSummary(ctx, msg.chatID))

	sb.WriteString("\n\u56de\u590d /tasks \u67e5\u770b\u4efb\u52a1\u5217\u8868\uff0c/model \u67e5\u770b\u6a21\u578b\u914d\u7f6e\u3002") // "回复 /tasks 查看任务列表，/model 查看模型配置。"
	return sb.String()
}

// formatCurrentModel returns the current LLM model info section.
func (g *Gateway) formatCurrentModel(_ context.Context, msg *incomingMessage) string {
	var sb strings.Builder
	sb.WriteString("\n")

	model := ""
	provider := ""
	if g.llmSelections != nil && g.llmResolver != nil {
		if sel, _, ok, _ := g.llmSelections.GetWithFallback(context.Background(), selectionScopes(msg)...); ok {
			if resolved, rok := g.llmResolver.Resolve(sel); rok {
				model = strings.TrimSpace(resolved.Model)
				provider = strings.TrimSpace(resolved.Provider)
			}
		}
	}
	if model == "" {
		model = strings.TrimSpace(g.llmProfile.Model)
		provider = strings.TrimSpace(g.llmProfile.Provider)
	}

	if model != "" {
		sb.WriteString(fmt.Sprintf("\u5f53\u524d\u6a21\u578b: %s", model)) // "当前模型"
		if provider != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", provider))
		}
		pricing := agentstorage.GetModelPricing(model)
		if pricing.InputPer1K > 0 {
			sb.WriteString(fmt.Sprintf("\n\u5355\u4ef7: $%.4f / $%.4f per 1K tokens (in/out)", // "单价"
				pricing.InputPer1K, pricing.OutputPer1K))
		}
		sb.WriteString("\n")
	} else {
		sb.WriteString("\u5f53\u524d\u6a21\u578b: \u672a\u914d\u7f6e\n") // "当前模型: 未配置"
	}

	return sb.String()
}

// formatCostSummary returns today's and this week's cost data from the CostTracker.
func (g *Gateway) formatCostSummary(ctx context.Context, now time.Time) string {
	if g.costTracker == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// Today
	today, err := g.costTracker.GetDailyCost(ctx, now)
	if err == nil && today != nil && today.RequestCount > 0 {
		sb.WriteString("\u4eca\u65e5\u7528\u91cf:\n") // "今日用量:"
		sb.WriteString(formatCostSummaryBlock(today))
	} else {
		sb.WriteString("\u4eca\u65e5\u7528\u91cf: \u6682\u65e0\u8bb0\u5f55\n") // "今日用量: 暂无记录"
	}

	// This week (Monday to now)
	weekStart := startOfWeek(now)
	weekly, err := g.costTracker.GetDateRangeCost(ctx, weekStart, now)
	if err == nil && weekly != nil && weekly.RequestCount > 0 {
		sb.WriteString("\n\u672c\u5468\u7d2f\u8ba1:\n") // "本周累计:"
		sb.WriteString(formatCostSummaryBlock(weekly))
	}

	return sb.String()
}

// formatCostSummaryBlock formats a CostSummary into readable lines.
func formatCostSummaryBlock(s *agentstorage.CostSummary) string {
	if s == nil {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Tokens: %s (in: %s, out: %s)\n",
		formatTokens(s.TotalTokens), formatTokens(s.InputTokens), formatTokens(s.OutputTokens)))
	sb.WriteString(fmt.Sprintf("  \u8d39\u7528: $%.4f\n", s.TotalCost)) // "费用"
	sb.WriteString(fmt.Sprintf("  \u8bf7\u6c42\u6570: %d\n", s.RequestCount)) // "请求数"
	if len(s.ByModel) > 0 {
		sb.WriteString("  \u6a21\u578b\u5206\u5e03: ") // "模型分布: "
		parts := make([]string, 0, len(s.ByModel))
		for model, cost := range s.ByModel {
			parts = append(parts, fmt.Sprintf("%s $%.4f", model, cost))
		}
		sort.Strings(parts)
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatTopTasks returns the top 3 tasks by token usage in this chat.
func (g *Gateway) formatTopTasks(ctx context.Context, chatID string) string {
	if g.taskStore == nil {
		return ""
	}
	tasks, err := g.taskStore.ListByChat(ctx, chatID, false, 100)
	if err != nil || len(tasks) == 0 {
		return ""
	}

	// Filter to tasks with token data and sort by TokensUsed descending.
	withTokens := make([]TaskRecord, 0, len(tasks))
	for _, t := range tasks {
		if t.TokensUsed > 0 {
			withTokens = append(withTokens, t)
		}
	}
	if len(withTokens) == 0 {
		return ""
	}
	sort.Slice(withTokens, func(i, j int) bool {
		return withTokens[i].TokensUsed > withTokens[j].TokensUsed
	})
	if len(withTokens) > 3 {
		withTokens = withTokens[:3]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\nTop %d \u6d88\u8017\u4efb\u52a1:\n", len(withTokens))) // "消耗任务"
	for i, t := range withTokens {
		desc := truncateForLark(t.Description, 40)
		if desc == "" {
			desc = t.TaskID
		}
		sb.WriteString(fmt.Sprintf("  %d. %s \u00b7 %s \u00b7 %s tokens\n", // middle dot
			i+1, shortID(t.TaskID), desc, formatTokens(t.TokensUsed)))
	}
	return sb.String()
}

// formatActiveTaskSummary returns a one-line active task count.
func (g *Gateway) formatActiveTaskSummary(ctx context.Context, chatID string) string {
	if g.taskStore == nil {
		return ""
	}
	tasks, err := g.taskStore.ListByChat(ctx, chatID, true, 20)
	if err != nil {
		return ""
	}
	if len(tasks) == 0 {
		return "\n\u5f53\u524d\u6d3b\u8dc3\u4efb\u52a1: 0\n" // "当前活跃任务: 0"
	}
	return fmt.Sprintf("\n\u5f53\u524d\u6d3b\u8dc3\u4efb\u52a1: %d\n", len(tasks)) // "当前活跃任务: N"
}

// startOfWeek returns the Monday 00:00 of the week containing t.
func startOfWeek(t time.Time) time.Time {
	weekday := t.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	monday := t.AddDate(0, 0, -int(weekday-time.Monday))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, t.Location())
}
