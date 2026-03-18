package morningbrief

import (
	"context"
	"strings"
	"testing"

	"alex/internal/app/signals"
)

type staticProvider struct {
	events []signals.SignalEvent
}

func (s *staticProvider) RecentSignals() []signals.SignalEvent { return s.events }

func TestMorningBriefSpec(t *testing.T) {
	tests := []struct {
		name       string
		events     []signals.SignalEvent
		wantTitle  string
		wantEmpty  bool
		wantContains []string
	}{
		{
			name:      "empty signals",
			events:    nil,
			wantTitle: "Morning Brief",
			wantEmpty: true,
		},
		{
			name: "mixed signals",
			events: []signals.SignalEvent{
				{ID: "e1", Content: "server down", Route: signals.RouteEscalate},
				{ID: "e2", Content: "PR merged", Route: signals.RouteSummarize},
				{ID: "e3", Content: "hello", Route: signals.RouteSuppress},
			},
			wantTitle:    "Morning Brief",
			wantContains: []string{"e1", "e2", "e3", "Needs Your Call", "Auto-Handled", "On Track"},
		},
		{
			name: "all escalated",
			events: []signals.SignalEvent{
				{ID: "e1", Content: "p0", Route: signals.RouteEscalate},
			},
			wantTitle:    "Morning Brief",
			wantContains: []string{"e1", "action_needed"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := NewMorningBriefSpec(&staticProvider{events: tt.events})
			if spec.Name() != "Morning Brief" {
				t.Errorf("Name() = %q", spec.Name())
			}
			content, err := spec.Generate(context.Background())
			if err != nil {
				t.Fatalf("Generate() error: %v", err)
			}
			if content.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", content.Title, tt.wantTitle)
			}

			formatted := spec.Format(content)
			if tt.wantEmpty {
				if formatted != "All clear — nothing needs your attention" {
					t.Errorf("empty state = %q", formatted)
				}
				return
			}
			for _, s := range tt.wantContains {
				if !strings.Contains(formatted, s) {
					t.Errorf("formatted missing %q", s)
				}
			}
		})
	}
}
