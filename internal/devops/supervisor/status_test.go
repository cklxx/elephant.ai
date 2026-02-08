package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatusFileWriteRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	sf := NewStatusFile(path)

	status := Status{
		Timestamp:          "2026-02-08T12:00:00Z",
		Mode:               "healthy",
		Components:         map[string]ComponentStatus{
			"main": {PID: 1234, Health: "healthy", DeployedSHA: "abc123"},
			"test": {PID: 5678, Health: "healthy", DeployedSHA: "def456"},
		},
		RestartCountWindow: 2,
		Autofix: AutofixStatus{
			State:      "idle",
			RunsWindow: 0,
		},
	}

	if err := sf.Write(status); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("status file not created: %v", err)
	}

	// Read back
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if got.Mode != "healthy" {
		t.Errorf("Mode = %q, want healthy", got.Mode)
	}
	if got.RestartCountWindow != 2 {
		t.Errorf("RestartCountWindow = %d, want 2", got.RestartCountWindow)
	}
	if comp, ok := got.Components["main"]; !ok {
		t.Error("missing main component")
	} else {
		if comp.PID != 1234 {
			t.Errorf("main PID = %d, want 1234", comp.PID)
		}
		if comp.Health != "healthy" {
			t.Errorf("main health = %q, want healthy", comp.Health)
		}
	}
}

func TestStatusFileReadFlatFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	// Write flat format as supervisor.sh does
	flat := `{
  "ts_utc": "2026-02-08T06:17:38Z",
  "mode": "healthy",
  "main_pid": "50536",
  "test_pid": "76157",
  "loop_pid": "28187",
  "main_health": "healthy",
  "test_health": "healthy",
  "loop_alive": true,
  "main_sha": "abc123",
  "main_deployed_sha": "4f978d87",
  "test_sha": "def456",
  "test_deployed_sha": "1f1531ab",
  "restart_count_window": 0,
  "autofix_state": "idle",
  "autofix_runs_window": 1
}`
	os.WriteFile(path, []byte(flat), 0o644)

	sf := NewStatusFile(path)
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if got.Mode != "healthy" {
		t.Errorf("Mode = %q, want healthy", got.Mode)
	}

	// Verify components were parsed from flat fields
	if len(got.Components) != 3 {
		t.Fatalf("Components count = %d, want 3", len(got.Components))
	}

	main := got.Components["main"]
	if main.PID != 50536 {
		t.Errorf("main PID = %d, want 50536", main.PID)
	}
	if main.Health != "healthy" {
		t.Errorf("main health = %q, want healthy", main.Health)
	}
	if main.DeployedSHA != "4f978d87" {
		t.Errorf("main deployed_sha = %q, want 4f978d87", main.DeployedSHA)
	}

	test := got.Components["test"]
	if test.PID != 76157 {
		t.Errorf("test PID = %d, want 76157", test.PID)
	}
	if test.DeployedSHA != "1f1531ab" {
		t.Errorf("test deployed_sha = %q, want 1f1531ab", test.DeployedSHA)
	}

	loop := got.Components["loop"]
	if loop.PID != 28187 {
		t.Errorf("loop PID = %d, want 28187", loop.PID)
	}
	if loop.Health != "alive" {
		t.Errorf("loop health = %q, want alive", loop.Health)
	}

	// Verify autofix parsed
	if got.Autofix.State != "idle" {
		t.Errorf("autofix state = %q, want idle", got.Autofix.State)
	}
	if got.Autofix.RunsWindow != 1 {
		t.Errorf("autofix runs_window = %d, want 1", got.Autofix.RunsWindow)
	}
}

func TestStatusFileReadFlatFormatEmptyPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	// Simulate production case: test_pid is empty string but health/sha data exists
	flat := `{
  "ts_utc": "2026-02-08T08:30:10Z",
  "mode": "degraded",
  "main_pid": "31727",
  "test_pid": "",
  "loop_pid": "28187",
  "main_health": "healthy",
  "test_health": "down",
  "loop_alive": true,
  "main_deployed_sha": "6f6082514e3dad6d12325ff1d268881c72fb40e2",
  "test_deployed_sha": "1f1531abf36330b256caaf37ec145d88dfc1680d",
  "restart_count_window": 0,
  "autofix_state": "idle",
  "autofix_runs_window": 0
}`
	os.WriteFile(path, []byte(flat), 0o644)

	sf := NewStatusFile(path)
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if len(got.Components) != 3 {
		t.Fatalf("Components count = %d, want 3 (got: %v)", len(got.Components), got.Components)
	}

	// test component should be present even with empty PID
	test, ok := got.Components["test"]
	if !ok {
		t.Fatal("missing test component despite having health and sha data")
	}
	if test.PID != 0 {
		t.Errorf("test PID = %d, want 0", test.PID)
	}
	if test.Health != "down" {
		t.Errorf("test health = %q, want down", test.Health)
	}
	if test.DeployedSHA != "1f1531abf36330b256caaf37ec145d88dfc1680d" {
		t.Errorf("test deployed_sha = %q, want 1f1531abf36330b256caaf37ec145d88dfc1680d", test.DeployedSHA)
	}

	// main should still work
	main := got.Components["main"]
	if main.PID != 31727 {
		t.Errorf("main PID = %d, want 31727", main.PID)
	}

	// loop should still work
	loop := got.Components["loop"]
	if loop.Health != "alive" {
		t.Errorf("loop health = %q, want alive", loop.Health)
	}
}

func TestStatusFileWriteReadWithCycleFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	sf := NewStatusFile(path)

	status := Status{
		Timestamp:        "2026-02-08T14:00:00Z",
		Mode:             "healthy",
		Components:       map[string]ComponentStatus{},
		CyclePhase:       "fast_gate",
		CycleResult:      "running",
		LastError:        "test restart failed",
		MainSHA:          "760001fe",
		LastProcessedSHA: "6f608251",
		LastValidatedSHA: "6f608251",
	}

	if err := sf.Write(status); err != nil {
		t.Fatalf("Write error: %v", err)
	}

	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if got.CyclePhase != "fast_gate" {
		t.Errorf("CyclePhase = %q, want fast_gate", got.CyclePhase)
	}
	if got.CycleResult != "running" {
		t.Errorf("CycleResult = %q, want running", got.CycleResult)
	}
	if got.LastError != "test restart failed" {
		t.Errorf("LastError = %q, want 'test restart failed'", got.LastError)
	}
	if got.MainSHA != "760001fe" {
		t.Errorf("MainSHA = %q, want 760001fe", got.MainSHA)
	}
	if got.LastProcessedSHA != "6f608251" {
		t.Errorf("LastProcessedSHA = %q, want 6f608251", got.LastProcessedSHA)
	}
	if got.LastValidatedSHA != "6f608251" {
		t.Errorf("LastValidatedSHA = %q, want 6f608251", got.LastValidatedSHA)
	}
}

func TestStatusFileReadFlatFormatWithCycleFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")

	flat := `{
  "ts_utc": "2026-02-08T14:30:00Z",
  "mode": "healthy",
  "main_pid": "50536",
  "main_health": "healthy",
  "main_deployed_sha": "4f978d87",
  "test_pid": "76157",
  "test_health": "healthy",
  "test_deployed_sha": "1f1531ab",
  "loop_pid": "28187",
  "loop_alive": true,
  "cycle_phase": "slow_gate",
  "cycle_result": "passed",
  "last_error": "previous error msg",
  "main_sha": "760001fe",
  "last_processed_sha": "6f608251",
  "last_validated_sha": "5a5b5c5d",
  "restart_count_window": 0,
  "autofix_state": "idle",
  "autofix_runs_window": 0
}`
	os.WriteFile(path, []byte(flat), 0o644)

	sf := NewStatusFile(path)
	got, err := sf.Read()
	if err != nil {
		t.Fatalf("Read error: %v", err)
	}

	if got.CyclePhase != "slow_gate" {
		t.Errorf("CyclePhase = %q, want slow_gate", got.CyclePhase)
	}
	if got.CycleResult != "passed" {
		t.Errorf("CycleResult = %q, want passed", got.CycleResult)
	}
	if got.LastError != "previous error msg" {
		t.Errorf("LastError = %q, want 'previous error msg'", got.LastError)
	}
	if got.MainSHA != "760001fe" {
		t.Errorf("MainSHA = %q, want 760001fe", got.MainSHA)
	}
	if got.LastProcessedSHA != "6f608251" {
		t.Errorf("LastProcessedSHA = %q, want 6f608251", got.LastProcessedSHA)
	}
	if got.LastValidatedSHA != "5a5b5c5d" {
		t.Errorf("LastValidatedSHA = %q, want 5a5b5c5d", got.LastValidatedSHA)
	}

	// Ensure existing flat fields still parse correctly
	if len(got.Components) != 3 {
		t.Fatalf("Components count = %d, want 3", len(got.Components))
	}
}

func TestStatusFileAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "status.json")
	sf := NewStatusFile(path)

	// Write initial
	sf.Write(Status{Mode: "healthy"})

	// Write update
	sf.Write(Status{Mode: "degraded"})

	// No tmp file should remain
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("tmp file should not exist after atomic write")
	}

	// Read should return latest
	got, _ := sf.Read()
	if got.Mode != "degraded" {
		t.Errorf("Mode = %q, want degraded", got.Mode)
	}
}
