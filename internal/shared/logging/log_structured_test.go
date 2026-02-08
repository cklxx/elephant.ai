package logging

import (
	"encoding/json"
	"testing"
)

func TestParseTextLogLineWithLogID(t *testing.T) {
	line := `2026-02-08 01:11:57 [INFO] [SERVICE] [Main] [log_id=log-abc123] lark.go:196 - Received message from user`
	entry := parseTextLogLine(line)

	if entry.Raw != line {
		t.Fatalf("raw mismatch: got %q", entry.Raw)
	}
	if entry.Timestamp != "2026-02-08 01:11:57" {
		t.Fatalf("timestamp mismatch: got %q", entry.Timestamp)
	}
	if entry.Level != "INFO" {
		t.Fatalf("level mismatch: got %q", entry.Level)
	}
	if entry.Category != "SERVICE" {
		t.Fatalf("category mismatch: got %q", entry.Category)
	}
	if entry.Component != "Main" {
		t.Fatalf("component mismatch: got %q", entry.Component)
	}
	if entry.LogID != "log-abc123" {
		t.Fatalf("log_id mismatch: got %q", entry.LogID)
	}
	if entry.SourceFile != "lark.go" {
		t.Fatalf("source_file mismatch: got %q", entry.SourceFile)
	}
	if entry.SourceLine != 196 {
		t.Fatalf("source_line mismatch: got %d", entry.SourceLine)
	}
	if entry.Message != "Received message from user" {
		t.Fatalf("message mismatch: got %q", entry.Message)
	}
}

func TestParseTextLogLineWithoutLogID(t *testing.T) {
	line := `2026-02-08 01:11:57 [WARN] [LLM] [Agent] handler.go:42 - Token limit exceeded`
	entry := parseTextLogLine(line)

	if entry.Timestamp != "2026-02-08 01:11:57" {
		t.Fatalf("timestamp mismatch: got %q", entry.Timestamp)
	}
	if entry.Level != "WARN" {
		t.Fatalf("level mismatch: got %q", entry.Level)
	}
	if entry.Category != "LLM" {
		t.Fatalf("category mismatch: got %q", entry.Category)
	}
	if entry.Component != "Agent" {
		t.Fatalf("component mismatch: got %q", entry.Component)
	}
	if entry.LogID != "" {
		t.Fatalf("expected empty log_id, got %q", entry.LogID)
	}
	if entry.SourceFile != "handler.go" {
		t.Fatalf("source_file mismatch: got %q", entry.SourceFile)
	}
	if entry.SourceLine != 42 {
		t.Fatalf("source_line mismatch: got %d", entry.SourceLine)
	}
	if entry.Message != "Token limit exceeded" {
		t.Fatalf("message mismatch: got %q", entry.Message)
	}
}

func TestParseTextLogLineUnparseable(t *testing.T) {
	line := "some random unstructured log line"
	entry := parseTextLogLine(line)

	if entry.Raw != line {
		t.Fatalf("raw mismatch: got %q", entry.Raw)
	}
	if entry.Timestamp != "" {
		t.Fatalf("expected empty timestamp, got %q", entry.Timestamp)
	}
	if entry.Level != "" {
		t.Fatalf("expected empty level, got %q", entry.Level)
	}
	if entry.Message != line {
		t.Fatalf("message should equal raw for unparseable lines: got %q", entry.Message)
	}
}

func TestParseTextLogLineErrorLevel(t *testing.T) {
	line := `2026-02-08 14:30:00 [ERROR] [SERVICE] [Gateway] [log_id=log-err-001] gateway.go:88 - Connection refused: dial tcp 127.0.0.1:5432`
	entry := parseTextLogLine(line)

	if entry.Level != "ERROR" {
		t.Fatalf("level mismatch: got %q", entry.Level)
	}
	if entry.LogID != "log-err-001" {
		t.Fatalf("log_id mismatch: got %q", entry.LogID)
	}
	if entry.Component != "Gateway" {
		t.Fatalf("component mismatch: got %q", entry.Component)
	}
}

func TestParseTextLogLineDebugLevel(t *testing.T) {
	line := `2026-02-08 09:00:00 [DEBUG] [LATENCY] [HTTP] server.go:15 - GET /api/health 2ms`
	entry := parseTextLogLine(line)

	if entry.Level != "DEBUG" {
		t.Fatalf("level mismatch: got %q", entry.Level)
	}
	if entry.Category != "LATENCY" {
		t.Fatalf("category mismatch: got %q", entry.Category)
	}
}

func TestParseRequestLogJSON(t *testing.T) {
	raw := `{"timestamp":"2026-02-08T01:11:57Z","request_id":"log-abc123:llm-1","log_id":"log-abc123","entry_type":"request","body_bytes":1024,"payload":{"model":"gpt-4","messages":[{"role":"user","content":"hello"}]}}`

	entry, ok := parseRequestLogJSON(raw)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if entry.Raw != raw {
		t.Fatalf("raw mismatch")
	}
	if entry.Timestamp != "2026-02-08T01:11:57Z" {
		t.Fatalf("timestamp mismatch: got %q", entry.Timestamp)
	}
	if entry.RequestID != "log-abc123:llm-1" {
		t.Fatalf("request_id mismatch: got %q", entry.RequestID)
	}
	if entry.LogID != "log-abc123" {
		t.Fatalf("log_id mismatch: got %q", entry.LogID)
	}
	if entry.EntryType != "request" {
		t.Fatalf("entry_type mismatch: got %q", entry.EntryType)
	}
	if entry.BodyBytes != 1024 {
		t.Fatalf("body_bytes mismatch: got %d", entry.BodyBytes)
	}
	if entry.Payload == nil {
		t.Fatal("expected non-nil payload")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(entry.Payload, &payload); err != nil {
		t.Fatalf("payload unmarshal failed: %v", err)
	}
	if payload["model"] != "gpt-4" {
		t.Fatalf("payload model mismatch: got %v", payload["model"])
	}
}

func TestParseRequestLogJSONDeriveLogID(t *testing.T) {
	raw := `{"timestamp":"2026-02-08T01:11:57Z","request_id":"log-derived-001:llm-2","entry_type":"response","body_bytes":512,"payload":null}`

	entry, ok := parseRequestLogJSON(raw)
	if !ok {
		t.Fatal("expected successful parse")
	}
	if entry.LogID != "log-derived-001" {
		t.Fatalf("expected derived log_id, got %q", entry.LogID)
	}
	if entry.Payload != nil {
		t.Fatalf("expected nil payload for null, got %v", entry.Payload)
	}
}

func TestParseRequestLogJSONInvalid(t *testing.T) {
	_, ok := parseRequestLogJSON("not valid json")
	if ok {
		t.Fatal("expected parse failure for invalid JSON")
	}

	_, ok = parseRequestLogJSON("")
	if ok {
		t.Fatal("expected parse failure for empty string")
	}

	_, ok = parseRequestLogJSON("   ")
	if ok {
		t.Fatal("expected parse failure for whitespace")
	}
}
