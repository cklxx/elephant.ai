package agent_eval

import (
	"fmt"
	"sort"
	"strings"
)

func buildFoundationMarkdownReport(result *FoundationEvaluationResult) string {
	var b strings.Builder

	b.WriteString("# Foundation Offline Evaluation Report\n\n")
	b.WriteString(fmt.Sprintf("- Run ID: `%s`\n", result.RunID))
	b.WriteString(fmt.Sprintf("- Generated At (UTC): `%s`\n", result.GeneratedAt.Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("- Mode/Preset/Toolset: `%s / %s / %s`\n", result.Mode, result.Preset, result.Toolset))
	b.WriteString(fmt.Sprintf("- Scenario Set: `%s`\n", result.CasesPath))
	b.WriteString(fmt.Sprintf("- Top-K: `%d`\n\n", result.TopK))

	b.WriteString("## Executive Summary\n\n")
	b.WriteString("| Dimension | Score |\n")
	b.WriteString("|---|---:|\n")
	b.WriteString(fmt.Sprintf("| Prompt Quality | %.1f |\n", result.Prompt.AverageScore))
	b.WriteString(fmt.Sprintf("| Tool Usability | %.1f |\n", result.Tools.AverageUsability))
	b.WriteString(fmt.Sprintf("| Tool Discoverability | %.1f |\n", result.Tools.AverageDiscoverability))
	b.WriteString(fmt.Sprintf("| Implicit Tool-Use (Top-%d hit rate) | %.1f%% |\n", result.TopK, result.Implicit.TopKHitRate*100))
	b.WriteString(fmt.Sprintf("| Overall | **%.1f** |\n\n", result.OverallScore))
	b.WriteString("| Metric | Value |\n")
	b.WriteString("|---|---:|\n")
	b.WriteString(fmt.Sprintf("| Implicit Eval Total Latency (ms) | %d |\n", result.Implicit.TotalEvaluationLatencyMs))
	b.WriteString(fmt.Sprintf("| Case Latency p50/p95/p99 (ms) | %.3f / %.3f / %.3f |\n", result.Implicit.CaseLatencyP50Ms, result.Implicit.CaseLatencyP95Ms, result.Implicit.CaseLatencyP99Ms))
	b.WriteString(fmt.Sprintf("| Throughput (cases/s) | %.2f |\n\n", result.Implicit.ThroughputCasesPerSec))

	b.WriteString("## Prompt Quality\n\n")
	b.WriteString(fmt.Sprintf("- Total prompts: %d\n", result.Prompt.TotalPrompts))
	b.WriteString(fmt.Sprintf("- Strong prompts (>=80): %d\n", result.Prompt.StrongCount))
	b.WriteString(fmt.Sprintf("- Weak prompts (<70): %d\n\n", result.Prompt.WeakCount))

	if len(result.Prompt.Scores) > 0 {
		b.WriteString("### Prompt Scoreboard\n\n")
		b.WriteString("| Prompt | Score | Words | Key Gaps |\n")
		b.WriteString("|---|---:|---:|---|\n")
		for _, score := range result.Prompt.Scores {
			gaps := "-"
			if len(score.Gaps) > 0 {
				gaps = strings.Join(score.Gaps, "; ")
			}
			b.WriteString(fmt.Sprintf("| `%s` | %.1f | %d | %s |\n", score.Name, score.Score, score.WordCount, escapeTable(gaps)))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Tool Usability & Discoverability\n\n")
	b.WriteString(fmt.Sprintf("- Total tools analyzed: %d\n", result.Tools.TotalTools))
	b.WriteString(fmt.Sprintf("- Pass rate (usability >=70): %.1f%%\n", result.Tools.PassRate))
	b.WriteString(fmt.Sprintf("- Critical tools (usability <50): %d\n\n", result.Tools.CriticalIssues))

	if len(result.Tools.IssueBreakdown) > 0 {
		b.WriteString("### Issue Breakdown\n\n")
		b.WriteString("| Issue | Count |\n")
		b.WriteString("|---|---:|\n")
		issues := make([]struct {
			name  string
			count int
		}, 0, len(result.Tools.IssueBreakdown))
		for issue, count := range result.Tools.IssueBreakdown {
			issues = append(issues, struct {
				name  string
				count int
			}{name: issue, count: count})
		}
		sort.Slice(issues, func(i, j int) bool {
			if issues[i].count == issues[j].count {
				return issues[i].name < issues[j].name
			}
			return issues[i].count > issues[j].count
		})
		for _, issue := range issues {
			b.WriteString(fmt.Sprintf("| `%s` | %d |\n", issue.name, issue.count))
		}
		b.WriteString("\n")
	}

	if len(result.Tools.Scores) > 0 {
		b.WriteString("### Weakest Tools (Top 15 by Usability)\n\n")
		b.WriteString("| Tool | Category | Usability | Discoverability | Issues |\n")
		b.WriteString("|---|---|---:|---:|---|\n")
		limit := 15
		if len(result.Tools.Scores) < limit {
			limit = len(result.Tools.Scores)
		}
		for i := 0; i < limit; i++ {
			t := result.Tools.Scores[i]
			issues := "-"
			if len(t.Issues) > 0 {
				issues = strings.Join(t.Issues, ", ")
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %.1f | %.1f | %s |\n",
				t.Name,
				t.Category,
				t.UsabilityScore,
				t.DiscoverabilityScore,
				escapeTable(issues),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Implicit Tool-Use Readiness\n\n")
	b.WriteString(fmt.Sprintf("- Total scenarios: %d\n", result.Implicit.TotalCases))
	b.WriteString(fmt.Sprintf("- Passed (Top-%d): %d/%d\n", result.TopK, result.Implicit.PassedCases, result.Implicit.TotalCases))
	b.WriteString(fmt.Sprintf("- Failed: %d\n", result.Implicit.FailedCases))
	b.WriteString(fmt.Sprintf("- Top-1 hit rate: %.1f%%\n", result.Implicit.Top1HitRate*100))
	b.WriteString(fmt.Sprintf("- Top-%d hit rate: %.1f%%\n", result.TopK, result.Implicit.TopKHitRate*100))
	b.WriteString(fmt.Sprintf("- MRR: %.3f\n\n", result.Implicit.MRR))

	failedCases := make([]FoundationCaseResult, 0, result.Implicit.FailedCases)
	successCases := make([]FoundationCaseResult, 0, result.Implicit.PassedCases)
	for _, c := range result.Implicit.CaseResults {
		if c.Passed {
			successCases = append(successCases, c)
		} else {
			failedCases = append(failedCases, c)
		}
	}

	if len(failedCases) > 0 {
		b.WriteString("### Failed Cases Breakdown\n\n")
		b.WriteString("| Case | Category | Failure Type | Expected | Top Matches | Failure Reason |\n")
		b.WriteString("|---|---|---|---|---|---|\n")
		for _, c := range failedCases {
			failureType := c.FailureType
			if strings.TrimSpace(failureType) == "" {
				failureType = "ranking"
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | `%s` | `%s` | %s | %s |\n",
				c.ID,
				c.Category,
				failureType,
				strings.Join(c.ExpectedTools, ", "),
				escapeTable(formatTopMatches(c.TopMatches)),
				escapeTable(c.Reason),
			))
		}
		b.WriteString("\n")
	}

	if len(successCases) > 0 {
		b.WriteString("### Success Cases (Sample)\n\n")
		b.WriteString("| Case | Expected | Hit Rank | Top Match | Why It Worked |\n")
		b.WriteString("|---|---|---:|---|---|\n")
		limit := 12
		if len(successCases) < limit {
			limit = len(successCases)
		}
		sort.Slice(successCases, func(i, j int) bool {
			if successCases[i].HitRank == successCases[j].HitRank {
				return successCases[i].ID < successCases[j].ID
			}
			return successCases[i].HitRank < successCases[j].HitRank
		})
		for i := 0; i < limit; i++ {
			c := successCases[i]
			top := "-"
			if len(c.TopMatches) > 0 {
				top = fmt.Sprintf("%s(%.2f)", c.TopMatches[0].Name, c.TopMatches[0].Score)
			}
			b.WriteString(fmt.Sprintf("| `%s` | `%s` | %d | %s | %s |\n",
				c.ID,
				strings.Join(c.ExpectedTools, ", "),
				c.HitRank,
				escapeTable(top),
				escapeTable(c.Reason),
			))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Recommendations\n\n")
	for _, rec := range result.Recommendations {
		b.WriteString(fmt.Sprintf("- %s\n", rec))
	}
	b.WriteString("\n")

	return b.String()
}

func formatTopMatches(matches []FoundationToolMatch) string {
	if len(matches) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(matches))
	for _, m := range matches {
		parts = append(parts, fmt.Sprintf("%s(%.2f)", m.Name, m.Score))
	}
	return strings.Join(parts, ", ")
}

func escapeTable(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", " ")
	return value
}
