package skills

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alex/internal/memory"
)

// WorkflowTrace captures a task's tool execution sequence.
type WorkflowTrace struct {
	TaskID    string     `json:"task_id"`
	UserID    string     `json:"user_id"`
	Tools     []ToolStep `json:"tools"`
	Outcome   string     `json:"outcome"`
	CreatedAt time.Time  `json:"created_at"`
}

// ToolStep captures a single tool step in a workflow trace.
type ToolStep struct {
	Name    string `json:"name"`
	Success bool   `json:"success"`
}

// ToolNames returns the tool sequence as ordered names.
func (w WorkflowTrace) ToolNames() []string {
	if len(w.Tools) == 0 {
		return nil
	}
	names := make([]string, 0, len(w.Tools))
	for _, step := range w.Tools {
		name := strings.TrimSpace(step.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

// SkillLearner extracts repeatable workflows and suggests new skills.
type SkillLearner struct {
	memoryService memory.Service
	library       *Library
	minOccurrence int
	minSteps      int
}

// NewSkillLearner constructs a SkillLearner.
func NewSkillLearner(memoryService memory.Service, library *Library, minOccurrence, minSteps int) *SkillLearner {
	if minOccurrence <= 0 {
		minOccurrence = 3
	}
	if minSteps <= 0 {
		minSteps = 2
	}
	return &SkillLearner{
		memoryService: memoryService,
		library:       library,
		minOccurrence: minOccurrence,
		minSteps:      minSteps,
	}
}

// SkillSuggestion represents a suggested skill derived from workflow patterns.
type SkillSuggestion struct {
	Name         string
	Description  string
	ToolSequence []string
	Occurrences  int
	SuccessRate  float64
	Confidence   float64
}

// AnalyzePatterns scans stored workflow traces and returns skill suggestions.
func (l *SkillLearner) AnalyzePatterns(ctx context.Context, userID string) []SkillSuggestion {
	traces := l.loadTraces(ctx, userID)
	if len(traces) == 0 {
		return nil
	}

	type stats struct {
		Tools   []string
		Count   int
		Success int
	}

	sequenceStats := make(map[string]*stats)
	for _, trace := range traces {
		tools := trace.ToolNames()
		if len(tools) < l.minSteps {
			continue
		}
		key := strings.Join(tools, "→")
		st := sequenceStats[key]
		if st == nil {
			st = &stats{Tools: tools}
			sequenceStats[key] = st
		}
		st.Count++
		if strings.EqualFold(trace.Outcome, "complete") || strings.EqualFold(trace.Outcome, "success") {
			st.Success++
		}
	}

	var suggestions []SkillSuggestion
	for _, st := range sequenceStats {
		if st.Count < l.minOccurrence {
			continue
		}
		if l.isAlreadyCovered(st.Tools) {
			continue
		}
		successRate := float64(st.Success) / float64(st.Count)
		confidence := successRate
		if confidence < 0.3 {
			confidence = 0.3
		}
		suggestions = append(suggestions, SkillSuggestion{
			Name:         generateSkillName(st.Tools),
			Description:  generateDescription(st.Tools),
			ToolSequence: st.Tools,
			Occurrences:  st.Count,
			SuccessRate:  successRate,
			Confidence:   confidence,
		})
	}
	return suggestions
}

// GenerateSkillFile renders a SKILL.md template for a suggestion.
func (l *SkillLearner) GenerateSkillFile(suggestion SkillSuggestion) string {
	var sb strings.Builder
	sb.WriteString("---\n")
	sb.WriteString(fmt.Sprintf("name: %s\n", suggestion.Name))
	sb.WriteString(fmt.Sprintf("description: \"%s\"\n", suggestion.Description))
	sb.WriteString("triggers:\n")
	sb.WriteString("  intent_patterns: []\n")
	sb.WriteString("  tool_signals:\n")
	for _, tool := range suggestion.ToolSequence {
		sb.WriteString(fmt.Sprintf("    - %s\n", tool))
	}
	sb.WriteString("priority: 5\n")
	sb.WriteString("---\n\n")
	sb.WriteString(fmt.Sprintf("# %s\n\n", suggestion.Name))
	sb.WriteString(fmt.Sprintf("*Auto-generated from %d occurrences (success rate: %.0f%%)*\n\n",
		suggestion.Occurrences, suggestion.SuccessRate*100))
	sb.WriteString("## Workflow Steps\n\n")
	for i, tool := range suggestion.ToolSequence {
		sb.WriteString(fmt.Sprintf("%d. Execute `%s`\n", i+1, tool))
	}
	return sb.String()
}

func (l *SkillLearner) loadTraces(ctx context.Context, userID string) []WorkflowTrace {
	if l == nil || l.memoryService == nil || strings.TrimSpace(userID) == "" {
		return nil
	}
	entries, err := l.memoryService.Recall(ctx, memory.Query{
		UserID:   userID,
		Keywords: []string{"workflow_trace"},
		Slots:    map[string]string{"type": "workflow_trace"},
		Limit:    200,
	})
	if err != nil || len(entries) == 0 {
		return nil
	}

	traces := make([]WorkflowTrace, 0, len(entries))
	for _, entry := range entries {
		var trace WorkflowTrace
		if err := json.Unmarshal([]byte(entry.Content), &trace); err != nil {
			continue
		}
		traces = append(traces, trace)
	}
	return traces
}

func (l *SkillLearner) isAlreadyCovered(sequence []string) bool {
	if l == nil || l.library == nil || len(sequence) == 0 {
		return false
	}
	for _, skill := range l.library.List() {
		if skill.Triggers == nil || len(skill.Triggers.ToolSignals) == 0 {
			continue
		}
		if sequenceCoveredBySignals(sequence, skill.Triggers.ToolSignals) {
			return true
		}
	}
	return false
}

func sequenceCoveredBySignals(sequence, signals []string) bool {
	if len(sequence) == 0 || len(signals) == 0 {
		return false
	}
	set := make(map[string]bool, len(signals))
	for _, signal := range signals {
		set[strings.ToLower(signal)] = true
	}
	for _, tool := range sequence {
		if !set[strings.ToLower(tool)] {
			return false
		}
	}
	return true
}

func generateSkillName(sequence []string) string {
	if len(sequence) == 0 {
		return "auto-skill"
	}
	return NormalizeName(strings.Join(sequence, "-"))
}

func generateDescription(sequence []string) string {
	if len(sequence) == 0 {
		return "Auto-generated workflow"
	}
	return fmt.Sprintf("Auto-generated workflow for %s", strings.Join(sequence, " → "))
}
