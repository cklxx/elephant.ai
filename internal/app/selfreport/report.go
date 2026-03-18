package selfreport

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/app/digest"
)

// Stats holds metrics for the self-report period.
type Stats struct {
	TasksCompleted  int
	TasksInProgress int
	AutoActed       int
	Escalated       int
	Corrected       int
	LLMCostUSD      float64
	SignalVolume     int
	Lessons         []string
}

// StatsProvider retrieves self-report metrics.
type StatsProvider interface {
	GetStats(ctx context.Context) (*Stats, error)
}

// SelfReportSpec implements digest.DigestSpec for the weekly self-report.
type SelfReportSpec struct {
	provider StatsProvider
}

// NewSelfReportSpec creates a SelfReportSpec.
func NewSelfReportSpec(provider StatsProvider) *SelfReportSpec {
	return &SelfReportSpec{provider: provider}
}

func (s *SelfReportSpec) Name() string { return "Self-Report" }

// Generate gathers stats and builds the report content.
func (s *SelfReportSpec) Generate(ctx context.Context) (*digest.Content, error) {
	stats, err := s.provider.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("get stats: %w", err)
	}
	return buildContent(stats), nil
}

// Format renders the Content as Markdown.
func (s *SelfReportSpec) Format(content *digest.Content) string {
	if isQuietWeek(content) {
		return "Quiet week — no tasks processed"
	}
	var sb strings.Builder
	sb.WriteString("# " + content.Title + "\n\n")
	for _, sec := range content.Sections {
		sb.WriteString(formatSection(sec))
	}
	return sb.String()
}

func buildContent(stats *Stats) *digest.Content {
	c := &digest.Content{
		Title:    "Self-Report",
		Metadata: map[string]string{},
	}
	c.Sections = append(c.Sections, activitySection(stats))
	c.Sections = append(c.Sections, decisionSection(stats))
	c.Sections = append(c.Sections, costSection(stats))
	c.Sections = append(c.Sections, lessonsSection(stats))
	return c
}

func activitySection(stats *Stats) digest.Section {
	return digest.Section{
		Heading: "Activity",
		Items: []digest.Item{
			{Label: "Tasks Completed", Value: fmt.Sprintf("%d", stats.TasksCompleted)},
			{Label: "In Progress", Value: fmt.Sprintf("%d", stats.TasksInProgress)},
			{Label: "Signal Volume", Value: fmt.Sprintf("%d", stats.SignalVolume)},
		},
	}
}

func decisionSection(stats *Stats) digest.Section {
	return digest.Section{
		Heading: "Decision Engine",
		Items: []digest.Item{
			{Label: "Auto-Acted", Value: fmt.Sprintf("%d", stats.AutoActed)},
			{Label: "Escalated", Value: fmt.Sprintf("%d", stats.Escalated)},
			{Label: "Corrected", Value: fmt.Sprintf("%d", stats.Corrected)},
		},
	}
}

func costSection(stats *Stats) digest.Section {
	return digest.Section{
		Heading: "Cost",
		Items: []digest.Item{
			{Label: "LLM Cost", Value: fmt.Sprintf("$%.2f", stats.LLMCostUSD)},
		},
	}
}

func lessonsSection(stats *Stats) digest.Section {
	items := make([]digest.Item, len(stats.Lessons))
	for i, l := range stats.Lessons {
		items[i] = digest.Item{Label: fmt.Sprintf("Lesson %d", i+1), Value: l}
	}
	return digest.Section{Heading: "What I Learned", Items: items}
}

func isQuietWeek(content *digest.Content) bool {
	for _, sec := range content.Sections {
		for _, item := range sec.Items {
			if item.Value != "0" && item.Value != "$0.00" && item.Value != "" {
				return false
			}
		}
	}
	return true
}

func formatSection(sec digest.Section) string {
	var sb strings.Builder
	sb.WriteString("## " + sec.Heading + "\n\n")
	if len(sec.Items) == 0 {
		sb.WriteString("_none_\n\n")
		return sb.String()
	}
	for _, item := range sec.Items {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", item.Label, item.Value))
	}
	sb.WriteString("\n")
	return sb.String()
}
