package decision

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAuditLogRecordAndGet(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	al, err := NewAuditLog(filepath.Join(t.TempDir(), "audit.json"), nowFn)
	if err != nil {
		t.Fatalf("new audit log: %v", err)
	}

	entry := AuditEntry{
		ID: "a1", PatternID: "p1", Category: "escalation",
		Action: "notify", Confidence: 0.95, AutoActed: true,
		UndoBy: now.Add(undoWindow), CreatedAt: now,
	}

	tests := []struct {
		name  string
		op    func() error
		check func(t *testing.T)
	}{
		{
			name: "record and retrieve",
			op:   func() error { return al.Record(entry) },
			check: func(t *testing.T) {
				got := al.Get("a1")
				if got == nil {
					t.Fatal("expected entry, got nil")
				}
				if got.PatternID != "p1" {
					t.Errorf("got pattern_id %q, want %q", got.PatternID, "p1")
				}
			},
		},
		{
			name: "get nonexistent returns nil",
			op:   func() error { return nil },
			check: func(t *testing.T) {
				if al.Get("nonexistent") != nil {
					t.Error("expected nil")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.op(); err != nil {
				t.Fatalf("op: %v", err)
			}
			tt.check(t)
		})
	}
}

func TestAuditLogListRecent(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	al, err := NewAuditLog("", func() time.Time { return now })
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	for _, id := range []string{"a1", "a2", "a3"} {
		_ = al.Record(AuditEntry{ID: id, AutoActed: true, UndoBy: now.Add(undoWindow), CreatedAt: now})
	}

	tests := []struct {
		name  string
		limit int
		want  int
	}{
		{name: "all", limit: 10, want: 3},
		{name: "limited", limit: 2, want: 2},
		{name: "zero", limit: 0, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := al.ListRecent(tt.limit)
			if len(got) != tt.want {
				t.Errorf("got %d, want %d", len(got), tt.want)
			}
		})
	}
}

func TestAuditLogUndo(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	currentTime := now
	nowFn := func() time.Time { return currentTime }

	al, err := NewAuditLog("", nowFn)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	_ = al.Record(AuditEntry{
		ID: "a1", AutoActed: true,
		UndoBy: now.Add(undoWindow), CreatedAt: now,
	})

	tests := []struct {
		name    string
		id      string
		advTime time.Duration
		wantErr bool
	}{
		{name: "undo within window", id: "a1", advTime: 0, wantErr: false},
		{name: "undo after window", id: "a1", advTime: 25 * time.Hour, wantErr: true},
		{name: "undo nonexistent", id: "nope", advTime: 0, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			currentTime = now.Add(tt.advTime)
			err := al.Undo(tt.id)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestAuditLogMetrics(t *testing.T) {
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	al, err := NewAuditLog("", func() time.Time { return now })
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	_ = al.Record(AuditEntry{ID: "a1", AutoActed: true, UndoBy: now.Add(undoWindow)})
	_ = al.Record(AuditEntry{ID: "a2", AutoActed: true, Corrected: true, UndoBy: now.Add(undoWindow)})
	_ = al.Record(AuditEntry{ID: "a3", AutoActed: false, UndoBy: now.Add(undoWindow)})

	m := al.Metrics()

	tests := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{name: "total", got: m.TotalDecisions, want: 3},
		{name: "auto_acted", got: m.AutoActed, want: 2},
		{name: "escalated", got: m.Escalated, want: 1},
		{name: "corrections", got: m.Corrections, want: 1},
		{name: "accuracy", got: m.AccuracyRate, want: 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestAuditLogPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.json")
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }

	al1, _ := NewAuditLog(path, nowFn)
	_ = al1.Record(AuditEntry{ID: "a1", AutoActed: true, UndoBy: now.Add(undoWindow)})

	al2, err := NewAuditLog(path, nowFn)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if al2.Get("a1") == nil {
		t.Error("entry should survive persistence reload")
	}
}
