package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFetchRecentLogIndexAggregatesAndSorts(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}

	tsAService := time.Date(2026, 2, 7, 10, 0, 0, 0, time.Local)
	tsBLLM := time.Date(2026, 2, 7, 11, 0, 0, 0, time.Local)
	tsBLatency := time.Date(2026, 2, 7, 12, 0, 0, 0, time.Local)
	tsARequest := time.Date(2026, 2, 7, 13, 0, 0, 0, time.Local)

	writeLogFile(t, filepath.Join(logDir, serviceLogFileName), strings.Join([]string{
		tsAService.Format("2006-01-02 15:04:05") + " [INFO] [SERVICE] [HTTP] [log_id=log-a] request",
		tsBLLM.Format("2006-01-02 15:04:05") + " [INFO] [SERVICE] [HTTP] [log_id=log-b] request",
	}, "\n")+"\n")
	writeLogFile(t, filepath.Join(logDir, llmLogFileName), tsBLLM.Format("2006-01-02 15:04:05")+" [INFO] [LLM] [OpenAI] [log_id=log-b] llm\n")
	writeLogFile(t, filepath.Join(logDir, latencyLogFileName), tsBLatency.Format("2006-01-02 15:04:05")+" [INFO] [LATENCY] [HTTP] [log_id=log-b] route=/api/tasks latency_ms=20\n")
	writeLogFile(t, filepath.Join(requestDir, requestLogFileName), strings.Join([]string{
		`{"timestamp":"` + tsARequest.Format(time.RFC3339Nano) + `","request_id":"log-a:llm-1","log_id":"log-a","entry_type":"request","body_bytes":10}`,
		`{"timestamp":"` + tsARequest.Add(time.Minute).Format(time.RFC3339Nano) + `","request_id":"log-c:llm-2","entry_type":"response","body_bytes":20}`,
	}, "\n")+"\n")

	t.Setenv(logDirEnvVar, logDir)
	t.Setenv(requestLogEnvVar, requestDir)

	entries := FetchRecentLogIndex(LogIndexOptions{Limit: 10})
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	if entries[0].LogID != "log-c" {
		t.Fatalf("expected newest entry log-c, got %q", entries[0].LogID)
	}
	if entries[1].LogID != "log-a" {
		t.Fatalf("expected second entry log-a, got %q", entries[1].LogID)
	}
	if entries[2].LogID != "log-b" {
		t.Fatalf("expected third entry log-b, got %q", entries[2].LogID)
	}

	a := entries[1]
	if a.ServiceCount != 1 || a.RequestCount != 1 || a.TotalCount != 2 {
		t.Fatalf("unexpected counts for log-a: %+v", a)
	}
	if len(a.Sources) != 2 || a.Sources[0] != "requests" || a.Sources[1] != "service" {
		t.Fatalf("unexpected sources for log-a: %+v", a.Sources)
	}

	b := entries[2]
	if b.ServiceCount != 1 || b.LLMCount != 1 || b.LatencyCount != 1 || b.TotalCount != 3 {
		t.Fatalf("unexpected counts for log-b: %+v", b)
	}
}

func TestFetchRecentLogIndexLimit(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}
	writeLogFile(t, filepath.Join(logDir, serviceLogFileName), strings.Join([]string{
		"2026-02-07 10:00:00 [INFO] [SERVICE] [API] [log_id=log-1] one",
		"2026-02-07 11:00:00 [INFO] [SERVICE] [API] [log_id=log-2] two",
		"2026-02-07 12:00:00 [INFO] [SERVICE] [API] [log_id=log-3] three",
	}, "\n")+"\n")

	t.Setenv(logDirEnvVar, logDir)
	t.Setenv(requestLogEnvVar, requestDir)

	entries := FetchRecentLogIndex(LogIndexOptions{Limit: 2})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].LogID != "log-3" || entries[1].LogID != "log-2" {
		t.Fatalf("unexpected order: %+v", entries)
	}
}

func writeLogFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
