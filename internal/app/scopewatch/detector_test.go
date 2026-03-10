package scopewatch

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/workitem"
	"alex/internal/domain/workitem/ports"
	"alex/internal/shared/notification"
)

// --- test doubles ---

type mockReader struct {
	items []*workitem.WorkItem
	err   error
}

func (m *mockReader) Provider() workitem.Provider { return workitem.ProviderJira }

func (m *mockReader) ListWorkItems(_ context.Context, _ ports.IssueQuery) (ports.ProviderIssuePage, error) {
	return ports.ProviderIssuePage{Items: m.items}, m.err
}

func (m *mockReader) GetWorkItem(_ context.Context, _, _ string) (*workitem.WorkItem, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockReader) ListComments(_ context.Context, _ ports.CommentQuery) (ports.ProviderCommentPage, error) {
	return ports.ProviderCommentPage{}, nil
}

func (m *mockReader) ListStatusChanges(_ context.Context, _ ports.StatusChangeQuery) (ports.ProviderStatusChangePage, error) {
	return ports.ProviderStatusChangePage{}, nil
}

func (m *mockReader) ResolveWorkspaces(_ context.Context) ([]ports.WorkspaceRef, error) {
	return nil, nil
}

type recordingNotifier struct {
	mu       sync.Mutex
	messages []string
	err      error
}

func (n *recordingNotifier) Send(_ context.Context, _ notification.Target, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, content)
	return n.err
}

func (n *recordingNotifier) Messages() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	return append([]string{}, n.messages...)
}

type recordingOutcome struct {
	mu       sync.Mutex
	outcomes []notification.AlertOutcome
}

func (r *recordingOutcome) RecordAlertOutcome(_ context.Context, _, _ string, outcome notification.AlertOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.outcomes = append(r.outcomes, outcome)
}

// --- helpers ---

func startedItem(id, title, desc string, meta map[string]string) *workitem.WorkItem {
	started := time.Now().Add(-1 * time.Hour)
	item := &workitem.WorkItem{
		ID:          id,
		Provider:    workitem.ProviderJira,
		WorkspaceID: "ws1",
		Key:         "PROJ-" + id,
		Title:       title,
		Description: desc,
		URL:         "https://jira.example.com/browse/PROJ-" + id,
		Assignee:    workitem.PersonRef{ExternalID: "user1", DisplayName: "Alice"},
		StatusClass: workitem.StatusInProgress,
		StartedAt:   &started,
		Metadata:    meta,
	}
	return item
}

func newTestDetector(reader ports.WorkItemReader, notifier notification.Notifier) *Detector {
	return NewDetector(reader, notifier, Config{
		Enabled:         true,
		LookbackSeconds: 3600,
		Channel:         "lark",
		ChatID:          "oc_test",
	})
}

// --- tests ---

func TestDetectChanges_NoItems(t *testing.T) {
	reader := &mockReader{}
	d := newTestDetector(reader, nil)

	events, err := d.DetectChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestDetectChanges_FirstScanNoEvents(t *testing.T) {
	item := startedItem("1", "Feature X", "Build feature X", nil)
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	events, err := d.DetectChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("first scan should produce no events, got %d", len(events))
	}
	if d.SnapshotCount() != 1 {
		t.Errorf("expected 1 snapshot, got %d", d.SnapshotCount())
	}
}

func TestDetectChanges_DescriptionChanged(t *testing.T) {
	item := startedItem("1", "Feature X", "Build feature X", nil)
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	// First scan — baseline
	_, _ = d.DetectChanges(context.Background())

	// Change description
	item.Description = "Build feature X with entirely new scope and requirements added"
	events, err := d.DetectChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ChangeType != ChangeDescriptionChanged {
		t.Errorf("expected description_changed, got %q", events[0].ChangeType)
	}
	if events[0].ItemKey != "PROJ-1" {
		t.Errorf("expected item key PROJ-1, got %q", events[0].ItemKey)
	}
}

func TestDetectChanges_DescriptionChangeIgnoredIfNotStarted(t *testing.T) {
	item := startedItem("1", "Feature X", "Build feature X", nil)
	item.StartedAt = nil // not started yet
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	_, _ = d.DetectChanges(context.Background())
	item.Description = "Completely different description"
	events, _ := d.DetectChanges(context.Background())

	if len(events) != 0 {
		t.Errorf("expected 0 events for unstarted item, got %d", len(events))
	}
}

func TestDetectChanges_PointsChanged(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{"story_points": "3"})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	_, _ = d.DetectChanges(context.Background())

	item.Metadata["story_points"] = "8"
	events, _ := d.DetectChanges(context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ChangeType != ChangePointsChanged {
		t.Errorf("expected points_changed, got %q", events[0].ChangeType)
	}
	if events[0].OldValue != "3" || events[0].NewValue != "8" {
		t.Errorf("expected 3→8, got %s→%s", events[0].OldValue, events[0].NewValue)
	}
}

func TestDetectChanges_AssigneeChanged(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", nil)
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	_, _ = d.DetectChanges(context.Background())

	item.Assignee = workitem.PersonRef{ExternalID: "user2", DisplayName: "Bob"}
	events, _ := d.DetectChanges(context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ChangeType != ChangeAssigneeChanged {
		t.Errorf("expected assignee_changed, got %q", events[0].ChangeType)
	}
}

func TestDetectChanges_DeadlineMoved(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{"deadline": "2026-03-15"})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	_, _ = d.DetectChanges(context.Background())

	item.Metadata["deadline"] = "2026-04-01"
	events, _ := d.DetectChanges(context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].ChangeType != ChangeDeadlineMoved {
		t.Errorf("expected deadline_moved, got %q", events[0].ChangeType)
	}
	if events[0].OldValue != "2026-03-15" || events[0].NewValue != "2026-04-01" {
		t.Errorf("expected 2026-03-15→2026-04-01, got %s→%s", events[0].OldValue, events[0].NewValue)
	}
}

func TestDetectChanges_MultipleChanges(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{
		"story_points": "3",
		"deadline":     "2026-03-15",
	})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	d := newTestDetector(reader, nil)

	_, _ = d.DetectChanges(context.Background())

	// Change description, points, and deadline simultaneously
	item.Description = "Completely rewritten scope"
	item.Metadata["story_points"] = "13"
	item.Metadata["deadline"] = "2026-05-01"
	events, _ := d.DetectChanges(context.Background())

	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	types := make(map[ChangeType]bool)
	for _, e := range events {
		types[e.ChangeType] = true
	}
	for _, want := range []ChangeType{ChangeDescriptionChanged, ChangePointsChanged, ChangeDeadlineMoved} {
		if !types[want] {
			t.Errorf("missing change type %q", want)
		}
	}
}

func TestDetectChanges_ReaderError(t *testing.T) {
	reader := &mockReader{err: fmt.Errorf("connection failed")}
	d := newTestDetector(reader, nil)

	_, err := d.DetectChanges(context.Background())
	if err == nil {
		t.Fatal("expected error from reader")
	}
	if !strings.Contains(err.Error(), "connection failed") {
		t.Errorf("expected connection failed error, got: %v", err)
	}
}

func TestNotifyScopeChanges_NoChanges(t *testing.T) {
	reader := &mockReader{}
	notifier := &recordingNotifier{}
	d := newTestDetector(reader, notifier)

	err := d.NotifyScopeChanges(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(notifier.Messages()) != 0 {
		t.Errorf("expected no messages for no changes, got %d", len(notifier.Messages()))
	}
}

func TestNotifyScopeChanges_SendsAlert(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{"story_points": "3"})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	notifier := &recordingNotifier{}
	d := newTestDetector(reader, notifier)

	// First scan — baseline
	_ = d.NotifyScopeChanges(context.Background())
	if len(notifier.Messages()) != 0 {
		t.Fatal("first scan should not send")
	}

	// Change points
	item.Metadata["story_points"] = "8"
	_ = d.NotifyScopeChanges(context.Background())

	msgs := notifier.Messages()
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if !strings.Contains(msgs[0], "Scope Change Alert") {
		t.Errorf("expected alert header, got: %s", msgs[0])
	}
	if !strings.Contains(msgs[0], "Story points changed") {
		t.Errorf("expected points change mention, got: %s", msgs[0])
	}
}

func TestNotifyScopeChanges_RecordsOutcome(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{"story_points": "3"})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	notifier := &recordingNotifier{}
	recorder := &recordingOutcome{}
	d := newTestDetector(reader, notifier)
	d.SetOutcomeRecorder(recorder)

	_, _ = d.DetectChanges(context.Background())
	item.Metadata["story_points"] = "8"
	_ = d.NotifyScopeChanges(context.Background())

	if len(recorder.outcomes) != 1 {
		t.Fatalf("expected 1 outcome, got %d", len(recorder.outcomes))
	}
	if recorder.outcomes[0] != notification.OutcomeSent {
		t.Errorf("expected outcome=sent, got %q", recorder.outcomes[0])
	}
}

func TestNotifyScopeChanges_RecordsFailedOutcome(t *testing.T) {
	item := startedItem("1", "Feature X", "Build X", map[string]string{"story_points": "3"})
	reader := &mockReader{items: []*workitem.WorkItem{item}}
	notifier := &recordingNotifier{err: fmt.Errorf("send failed")}
	recorder := &recordingOutcome{}
	d := newTestDetector(reader, notifier)
	d.SetOutcomeRecorder(recorder)

	_, _ = d.DetectChanges(context.Background())
	item.Metadata["story_points"] = "8"
	err := d.NotifyScopeChanges(context.Background())

	if err == nil {
		t.Fatal("expected error from failed notifier")
	}
	if len(recorder.outcomes) != 1 || recorder.outcomes[0] != notification.OutcomeFailed {
		t.Errorf("expected outcome=failed, got %v", recorder.outcomes)
	}
}

func TestFormatScopeChangeAlert(t *testing.T) {
	events := []ScopeChangeEvent{
		{ItemKey: "PROJ-1", ItemTitle: "Feature X", ChangeType: ChangeDescriptionChanged, OldValue: "hash:abc", NewValue: "hash:def"},
		{ItemKey: "PROJ-2", ItemTitle: "Feature Y", ChangeType: ChangePointsChanged, OldValue: "3", NewValue: "8"},
	}

	msg := formatScopeChangeAlert(events)
	if !strings.Contains(msg, "2 change(s) detected") {
		t.Errorf("expected count in message, got: %s", msg)
	}
	if !strings.Contains(msg, "PROJ-1") {
		t.Errorf("expected PROJ-1 in message, got: %s", msg)
	}
	if !strings.Contains(msg, "Description changed") {
		t.Errorf("expected description change in message, got: %s", msg)
	}
	if !strings.Contains(msg, "3 → 8") {
		t.Errorf("expected points change in message, got: %s", msg)
	}
}

func TestChangeTypeConstants(t *testing.T) {
	all := []ChangeType{
		ChangeDescriptionChanged,
		ChangePointsChanged,
		ChangeAssigneeChanged,
		ChangeDeadlineMoved,
	}
	seen := make(map[ChangeType]bool)
	for _, ct := range all {
		if seen[ct] {
			t.Errorf("duplicate ChangeType: %q", ct)
		}
		seen[ct] = true
	}
}

func TestSnapshotKey(t *testing.T) {
	item := &workitem.WorkItem{Provider: workitem.ProviderJira, WorkspaceID: "ws1", ID: "123"}
	key := snapshotKey(item)
	if key != "jira:ws1:123" {
		t.Errorf("expected jira:ws1:123, got %q", key)
	}
}

func TestHashString(t *testing.T) {
	h1 := hashString("hello")
	h2 := hashString("hello")
	h3 := hashString("world")

	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
	if h1 == h3 {
		t.Error("different input should produce different hash")
	}
	if len(h1) != 64 {
		t.Errorf("expected 64 char hex hash, got %d", len(h1))
	}
}
