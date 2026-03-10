package app_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/app/blocker"
	"alex/internal/app/milestone"
	"alex/internal/app/prepbrief"
	"alex/internal/app/pulse"
	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
	"alex/internal/shared/notification"
)

// --- in-memory mock notifier ---

type memNotifier struct {
	mu       sync.Mutex
	messages []sentMessage
}

type sentMessage struct {
	target  notification.Target
	content string
}

func (n *memNotifier) Send(_ context.Context, target notification.Target, content string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.messages = append(n.messages, sentMessage{target: target, content: content})
	return nil
}

func (n *memNotifier) allContent() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]string, len(n.messages))
	for i, m := range n.messages {
		out[i] = m.content
	}
	return out
}

func (n *memNotifier) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.messages)
}

// --- helpers ---

func newTestStore(t *testing.T) *taskstore.LocalStore {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "tasks.json")
	s := taskstore.New(taskstore.WithFilePath(fp))
	t.Cleanup(func() { s.Close() })
	return s
}

// seedTasks creates a realistic set of tasks covering all statuses.
//
// Note: LocalStore.Create() always sets UpdatedAt to time.Now(), so
// time-threshold-based detections (stale progress, input wait timeout)
// cannot be triggered through the public API alone. The test focuses on
// detections that do not depend on elapsed time: errors, dependencies,
// status-based classification, and member scoping.
func seedTasks(t *testing.T, store *taskstore.LocalStore) map[string]string {
	t.Helper()
	ctx := context.Background()
	now := time.Now()

	completedAt := now.Add(-1 * time.Hour)
	startedAt := now.Add(-3 * time.Hour)

	tasks := []*task.Task{
		{
			TaskID:        "task-completed-1",
			Description:   "implement user auth",
			Status:        task.StatusCompleted,
			UserID:        "alice",
			TokensUsed:    5000,
			CostUSD:       0.15,
			CreatedAt:     now.Add(-48 * time.Hour),
			StartedAt:     &startedAt,
			CompletedAt:   &completedAt,
			AnswerPreview: "Auth module implemented with JWT tokens",
		},
		{
			TaskID:        "task-completed-2",
			Description:   "fix database migration",
			Status:        task.StatusCompleted,
			UserID:        "alice",
			TokensUsed:    2000,
			CostUSD:       0.08,
			CreatedAt:     now.Add(-24 * time.Hour),
			StartedAt:     &startedAt,
			CompletedAt:   &completedAt,
			AnswerPreview: "Migration script corrected",
		},
		{
			// Running task with an error — triggers ReasonHasError in Radar
			// and error-based blocker in PrepBrief.
			TaskID:           "task-running-error",
			Description:      "refactor payment module",
			Status:           task.StatusRunning,
			UserID:           "alice",
			Error:            "nil pointer in payment gateway adapter",
			TokensUsed:       3000,
			CurrentIteration: 5,
			CreatedAt:        now.Add(-3 * time.Hour),
		},
		{
			TaskID:      "task-running-fresh",
			Description: "add metrics endpoint",
			Status:      task.StatusRunning,
			UserID:      "bob",
			TokensUsed:  1000,
			CreatedAt:   now.Add(-30 * time.Minute),
		},
		{
			TaskID:      "task-failed",
			Description: "deploy staging environment",
			Status:      task.StatusFailed,
			UserID:      "alice",
			Error:       "connection timeout to staging cluster",
			TokensUsed:  800,
			CreatedAt:   now.Add(-2 * time.Hour),
			CompletedAt: &completedAt,
		},
		{
			TaskID:      "task-waiting",
			Description: "review API design doc",
			Status:      task.StatusWaitingInput,
			UserID:      "alice",
			CreatedAt:   now.Add(-4 * time.Hour),
		},
		{
			// Depends on an active (non-completed) task — triggers ReasonDepBlocked.
			TaskID:      "task-with-dep",
			Description: "integrate payment with auth",
			Status:      task.StatusRunning,
			UserID:      "alice",
			DependsOn:   []string{"task-running-error"},
			CreatedAt:   now.Add(-1 * time.Hour),
		},
	}

	ids := make(map[string]string, len(tasks))
	for _, tk := range tasks {
		if err := store.Create(ctx, tk); err != nil {
			t.Fatalf("seed task %s: %v", tk.TaskID, err)
		}
		ids[tk.TaskID] = tk.Description
	}
	return ids
}

// --- integration tests ---

// TestLeaderE2E_FullFlow exercises the complete leader agent feature set
// end-to-end: task creation → blocker radar → weekly pulse → milestone → prep brief.
func TestLeaderE2E_FullFlow(t *testing.T) {
	store := newTestStore(t)
	notif := &memNotifier{}
	ctx := context.Background()

	// Step 0: Seed tasks via task.Store.
	ids := seedTasks(t, store)
	t.Logf("Seeded %d tasks", len(ids))

	// Verify tasks are persisted and retrievable.
	all, total, err := store.List(ctx, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != len(ids) {
		t.Fatalf("expected %d tasks, got %d", len(ids), total)
	}
	t.Logf("Store verified: %d tasks", len(all))

	// Step 1: Blocker Radar — detect blocked tasks.
	t.Run("BlockerRadar", func(t *testing.T) {
		radar := blocker.NewRadar(store, notif, blocker.Config{
			Enabled:            true,
			StaleThreshold:     30 * time.Minute,
			InputWaitThreshold: 15 * time.Minute,
			Channel:            "lark",
			ChatID:             "test-chat",
		})

		result, err := radar.Scan(ctx)
		if err != nil {
			t.Fatalf("Scan: %v", err)
		}

		if result.TasksScanned == 0 {
			t.Fatal("expected non-zero tasks scanned")
		}
		t.Logf("Scanned %d tasks, found %d alerts", result.TasksScanned, len(result.Alerts))

		if len(result.Alerts) == 0 {
			t.Fatal("expected at least one blocker alert")
		}

		// Verify error-based and dependency-based alerts.
		foundError := false
		foundDep := false
		for _, a := range result.Alerts {
			t.Logf("  alert: %s — %s (%s)", a.Task.TaskID, a.Reason, a.Detail)
			switch a.Reason {
			case blocker.ReasonHasError:
				foundError = true
				if a.Task.TaskID != "task-running-error" {
					t.Errorf("error alert for unexpected task: %s", a.Task.TaskID)
				}
			case blocker.ReasonDepBlocked:
				foundDep = true
				if a.Task.TaskID != "task-with-dep" {
					t.Errorf("dependency alert for unexpected task: %s", a.Task.TaskID)
				}
			}
		}
		if !foundError {
			t.Error("expected has_error alert for task-running-error")
		}
		if !foundDep {
			t.Error("expected dependency_blocked alert for task-with-dep")
		}

		// Verify SendAlerts sends notification.
		beforeCount := notif.count()
		_, err = radar.SendAlerts(ctx)
		if err != nil {
			t.Fatalf("SendAlerts: %v", err)
		}
		if notif.count() <= beforeCount {
			t.Error("expected notification to be sent by SendAlerts")
		}

		// Verify notification content contains alert details.
		msgs := notif.allContent()
		lastMsg := msgs[len(msgs)-1]
		if !strings.Contains(lastMsg, "Blocker Radar") {
			t.Error("notification missing 'Blocker Radar' header")
		}
		t.Logf("Blocker Radar notification sent (%d bytes)", len(lastMsg))
	})

	// Step 2: Weekly Pulse — verify digest contains seeded tasks.
	t.Run("WeeklyPulse", func(t *testing.T) {
		gen := pulse.NewGenerator(store)
		p, err := gen.Generate(ctx)
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}

		if p.TasksCompleted == 0 {
			t.Error("expected completed tasks in pulse")
		}
		if len(p.InProgress) == 0 {
			t.Error("expected in-progress tasks in pulse")
		}
		if len(p.Blocked) == 0 {
			t.Error("expected blocked tasks in pulse")
		}

		t.Logf("Pulse: completed=%d, in_progress=%d, blocked=%d, tokens=%d",
			p.TasksCompleted, len(p.InProgress), len(p.Blocked), p.TotalTokens)

		// Verify specific tasks appear in the right categories.
		foundCompleted := false
		for _, tk := range p.Completed {
			if tk.TaskID == "task-completed-1" {
				foundCompleted = true
			}
		}
		if !foundCompleted {
			t.Error("expected task-completed-1 in pulse completed list")
		}

		foundBlocked := false
		for _, tk := range p.Blocked {
			if tk.TaskID == "task-failed" || tk.TaskID == "task-waiting" {
				foundBlocked = true
			}
		}
		if !foundBlocked {
			t.Error("expected failed/waiting tasks in pulse blocked list")
		}

		// Verify markdown output.
		md := pulse.FormatMarkdown(p)
		if !strings.Contains(md, "Weekly Pulse") {
			t.Error("markdown missing 'Weekly Pulse' header")
		}
		if !strings.Contains(md, "implement user auth") {
			t.Error("markdown missing completed task description")
		}

		// Verify service sends notification.
		beforeCount := notif.count()
		svc := pulse.NewService(store, notif, "lark", "test-chat")
		if err := svc.GenerateAndSend(ctx); err != nil {
			t.Fatalf("GenerateAndSend: %v", err)
		}
		if notif.count() <= beforeCount {
			t.Error("expected notification from pulse service")
		}
		t.Logf("Weekly Pulse markdown generated (%d bytes)", len(md))
	})

	// Step 3: Milestone check-in — verify summary generated.
	// Note: ChatID left empty so the service uses global ListActive/ListByStatus
	// instead of chat-scoped ListByChat (our seeded tasks have no ChatID).
	t.Run("MilestoneCheckin", func(t *testing.T) {
		cfg := milestone.Config{
			Enabled:          true,
			LookbackDuration: 72 * time.Hour, // 3-day window to capture our seeded tasks
			Channel:          "lark",
			IncludeActive:    true,
			IncludeCompleted: true,
		}
		svc := milestone.NewService(store, notif, cfg)

		sum, err := svc.GenerateSummary(ctx)
		if err != nil {
			t.Fatalf("GenerateSummary: %v", err)
		}

		if len(sum.ActiveTasks) == 0 {
			t.Error("expected active tasks in milestone summary")
		}
		if len(sum.CompletedIn) == 0 {
			t.Error("expected completed tasks in milestone summary")
		}
		if len(sum.FailedIn) == 0 {
			t.Error("expected failed tasks in milestone summary")
		}

		t.Logf("Milestone: active=%d, completed=%d, failed=%d, tokens=%d",
			len(sum.ActiveTasks), len(sum.CompletedIn), len(sum.FailedIn), sum.TotalTokens)

		// Verify format.
		md := milestone.FormatSummary(sum)
		if !strings.Contains(md, "Milestone Check-in") {
			t.Error("markdown missing 'Milestone Check-in' header")
		}
		if !strings.Contains(md, "Active:") {
			t.Error("markdown missing 'Active:' summary line")
		}
		if !strings.Contains(md, fmt.Sprintf("Active:** %d", len(sum.ActiveTasks))) {
			t.Errorf("markdown should show %d active tasks", len(sum.ActiveTasks))
		}

		// Verify SendCheckin sends notification.
		beforeCount := notif.count()
		if err := svc.SendCheckin(ctx); err != nil {
			t.Fatalf("SendCheckin: %v", err)
		}
		if notif.count() <= beforeCount {
			t.Error("expected notification from milestone service")
		}
		t.Logf("Milestone check-in markdown generated (%d bytes)", len(md))
	})

	// Step 4: 1:1 Prep Brief — verify member brief generated.
	t.Run("PrepBrief", func(t *testing.T) {
		cfg := prepbrief.DefaultConfig()
		cfg.Channel = "lark"
		cfg.ChatID = "test-chat"
		svc := prepbrief.NewService(store, notif, cfg)

		brief, err := svc.Generate(ctx, "alice")
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}

		if brief.MemberID != "alice" {
			t.Errorf("expected member 'alice', got %q", brief.MemberID)
		}
		if len(brief.RecentWins) == 0 {
			t.Error("expected recent wins for alice")
		}
		if len(brief.OpenItems) == 0 {
			t.Error("expected open items for alice")
		}
		if len(brief.Blockers) == 0 {
			t.Error("expected blockers for alice")
		}

		t.Logf("Brief (alice): wins=%d, open=%d, blockers=%d, pending=%d",
			len(brief.RecentWins), len(brief.OpenItems), len(brief.Blockers), len(brief.Pending))

		// Verify specific categorisation.
		foundAuth := false
		for _, tk := range brief.RecentWins {
			if tk.TaskID == "task-completed-1" {
				foundAuth = true
			}
		}
		if !foundAuth {
			t.Error("expected task-completed-1 in alice's recent wins")
		}

		// Verify blockers include error and waiting-input tasks.
		blockerTaskIDs := make(map[string]bool)
		for _, bl := range brief.Blockers {
			blockerTaskIDs[bl.Task.TaskID] = true
			t.Logf("  blocker: %s — %s", bl.Task.TaskID, bl.Reason)
		}
		if !blockerTaskIDs["task-running-error"] {
			t.Error("expected task-running-error in alice's blockers (has error)")
		}
		if !blockerTaskIDs["task-waiting"] {
			t.Error("expected task-waiting in alice's blockers (waiting input)")
		}

		// Verify markdown output.
		md := prepbrief.FormatBrief(brief)
		if !strings.Contains(md, "1:1 Prep Brief") {
			t.Error("markdown missing '1:1 Prep Brief' header")
		}
		if !strings.Contains(md, "alice") {
			t.Error("markdown missing member name")
		}
		if !strings.Contains(md, "Recent Wins") {
			t.Error("markdown missing 'Recent Wins' section")
		}
		if !strings.Contains(md, "Blockers") {
			t.Error("markdown missing 'Blockers' section")
		}
		if !strings.Contains(md, "Suggested Discussion Points") {
			t.Error("markdown missing 'Suggested Discussion Points' section")
		}

		// Verify SendBrief sends notification.
		beforeCount := notif.count()
		_, err = svc.SendBrief(ctx, "alice")
		if err != nil {
			t.Fatalf("SendBrief: %v", err)
		}
		if notif.count() <= beforeCount {
			t.Error("expected notification from prepbrief service")
		}
		t.Logf("Prep Brief markdown generated (%d bytes)", len(md))

		// Verify bob gets a different brief (scoped by member).
		bobBrief, err := svc.Generate(ctx, "bob")
		if err != nil {
			t.Fatalf("Generate(bob): %v", err)
		}
		if len(bobBrief.RecentWins) != 0 {
			t.Errorf("expected 0 recent wins for bob, got %d", len(bobBrief.RecentWins))
		}
		if len(bobBrief.OpenItems) == 0 {
			t.Error("expected open items for bob (task-running-fresh)")
		}
		t.Logf("Brief (bob): wins=%d, open=%d, blockers=%d",
			len(bobBrief.RecentWins), len(bobBrief.OpenItems), len(bobBrief.Blockers))
	})

	// Final: verify all notifications were collected.
	t.Logf("Total notifications sent: %d", notif.count())
	if notif.count() < 4 {
		t.Errorf("expected at least 4 notifications (radar + pulse + milestone + brief), got %d", notif.count())
	}
}
