package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newTestJob(id, name string) Job {
	return Job{
		ID:       id,
		Name:     name,
		CronExpr: "0 9 * * 1",
		Trigger:  "daily_briefing",
		Payload:  json.RawMessage(`{"key":"value"}`),
		Status:   JobStatusPending,
	}
}

func mustSave(t *testing.T, store JobStore, job Job) {
	t.Helper()
	if err := store.Save(context.Background(), job); err != nil {
		t.Fatalf("Save(%s): %v", job.ID, err)
	}
}

// ---------------------------------------------------------------------------
// Job model tests
// ---------------------------------------------------------------------------

func TestJobStatus_IsValid(t *testing.T) {
	tests := []struct {
		status JobStatus
		valid  bool
	}{
		{JobStatusPending, true},
		{JobStatusActive, true},
		{JobStatusPaused, true},
		{JobStatusCompleted, true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := tt.status.IsValid(); got != tt.valid {
			t.Errorf("JobStatus(%q).IsValid() = %v, want %v", tt.status, got, tt.valid)
		}
	}
}

func TestJob_Validate(t *testing.T) {
	tests := []struct {
		name    string
		job     Job
		wantErr bool
	}{
		{
			name:    "valid job",
			job:     newTestJob("j1", "Job 1"),
			wantErr: false,
		},
		{
			name:    "missing id",
			job:     Job{Name: "x", CronExpr: "* * * * *", Status: JobStatusPending},
			wantErr: true,
		},
		{
			name:    "missing name",
			job:     Job{ID: "j1", CronExpr: "* * * * *", Status: JobStatusPending},
			wantErr: true,
		},
		{
			name:    "missing cron_expr",
			job:     Job{ID: "j1", Name: "x", Status: JobStatusPending},
			wantErr: true,
		},
		{
			name:    "invalid status",
			job:     Job{ID: "j1", Name: "x", CronExpr: "* * * * *", Status: "bad"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FileJobStore — CRUD
// ---------------------------------------------------------------------------

func TestFileJobStore_SaveAndLoad(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	job := newTestJob("save-load", "Save Load Test")
	mustSave(t, store, job)

	loaded, err := store.Load(ctx, "save-load")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID != "save-load" {
		t.Errorf("ID = %q, want save-load", loaded.ID)
	}
	if loaded.Name != "Save Load Test" {
		t.Errorf("Name = %q, want 'Save Load Test'", loaded.Name)
	}
	if loaded.Status != JobStatusPending {
		t.Errorf("Status = %q, want pending", loaded.Status)
	}
	if loaded.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set automatically")
	}
	if loaded.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set automatically")
	}
	// Compare payload semantically (MarshalIndent may reformat whitespace)
	var origPayload, loadedPayload map[string]any
	if err := json.Unmarshal([]byte(`{"key":"value"}`), &origPayload); err != nil {
		t.Fatalf("Unmarshal orig payload: %v", err)
	}
	if err := json.Unmarshal(loaded.Payload, &loadedPayload); err != nil {
		t.Fatalf("Unmarshal loaded payload: %v", err)
	}
	if fmt.Sprint(origPayload) != fmt.Sprint(loadedPayload) {
		t.Errorf("Payload mismatch: got %s", loaded.Payload)
	}
}

func TestFileJobStore_SaveOverwrite(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	job := newTestJob("overwrite", "Original")
	mustSave(t, store, job)

	first, _ := store.Load(ctx, "overwrite")
	origCreated := first.CreatedAt

	// Wait a tiny bit so UpdatedAt differs
	time.Sleep(5 * time.Millisecond)

	job.Name = "Updated"
	mustSave(t, store, job)

	loaded, err := store.Load(ctx, "overwrite")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Name != "Updated" {
		t.Errorf("Name = %q, want Updated", loaded.Name)
	}
	// CreatedAt should be preserved from the first save.
	if !loaded.CreatedAt.Equal(origCreated) {
		t.Errorf("CreatedAt changed on overwrite: got %v, want %v", loaded.CreatedAt, origCreated)
	}
	// UpdatedAt should advance.
	if !loaded.UpdatedAt.After(origCreated) {
		t.Error("UpdatedAt should be later than original CreatedAt")
	}
}

func TestFileJobStore_SaveValidation(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	// Missing ID should fail validation
	err := store.Save(ctx, Job{Name: "x", CronExpr: "* * * * *", Status: JobStatusPending})
	if err == nil {
		t.Error("expected validation error for missing ID")
	}
}

func TestFileJobStore_LoadNotFound(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got: %v", err)
	}
}

func TestFileJobStore_LoadEmptyID(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	_, err := store.Load(ctx, "")
	if err == nil {
		t.Error("expected error for empty job ID")
	}
}

func TestFileJobStore_List(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	// List on empty store
	jobs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List (empty): %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}

	// Add several jobs with small gaps so CreatedAt ordering is deterministic
	mustSave(t, store, newTestJob("job-a", "A"))
	time.Sleep(2 * time.Millisecond)
	mustSave(t, store, newTestJob("job-b", "B"))
	time.Sleep(2 * time.Millisecond)
	mustSave(t, store, newTestJob("job-c", "C"))

	jobs, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 3 {
		t.Fatalf("expected 3 jobs, got %d", len(jobs))
	}

	// Verify sorted by CreatedAt ascending
	for i := 1; i < len(jobs); i++ {
		if jobs[i].CreatedAt.Before(jobs[i-1].CreatedAt) {
			t.Errorf("jobs not sorted by CreatedAt: [%d]=%v > [%d]=%v",
				i-1, jobs[i-1].CreatedAt, i, jobs[i].CreatedAt)
		}
	}
}

func TestFileJobStore_ListNonExistentDir(t *testing.T) {
	store := NewFileJobStore(filepath.Join(t.TempDir(), "does-not-exist"))
	ctx := context.Background()

	jobs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List on nonexistent dir should return nil error, got: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestFileJobStore_ListSkipsNonJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileJobStore(dir)
	ctx := context.Background()

	mustSave(t, store, newTestJob("real-job", "Real"))

	// Write a non-JSON file into the directory
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not a job"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write a subdirectory
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	jobs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestFileJobStore_Delete(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	mustSave(t, store, newTestJob("del-me", "Delete Me"))

	if err := store.Delete(ctx, "del-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Verify gone
	_, err := store.Load(ctx, "del-me")
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound after delete, got: %v", err)
	}
}

func TestFileJobStore_DeleteNotFound(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	err := store.Delete(ctx, "no-such-job")
	if err == nil {
		t.Fatal("expected error deleting nonexistent job")
	}
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got: %v", err)
	}
}

func TestFileJobStore_DeleteEmptyID(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	err := store.Delete(ctx, "")
	if err == nil {
		t.Error("expected error for empty job ID")
	}
}

func TestFileJobStore_UpdateStatus(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	mustSave(t, store, newTestJob("status-test", "Status"))

	// Transition pending -> active
	if err := store.UpdateStatus(ctx, "status-test", JobStatusActive); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, err := store.Load(ctx, "status-test")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Status != JobStatusActive {
		t.Errorf("Status = %q, want active", loaded.Status)
	}

	// Transition active -> paused
	if err := store.UpdateStatus(ctx, "status-test", JobStatusPaused); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	loaded, _ = store.Load(ctx, "status-test")
	if loaded.Status != JobStatusPaused {
		t.Errorf("Status = %q, want paused", loaded.Status)
	}

	// Transition paused -> completed
	if err := store.UpdateStatus(ctx, "status-test", JobStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	loaded, _ = store.Load(ctx, "status-test")
	if loaded.Status != JobStatusCompleted {
		t.Errorf("Status = %q, want completed", loaded.Status)
	}
}

func TestFileJobStore_UpdateStatusNotFound(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	err := store.UpdateStatus(ctx, "ghost", JobStatusActive)
	if err == nil {
		t.Fatal("expected error for nonexistent job")
	}
	if !errors.Is(err, ErrJobNotFound) {
		t.Errorf("expected ErrJobNotFound, got: %v", err)
	}
}

func TestFileJobStore_UpdateStatusInvalid(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	mustSave(t, store, newTestJob("invalid-status", "Test"))

	err := store.UpdateStatus(ctx, "invalid-status", "bogus")
	if err == nil {
		t.Error("expected error for invalid status")
	}
}

func TestFileJobStore_UpdateStatusEmptyID(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	err := store.UpdateStatus(ctx, "", JobStatusActive)
	if err == nil {
		t.Error("expected error for empty job ID")
	}
}

func TestFileJobStore_UpdateStatusPreservesFields(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	job := newTestJob("preserve", "Preserve Fields")
	job.Trigger = "okr_review"
	job.Payload = json.RawMessage(`{"goal_id":"q1"}`)
	mustSave(t, store, job)

	if err := store.UpdateStatus(ctx, "preserve", JobStatusActive); err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}

	loaded, _ := store.Load(ctx, "preserve")
	if loaded.Trigger != "okr_review" {
		t.Errorf("Trigger = %q, want okr_review", loaded.Trigger)
	}
	var origP, loadedP map[string]any
	if err := json.Unmarshal([]byte(`{"goal_id":"q1"}`), &origP); err != nil {
		t.Fatalf("Unmarshal orig payload: %v", err)
	}
	if err := json.Unmarshal(loaded.Payload, &loadedP); err != nil {
		t.Fatalf("Unmarshal loaded payload: %v", err)
	}
	if fmt.Sprint(origP) != fmt.Sprint(loadedP) {
		t.Errorf("Payload changed after UpdateStatus: got %s", loaded.Payload)
	}
	if loaded.Name != "Preserve Fields" {
		t.Errorf("Name = %q, want 'Preserve Fields'", loaded.Name)
	}
}

// ---------------------------------------------------------------------------
// FileJobStore — directory auto-creation
// ---------------------------------------------------------------------------

func TestFileJobStore_SaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "jobs")
	store := NewFileJobStore(dir)

	mustSave(t, store, newTestJob("auto-dir", "Auto Dir"))

	// Verify the directory and file exist
	info, err := os.Stat(filepath.Join(dir, "auto-dir.json"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if info.IsDir() {
		t.Error("expected a file, not a directory")
	}
}

// ---------------------------------------------------------------------------
// FileJobStore — concurrent access
// ---------------------------------------------------------------------------

func TestFileJobStore_ConcurrentSaves(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			job := newTestJob(
				fmt.Sprintf("concurrent-%d", idx),
				fmt.Sprintf("Job %d", idx),
			)
			if err := store.Save(ctx, job); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent Save error: %v", err)
	}

	jobs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(jobs) != n {
		t.Errorf("expected %d jobs, got %d", n, len(jobs))
	}
}

func TestFileJobStore_ConcurrentReadWrite(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	// Seed one job
	mustSave(t, store, newTestJob("rw-target", "Target"))

	const n = 30
	var wg sync.WaitGroup
	errs := make(chan error, n*3)

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.Load(ctx, "rw-target")
			if err != nil {
				errs <- err
			}
		}()
	}

	// Concurrent status updates
	statuses := []JobStatus{JobStatusActive, JobStatusPaused, JobStatusPending}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := statuses[idx%len(statuses)]
			if err := store.UpdateStatus(ctx, "rw-target", s); err != nil {
				errs <- err
			}
		}(i)
	}

	// Concurrent lists
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.List(ctx)
			if err != nil {
				errs <- err
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent read/write error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// FileJobStore — interface compliance
// ---------------------------------------------------------------------------

// Compile-time check that FileJobStore implements JobStore.
var _ JobStore = (*FileJobStore)(nil)

// ---------------------------------------------------------------------------
// FileJobStore — JSON round-trip fidelity
// ---------------------------------------------------------------------------

func TestFileJobStore_JSONRoundTrip(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	now := time.Now().UTC().Truncate(time.Millisecond)
	job := Job{
		ID:        "roundtrip",
		Name:      "Round Trip",
		CronExpr:  "30 8 * * 1-5",
		Trigger:   "email_digest",
		Payload:   json.RawMessage(`{"recipients":["a@b.com"],"format":"html"}`),
		Status:    JobStatusActive,
		LastRun:   now.Add(-1 * time.Hour),
		NextRun:   now.Add(23 * time.Hour),
		CreatedAt: now,
	}

	mustSave(t, store, job)

	loaded, err := store.Load(ctx, "roundtrip")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Trigger != "email_digest" {
		t.Errorf("Trigger = %q, want email_digest", loaded.Trigger)
	}
	if loaded.CronExpr != "30 8 * * 1-5" {
		t.Errorf("CronExpr = %q, want '30 8 * * 1-5'", loaded.CronExpr)
	}

	// Payload must survive round-trip exactly
	var orig, got map[string]any
	if err := json.Unmarshal(job.Payload, &orig); err != nil {
		t.Fatalf("unmarshal payload orig: %v", err)
	}
	if err := json.Unmarshal(loaded.Payload, &got); err != nil {
		t.Fatalf("unmarshal payload loaded: %v", err)
	}
	if len(orig) != len(got) {
		t.Errorf("Payload mismatch: orig=%v got=%v", orig, got)
	}

	// Time fields should survive (within tolerance for JSON marshalling)
	if loaded.LastRun.Sub(job.LastRun).Abs() > time.Millisecond {
		t.Errorf("LastRun drift: %v vs %v", loaded.LastRun, job.LastRun)
	}
	if loaded.NextRun.Sub(job.NextRun).Abs() > time.Millisecond {
		t.Errorf("NextRun drift: %v vs %v", loaded.NextRun, job.NextRun)
	}
}

// ---------------------------------------------------------------------------
// Edge cases: corrupt file, nil payload
// ---------------------------------------------------------------------------

func TestFileJobStore_CorruptFileSkippedInList(t *testing.T) {
	dir := t.TempDir()
	store := NewFileJobStore(dir)
	ctx := context.Background()

	// Save a valid job
	mustSave(t, store, newTestJob("good-job", "Good"))

	// Write a corrupt JSON file
	if err := os.WriteFile(filepath.Join(dir, "bad-job.json"), []byte("{{{invalid"), 0o644); err != nil {
		t.Fatal(err)
	}

	jobs, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// Should only return the valid job
	if len(jobs) != 1 {
		t.Errorf("expected 1 job (corrupt skipped), got %d", len(jobs))
	}
	if jobs[0].ID != "good-job" {
		t.Errorf("expected good-job, got %s", jobs[0].ID)
	}
}

func TestFileJobStore_NilPayload(t *testing.T) {
	store := NewFileJobStore(t.TempDir())
	ctx := context.Background()

	job := Job{
		ID:       "no-payload",
		Name:     "No Payload",
		CronExpr: "0 0 * * *",
		Trigger:  "cleanup",
		Status:   JobStatusPending,
	}
	mustSave(t, store, job)

	loaded, err := store.Load(ctx, "no-payload")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Payload != nil {
		t.Errorf("expected nil Payload, got %s", loaded.Payload)
	}
}
