package cards

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// parseCard parses a card JSON string and returns the top-level map.
func parseCard(t *testing.T, cardJSON string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &m); err != nil {
		t.Fatalf("invalid card JSON: %v\nraw: %s", err, cardJSON)
	}
	return m
}

func cardHeader(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	h, ok := m["header"].(map[string]any)
	if !ok {
		t.Fatal("missing header in card")
	}
	return h
}

func headerTitle(t *testing.T, m map[string]any) string {
	t.Helper()
	h := cardHeader(t, m)
	title, ok := h["title"].(map[string]any)
	if !ok {
		t.Fatal("missing header.title")
	}
	return title["content"].(string)
}

func headerTemplate(t *testing.T, m map[string]any) string {
	t.Helper()
	h := cardHeader(t, m)
	return h["template"].(string)
}

func cardElements(t *testing.T, m map[string]any) []any {
	t.Helper()
	elems, ok := m["elements"].([]any)
	if !ok {
		t.Fatal("missing elements in card")
	}
	return elems
}

// --- BlockerAlertCard tests ---

func TestBlockerAlertCard_BasicFields(t *testing.T) {
	card := BlockerAlertCard("t1", "deploy service", "stale_progress", "no update for 30m", 30*time.Minute, "Restart the task")
	m := parseCard(t, card)

	if title := headerTitle(t, m); title != "Blocked Task Alert" {
		t.Errorf("title = %q, want 'Blocked Task Alert'", title)
	}
	if tmpl := headerTemplate(t, m); tmpl != "red" {
		t.Errorf("template = %q, want red", tmpl)
	}

	checks := []string{"deploy service", "t1", "no update for 30m", "30m", "Restart the task"}
	for _, c := range checks {
		if !strings.Contains(card, c) {
			t.Errorf("card missing %q", c)
		}
	}
}

func TestBlockerAlertCard_EmptyDescription(t *testing.T) {
	card := BlockerAlertCard("task-abc", "", "has_error", "connection refused", 0, "Fix it")
	if !strings.Contains(card, "task-abc") {
		t.Error("should use taskID when description is empty")
	}
}

func TestBlockerAlertCard_NoDuration(t *testing.T) {
	card := BlockerAlertCard("t1", "test", "error", "details", 0, "action")
	if strings.Contains(card, "Duration") {
		t.Error("should not include Duration when zero")
	}
}

func TestBlockerAlertCard_ValidJSON(t *testing.T) {
	card := BlockerAlertCard("t1", "desc", "reason", "detail", time.Hour, "action")
	m := parseCard(t, card)
	elems := cardElements(t, m)
	if len(elems) < 3 {
		t.Errorf("expected at least 3 elements (markdown, hr, markdown), got %d", len(elems))
	}
}

// --- WeeklyPulseCard tests ---

func TestWeeklyPulseCard_BasicStructure(t *testing.T) {
	metrics := []PulseMetric{
		{Label: "Tasks Completed", Value: "12", Trend: "up"},
		{Label: "Success Rate", Value: "85%", Trend: "flat"},
		{Label: "Avg Time", Value: "2h", Trend: "down"},
	}
	blockers := []BlockerEntry{
		{Description: "migrate db", Status: "failed", Reason: "timeout"},
	}

	card := WeeklyPulseCard("Mar 3", "Mar 10", metrics, blockers)
	m := parseCard(t, card)

	if title := headerTitle(t, m); !strings.Contains(title, "Weekly Pulse") {
		t.Errorf("title = %q, want to contain 'Weekly Pulse'", title)
	}
	if tmpl := headerTemplate(t, m); tmpl != "blue" {
		t.Errorf("template = %q, want blue", tmpl)
	}

	checks := []string{"Tasks Completed", "12", "↑", "85%", "→", "2h", "↓", "migrate db", "timeout"}
	for _, c := range checks {
		if !strings.Contains(card, c) {
			t.Errorf("card missing %q", c)
		}
	}
}

func TestWeeklyPulseCard_NoBlockers(t *testing.T) {
	metrics := []PulseMetric{{Label: "Tasks", Value: "5"}}
	card := WeeklyPulseCard("Mar 3", "Mar 10", metrics, nil)
	if strings.Contains(card, "Top Blockers") {
		t.Error("should not include blockers section when empty")
	}
}

func TestWeeklyPulseCard_NoTrend(t *testing.T) {
	metrics := []PulseMetric{{Label: "Cost", Value: "$1.50", Trend: ""}}
	card := WeeklyPulseCard("Mar 3", "Mar 10", metrics, nil)
	// Should not have any arrow
	for _, arrow := range []string{"↑", "↓", "→"} {
		if strings.Contains(card, arrow) {
			t.Errorf("should not contain arrow %q for empty trend", arrow)
		}
	}
}

// --- DailySummaryCard tests ---

func TestDailySummaryCard_BasicStructure(t *testing.T) {
	items := []string{"Fix blocked task t3", "Review PR #42"}
	card := DailySummaryCard("Mar 10, 2026", 5, 3, 2, 1, 0.75, items)
	m := parseCard(t, card)

	if title := headerTitle(t, m); !strings.Contains(title, "Daily Summary") {
		t.Errorf("title = %q, want to contain 'Daily Summary'", title)
	}
	if tmpl := headerTemplate(t, m); tmpl != "turquoise" {
		t.Errorf("template = %q, want turquoise", tmpl)
	}

	checks := []string{"New:** 5", "Completed:** 3", "In Progress:** 2", "Blocked:** 1", "75%", "Fix blocked task t3", "Review PR #42"}
	for _, c := range checks {
		if !strings.Contains(card, c) {
			t.Errorf("card missing %q", c)
		}
	}
}

func TestDailySummaryCard_NoActionItems(t *testing.T) {
	card := DailySummaryCard("Mar 10", 0, 0, 0, 0, 0, nil)
	if strings.Contains(card, "Action Items") {
		t.Error("should not include action items section when empty")
	}
}

func TestDailySummaryCard_ZeroRate(t *testing.T) {
	card := DailySummaryCard("Mar 10", 0, 0, 0, 0, 0, nil)
	if !strings.Contains(card, "0%") {
		t.Error("should show 0% for zero completion rate")
	}
}

// --- MilestoneCard tests ---

func TestMilestoneCard_BasicStructure(t *testing.T) {
	active := []string{"deploy v2", "write docs"}
	recent := []string{"fix bug #123"}
	card := MilestoneCard("Sprint Progress", 7, 10, active, recent)
	m := parseCard(t, card)

	if title := headerTitle(t, m); title != "Sprint Progress" {
		t.Errorf("title = %q, want 'Sprint Progress'", title)
	}
	if tmpl := headerTemplate(t, m); tmpl != "green" {
		t.Errorf("template = %q, want green", tmpl)
	}

	checks := []string{"70%", "7 / 10", "█", "░", "deploy v2", "write docs", "fix bug #123", "Recently Completed", "Active"}
	for _, c := range checks {
		if !strings.Contains(card, c) {
			t.Errorf("card missing %q", c)
		}
	}
}

func TestMilestoneCard_ZeroTotal(t *testing.T) {
	card := MilestoneCard("Empty Sprint", 0, 0, nil, nil)
	if !strings.Contains(card, "0%") {
		t.Error("should show 0% for zero total")
	}
}

func TestMilestoneCard_FullCompletion(t *testing.T) {
	card := MilestoneCard("Done", 10, 10, nil, nil)
	if !strings.Contains(card, "100%") {
		t.Error("should show 100% for full completion")
	}
	// Should be all filled blocks
	if !strings.Contains(card, "████████████████████") {
		t.Error("expected fully filled progress bar")
	}
}

func TestMilestoneCard_NoActiveTasks(t *testing.T) {
	card := MilestoneCard("Sprint", 5, 10, nil, []string{"task1"})
	if strings.Contains(card, "Active:") {
		t.Error("should not include Active section when empty")
	}
}

func TestMilestoneCard_NoRecentCompletions(t *testing.T) {
	card := MilestoneCard("Sprint", 5, 10, []string{"task1"}, nil)
	if strings.Contains(card, "Recently Completed") {
		t.Error("should not include Recently Completed section when empty")
	}
}

// --- Helper tests ---

func TestBuildProgressBar(t *testing.T) {
	tests := []struct {
		pct  float64
		want string
	}{
		{0, "░░░░░░░░░░░░░░░░░░░░"},
		{50, "██████████░░░░░░░░░░"},
		{100, "████████████████████"},
		{-10, "░░░░░░░░░░░░░░░░░░░░"},
		{150, "████████████████████"},
	}
	for _, tt := range tests {
		got := buildProgressBar(tt.pct)
		if got != tt.want {
			t.Errorf("buildProgressBar(%.0f) = %q, want %q", tt.pct, got, tt.want)
		}
	}
}

func TestTrendArrow(t *testing.T) {
	tests := []struct {
		trend, want string
	}{
		{"up", "↑"},
		{"down", "↓"},
		{"flat", "→"},
		{"", ""},
		{"unknown", ""},
	}
	for _, tt := range tests {
		got := trendArrow(tt.trend)
		if got != tt.want {
			t.Errorf("trendArrow(%q) = %q, want %q", tt.trend, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "he..." {
		t.Errorf("truncate = %q, want he...", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate short = %q, want hi", got)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{2 * time.Hour, "2h"},
		{90 * time.Minute, "1h30m"},
	}
	for _, tt := range tests {
		got := formatDuration(tt.d)
		if got != tt.want {
			t.Errorf("formatDuration(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

// --- All cards produce valid JSON ---

func TestAllCards_ValidJSON(t *testing.T) {
	cards := []string{
		BlockerAlertCard("t1", "desc", "reason", "detail", time.Hour, "action"),
		WeeklyPulseCard("Mar 3", "Mar 10", []PulseMetric{{Label: "X", Value: "1"}}, nil),
		DailySummaryCard("Mar 10", 1, 2, 3, 4, 0.5, []string{"item"}),
		MilestoneCard("Title", 5, 10, []string{"a"}, []string{"b"}),
	}
	for i, c := range cards {
		var m map[string]any
		if err := json.Unmarshal([]byte(c), &m); err != nil {
			t.Errorf("card[%d] invalid JSON: %v", i, err)
		}
		// All must have config, header, elements
		for _, key := range []string{"config", "header", "elements"} {
			if _, ok := m[key]; !ok {
				t.Errorf("card[%d] missing key %q", i, key)
			}
		}
	}
}

// --- Wide screen mode always set ---

func TestAllCards_WideScreenMode(t *testing.T) {
	cards := []string{
		BlockerAlertCard("t1", "d", "r", "d", 0, "a"),
		WeeklyPulseCard("a", "b", nil, nil),
		DailySummaryCard("d", 0, 0, 0, 0, 0, nil),
		MilestoneCard("t", 0, 0, nil, nil),
	}
	for i, c := range cards {
		m := parseCard(t, c)
		cfg, ok := m["config"].(map[string]any)
		if !ok {
			t.Errorf("card[%d] missing config", i)
			continue
		}
		if wsm, ok := cfg["wide_screen_mode"].(bool); !ok || !wsm {
			t.Errorf("card[%d] wide_screen_mode not true", i)
		}
	}
}
