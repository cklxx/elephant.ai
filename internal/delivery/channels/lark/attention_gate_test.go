package lark

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAttentionGate_Assess_ScoreAndRoute(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "hotfix"},
	})

	tests := []struct {
		name      string
		content   string
		wantScore int
		wantRoute AttentionRoute
	}{
		{name: "empty", content: "", wantScore: 0, wantRoute: AttentionRouteSuppress},
		{name: "summarize", content: "can you review this PR", wantScore: 40, wantRoute: AttentionRouteSummarize},
		{name: "queue", content: "can you review this PR today", wantScore: 60, wantRoute: AttentionRouteQueue},
		{name: "notify now", content: "deploy the HOTFIX branch", wantScore: 80, wantRoute: AttentionRouteNotifyNow},
		{name: "escalate", content: "P0 production outage asap!!!", wantScore: 100, wantRoute: AttentionRouteEscalate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gate.Assess(tt.content)
			if got.Score != tt.wantScore {
				t.Fatalf("Assess(%q).Score = %d, want %d", tt.content, got.Score, tt.wantScore)
			}
			if got.Route != tt.wantRoute {
				t.Fatalf("Assess(%q).Route = %q, want %q", tt.content, got.Route, tt.wantRoute)
			}
		})
	}
}

func TestAttentionGate_RouteForScore_DefaultThresholds(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	tests := []struct {
		score int
		want  AttentionRoute
	}{
		{0, AttentionRouteSuppress},
		{39, AttentionRouteSuppress},
		{40, AttentionRouteSummarize},
		{59, AttentionRouteSummarize},
		{60, AttentionRouteQueue},
		{79, AttentionRouteQueue},
		{80, AttentionRouteNotifyNow},
		{89, AttentionRouteNotifyNow},
		{90, AttentionRouteEscalate},
		{100, AttentionRouteEscalate},
	}

	for _, tt := range tests {
		if got := gate.RouteForScore(tt.score); got != tt.want {
			t.Errorf("RouteForScore(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestAttentionGate_RouteForScore_CustomThresholds(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:            true,
		SummarizeThreshold: 25,
		QueueThreshold:     50,
		NotifyNowThreshold: 70,
		EscalateThreshold:  95,
	})

	tests := []struct {
		score int
		want  AttentionRoute
	}{
		{24, AttentionRouteSuppress},
		{25, AttentionRouteSummarize},
		{50, AttentionRouteQueue},
		{70, AttentionRouteNotifyNow},
		{95, AttentionRouteEscalate},
	}

	for _, tt := range tests {
		if got := gate.RouteForScore(tt.score); got != tt.want {
			t.Errorf("RouteForScore(%d) = %q, want %q", tt.score, got, tt.want)
		}
	}
}

func TestAttentionGate_ClassifyUrgency_Disabled(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: false})
	if got := gate.ClassifyUrgency("hello"); got != UrgencyNormal {
		t.Errorf("disabled gate should return UrgencyNormal, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_BackwardCompatibilityUses80Plus(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:            true,
		SummarizeThreshold: 20,
		QueueThreshold:     40,
		NotifyNowThreshold: 60,
		EscalateThreshold:  90,
	})

	if got := gate.ClassifyUrgency("can you review this PR today"); got != UrgencyLow {
		t.Fatalf("score 60 should still map to UrgencyLow for legacy callers, got %d", got)
	}
	if got := gate.ClassifyUrgency("server is down"); got != UrgencyHigh {
		t.Fatalf("score 80 should map to UrgencyHigh for legacy callers, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_EmptyContent(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	if got := gate.ClassifyUrgency(""); got != UrgencyLow {
		t.Errorf("empty content should be UrgencyLow, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_ConfiguredKeyword(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "hotfix"},
	})
	if got := gate.ClassifyUrgency("We need a P0 fix now"); got != UrgencyHigh {
		t.Errorf("configured keyword P0 should be UrgencyHigh, got %d", got)
	}
	if got := gate.ClassifyUrgency("deploy the HOTFIX branch"); got != UrgencyHigh {
		t.Errorf("case-insensitive keyword hotfix should be UrgencyHigh, got %d", got)
	}
}

func TestAttentionGate_ClassifyUrgency_BuiltinPatterns(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	tests := []struct {
		content string
		want    UrgencyLevel
	}{
		{"服务紧急需要修复", UrgencyHigh},
		{"this is urgent please help", UrgencyHigh},
		{"the build failed again", UrgencyHigh},
		{"server is down", UrgencyHigh},
		{"I'm blocked on this", UrgencyHigh},
		{"系统出错了", UrgencyHigh},
		{"请帮忙看一下这个文件", UrgencyLow},
		{"can you review this PR", UrgencyLow},
	}
	for _, tt := range tests {
		if got := gate.ClassifyUrgency(tt.content); got != tt.want {
			t.Errorf("ClassifyUrgency(%q) = %d, want %d", tt.content, got, tt.want)
		}
	}
}

func TestAttentionGate_ClassifyUrgency_Exclamations(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	// Short message with 3+ exclamation marks → UrgencyHigh
	if got := gate.ClassifyUrgency("help!!!"); got != UrgencyHigh {
		t.Errorf("triple exclamation should be UrgencyHigh, got %d", got)
	}
	// Long message with exclamations → UrgencyLow (length >= 50 runes, no urgency keywords)
	long := "this is a very long message that should not be considered particularly noteworthy at all!!!"
	if got := gate.ClassifyUrgency(long); got != UrgencyLow {
		t.Errorf("long message with exclamations should be UrgencyLow, got %d", got)
	}
}

func TestAttentionGate_RecordDispatch_WithinBudget(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 10 * time.Minute,
		BudgetMax:    3,
	})
	now := time.Now()
	for i := 0; i < 3; i++ {
		if !gate.RecordDispatch("chat-1", now) {
			t.Errorf("dispatch %d should be within budget", i)
		}
	}
	// 4th should be over budget
	if gate.RecordDispatch("chat-1", now) {
		t.Error("4th dispatch should be over budget")
	}
}

func TestAttentionGate_RecordDispatch_WindowExpiry(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    2,
	})
	t0 := time.Now()
	gate.RecordDispatch("chat-1", t0)
	gate.RecordDispatch("chat-1", t0)
	// Over budget at t0
	if gate.RecordDispatch("chat-1", t0) {
		t.Error("should be over budget at t0")
	}
	// After window expires, budget resets
	t1 := t0.Add(6 * time.Minute)
	if !gate.RecordDispatch("chat-1", t1) {
		t.Error("should be within budget after window expiry")
	}
}

func TestAttentionGate_RecordDispatch_NoBudgetLimit(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:   true,
		BudgetMax: 0, // disabled
	})
	now := time.Now()
	for i := 0; i < 100; i++ {
		if !gate.RecordDispatch("chat-1", now) {
			t.Errorf("dispatch %d should always pass with no budget limit", i)
		}
	}
}

func TestAttentionGate_AutoAckMessage_Default(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	if got := gate.AutoAckMessage(); got != defaultAutoAckMessage {
		t.Errorf("default auto-ack = %q, want %q", got, defaultAutoAckMessage)
	}
}

func TestAttentionGate_AutoAckMessage_Custom(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		AutoAckMessage: "Got it!",
	})
	if got := gate.AutoAckMessage(); got != "Got it!" {
		t.Errorf("custom auto-ack = %q, want %q", got, "Got it!")
	}
}

// ---------- Bug fix: blank keyword filtering ----------

func TestAttentionGate_BlankKeywordsFiltered(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "", "  ", "\t", "hotfix", " "},
	})
	// Only "p0" and "hotfix" should survive.
	if len(gate.lowerKeywords) != 2 {
		t.Fatalf("lowerKeywords = %v, want 2 entries", gate.lowerKeywords)
	}
	if gate.lowerKeywords[0] != "p0" || gate.lowerKeywords[1] != "hotfix" {
		t.Errorf("lowerKeywords = %v, want [p0 hotfix]", gate.lowerKeywords)
	}
}

func TestAttentionGate_BlankKeywordDoesNotMatchEverything(t *testing.T) {
	// Before fix: an empty keyword "" would match every message via
	// strings.Contains(lower, ""), making everything UrgencyHigh.
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"", "  "},
	})
	// A routine message must NOT be classified as UrgencyHigh.
	if got := gate.ClassifyUrgency("just a normal message"); got != UrgencyLow {
		t.Errorf("blank keywords should not match, got urgency %d", got)
	}
}

func TestAttentionGate_KeywordTrimmedWhitespace(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"  hotfix  "},
	})
	if got := gate.ClassifyUrgency("please apply the hotfix"); got != UrgencyHigh {
		t.Errorf("trimmed keyword should match, got urgency %d", got)
	}
}

// ---------- Bug fix: stale budget GC ----------

func TestAttentionGate_BudgetGC_RemovesStaleChatIDs(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	t0 := time.Now()

	// Populate more than budgetGCThreshold chat entries.
	for i := 0; i < budgetGCThreshold+10; i++ {
		chatID := "chat-" + time.Duration(i).String()
		gate.RecordDispatch(chatID, t0)
	}

	if len(gate.budgets) != budgetGCThreshold+10 {
		t.Fatalf("budgets = %d, want %d", len(gate.budgets), budgetGCThreshold+10)
	}

	// Advance time past the budget window so all entries are stale.
	t1 := t0.Add(10 * time.Minute)

	// A new dispatch triggers GC since we're above threshold.
	gate.RecordDispatch("fresh-chat", t1)

	// All old entries should be GC'd; only "fresh-chat" remains.
	if len(gate.budgets) != 1 {
		t.Errorf("after GC, budgets = %d, want 1 (only fresh-chat)", len(gate.budgets))
	}
	if _, ok := gate.budgets["fresh-chat"]; !ok {
		t.Error("fresh-chat should survive GC")
	}
}

func TestAttentionGate_BudgetGC_KeepsActiveChatIDs(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	t0 := time.Now()

	// Populate entries above threshold.
	for i := 0; i < budgetGCThreshold+5; i++ {
		chatID := "chat-" + time.Duration(i).String()
		gate.RecordDispatch(chatID, t0)
	}

	// Dispatch within the same window — entries are still active.
	t1 := t0.Add(2 * time.Minute)
	gate.RecordDispatch("trigger", t1)

	// All chats still have recent activity, so none should be GC'd.
	// We expect threshold+5 original + 1 trigger = threshold+6.
	if len(gate.budgets) != budgetGCThreshold+6 {
		t.Errorf("active entries should survive GC, budgets = %d, want %d",
			len(gate.budgets), budgetGCThreshold+6)
	}
}

// ---------- ShouldDispatch with FocusTimeChecker ----------

type mockFocusChecker struct {
	suppressed map[string]bool
}

func (m *mockFocusChecker) ShouldSuppress(userID string, _ time.Time) bool {
	return m.suppressed[userID]
}

func TestShouldDispatch_UrgentBypassesFocusTime(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0"},
	})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"alice": true}})

	urgency, ok := gate.ShouldDispatch("P0 incident", "chat-1", "alice", time.Now())
	if urgency != UrgencyHigh {
		t.Errorf("urgency = %d, want UrgencyHigh", urgency)
	}
	if !ok {
		t.Error("P0 messages should bypass focus time")
	}
}

func TestShouldDispatch_SuppressedDuringFocusTime(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"alice": true}})

	urgency, ok := gate.ShouldDispatch("routine update", "chat-1", "alice", time.Now())
	if urgency != UrgencyLow {
		t.Errorf("urgency = %d, want UrgencyLow", urgency)
	}
	if ok {
		t.Error("routine message should be suppressed during focus time")
	}
}

func TestShouldDispatch_NotSuppressedOutsideFocusTime(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"alice": false}})

	_, ok := gate.ShouldDispatch("routine update", "chat-1", "alice", time.Now())
	if !ok {
		t.Error("message should pass through when user is not in focus time")
	}
}

func TestShouldDispatch_NoFocusCheckerPassesThrough(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	// No focus checker set.

	_, ok := gate.ShouldDispatch("routine update", "chat-1", "alice", time.Now())
	if !ok {
		t.Error("message should pass through when no focus checker is set")
	}
}

func TestShouldDispatch_GateDisabledPassesThrough(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: false})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"alice": true}})

	urgency, ok := gate.ShouldDispatch("hello", "chat-1", "alice", time.Now())
	if urgency != UrgencyNormal {
		t.Errorf("urgency = %d, want UrgencyNormal when gate disabled", urgency)
	}
	if !ok {
		t.Error("disabled gate should pass through all messages")
	}
}

func TestShouldDispatch_BudgetEnforcedAfterFocusCheck(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 10 * time.Minute,
		BudgetMax:    1,
	})
	// No focus time suppression.
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{}})

	now := time.Now()
	_, ok1 := gate.ShouldDispatch("msg1", "chat-1", "bob", now)
	_, ok2 := gate.ShouldDispatch("msg2", "chat-1", "bob", now)

	if !ok1 {
		t.Error("first message should be within budget")
	}
	if ok2 {
		t.Error("second message should be over budget")
	}
}

// ---------- Quiet hours enforcement ----------

func timeAtHour(hour int) time.Time {
	return time.Date(2026, 3, 10, hour, 30, 0, 0, time.UTC)
}

func TestInQuietHours_NormalRange(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	tests := []struct {
		hour int
		want bool
	}{
		{21, false},
		{22, true},
		{23, true},
		{0, true},
		{7, true},
		{8, false},
		{12, false},
	}
	for _, tc := range tests {
		if got := gate.inQuietHours(tc.hour); got != tc.want {
			t.Errorf("inQuietHours(%d) with 22-8 = %v, want %v", tc.hour, got, tc.want)
		}
	}
}

func TestInQuietHours_NoWrap(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 1,
		QuietHoursEnd:   6,
	})
	tests := []struct {
		hour int
		want bool
	}{
		{0, false},
		{1, true},
		{5, true},
		{6, false},
		{12, false},
		{23, false},
	}
	for _, tc := range tests {
		if got := gate.inQuietHours(tc.hour); got != tc.want {
			t.Errorf("inQuietHours(%d) with 1-6 = %v, want %v", tc.hour, got, tc.want)
		}
	}
}

func TestInQuietHours_Disabled(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 0,
		QuietHoursEnd:   0,
	})
	for h := 0; h < 24; h++ {
		if gate.inQuietHours(h) {
			t.Errorf("inQuietHours(%d) should be false when start==end", h)
		}
	}
}

func TestShouldDispatch_QuietHoursBlocksNonUrgent(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})

	// At 23:30 (quiet hours) — routine message should be queued.
	urgency, ok := gate.ShouldDispatch("routine update", "chat-1", "alice", timeAtHour(23))
	if urgency != UrgencyLow {
		t.Errorf("urgency = %d, want UrgencyLow", urgency)
	}
	if ok {
		t.Error("non-urgent message during quiet hours should be suppressed")
	}
	if gate.QueueLen() != 1 {
		t.Errorf("QueueLen = %d, want 1", gate.QueueLen())
	}
}

func TestShouldDispatch_QuietHoursAllowsUrgent(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
		UrgentKeywords:  []string{"P0"},
	})

	// At 23:30 (quiet hours) — urgent message should pass through.
	urgency, ok := gate.ShouldDispatch("P0 production outage", "chat-1", "alice", timeAtHour(23))
	if urgency != UrgencyHigh {
		t.Errorf("urgency = %d, want UrgencyHigh", urgency)
	}
	if !ok {
		t.Error("urgent message during quiet hours should pass through")
	}
	// Nothing should be queued for urgent messages.
	if gate.QueueLen() != 0 {
		t.Errorf("QueueLen = %d, want 0 (urgent messages not queued)", gate.QueueLen())
	}
}

func TestShouldDispatch_OutsideQuietHoursPassesThrough(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})

	// At 12:30 (not quiet hours) — routine message passes normally.
	_, ok := gate.ShouldDispatch("routine update", "chat-1", "alice", timeAtHour(12))
	if !ok {
		t.Error("message outside quiet hours should pass through")
	}
	if gate.QueueLen() != 0 {
		t.Errorf("QueueLen = %d, want 0", gate.QueueLen())
	}
}

func TestShouldDispatch_QuietHoursDisabledPassesThrough(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 0,
		QuietHoursEnd:   0,
	})

	// Quiet hours disabled — message at any hour passes.
	_, ok := gate.ShouldDispatch("routine", "chat-1", "alice", timeAtHour(3))
	if !ok {
		t.Error("message should pass when quiet hours disabled")
	}
}

func TestDrainQueue(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})

	// Queue 3 messages during quiet hours.
	for i := 0; i < 3; i++ {
		gate.ShouldDispatch("msg", "chat-1", "alice", timeAtHour(23))
	}
	if gate.QueueLen() != 3 {
		t.Fatalf("QueueLen = %d, want 3", gate.QueueLen())
	}

	// Drain returns all and clears.
	msgs := gate.DrainQueue()
	if len(msgs) != 3 {
		t.Fatalf("DrainQueue returned %d, want 3", len(msgs))
	}
	if gate.QueueLen() != 0 {
		t.Errorf("QueueLen after drain = %d, want 0", gate.QueueLen())
	}
	// Verify message fields.
	if msgs[0].Content != "msg" || msgs[0].ChatID != "chat-1" || msgs[0].UserID != "alice" {
		t.Errorf("unexpected message: %+v", msgs[0])
	}
	if msgs[0].AttentionScore != 20 {
		t.Errorf("queued score = %d, want 20", msgs[0].AttentionScore)
	}
	if msgs[0].Route != AttentionRouteSuppress {
		t.Errorf("queued route = %q, want %q", msgs[0].Route, AttentionRouteSuppress)
	}
	if msgs[0].Urgency != UrgencyLow {
		t.Errorf("queued urgency = %d, want UrgencyLow", msgs[0].Urgency)
	}
}

func TestDrainQueue_EmptyReturnsNil(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{Enabled: true})
	if msgs := gate.DrainQueue(); msgs != nil {
		t.Errorf("DrainQueue on empty should return nil, got %d", len(msgs))
	}
}

func TestShouldDispatch_QuietHoursThenFocusTime(t *testing.T) {
	// Quiet hours take precedence over focus time check (earlier in pipeline).
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	gate.SetFocusTimeChecker(&mockFocusChecker{suppressed: map[string]bool{"alice": true}})

	// During quiet hours: should be queued (quiet hours path), not just suppressed.
	gate.ShouldDispatch("msg", "chat-1", "alice", timeAtHour(23))
	if gate.QueueLen() != 1 {
		t.Errorf("expected message to be queued during quiet hours, not just focus-suppressed")
	}
}

func TestShouldDispatch_QuietHoursBuiltinUrgencyBypass(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})

	// "server is down" matches built-in urgency → should bypass quiet hours.
	urgency, ok := gate.ShouldDispatch("server is down", "chat-1", "ops", timeAtHour(2))
	if urgency != UrgencyHigh {
		t.Errorf("urgency = %d, want UrgencyHigh", urgency)
	}
	if !ok {
		t.Error("built-in urgent patterns should bypass quiet hours")
	}
}

func TestShouldDispatch_QuietHoursBudgetNotConsumed(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
		BudgetWindow:    10 * time.Minute,
		BudgetMax:       1,
	})

	// During quiet hours, message is queued (not dispatched), so budget
	// should NOT be consumed.
	gate.ShouldDispatch("msg1", "chat-1", "alice", timeAtHour(23))
	if gate.QueueLen() != 1 {
		t.Fatal("expected message queued")
	}

	// After quiet hours end, a fresh message should still be within budget.
	_, ok := gate.ShouldDispatch("msg2", "chat-1", "alice", timeAtHour(9))
	if !ok {
		t.Error("budget should not have been consumed by queued message")
	}
}

func TestShouldDispatch_QuietHoursMidnightEdge(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})

	// Exactly at midnight (hour 0) → quiet hours.
	midnight := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC)
	_, ok := gate.ShouldDispatch("test", "chat-1", "u", midnight)
	if ok {
		t.Error("midnight should be in quiet hours (22-8)")
	}

	// Exactly at 8:00 → NOT quiet hours (end is exclusive).
	eightAM := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	_, ok = gate.ShouldDispatch("test", "chat-1", "u", eightAM)
	if !ok {
		t.Error("8:00 should be outside quiet hours (end exclusive)")
	}
}

func TestAttentionGate_BudgetGC_EmptyTimestamps(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	// Manually inject an entry with empty timestamps.
	gate.mu.Lock()
	for i := 0; i < budgetGCThreshold+1; i++ {
		gate.budgets["empty-"+time.Duration(i).String()] = &chatBudget{}
	}
	gate.mu.Unlock()

	now := time.Now()
	gate.RecordDispatch("live", now)

	// All empty entries should be GC'd.
	if len(gate.budgets) != 1 {
		t.Errorf("empty-timestamp entries should be GC'd, budgets = %d, want 1", len(gate.budgets))
	}
}

// ---------- DrainQueue timer tests ----------

func TestDrainTimer_AutoDeliversWhenQuietHoursEnd(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	gate.drainInterval = 50 * time.Millisecond

	// Start in quiet hours (hour 23).
	var currentHour atomic.Int32
	currentHour.Store(23)
	gate.nowFn = func() time.Time {
		h := int(currentHour.Load())
		return time.Date(2026, 3, 10, h, 30, 0, 0, time.UTC)
	}

	// Queue messages during quiet hours.
	gate.ShouldDispatch("msg-1", "chat-a", "alice", gate.now())
	gate.ShouldDispatch("msg-2", "chat-b", "bob", gate.now())
	if gate.QueueLen() != 2 {
		t.Fatalf("QueueLen = %d, want 2", gate.QueueLen())
	}

	// Track delivered messages via callback.
	var mu sync.Mutex
	var delivered []QueuedMessage
	cb := func(msgs []QueuedMessage) {
		mu.Lock()
		delivered = append(delivered, msgs...)
		mu.Unlock()
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gate.StartDrainTimer(ctx, cb)

	// Give the goroutine time to start and read wasQuiet before we change the clock.
	time.Sleep(100 * time.Millisecond)

	// Simulate quiet hours ending: advance clock to hour 9.
	currentHour.Store(9)

	// Wait for the drain timer to fire.
	deadline := time.After(3 * time.Second)
	for {
		mu.Lock()
		n := len(delivered)
		mu.Unlock()
		if n == 2 {
			break
		}
		select {
		case <-deadline:
			mu.Lock()
			t.Fatalf("timed out waiting for drain; delivered %d messages", len(delivered))
			mu.Unlock()
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	gate.StopDrainTimer()

	// Verify all messages were delivered.
	mu.Lock()
	defer mu.Unlock()
	if len(delivered) != 2 {
		t.Fatalf("delivered %d messages, want 2", len(delivered))
	}
	if delivered[0].Content != "msg-1" || delivered[0].ChatID != "chat-a" {
		t.Errorf("unexpected first message: %+v", delivered[0])
	}
	if delivered[1].Content != "msg-2" || delivered[1].ChatID != "chat-b" {
		t.Errorf("unexpected second message: %+v", delivered[1])
	}
	// Queue should be empty after drain.
	if gate.QueueLen() != 0 {
		t.Errorf("QueueLen after drain = %d, want 0", gate.QueueLen())
	}
}

func TestDrainTimer_StopsCleanlyOnShutdown(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	gate.drainInterval = 50 * time.Millisecond

	// Stay in quiet hours throughout.
	gate.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 23, 0, 0, 0, time.UTC)
	}

	var callCount atomic.Int32
	cb := func(msgs []QueuedMessage) {
		callCount.Add(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	gate.StartDrainTimer(ctx, cb)

	// Let the ticker run a few cycles.
	time.Sleep(200 * time.Millisecond)

	// Cancel and stop — should not hang.
	cancel()
	done := make(chan struct{})
	go func() {
		gate.StopDrainTimer()
		close(done)
	}()

	select {
	case <-done:
		// Good — stopped cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("StopDrainTimer did not return within 2 seconds")
	}

	// Callback should not have been called (still in quiet hours, no transition).
	if c := callCount.Load(); c != 0 {
		t.Errorf("callback called %d times, want 0 (no quiet→active transition)", c)
	}
}

func TestDrainTimer_NoDrainWhenQueueEmpty(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	gate.drainInterval = 50 * time.Millisecond

	var currentHour atomic.Int32
	currentHour.Store(23)
	gate.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, int(currentHour.Load()), 30, 0, 0, time.UTC)
	}

	// Don't queue any messages — just transition out of quiet hours.
	var callCount atomic.Int32
	cb := func(msgs []QueuedMessage) {
		callCount.Add(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	gate.StartDrainTimer(ctx, cb)

	// Let goroutine start, then transition out of quiet hours.
	time.Sleep(100 * time.Millisecond)
	currentHour.Store(9)
	time.Sleep(200 * time.Millisecond)

	gate.StopDrainTimer()

	// Callback should not be called with empty queue.
	if c := callCount.Load(); c != 0 {
		t.Errorf("callback called %d times, want 0 (queue was empty)", c)
	}
}

func TestDrainTimer_NoopWhenQuietHoursDisabled(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 0,
		QuietHoursEnd:   0, // disabled
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gate.StartDrainTimer(ctx, func(msgs []QueuedMessage) {
		t.Fatal("callback should never be called when quiet hours disabled")
	})

	// drainCancel should be nil since StartDrainTimer is a no-op.
	if gate.drainCancel != nil {
		t.Error("drainCancel should be nil when quiet hours disabled")
	}
}

func TestDrainTimer_DoubleStartIsNoop(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:         true,
		QuietHoursStart: 22,
		QuietHoursEnd:   8,
	})
	gate.drainInterval = 50 * time.Millisecond
	gate.nowFn = func() time.Time {
		return time.Date(2026, 3, 10, 23, 0, 0, 0, time.UTC)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cb := func(msgs []QueuedMessage) {}

	gate.StartDrainTimer(ctx, cb)
	if gate.drainCancel == nil {
		t.Fatal("drainCancel should be set after first start")
	}

	// Second call should be a no-op (drainCancel stays non-nil, no panic).
	gate.StartDrainTimer(ctx, cb)

	gate.StopDrainTimer()
}
