package morningbrief

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/app/digest"
	"alex/internal/app/signals"
)

// SignalProvider retrieves recent scored signals.
type SignalProvider interface {
	RecentSignals() []signals.SignalEvent
}

// MorningBriefSpec implements digest.DigestSpec for the morning brief.
type MorningBriefSpec struct {
	signals SignalProvider
}

// NewMorningBriefSpec creates a MorningBriefSpec.
func NewMorningBriefSpec(sp SignalProvider) *MorningBriefSpec {
	return &MorningBriefSpec{signals: sp}
}

func (m *MorningBriefSpec) Name() string { return "Morning Brief" }

// Generate gathers recent signals and categorizes them by route.
func (m *MorningBriefSpec) Generate(_ context.Context) (*digest.Content, error) {
	events := m.signals.RecentSignals()
	needsCall, autoHandled, onTrack := categorize(events)

	content := &digest.Content{
		Title:    "Morning Brief",
		Metadata: map[string]string{"signal_count": fmt.Sprintf("%d", len(events))},
	}
	content.Sections = append(content.Sections, buildSection("Needs Your Call", needsCall))
	content.Sections = append(content.Sections, buildSection("Auto-Handled", autoHandled))
	content.Sections = append(content.Sections, buildSection("On Track", onTrack))
	return content, nil
}

// Format renders the Content as Markdown.
func (m *MorningBriefSpec) Format(content *digest.Content) string {
	if isAllEmpty(content.Sections) {
		return "All clear — nothing needs your attention"
	}
	var sb strings.Builder
	sb.WriteString("# " + content.Title + "\n\n")
	for _, sec := range content.Sections {
		sb.WriteString(formatSection(sec))
	}
	return sb.String()
}

func categorize(events []signals.SignalEvent) (needsCall, autoHandled, onTrack []signals.SignalEvent) {
	for _, e := range events {
		switch e.Route {
		case signals.RouteEscalate, signals.RouteNotifyNow, signals.RouteQueue:
			needsCall = append(needsCall, e)
		case signals.RouteSummarize:
			autoHandled = append(autoHandled, e)
		default:
			onTrack = append(onTrack, e)
		}
	}
	return
}

func buildSection(heading string, events []signals.SignalEvent) digest.Section {
	items := make([]digest.Item, len(events))
	for i, e := range events {
		items[i] = digest.Item{
			Label:  e.ID,
			Value:  e.Content,
			Status: routeToStatus(e.Route),
		}
	}
	return digest.Section{Heading: heading, Items: items}
}

func routeToStatus(r signals.AttentionRoute) string {
	switch r {
	case signals.RouteEscalate, signals.RouteNotifyNow:
		return "action_needed"
	case signals.RouteQueue:
		return "warning"
	default:
		return "ok"
	}
}

func formatSection(sec digest.Section) string {
	var sb strings.Builder
	sb.WriteString("## " + sec.Heading + "\n\n")
	if len(sec.Items) == 0 {
		sb.WriteString("_none_\n\n")
		return sb.String()
	}
	for _, item := range sec.Items {
		sb.WriteString(fmt.Sprintf("- **%s**: %s [%s]\n", item.Label, item.Value, item.Status))
	}
	sb.WriteString("\n")
	return sb.String()
}

func isAllEmpty(sections []digest.Section) bool {
	for _, s := range sections {
		if len(s.Items) > 0 {
			return false
		}
	}
	return true
}
