package bridge

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOutputReader_ReadsExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.jsonl")
	doneFile := filepath.Join(dir, ".done")

	// Write events then done sentinel.
	content := `{"type":"tool","tool_name":"Bash","summary":"command=ls","files":[],"iter":1}
{"type":"result","answer":"done","tokens":100,"cost":0.01,"iters":1,"is_error":false}
`
	if err := os.WriteFile(outFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	// Write done sentinel.
	if err := os.WriteFile(doneFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	reader := NewOutputReader(outFile, doneFile)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var events []SDKEvent
	for ev := range reader.Read(ctx) {
		events = append(events, ev)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != SDKEventTool {
		t.Errorf("event[0].Type = %q, want tool", events[0].Type)
	}
	if events[0].ToolName != "Bash" {
		t.Errorf("event[0].ToolName = %q, want Bash", events[0].ToolName)
	}
	if events[1].Type != SDKEventResult {
		t.Errorf("event[1].Type = %q, want result", events[1].Type)
	}
	if events[1].Answer != "done" {
		t.Errorf("event[1].Answer = %q, want done", events[1].Answer)
	}
}

func TestOutputReader_TailsGrowingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.jsonl")
	doneFile := filepath.Join(dir, ".done")

	// Create empty file.
	f, err := os.Create(outFile)
	if err != nil {
		t.Fatal(err)
	}

	reader := NewOutputReader(outFile, doneFile)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events := reader.Read(ctx)

	// Write events after a delay.
	go func() {
		time.Sleep(300 * time.Millisecond)
		_, _ = f.WriteString(`{"type":"tool","tool_name":"Write","summary":"file_path=/a.go","files":["/a.go"],"iter":1}` + "\n")
		_ = f.Sync()

		time.Sleep(300 * time.Millisecond)
		_, _ = f.WriteString(`{"type":"result","answer":"ok","tokens":50,"cost":0,"iters":1,"is_error":false}` + "\n")
		_ = f.Sync()
		f.Close()

		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(doneFile, nil, 0o644)
	}()

	var received []SDKEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}
	if received[0].ToolName != "Write" {
		t.Errorf("event[0].ToolName = %q, want Write", received[0].ToolName)
	}
	if received[1].Answer != "ok" {
		t.Errorf("event[1].Answer = %q, want ok", received[1].Answer)
	}
}

func TestOutputReader_ResumeFromOffset(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.jsonl")
	doneFile := filepath.Join(dir, ".done")

	line1 := `{"type":"tool","tool_name":"Bash","summary":"command=ls","files":[],"iter":1}` + "\n"
	line2 := `{"type":"result","answer":"done","tokens":100,"cost":0.01,"iters":1,"is_error":false}` + "\n"

	if err := os.WriteFile(outFile, []byte(line1+line2), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doneFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Skip first line by setting offset.
	reader := NewOutputReader(outFile, doneFile)
	reader.SetOffset(int64(len(line1)))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var events []SDKEvent
	for ev := range reader.Read(ctx) {
		events = append(events, ev)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event (skipped first), got %d", len(events))
	}
	if events[0].Type != SDKEventResult {
		t.Errorf("expected result event, got %q", events[0].Type)
	}
}

func TestOutputReader_CancellationStopsReader(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.jsonl")

	// Create file but no done sentinel — reader should block.
	if err := os.WriteFile(outFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	reader := NewOutputReader(outFile, filepath.Join(dir, ".done"))
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	var events []SDKEvent
	for ev := range reader.Read(ctx) {
		events = append(events, ev)
	}

	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

func TestOutputReader_WaitsForFileCreation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.jsonl")
	doneFile := filepath.Join(dir, ".done")

	reader := NewOutputReader(outFile, doneFile)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	events := reader.Read(ctx)

	// Create file after a delay.
	go func() {
		time.Sleep(300 * time.Millisecond)
		content := `{"type":"result","answer":"created late","tokens":10,"cost":0,"iters":1,"is_error":false}` + "\n"
		_ = os.WriteFile(outFile, []byte(content), 0o644)
		time.Sleep(100 * time.Millisecond)
		_ = os.WriteFile(doneFile, nil, 0o644)
	}()

	var received []SDKEvent
	for ev := range events {
		received = append(received, ev)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Answer != "created late" {
		t.Errorf("Answer = %q, want 'created late'", received[0].Answer)
	}
}

func TestBridgeOutputPaths(t *testing.T) {
	t.Parallel()

	workDir := "/tmp/workspace"
	taskID := "task-123"

	dir := bridgeOutputDir(workDir, taskID)
	if dir != "/tmp/workspace/.elephant/bridge/task-123" {
		t.Errorf("bridgeOutputDir = %q", dir)
	}

	out := bridgeOutputFile(workDir, taskID)
	if out != "/tmp/workspace/.elephant/bridge/task-123/output.jsonl" {
		t.Errorf("bridgeOutputFile = %q", out)
	}

	status := bridgeStatusFile(workDir, taskID)
	if status != "/tmp/workspace/.elephant/bridge/task-123/status.json" {
		t.Errorf("bridgeStatusFile = %q", status)
	}

	done := bridgeDoneFile(workDir, taskID)
	if done != "/tmp/workspace/.elephant/bridge/task-123/.done" {
		t.Errorf("bridgeDoneFile = %q", done)
	}
}
