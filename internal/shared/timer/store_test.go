package timer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStoreSaveAndGet(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tmr := Timer{
		ID:        "tmr-test1",
		Name:      "test timer",
		Type:      TimerTypeOnce,
		Delay:     "5m",
		FireAt:    time.Now().Add(5 * time.Minute).UTC().Truncate(time.Second),
		Task:      "check the weather",
		SessionID: "session-abc",
		UserID:    "user-1",
		Channel:   "lark",
		ChatID:    "oc_123",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		Status:    StatusActive,
	}

	if err := store.Save(tmr); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// File should exist on disk.
	if _, err := os.Stat(filepath.Join(dir, "tmr-test1.yaml")); err != nil {
		t.Fatalf("expected file on disk: %v", err)
	}

	got, err := store.Get("tmr-test1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != tmr.ID {
		t.Errorf("ID: got %q, want %q", got.ID, tmr.ID)
	}
	if got.Name != tmr.Name {
		t.Errorf("Name: got %q, want %q", got.Name, tmr.Name)
	}
	if got.Task != tmr.Task {
		t.Errorf("Task: got %q, want %q", got.Task, tmr.Task)
	}
	if got.SessionID != tmr.SessionID {
		t.Errorf("SessionID: got %q, want %q", got.SessionID, tmr.SessionID)
	}
	if got.Status != StatusActive {
		t.Errorf("Status: got %q, want %q", got.Status, StatusActive)
	}
	if !got.FireAt.Equal(tmr.FireAt) {
		t.Errorf("FireAt: got %v, want %v", got.FireAt, tmr.FireAt)
	}
}

func TestStoreGetNotFound(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	_, err = store.Get("tmr-nonexistent")
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist, got %v", err)
	}
}

func TestStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tmr := Timer{
		ID:        "tmr-del",
		Name:      "deletable",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(time.Hour).UTC(),
		Task:      "do stuff",
		CreatedAt: time.Now().UTC(),
		Status:    StatusActive,
	}
	if err := store.Save(tmr); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete("tmr-del"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err = store.Get("tmr-del")
	if !os.IsNotExist(err) {
		t.Errorf("expected os.ErrNotExist after delete, got %v", err)
	}
}

func TestStoreDeleteNonexistent(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	// Deleting a non-existent timer should not error.
	if err := store.Delete("tmr-ghost"); err != nil {
		t.Errorf("Delete non-existent: %v", err)
	}
}

func TestStoreLoadAll(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)

	timers := []Timer{
		{ID: "tmr-a", Name: "alpha", Type: TimerTypeOnce, FireAt: now.Add(time.Hour), Task: "a", CreatedAt: now, Status: StatusActive},
		{ID: "tmr-b", Name: "beta", Type: TimerTypeRecurring, Schedule: "0 9 * * *", Task: "b", CreatedAt: now, Status: StatusActive},
		{ID: "tmr-c", Name: "gamma", Type: TimerTypeOnce, FireAt: now.Add(2 * time.Hour), Task: "c", CreatedAt: now, Status: StatusFired},
	}

	for _, tmr := range timers {
		if err := store.Save(tmr); err != nil {
			t.Fatalf("Save %s: %v", tmr.ID, err)
		}
	}

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("LoadAll: got %d timers, want 3", len(loaded))
	}

	ids := make(map[string]bool)
	for _, tmr := range loaded {
		ids[tmr.ID] = true
	}
	for _, id := range []string{"tmr-a", "tmr-b", "tmr-c"} {
		if !ids[id] {
			t.Errorf("LoadAll: missing timer %s", id)
		}
	}
}

func TestStoreLoadAllEmpty(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	loaded, err := store.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("LoadAll empty: got %d, want 0", len(loaded))
	}
}

func TestStoreSaveOverwrite(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	tmr := Timer{
		ID:        "tmr-ow",
		Name:      "original",
		Type:      TimerTypeOnce,
		FireAt:    time.Now().Add(time.Hour).UTC().Truncate(time.Second),
		Task:      "original task",
		CreatedAt: time.Now().UTC().Truncate(time.Second),
		Status:    StatusActive,
	}
	if err := store.Save(tmr); err != nil {
		t.Fatalf("Save: %v", err)
	}

	tmr.Status = StatusFired
	tmr.Name = "updated"
	if err := store.Save(tmr); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}

	got, err := store.Get("tmr-ow")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusFired {
		t.Errorf("Status: got %q, want %q", got.Status, StatusFired)
	}
	if got.Name != "updated" {
		t.Errorf("Name: got %q, want %q", got.Name, "updated")
	}
}

func TestTimerValidation(t *testing.T) {
	tests := []struct {
		name    string
		timer   Timer
		wantErr bool
	}{
		{
			name:    "empty ID",
			timer:   Timer{Name: "test", Task: "do", Type: TimerTypeOnce, FireAt: time.Now().Add(time.Hour)},
			wantErr: true,
		},
		{
			name:    "empty name",
			timer:   Timer{ID: "tmr-1", Task: "do", Type: TimerTypeOnce, FireAt: time.Now().Add(time.Hour)},
			wantErr: true,
		},
		{
			name:    "empty task",
			timer:   Timer{ID: "tmr-1", Name: "test", Type: TimerTypeOnce, FireAt: time.Now().Add(time.Hour)},
			wantErr: true,
		},
		{
			name:    "once without fire_at",
			timer:   Timer{ID: "tmr-1", Name: "test", Task: "do", Type: TimerTypeOnce},
			wantErr: true,
		},
		{
			name:    "recurring without schedule",
			timer:   Timer{ID: "tmr-1", Name: "test", Task: "do", Type: TimerTypeRecurring},
			wantErr: true,
		},
		{
			name:    "invalid type",
			timer:   Timer{ID: "tmr-1", Name: "test", Task: "do", Type: "bogus"},
			wantErr: true,
		},
		{
			name:    "valid once",
			timer:   Timer{ID: "tmr-1", Name: "test", Task: "do", Type: TimerTypeOnce, FireAt: time.Now().Add(time.Hour)},
			wantErr: false,
		},
		{
			name:    "valid recurring",
			timer:   Timer{ID: "tmr-1", Name: "test", Task: "do", Type: TimerTypeRecurring, Schedule: "0 9 * * *"},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.timer.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate: got error=%v, wantErr=%v", err, tc.wantErr)
			}
		})
	}
}

func TestNewTimerID(t *testing.T) {
	id1 := NewTimerID()
	id2 := NewTimerID()
	if id1 == id2 {
		t.Error("NewTimerID should generate unique IDs")
	}
	if len(id1) < 5 || id1[:4] != "tmr-" {
		t.Errorf("NewTimerID: unexpected format: %q", id1)
	}
}
