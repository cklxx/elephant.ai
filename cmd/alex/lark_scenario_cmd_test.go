package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRunLarkScenarioRun_PassWritesReports(t *testing.T) {
	dir := t.TempDir()

	writeScenario(t, dir, "pass.yaml", `
name: pass
setup:
  config:
    session_prefix: "test-lark"
    allow_direct: true
  llm_mode: mock
turns:
  - sender_id: "ou_user_001"
    chat_id: "oc_chat_001"
    message_id: "om_msg_001"
    content: "hi"
    mock_response:
      answer: "ok"
    assertions:
      messenger:
        - method: ReplyMessage
          content_contains: ["ok"]
`)

	jsonOut := filepath.Join(dir, "report.json")
	mdOut := filepath.Join(dir, "report.md")

	if err := runLarkScenarioRun([]string{"--dir", dir, "--json-out", jsonOut, "--md-out", mdOut, "--name", "pass"}); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	if b, err := os.ReadFile(jsonOut); err != nil || len(b) == 0 {
		t.Fatalf("expected json report to be written, err=%v len=%d", err, len(b))
	}
	if b, err := os.ReadFile(mdOut); err != nil || len(b) == 0 {
		t.Fatalf("expected md report to be written, err=%v len=%d", err, len(b))
	}
}

func TestRunLarkScenarioRun_FailReturnsExitCode1(t *testing.T) {
	dir := t.TempDir()

	writeScenario(t, dir, "pass.yaml", `
name: pass
setup:
  config:
    session_prefix: "test-lark"
    allow_direct: true
  llm_mode: mock
turns:
  - sender_id: "ou_user_001"
    chat_id: "oc_chat_001"
    message_id: "om_msg_001"
    content: "hi"
    mock_response:
      answer: "ok"
    assertions:
      messenger:
        - method: ReplyMessage
          content_contains: ["ok"]
`)
	writeScenario(t, dir, "fail.yaml", `
name: fail
setup:
  config:
    session_prefix: "test-lark"
    allow_direct: true
  llm_mode: mock
turns:
  - sender_id: "ou_user_001"
    chat_id: "oc_chat_001"
    message_id: "om_msg_001"
    content: "hi"
    mock_response:
      answer: "ok"
    assertions:
      messenger:
        - method: ReplyMessage
          content_contains: ["missing"]
`)

	err := runLarkScenarioRun([]string{"--dir", dir})
	if err == nil {
		t.Fatalf("expected failure, got nil")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.Code)
	}
}

func writeScenario(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write scenario %s: %v", path, err)
	}
}
