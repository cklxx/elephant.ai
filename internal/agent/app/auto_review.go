package app

import (
	"fmt"
	"math"
	"strings"

	"alex/internal/agent/ports"
)

// AutoReviewOptions control the automatic reviewer behaviour that runs in the
// core agent after each task.
type AutoReviewOptions struct {
	Enabled           bool
	MinPassingScore   float64
	EnableAutoRework  bool
	MaxReworkAttempts int
}

func defaultAutoReviewOptions() *AutoReviewOptions {
	return &AutoReviewOptions{
		Enabled:           true,
		MinPassingScore:   0.45,
		EnableAutoRework:  true,
		MaxReworkAttempts: 1,
	}
}

func cloneAutoReviewOptions(options *AutoReviewOptions) *AutoReviewOptions {
	if options == nil {
		return nil
	}
	cloned := *options
	return &cloned
}

// ResultAutoReviewer grades agent answers using lightweight heuristics so that
// obviously incomplete work can be routed through the automatic rework loop.
type ResultAutoReviewer struct {
	options *AutoReviewOptions
}

func NewResultAutoReviewer(options *AutoReviewOptions) *ResultAutoReviewer {
	if options == nil {
		options = defaultAutoReviewOptions()
	}
	return &ResultAutoReviewer{options: cloneAutoReviewOptions(options)}
}

func (rar *ResultAutoReviewer) UpdateOptions(options *AutoReviewOptions) {
	rar.options = cloneAutoReviewOptions(options)
}

func (rar *ResultAutoReviewer) Review(result *ports.TaskResult) *ports.ResultAssessment {
	assessment := &ports.ResultAssessment{
		Grade: "F",
	}
	if rar == nil || rar.options == nil {
		assessment.Notes = []string{"auto reviewer disabled"}
		assessment.NeedsRework = false
		return assessment
	}
	if result == nil {
		assessment.Notes = []string{"missing task result"}
		assessment.NeedsRework = true
		return assessment
	}
	trimmed := strings.TrimSpace(result.Answer)
	if trimmed == "" {
		assessment.Score = 0
		assessment.Notes = []string{"final answer is empty"}
		assessment.NeedsRework = true
		return assessment
	}

	score := 0.55
	notes := make([]string, 0, 4)
	wordCount := len(strings.Fields(trimmed))
	switch {
	case wordCount < 40:
		score -= 0.25
		notes = append(notes, "answer is extremely short and likely incomplete")
	case wordCount < 120:
		score += 0.05
	default:
		score += 0.08
	}

	lowered := strings.ToLower(trimmed)
	if strings.Contains(lowered, "i cannot") || strings.Contains(lowered, "i can't") || strings.Contains(lowered, "i'm unable") {
		score -= 0.25
		notes = append(notes, "answer explicitly says it cannot solve the task")
	}
	if strings.Contains(lowered, "todo") || strings.Contains(lowered, "pending") {
		score -= 0.1
		notes = append(notes, "answer references TODO/pending work")
	}
	if strings.Contains(lowered, "apolog") {
		score -= 0.05
	}
	if strings.Contains(lowered, "cannot reproduce") {
		score -= 0.1
	}
	if strings.Contains(trimmed, "```") {
		score += 0.05
	}
	if result.StopReason == "max_iterations" {
		score -= 0.1
		notes = append(notes, "stopped because max iterations were reached")
	}
	if result.Iterations <= 1 && wordCount < 80 {
		score -= 0.05
		notes = append(notes, "only a single short iteration was executed")
	}

	score = math.Max(0, math.Min(1, score))
	assessment.Score = score
	assessment.Grade = rar.scoreToGrade(score)
	assessment.NeedsRework = score < rar.options.MinPassingScore
	assessment.Notes = notes
	return assessment
}

func (rar *ResultAutoReviewer) scoreToGrade(score float64) string {
	switch {
	case score >= 0.85:
		return "A"
	case score >= 0.7:
		return "B"
	case score >= 0.55:
		return "C"
	case score >= 0.4:
		return "D"
	default:
		return "F"
	}
}

func buildReworkPrompt(originalTask, priorAnswer string, assessment *ports.ResultAssessment, attempt int) string {
	builder := &strings.Builder{}
	builder.WriteString(strings.TrimSpace(originalTask))
	builder.WriteString("\n\n---\n")
	builder.WriteString("An automated reviewer graded the previous answer as ")
	if assessment != nil {
		builder.WriteString(fmt.Sprintf("%s (%.2f).", assessment.Grade, assessment.Score))
		if len(assessment.Notes) > 0 {
			builder.WriteString(" Key issues: ")
			builder.WriteString(strings.Join(assessment.Notes, "; "))
			if !strings.HasSuffix(builder.String(), ".") {
				builder.WriteString(".")
			}
		}
	} else {
		builder.WriteString("insufficient.")
	}
	builder.WriteString(" Please produce a corrected, comprehensive response that fully resolves the user's request.")
	if attempt > 0 {
		builder.WriteString(fmt.Sprintf(" This is automated rework attempt #%d, be significantly more concrete.", attempt+1))
	}
	sanitized := strings.TrimSpace(priorAnswer)
	if sanitized != "" {
		builder.WriteString("\n\nPrevious answer for reference (truncated):\n")
		builder.WriteString(truncateAnswer(sanitized, 1500))
	}
	return builder.String()
}

func truncateAnswer(answer string, limit int) string {
	if len(answer) <= limit {
		return answer
	}
	trimmed := strings.TrimSpace(answer[:limit])
	if !strings.HasSuffix(trimmed, "...") {
		trimmed += "..."
	}
	return trimmed
}
