package process

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExecBackend_Attached_Pipes(t *testing.T) {
	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:    "test-echo",
		Command: "echo",
		Args:    []string{"hello world"},
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(h.Stdout())
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(out)); got != "hello world" {
		t.Fatalf("expected 'hello world', got %q", got)
	}

	if err := h.Wait(); err != nil {
		t.Fatal(err)
	}
	if h.PID() == 0 {
		t.Fatal("expected non-zero PID")
	}
}

func TestExecBackend_Attached_StderrTail(t *testing.T) {
	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:    "test-stderr",
		Command: "sh",
		Args:    []string{"-c", "echo stderr-output >&2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Wait()

	// Give stderr goroutine a moment to flush.
	time.Sleep(50 * time.Millisecond)

	tail := h.StderrTail()
	if !strings.Contains(tail, "stderr-output") {
		t.Fatalf("expected stderr tail to contain 'stderr-output', got %q", tail)
	}
}

func TestExecBackend_Attached_Stop(t *testing.T) {
	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:    "test-stop",
		Command: "sleep",
		Args:    []string{"60"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !h.Alive() {
		t.Fatal("expected alive")
	}

	if err := h.Stop(); err != nil {
		t.Fatal(err)
	}

	// Should be done now.
	select {
	case <-h.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("process did not exit after Stop")
	}
}

func TestExecBackend_Attached_Stdin(t *testing.T) {
	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:    "test-stdin",
		Command: "cat",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = h.Stdin().Write([]byte("input data\n"))
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Stdin().Close()

	out, _ := io.ReadAll(h.Stdout())
	if got := strings.TrimSpace(string(out)); got != "input data" {
		t.Fatalf("expected 'input data', got %q", got)
	}
	_ = h.Wait()
}

func TestExecBackend_Detached_OutputFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")
	statusFile := filepath.Join(dir, "status.json")

	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:       "test-detached",
		Command:    "echo",
		Args:       []string{"detached output"},
		Detached:   true,
		OutputFile: outFile,
		StatusFile: statusFile,
	})
	if err != nil {
		t.Fatal(err)
	}

	_ = h.Wait()

	// stdout should be nil in detached mode.
	if h.Stdout() != nil {
		t.Fatal("expected nil stdout in detached mode")
	}

	// Output should be in file.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "detached output") {
		t.Fatalf("expected output file to contain 'detached output', got %q", string(data))
	}

	// Status file should exist with PID.
	statusData, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(statusData), `"pid":`) {
		t.Fatalf("expected status file to contain PID, got %q", string(statusData))
	}
}

func TestExecBackend_Detached_RequiresOutputFile(t *testing.T) {
	backend := &ExecBackend{}
	_, err := backend.Start(context.Background(), ProcessConfig{
		Name:     "test-detached-err",
		Command:  "echo",
		Detached: true,
	})
	if err == nil {
		t.Fatal("expected error for detached without OutputFile")
	}
}

func TestExecBackend_Name(t *testing.T) {
	backend := &ExecBackend{}
	h, err := backend.Start(context.Background(), ProcessConfig{
		Name:    "my-process",
		Command: "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	_ = h.Wait()

	if h.Name() != "my-process" {
		t.Fatalf("expected 'my-process', got %q", h.Name())
	}
}
