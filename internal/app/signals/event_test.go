package signals

import "testing"

func TestSignalSourceConstants(t *testing.T) {
	tests := []struct {
		source SignalSource
		want   string
	}{
		{SourceLark, "lark"},
		{SourceGit, "git"},
		{SourceCalendar, "calendar"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.source) != tt.want {
				t.Errorf("got %q, want %q", tt.source, tt.want)
			}
		})
	}
}

func TestAttentionRouteConstants(t *testing.T) {
	tests := []struct {
		route AttentionRoute
		want  string
	}{
		{RouteSuppress, "suppress"},
		{RouteSummarize, "summarize"},
		{RouteQueue, "queue"},
		{RouteNotifyNow, "notify_now"},
		{RouteEscalate, "escalate"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.route) != tt.want {
				t.Errorf("got %q, want %q", tt.route, tt.want)
			}
		})
	}
}
