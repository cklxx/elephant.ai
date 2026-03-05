package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
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

	if err := runLarkScenarioRun([]string{"--mode", "mock", "--dir", dir, "--json-out", jsonOut, "--md-out", mdOut, "--name", "pass"}); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	if b, err := os.ReadFile(jsonOut); err != nil || len(b) == 0 {
		t.Fatalf("expected json report to be written, err=%v len=%d", err, len(b))
	}
	if b, err := os.ReadFile(mdOut); err != nil || len(b) == 0 {
		t.Fatalf("expected md report to be written, err=%v len=%d", err, len(b))
	}
}

func TestRunLarkCommand_RejectsTeamSubcommand(t *testing.T) {
	err := runLarkCommand([]string{"team"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var exitErr *ExitCodeError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected ExitCodeError, got %T: %v", err, err)
	}
	if exitErr.Code != 2 {
		t.Fatalf("expected exit code 2, got %d", exitErr.Code)
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

	err := runLarkScenarioRun([]string{"--mode", "mock", "--dir", dir})
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

func TestRunLarkScenarioRun_SkipsManualByDefault(t *testing.T) {
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
	writeScenario(t, dir, "manual_fail.yaml", `
name: manual_fail
tags: ["manual"]
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

	jsonOut := filepath.Join(dir, "report.json")
	if err := runLarkScenarioRun([]string{"--mode", "mock", "--dir", dir, "--json-out", jsonOut}); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	b, err := os.ReadFile(jsonOut)
	if err != nil {
		t.Fatalf("read json report: %v", err)
	}
	var report struct {
		Summary struct {
			Total   int `json:"total"`
			Passed  int `json:"passed"`
			Failed  int `json:"failed"`
			Skipped int `json:"skipped"`
		} `json:"summary"`
		Scenarios []struct {
			Name string `json:"name"`
		} `json:"scenarios"`
	}
	if err := json.Unmarshal(b, &report); err != nil {
		t.Fatalf("unmarshal json report: %v", err)
	}
	if report.Summary.Total != 1 || report.Summary.Passed != 1 || report.Summary.Failed != 0 || report.Summary.Skipped != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Scenarios) != 1 || report.Scenarios[0].Name != "pass" {
		t.Fatalf("unexpected scenarios: %+v", report.Scenarios)
	}
}

func TestRunLarkScenarioRun_RunsManualWhenNamed(t *testing.T) {
	dir := t.TempDir()

	writeScenario(t, dir, "manual_fail.yaml", `
name: manual_fail
tags: ["manual"]
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

	err := runLarkScenarioRun([]string{"--mode", "mock", "--dir", dir, "--name", "manual_fail"})
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

func TestRunLarkScenarioRun_HTTPPassWritesReports(t *testing.T) {
	dir := t.TempDir()

	writeScenario(t, dir, "http_pass.yaml", `
name: http_pass
tags: ["http"]
turns:
  - sender_id: "ou_http_user"
    chat_id: "oc_http_chat"
    content: "/reset"
    assertions:
      messenger:
        - method: ReplyMessage
          content_contains: ["/new"]
`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dev/inject" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&map[string]any{})
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(larkInjectResponse{
			Replies: []larkInjectReply{
				{Method: "ReplyMessage", Content: `{"text":"请使用 /new 开始新会话"}`, MsgType: "text"},
			},
			DurationMs: 23,
		})
	}))
	defer server.Close()

	jsonOut := filepath.Join(dir, "report.json")
	mdOut := filepath.Join(dir, "report.md")

	if err := runLarkScenarioRun([]string{"--mode", "http", "--dir", dir, "--base-url", server.URL, "--json-out", jsonOut, "--md-out", mdOut}); err != nil {
		t.Fatalf("expected success, got err: %v", err)
	}

	if b, err := os.ReadFile(jsonOut); err != nil || len(b) == 0 {
		t.Fatalf("expected json report to be written, err=%v len=%d", err, len(b))
	}
	if b, err := os.ReadFile(mdOut); err != nil || len(b) == 0 {
		t.Fatalf("expected md report to be written, err=%v len=%d", err, len(b))
	}
}

func TestRunLarkScenarioRun_HTTPRejectsExecutorAssertions(t *testing.T) {
	dir := t.TempDir()

	writeScenario(t, dir, "http_fail_executor.yaml", `
name: http_fail_executor
tags: ["http"]
turns:
  - sender_id: "ou_http_user"
    chat_id: "oc_http_chat"
    content: "hello"
    assertions:
      executor:
        called: false
`)

	var requests int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(larkInjectResponse{
			Replies: []larkInjectReply{{Method: "ReplyMessage", Content: `{"text":"ok"}`}},
		})
	}))
	defer server.Close()

	jsonOut := filepath.Join(dir, "report.json")
	err := runLarkScenarioRun([]string{"--mode", "http", "--dir", dir, "--base-url", server.URL, "--json-out", jsonOut})
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
	if got := atomic.LoadInt32(&requests); got != 0 {
		t.Fatalf("expected no HTTP requests for unsupported executor assertions, got %d", got)
	}

	b, readErr := os.ReadFile(jsonOut)
	if readErr != nil {
		t.Fatalf("read json report: %v", readErr)
	}
	if !strings.Contains(string(b), "executor assertions are not supported in --mode http") {
		t.Fatalf("expected unsupported executor assertion message in report, got %s", string(b))
	}
}

func writeScenario(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write scenario %s: %v", path, err)
	}
}
