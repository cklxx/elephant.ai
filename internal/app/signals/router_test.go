package signals

import (
	"context"
	"testing"
	"time"
)

type stubFocusChecker struct{ suppress bool }

func (s *stubFocusChecker) ShouldSuppress(_ string, _ time.Time) bool { return s.suppress }

func TestRouterRoute(t *testing.T) {
	now := time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC) // 14:00, not quiet
	cfg := DefaultConfig()
	cfg.QuietHoursStart = 22
	cfg.QuietHoursEnd = 8
	cfg.BudgetMax = 0 // no budget limit

	tests := []struct {
		name  string
		score int
		want  AttentionRoute
	}{
		{"suppress", 10, RouteSuppress},
		{"summarize", 45, RouteSummarize},
		{"queue", 65, RouteQueue},
		{"notify_now", 85, RouteNotifyNow},
		{"escalate", 95, RouteEscalate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewRouter(cfg, func() time.Time { return now })
			event := &SignalEvent{Score: tt.score, ChatID: "c1", UserID: "u1"}
			got := router.Route(context.Background(), event)
			if got != tt.want {
				t.Errorf("Route(score=%d) = %q, want %q", tt.score, got, tt.want)
			}
		})
	}
}

func TestRouterQuietHours(t *testing.T) {
	cfg := DefaultConfig()
	cfg.QuietHoursStart = 22
	cfg.QuietHoursEnd = 8
	cfg.BudgetMax = 0
	quietTime := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	router := NewRouter(cfg, func() time.Time { return quietTime })

	event := &SignalEvent{Score: 50, ChatID: "c1", UserID: "u1"}
	got := router.Route(context.Background(), event)
	if got != RouteQueue {
		t.Errorf("quiet hours: got %q, want %q", got, RouteQueue)
	}
	if len(router.DrainQueue()) != 1 {
		t.Error("expected 1 queued signal")
	}
}

func TestRouterFocusTimeSuppression(t *testing.T) {
	now := time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)
	cfg := DefaultConfig()
	cfg.BudgetMax = 0
	router := NewRouter(cfg, func() time.Time { return now },
		WithFocusChecker(&stubFocusChecker{suppress: true}))

	event := &SignalEvent{Score: 50, ChatID: "c1", UserID: "u1"}
	got := router.Route(context.Background(), event)
	if got != RouteSuppress {
		t.Errorf("focus time: got %q, want %q", got, RouteSuppress)
	}
}

func TestRouterBudgetEnforcement(t *testing.T) {
	now := time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC)
	cfg := DefaultConfig()
	cfg.BudgetMax = 2
	cfg.BudgetWindow = 10 * time.Minute
	router := NewRouter(cfg, func() time.Time { return now })

	for range 2 {
		event := &SignalEvent{Score: 50, ChatID: "c1", UserID: "u1"}
		router.Route(context.Background(), event)
	}

	event := &SignalEvent{Score: 50, ChatID: "c1", UserID: "u1"}
	got := router.Route(context.Background(), event)
	if got != RouteSuppress {
		t.Errorf("over budget: got %q, want %q", got, RouteSuppress)
	}
}

func TestRouterHighPriorityBypassesQuietHours(t *testing.T) {
	quietTime := time.Date(2026, 1, 1, 23, 0, 0, 0, time.UTC)
	cfg := DefaultConfig()
	cfg.QuietHoursStart = 22
	cfg.QuietHoursEnd = 8
	cfg.BudgetMax = 0
	router := NewRouter(cfg, func() time.Time { return quietTime })

	event := &SignalEvent{Score: 95, ChatID: "c1", UserID: "u1"}
	got := router.Route(context.Background(), event)
	if got != RouteEscalate {
		t.Errorf("escalate during quiet: got %q, want %q", got, RouteEscalate)
	}
}
