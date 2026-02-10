package subprocess

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSubprocess_Detached_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")
	statusFile := filepath.Join(dir, "status.json")

	s := New(Config{
		Command:    "echo",
		Args:       []string{"hello detached"},
		Detached:   true,
		OutputFile: outFile,
		StatusFile: statusFile,
	})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := s.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}

	// Check output file.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(data), "hello detached") {
		t.Errorf("output = %q, want to contain 'hello detached'", string(data))
	}

	// Check status file.
	statusData, err := os.ReadFile(statusFile)
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(string(statusData), `"pid":`) {
		t.Errorf("status = %q, want to contain pid", string(statusData))
	}
}

func TestSubprocess_Detached_RequiresOutputFile(t *testing.T) {
	s := New(Config{
		Command:  "echo",
		Args:     []string{"test"},
		Detached: true,
		// No OutputFile.
	})

	err := s.Start(context.Background())
	if err == nil {
		t.Fatal("expected error for missing OutputFile")
	}
	if !strings.Contains(err.Error(), "OutputFile") {
		t.Errorf("error = %q, want to contain OutputFile", err.Error())
	}
}

func TestSubprocess_Detached_StdoutIsNil(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")

	s := New(Config{
		Command:    "echo",
		Args:       []string{"test"},
		Detached:   true,
		OutputFile: outFile,
	})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Stop()

	if s.Stdout() != nil {
		t.Error("expected Stdout() to be nil in detached mode")
	}
}

func TestSubprocess_Detached_PID(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")

	s := New(Config{
		Command:    "sleep",
		Args:       []string{"5"},
		Detached:   true,
		OutputFile: outFile,
	})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer s.Stop()

	pid := s.PID()
	if pid == 0 {
		t.Error("expected non-zero PID")
	}
}

func TestSubprocess_Detached_Stop(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")

	s := New(Config{
		Command:    "sleep",
		Args:       []string{"60"},
		Detached:   true,
		OutputFile: outFile,
	})

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Stop should terminate the process.
	if err := s.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Wait should return quickly.
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()

	select {
	case <-done:
		// OK.
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for process to exit after Stop")
	}
}

func TestSubprocess_Detached_Done(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "output.txt")

	s := New(Config{
		Command:    "echo",
		Args:       []string{"quick"},
		Detached:   true,
		OutputFile: outFile,
	})

	// Before start, Done is nil.
	if s.Done() != nil {
		t.Error("Done() should be nil before Start")
	}

	if err := s.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}

	doneCh := s.Done()
	if doneCh == nil {
		t.Fatal("Done() should not be nil after Start")
	}

	select {
	case <-doneCh:
		// OK — process finished.
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Done channel")
	}
}
