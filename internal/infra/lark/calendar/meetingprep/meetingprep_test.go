package meetingprep

import (
	"strings"
	"testing"
	"time"
)

// ---------- Test helpers ----------

// refTime is a fixed reference time: 2026-02-03 14:00 UTC.
var refTime = time.Date(2026, 2, 3, 14, 0, 0, 0, time.UTC)

func timePtr(t time.Time) *time.Time { return &t }

// fullContext returns a richly populated MeetingContext for testing.
func fullContext() MeetingContext {
	yesterday := refTime.Add(-24 * time.Hour)
	lastWeek := refTime.Add(-7 * 24 * time.Hour)
	tomorrow := refTime.Add(24 * time.Hour)
	overdue := refTime.Add(-2 * time.Hour) // before meeting start

	return MeetingContext{
		Event: EventInfo{
			EventID:     "evt-001",
			Summary:     "Weekly Engineering Sync",
			Description: "Discuss sprint progress and blockers.\nReview deployment plan.",
			StartTime:   refTime,
			EndTime:     refTime.Add(1 * time.Hour),
			Location:    "Meeting Room A",
			IsRecurring: true,
		},
		Attendees: []AttendeeInfo{
			{UserID: "u1", Name: "Alice", Email: "alice@example.com", Role: "organizer"},
			{UserID: "u2", Name: "Bob", Email: "bob@example.com", Role: "required"},
			{UserID: "u3", Name: "Charlie", Email: "charlie@example.com", Role: "optional"},
		},
		RecentNotes: []NoteReference{
			{Date: yesterday, Title: "Weekly Sync Jan-27", Summary: "Discussed CI pipeline improvements"},
			{Date: lastWeek, Title: "Weekly Sync Jan-20", Summary: "Reviewed Q1 OKRs"},
		},
		RelatedDecisions: []DecisionReference{
			{Decision: "Adopt Go modules for dependency management", Rationale: "Better reproducibility", Date: lastWeek, Resolved: false},
			{Decision: "Use PostgreSQL for primary storage", Rationale: "Team expertise", Date: lastWeek, Resolved: true},
		},
		PendingTasks: []TaskReference{
			{TaskID: "t1", Summary: "Update CI pipeline", Assignee: "Bob", DueTime: timePtr(overdue), Completed: false},
			{TaskID: "t2", Summary: "Write migration scripts", Assignee: "Alice", DueTime: timePtr(tomorrow), Completed: false},
			{TaskID: "t3", Summary: "Setup monitoring", Assignee: "Charlie", DueTime: nil, Completed: false},
			{TaskID: "t4", Summary: "Old cleanup task", Assignee: "Bob", DueTime: timePtr(lastWeek), Completed: true},
		},
		UserTimezone: "Asia/Shanghai",
	}
}

// ---------- PrepareMeeting tests ----------

func TestPrepareMeeting_FullContext(t *testing.T) {
	ctx := fullContext()
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// EventSummary should contain the title, location, and recurring marker.
	if !strings.Contains(doc.EventSummary, "Weekly Engineering Sync") {
		t.Errorf("EventSummary missing title: %q", doc.EventSummary)
	}
	if !strings.Contains(doc.EventSummary, "Meeting Room A") {
		t.Errorf("EventSummary missing location: %q", doc.EventSummary)
	}
	if !strings.Contains(doc.EventSummary, "[recurring]") {
		t.Errorf("EventSummary missing recurring marker: %q", doc.EventSummary)
	}

	// TimeInfo should contain the timezone.
	if !strings.Contains(doc.TimeInfo, "Asia/Shanghai") {
		t.Errorf("TimeInfo missing timezone: %q", doc.TimeInfo)
	}
	if !strings.Contains(doc.TimeInfo, "1h") {
		t.Errorf("TimeInfo missing duration: %q", doc.TimeInfo)
	}

	// AttendeeList should contain all names and roles.
	for _, name := range []string{"Alice", "Bob", "Charlie"} {
		if !strings.Contains(doc.AttendeeList, name) {
			t.Errorf("AttendeeList missing %q: %q", name, doc.AttendeeList)
		}
	}
	if !strings.Contains(doc.AttendeeList, "organizer") {
		t.Errorf("AttendeeList missing organizer role: %q", doc.AttendeeList)
	}

	// AgendaSuggestions should be non-empty.
	if len(doc.AgendaSuggestions) == 0 {
		t.Error("expected non-empty AgendaSuggestions")
	}

	// HistoricalContext should reference past notes.
	if !strings.Contains(doc.HistoricalContext, "Weekly Sync Jan-27") {
		t.Errorf("HistoricalContext missing recent note: %q", doc.HistoricalContext)
	}

	// OpenDecisions should contain the unresolved decision.
	if !strings.Contains(doc.OpenDecisions, "Adopt Go modules") {
		t.Errorf("OpenDecisions missing unresolved decision: %q", doc.OpenDecisions)
	}
	// By default, resolved decisions are excluded.
	if strings.Contains(doc.OpenDecisions, "PostgreSQL") {
		t.Errorf("OpenDecisions should not include resolved decision: %q", doc.OpenDecisions)
	}

	// ActionItems should contain pending tasks and exclude completed.
	if !strings.Contains(doc.ActionItems, "Update CI pipeline") {
		t.Errorf("ActionItems missing pending task: %q", doc.ActionItems)
	}
	if strings.Contains(doc.ActionItems, "Old cleanup task") {
		t.Errorf("ActionItems should not include completed task: %q", doc.ActionItems)
	}

	// FullMarkdown should contain major sections.
	for _, section := range []string{
		"# Meeting Prep",
		"## Attendees",
		"## Suggested Agenda",
		"## Historical Context",
		"## Open Decisions",
		"## Action Items",
	} {
		if !strings.Contains(doc.FullMarkdown, section) {
			t.Errorf("FullMarkdown missing section %q", section)
		}
	}
}

func TestPrepareMeeting_EmptyOptionalFields(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-002",
			Summary:   "Quick Chat",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(doc.EventSummary, "Quick Chat") {
		t.Errorf("EventSummary = %q; want to contain 'Quick Chat'", doc.EventSummary)
	}
	if doc.AttendeeList != "" {
		t.Errorf("expected empty AttendeeList, got %q", doc.AttendeeList)
	}
	if doc.HistoricalContext != "" {
		t.Errorf("expected empty HistoricalContext, got %q", doc.HistoricalContext)
	}
	if doc.OpenDecisions != "" {
		t.Errorf("expected empty OpenDecisions, got %q", doc.OpenDecisions)
	}
	if doc.ActionItems != "" {
		t.Errorf("expected empty ActionItems, got %q", doc.ActionItems)
	}

	// FullMarkdown should still render without optional sections.
	if !strings.Contains(doc.FullMarkdown, "# Meeting Prep") {
		t.Error("FullMarkdown missing header")
	}
	// Should NOT contain sections that have no data.
	if strings.Contains(doc.FullMarkdown, "## Attendees") {
		t.Error("FullMarkdown should not contain Attendees section when empty")
	}
	if strings.Contains(doc.FullMarkdown, "## Historical Context") {
		t.Error("FullMarkdown should not contain Historical Context section when empty")
	}
}

func TestPrepareMeeting_NoTitle(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-003",
			StartTime: refTime,
			EndTime:   refTime.Add(15 * time.Minute),
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.EventSummary, "(No title)") {
		t.Errorf("EventSummary = %q; expected '(No title)' fallback", doc.EventSummary)
	}
}

func TestPrepareMeetingWithConfig_IncludeCompleted(t *testing.T) {
	ctx := fullContext()
	cfg := PrepConfig{
		MaxHistoricalNotes: 3,
		MaxAgendaItems:     7,
		IncludeCompleted:   true,
	}
	doc, err := PrepareMeetingWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Resolved decision should now appear.
	if !strings.Contains(doc.OpenDecisions, "PostgreSQL") {
		t.Errorf("expected resolved decision when IncludeCompleted=true: %q", doc.OpenDecisions)
	}
	if !strings.Contains(doc.OpenDecisions, "RESOLVED") {
		t.Errorf("expected RESOLVED label: %q", doc.OpenDecisions)
	}

	// Completed task should now appear.
	if !strings.Contains(doc.ActionItems, "Old cleanup task") {
		t.Errorf("expected completed task when IncludeCompleted=true: %q", doc.ActionItems)
	}
	if !strings.Contains(doc.ActionItems, "[x]") {
		t.Errorf("expected [x] marker for completed task: %q", doc.ActionItems)
	}
}

func TestPrepareMeetingWithConfig_MaxHistoricalNotes(t *testing.T) {
	ctx := fullContext()
	// Add extra notes beyond the limit.
	ctx.RecentNotes = append(ctx.RecentNotes,
		NoteReference{Date: refTime.Add(-14 * 24 * time.Hour), Title: "Old Note 1", Summary: "Very old content"},
		NoteReference{Date: refTime.Add(-21 * 24 * time.Hour), Title: "Old Note 2", Summary: "Ancient content"},
	)

	cfg := PrepConfig{
		MaxHistoricalNotes: 2,
		MaxAgendaItems:     7,
		IncludeCompleted:   false,
	}
	doc, err := PrepareMeetingWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only first 2 notes should appear.
	if !strings.Contains(doc.HistoricalContext, "Weekly Sync Jan-27") {
		t.Errorf("HistoricalContext should contain first note: %q", doc.HistoricalContext)
	}
	if !strings.Contains(doc.HistoricalContext, "Weekly Sync Jan-20") {
		t.Errorf("HistoricalContext should contain second note: %q", doc.HistoricalContext)
	}
	if strings.Contains(doc.HistoricalContext, "Old Note 1") {
		t.Errorf("HistoricalContext should not contain third note: %q", doc.HistoricalContext)
	}
}

func TestPrepareMeetingWithConfig_MaxAgendaItems(t *testing.T) {
	ctx := fullContext()
	// Add many tasks to generate many agenda items.
	for i := 0; i < 10; i++ {
		ctx.PendingTasks = append(ctx.PendingTasks, TaskReference{
			TaskID:  "extra-" + string(rune('a'+i)),
			Summary: "Extra task " + string(rune('A'+i)),
		})
	}

	cfg := PrepConfig{
		MaxHistoricalNotes: 3,
		MaxAgendaItems:     3,
		IncludeCompleted:   false,
	}
	doc, err := PrepareMeetingWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.AgendaSuggestions) > 3 {
		t.Errorf("expected at most 3 agenda items, got %d", len(doc.AgendaSuggestions))
	}
}

// ---------- SuggestAgenda tests ----------

func TestSuggestAgenda_WithDecisionsAndTasks(t *testing.T) {
	ctx := fullContext()
	items := SuggestAgenda(ctx)

	if len(items) == 0 {
		t.Fatal("expected non-empty agenda suggestions")
	}

	// Should include the topic from description.
	hasDescription := false
	for _, item := range items {
		if strings.HasPrefix(item, "Topic:") {
			hasDescription = true
			break
		}
	}
	if !hasDescription {
		t.Error("expected agenda item from event description")
	}

	// Should include "Review:" for unresolved decision.
	hasReview := false
	for _, item := range items {
		if strings.Contains(item, "Review:") && strings.Contains(item, "Adopt Go modules") {
			hasReview = true
			break
		}
	}
	if !hasReview {
		t.Error("expected 'Review:' item for unresolved decision")
	}

	// Should NOT include "Review:" for resolved decision.
	for _, item := range items {
		if strings.Contains(item, "Review:") && strings.Contains(item, "PostgreSQL") {
			t.Error("should not suggest review for resolved decision")
		}
	}
}

func TestSuggestAgenda_OverdueTasks(t *testing.T) {
	overdue := refTime.Add(-1 * time.Hour)
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-010",
			Summary:   "Standup",
			StartTime: refTime,
			EndTime:   refTime.Add(15 * time.Minute),
		},
		PendingTasks: []TaskReference{
			{TaskID: "t1", Summary: "Overdue task A", DueTime: timePtr(overdue), Completed: false},
			{TaskID: "t2", Summary: "Overdue task B", DueTime: timePtr(overdue.Add(-24 * time.Hour)), Completed: false},
			{TaskID: "t3", Summary: "Completed overdue", DueTime: timePtr(overdue), Completed: true},
		},
	}

	items := SuggestAgenda(ctx)

	followUpCount := 0
	for _, item := range items {
		if strings.HasPrefix(item, "Follow up:") {
			followUpCount++
		}
	}
	if followUpCount != 2 {
		t.Errorf("expected 2 'Follow up:' items for overdue tasks, got %d; items: %v", followUpCount, items)
	}

	// Completed tasks should not appear.
	for _, item := range items {
		if strings.Contains(item, "Completed overdue") {
			t.Error("completed overdue task should not appear in agenda")
		}
	}
}

func TestSuggestAgenda_CarryOverFromNotes(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-011",
			Summary:   "Recurring Sync",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		RecentNotes: []NoteReference{
			{Date: refTime.Add(-24 * time.Hour), Title: "Last sync", Summary: "Discuss deployment timeline"},
		},
	}

	items := SuggestAgenda(ctx)
	hasCarryOver := false
	for _, item := range items {
		if strings.HasPrefix(item, "Carry over:") {
			hasCarryOver = true
			break
		}
	}
	if !hasCarryOver {
		t.Errorf("expected 'Carry over:' item from recent notes, got: %v", items)
	}
}

func TestSuggestAgenda_EmptyContext(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-012",
			Summary:   "Empty Meeting",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
	}

	items := SuggestAgenda(ctx)
	if len(items) != 0 {
		t.Errorf("expected no agenda items for empty context, got %d: %v", len(items), items)
	}
}

func TestSuggestAgenda_DescriptionOnly(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:     "evt-013",
			Summary:     "Planning",
			Description: "Roadmap review for Q2",
			StartTime:   refTime,
			EndTime:     refTime.Add(1 * time.Hour),
		},
	}

	items := SuggestAgenda(ctx)
	if len(items) != 1 {
		t.Fatalf("expected 1 agenda item, got %d: %v", len(items), items)
	}
	if items[0] != "Topic: Roadmap review for Q2" {
		t.Errorf("unexpected agenda item: %q", items[0])
	}
}

func TestSuggestAgenda_PendingNonOverdueTasks(t *testing.T) {
	future := refTime.Add(48 * time.Hour)
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-014",
			Summary:   "Sync",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		PendingTasks: []TaskReference{
			{TaskID: "t1", Summary: "Future task", DueTime: timePtr(future), Completed: false},
			{TaskID: "t2", Summary: "No due date task", Completed: false},
		},
	}

	items := SuggestAgenda(ctx)
	discussCount := 0
	for _, item := range items {
		if strings.HasPrefix(item, "Discuss:") {
			discussCount++
		}
	}
	if discussCount != 2 {
		t.Errorf("expected 2 'Discuss:' items, got %d: %v", discussCount, items)
	}
}

// ---------- FormatMarkdown tests ----------

func TestFormatMarkdown_FullDocument(t *testing.T) {
	doc := &PrepDocument{
		EventSummary:      "Team Standup @ Room B [recurring]",
		TimeInfo:          "2026-02-03 14:00 - 14:30 (30 min, UTC)",
		AttendeeList:      "- Alice (organizer)\n- Bob (required)",
		AgendaSuggestions: []string{"Topic: Sprint review", "Review: API design", "Follow up: CI setup"},
		HistoricalContext: "- **Last Standup** (2026-02-02): Discussed blockers",
		OpenDecisions:     "- [OPEN] API design — Rationale: Need consensus (2026-01-27)",
		ActionItems:       "- [ ] CI setup (@Bob) due 2026-02-01",
	}

	md := FormatMarkdown(doc)

	required := []string{
		"# Meeting Prep",
		"## Team Standup @ Room B [recurring]",
		"**When:** 2026-02-03 14:00 - 14:30 (30 min, UTC)",
		"## Attendees",
		"- Alice (organizer)",
		"- Bob (required)",
		"## Suggested Agenda",
		"1. Topic: Sprint review",
		"2. Review: API design",
		"3. Follow up: CI setup",
		"## Historical Context",
		"## Open Decisions",
		"## Action Items",
	}
	for _, s := range required {
		if !strings.Contains(md, s) {
			t.Errorf("FormatMarkdown missing %q", s)
		}
	}
}

func TestFormatMarkdown_MinimalDocument(t *testing.T) {
	doc := &PrepDocument{
		EventSummary: "Quick Chat",
	}
	md := FormatMarkdown(doc)

	if !strings.Contains(md, "# Meeting Prep") {
		t.Error("missing header")
	}
	if !strings.Contains(md, "## Quick Chat") {
		t.Error("missing event summary")
	}
	// Optional sections should be absent.
	for _, section := range []string{"## Attendees", "## Suggested Agenda", "## Historical Context", "## Open Decisions", "## Action Items"} {
		if strings.Contains(md, section) {
			t.Errorf("should not contain empty section %q", section)
		}
	}
}

// ---------- Edge cases ----------

func TestPrepareMeeting_NoAttendees(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-020",
			Summary:   "Solo Review",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		Attendees: []AttendeeInfo{},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.AttendeeList != "" {
		t.Errorf("expected empty AttendeeList, got %q", doc.AttendeeList)
	}
}

func TestPrepareMeeting_NoTasks(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-021",
			Summary:   "Brainstorm",
			StartTime: refTime,
			EndTime:   refTime.Add(1 * time.Hour),
		},
		PendingTasks: []TaskReference{},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.ActionItems != "" {
		t.Errorf("expected empty ActionItems, got %q", doc.ActionItems)
	}
}

func TestPrepareMeeting_NoHistory(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-022",
			Summary:   "First Meeting",
			StartTime: refTime,
			EndTime:   refTime.Add(1 * time.Hour),
		},
		RecentNotes: []NoteReference{},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.HistoricalContext != "" {
		t.Errorf("expected empty HistoricalContext, got %q", doc.HistoricalContext)
	}
}

func TestPrepareMeeting_AllTasksCompleted(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-023",
			Summary:   "Wrap-up",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		PendingTasks: []TaskReference{
			{TaskID: "t1", Summary: "Done task 1", Completed: true},
			{TaskID: "t2", Summary: "Done task 2", Completed: true},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default config excludes completed tasks.
	if doc.ActionItems != "" {
		t.Errorf("expected empty ActionItems when all completed, got %q", doc.ActionItems)
	}
}

func TestPrepareMeeting_AllDecisionsResolved(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-024",
			Summary:   "Retrospective",
			StartTime: refTime,
			EndTime:   refTime.Add(1 * time.Hour),
		},
		RelatedDecisions: []DecisionReference{
			{Decision: "Use Docker", Resolved: true, Date: refTime.Add(-7 * 24 * time.Hour)},
			{Decision: "Adopt k8s", Resolved: true, Date: refTime.Add(-3 * 24 * time.Hour)},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default config excludes resolved decisions.
	if doc.OpenDecisions != "" {
		t.Errorf("expected empty OpenDecisions when all resolved, got %q", doc.OpenDecisions)
	}
}

func TestPrepareMeeting_AttendeeFallbacks(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-025",
			Summary:   "Test",
			StartTime: refTime,
			EndTime:   refTime.Add(15 * time.Minute),
		},
		Attendees: []AttendeeInfo{
			{UserID: "u1", Name: "", Email: "anon@example.com", Role: "required"},
			{UserID: "u2", Name: "", Email: "", Role: ""},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// First attendee should fall back to email.
	if !strings.Contains(doc.AttendeeList, "anon@example.com") {
		t.Errorf("expected email fallback in AttendeeList: %q", doc.AttendeeList)
	}
	// Second attendee should fall back to UserID.
	if !strings.Contains(doc.AttendeeList, "u2") {
		t.Errorf("expected UserID fallback in AttendeeList: %q", doc.AttendeeList)
	}
	// Second attendee should get default role.
	if !strings.Contains(doc.AttendeeList, "attendee") {
		t.Errorf("expected default 'attendee' role in AttendeeList: %q", doc.AttendeeList)
	}
}

func TestPrepareMeeting_UTCFallbackTimezone(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-026",
			Summary:   "UTC Test",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		UserTimezone: "", // empty -> should fall back to UTC
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.TimeInfo, "UTC") {
		t.Errorf("expected UTC in TimeInfo when timezone is empty: %q", doc.TimeInfo)
	}
}

func TestPrepareMeeting_InvalidTimezone(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-027",
			Summary:   "Bad TZ Test",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		UserTimezone: "Invalid/Zone",
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to UTC gracefully.
	if !strings.Contains(doc.TimeInfo, "UTC") {
		t.Errorf("expected UTC fallback for invalid timezone: %q", doc.TimeInfo)
	}
}

// ---------- DefaultPrepConfig tests ----------

func TestDefaultPrepConfig(t *testing.T) {
	cfg := DefaultPrepConfig()
	if cfg.MaxHistoricalNotes != 3 {
		t.Errorf("MaxHistoricalNotes = %d; want 3", cfg.MaxHistoricalNotes)
	}
	if cfg.MaxAgendaItems != 7 {
		t.Errorf("MaxAgendaItems = %d; want 7", cfg.MaxAgendaItems)
	}
	if cfg.IncludeCompleted != false {
		t.Error("IncludeCompleted should default to false")
	}
}

// ---------- buildTimeInfo formatting ----------

func TestBuildTimeInfo_ShortDuration(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-030",
			Summary:   "Quick",
			StartTime: refTime,
			EndTime:   refTime.Add(15 * time.Minute),
		},
		UserTimezone: "UTC",
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.TimeInfo, "15 min") {
		t.Errorf("TimeInfo should contain '15 min': %q", doc.TimeInfo)
	}
}

func TestBuildTimeInfo_MultiHour(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-031",
			Summary:   "Workshop",
			StartTime: refTime,
			EndTime:   refTime.Add(2*time.Hour + 30*time.Minute),
		},
		UserTimezone: "UTC",
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.TimeInfo, "2h30m") {
		t.Errorf("TimeInfo should contain '2h30m': %q", doc.TimeInfo)
	}
}

// ---------- Integration: full round-trip ----------

func TestPrepareMeeting_FullRoundTrip(t *testing.T) {
	ctx := fullContext()
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Re-format should produce the same markdown.
	reformatted := FormatMarkdown(doc)
	if reformatted != doc.FullMarkdown {
		t.Errorf("FormatMarkdown(doc) != doc.FullMarkdown;\ngot:\n%s\nwant:\n%s", reformatted, doc.FullMarkdown)
	}
}

func TestPrepareMeeting_TaskDueTimeNil(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-040",
			Summary:   "Sync",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		PendingTasks: []TaskReference{
			{TaskID: "t1", Summary: "No deadline task", Assignee: "Alice", DueTime: nil, Completed: false},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.ActionItems, "No deadline task") {
		t.Errorf("ActionItems missing task: %q", doc.ActionItems)
	}
	// Should not contain "due" since DueTime is nil.
	if strings.Contains(doc.ActionItems, "due ") {
		t.Errorf("ActionItems should not contain due date for nil DueTime: %q", doc.ActionItems)
	}
}

func TestPrepareMeeting_NoteWithoutSummary(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-041",
			Summary:   "Check",
			StartTime: refTime,
			EndTime:   refTime.Add(30 * time.Minute),
		},
		RecentNotes: []NoteReference{
			{Date: refTime.Add(-24 * time.Hour), Title: "Titleonly"},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.HistoricalContext, "Titleonly") {
		t.Errorf("HistoricalContext missing title: %q", doc.HistoricalContext)
	}
	// Note without summary should not produce a carry-over agenda item.
	for _, item := range doc.AgendaSuggestions {
		if strings.HasPrefix(item, "Carry over:") {
			t.Errorf("should not produce carry-over for note without summary: %v", doc.AgendaSuggestions)
		}
	}
}

func TestPrepareMeeting_DecisionWithoutRationale(t *testing.T) {
	ctx := MeetingContext{
		Event: EventInfo{
			EventID:   "evt-042",
			Summary:   "Review",
			StartTime: refTime,
			EndTime:   refTime.Add(1 * time.Hour),
		},
		RelatedDecisions: []DecisionReference{
			{Decision: "Migrate to cloud", Date: refTime.Add(-7 * 24 * time.Hour), Resolved: false},
		},
	}
	doc, err := PrepareMeeting(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(doc.OpenDecisions, "Migrate to cloud") {
		t.Errorf("OpenDecisions missing decision: %q", doc.OpenDecisions)
	}
	// Should not contain "Rationale:" when rationale is empty.
	if strings.Contains(doc.OpenDecisions, "Rationale:") {
		t.Errorf("OpenDecisions should not contain empty rationale field: %q", doc.OpenDecisions)
	}
}
