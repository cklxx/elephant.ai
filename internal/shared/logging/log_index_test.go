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
	// Each log_id needs 3+ lines to pass the noise filter (TotalCount > 2).
	writeLogFile(t, filepath.Join(logDir, serviceLogFileName), strings.Join([]string{
		"2026-02-07 10:00:00 [INFO] [SERVICE] [API] [log_id=log-1] one",
		"2026-02-07 10:00:01 [INFO] [SERVICE] [API] [log_id=log-1] one-b",
		"2026-02-07 10:00:02 [INFO] [SERVICE] [API] [log_id=log-1] one-c",
		"2026-02-07 11:00:00 [INFO] [SERVICE] [API] [log_id=log-2] two",
		"2026-02-07 11:00:01 [INFO] [SERVICE] [API] [log_id=log-2] two-b",
		"2026-02-07 11:00:02 [INFO] [SERVICE] [API] [log_id=log-2] two-c",
		"2026-02-07 12:00:00 [INFO] [SERVICE] [API] [log_id=log-3] three",
		"2026-02-07 12:00:01 [INFO] [SERVICE] [API] [log_id=log-3] three-b",
		"2026-02-07 12:00:02 [INFO] [SERVICE] [API] [log_id=log-3] three-c",
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

func TestFetchRecentLogIndexOffset(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}
	// Each log_id needs 3+ lines to pass the noise filter (TotalCount > 2).
	writeLogFile(t, filepath.Join(logDir, serviceLogFileName), strings.Join([]string{
		"2026-02-07 10:00:00 [INFO] [SERVICE] [API] [log_id=log-1] one",
		"2026-02-07 10:00:01 [INFO] [SERVICE] [API] [log_id=log-1] one-b",
		"2026-02-07 10:00:02 [INFO] [SERVICE] [API] [log_id=log-1] one-c",
		"2026-02-07 11:00:00 [INFO] [SERVICE] [API] [log_id=log-2] two",
		"2026-02-07 11:00:01 [INFO] [SERVICE] [API] [log_id=log-2] two-b",
		"2026-02-07 11:00:02 [INFO] [SERVICE] [API] [log_id=log-2] two-c",
		"2026-02-07 12:00:00 [INFO] [SERVICE] [API] [log_id=log-3] three",
		"2026-02-07 12:00:01 [INFO] [SERVICE] [API] [log_id=log-3] three-b",
		"2026-02-07 12:00:02 [INFO] [SERVICE] [API] [log_id=log-3] three-c",
		"2026-02-07 13:00:00 [INFO] [SERVICE] [API] [log_id=log-4] four",
		"2026-02-07 13:00:01 [INFO] [SERVICE] [API] [log_id=log-4] four-b",
		"2026-02-07 13:00:02 [INFO] [SERVICE] [API] [log_id=log-4] four-c",
		"2026-02-07 14:00:00 [INFO] [SERVICE] [API] [log_id=log-5] five",
		"2026-02-07 14:00:01 [INFO] [SERVICE] [API] [log_id=log-5] five-b",
		"2026-02-07 14:00:02 [INFO] [SERVICE] [API] [log_id=log-5] five-c",
	}, "\n")+"\n")

	t.Setenv(logDirEnvVar, logDir)
	t.Setenv(requestLogEnvVar, requestDir)

	// Sorted desc: log-5, log-4, log-3, log-2, log-1

	// Page 1: offset=0, limit=2 → log-5, log-4
	page1 := FetchRecentLogIndex(LogIndexOptions{Limit: 2, Offset: 0})
	if len(page1) != 2 {
		t.Fatalf("page1: expected 2 entries, got %d", len(page1))
	}
	if page1[0].LogID != "log-5" || page1[1].LogID != "log-4" {
		t.Fatalf("page1: unexpected IDs: %s, %s", page1[0].LogID, page1[1].LogID)
	}

	// Page 2: offset=2, limit=2 → log-3, log-2
	page2 := FetchRecentLogIndex(LogIndexOptions{Limit: 2, Offset: 2})
	if len(page2) != 2 {
		t.Fatalf("page2: expected 2 entries, got %d", len(page2))
	}
	if page2[0].LogID != "log-3" || page2[1].LogID != "log-2" {
		t.Fatalf("page2: unexpected IDs: %s, %s", page2[0].LogID, page2[1].LogID)
	}

	// Page 3: offset=4, limit=2 → log-1
	page3 := FetchRecentLogIndex(LogIndexOptions{Limit: 2, Offset: 4})
	if len(page3) != 1 {
		t.Fatalf("page3: expected 1 entry, got %d", len(page3))
	}
	if page3[0].LogID != "log-1" {
		t.Fatalf("page3: unexpected ID: %s", page3[0].LogID)
	}

	// Page 4: offset=5, limit=2 → empty
	page4 := FetchRecentLogIndex(LogIndexOptions{Limit: 2, Offset: 5})
	if len(page4) != 0 {
		t.Fatalf("page4: expected 0 entries, got %d", len(page4))
	}

	// Negative offset treated as 0
	negOffset := FetchRecentLogIndex(LogIndexOptions{Limit: 2, Offset: -1})
	if len(negOffset) != 2 {
		t.Fatalf("neg offset: expected 2 entries, got %d", len(negOffset))
	}
	if negOffset[0].LogID != "log-5" {
		t.Fatalf("neg offset: expected log-5, got %s", negOffset[0].LogID)
	}
}

func TestFetchRecentLogIndexFiltersNoise(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}
	// log-noise has only 2 service lines and no LLM/request activity → noise, should be filtered.
	// log-real has 3 service lines → passes noise filter.
	// log-llm has 1 service line + 1 LLM line → RequestCount/LLMCount > 0, passes.
	writeLogFile(t, filepath.Join(logDir, serviceLogFileName), strings.Join([]string{
		"2026-02-07 10:00:00 [INFO] [SERVICE] [API] [log_id=log-noise] GET /api/dev/logs/index",
		"2026-02-07 10:00:01 [INFO] [SERVICE] [API] [log_id=log-noise] done",
		"2026-02-07 11:00:00 [INFO] [SERVICE] [API] [log_id=log-real] request",
		"2026-02-07 11:00:01 [INFO] [SERVICE] [API] [log_id=log-real] processing",
		"2026-02-07 11:00:02 [INFO] [SERVICE] [API] [log_id=log-real] done",
		"2026-02-07 12:00:00 [INFO] [SERVICE] [API] [log_id=log-llm] request",
	}, "\n")+"\n")
	writeLogFile(t, filepath.Join(logDir, llmLogFileName),
		"2026-02-07 12:00:01 [INFO] [LLM] [OpenAI] [log_id=log-llm] completion\n")

	t.Setenv(logDirEnvVar, logDir)
	t.Setenv(requestLogEnvVar, requestDir)

	entries := FetchRecentLogIndex(LogIndexOptions{Limit: 10})
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (noise filtered), got %d", len(entries))
	}

	ids := map[string]bool{}
	for _, e := range entries {
		ids[e.LogID] = true
	}
	if ids["log-noise"] {
		t.Fatal("expected log-noise to be filtered out")
	}
	if !ids["log-real"] {
		t.Fatal("expected log-real to be present")
	}
	if !ids["log-llm"] {
		t.Fatal("expected log-llm to be present")
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
