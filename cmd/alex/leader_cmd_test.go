package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func sampleDashboardResponse() leaderDashboardResponse {
	return leaderDashboardResponse{
		TasksByStatus: leaderTaskStatusCounts{
			Pending:    3,
			InProgress: 2,
			Blocked:    1,
			Completed:  15,
		},
		RecentBlockers: []leaderBlockerAlert{
			{TaskID: "TASK-abc", Reason: "stale_progress", Description: "Fix login bug", Detail: "no progress for 45m"},
			{TaskID: "TASK-def", Reason: "waiting_input", Description: "Deploy staging", Detail: "waiting for input 20m"},
		},
		ScheduledJobs: []leaderScheduledJob{
			{Name: "blocker_radar", CronExpr: "*/10 * * * *", Status: "active", NextRun: time.Date(2026, 3, 10, 15, 20, 0, 0, time.UTC)},
			{Name: "daily_summary", CronExpr: "0 18 * * *", Status: "active", NextRun: time.Date(2026, 3, 10, 18, 0, 0, 0, time.UTC)},
			{Name: "weekly_pulse", CronExpr: "0 9 * * 1", Status: "disabled"},
		},
	}
}

func newMockDashboardServer(t *testing.T) *httptest.Server {
	t.Helper()
	resp := sampleDashboardResponse()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("mock server encode: %v", err)
		}
	}))
}

func TestRunLeaderCommand_Help(t *testing.T) {
	err := runLeaderCommand([]string{"help"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunLeaderCommand_NoArgs(t *testing.T) {
	err := runLeaderCommand(nil)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunLeaderCommand_UnknownSubcommand(t *testing.T) {
	err := runLeaderCommand([]string{"bogus"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	if !strings.Contains(err.Error(), "unknown leader subcommand") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLeaderStatus_MockServer(t *testing.T) {
	srv := newMockDashboardServer(t)
	defer srv.Close()

	err := runLeaderStatus([]string{"--url", srv.URL})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunLeaderStatus_JSON(t *testing.T) {
	srv := newMockDashboardServer(t)
	defer srv.Close()

	// Capture by calling fetchLeaderDashboard + printLeaderJSON directly
	resp, err := fetchLeaderDashboard(srv.URL)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	var buf bytes.Buffer
	if err := printLeaderJSON(&buf, resp); err != nil {
		t.Fatalf("printLeaderJSON: %v", err)
	}

	var decoded leaderDashboardResponse
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("json decode output: %v", err)
	}
	if decoded.TasksByStatus.Pending != 3 {
		t.Errorf("expected pending=3, got %d", decoded.TasksByStatus.Pending)
	}
	if len(decoded.RecentBlockers) != 2 {
		t.Errorf("expected 2 blockers, got %d", len(decoded.RecentBlockers))
	}
}

func TestRunLeaderStatus_Unreachable(t *testing.T) {
	err := runLeaderStatus([]string{"--url", "http://127.0.0.1:1"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "server unreachable") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLeaderDashboard_MockServer(t *testing.T) {
	srv := newMockDashboardServer(t)
	defer srv.Close()

	err := runLeaderDashboard([]string{"--url", srv.URL})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestPrintLeaderStatus(t *testing.T) {
	resp := sampleDashboardResponse()
	var buf bytes.Buffer
	printLeaderStatus(&buf, &resp)

	out := buf.String()
	if !strings.Contains(out, "Leader Agent Status") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "Pending:      3") {
		t.Error("missing pending count")
	}
	if !strings.Contains(out, "In Progress:  2") {
		t.Error("missing in_progress count")
	}
	if !strings.Contains(out, "Blocked:      1") {
		t.Error("missing blocked count")
	}
	if !strings.Contains(out, "Completed:    15") {
		t.Error("missing completed count")
	}
	if !strings.Contains(out, "TASK-abc") {
		t.Error("missing blocker task ID")
	}
	if !strings.Contains(out, "blocker_radar") {
		t.Error("missing scheduled job")
	}
	if !strings.Contains(out, "disabled") {
		t.Error("missing disabled job status")
	}
}

func TestPrintLeaderDashboard(t *testing.T) {
	resp := sampleDashboardResponse()
	var buf bytes.Buffer
	printLeaderDashboard(&buf, &resp)

	out := buf.String()
	if !strings.Contains(out, "┌─ Tasks") {
		t.Error("missing Tasks box")
	}
	if !strings.Contains(out, "Pending: 3") {
		t.Error("missing pending in dashboard")
	}
	if !strings.Contains(out, "┌─ Blockers (2)") {
		t.Error("missing Blockers box")
	}
	if !strings.Contains(out, "TASK-abc") {
		t.Error("missing blocker in dashboard")
	}
	if !strings.Contains(out, "┌─ Jobs (3)") {
		t.Error("missing Jobs box")
	}
	if !strings.Contains(out, "(disabled)") {
		t.Error("missing disabled job in dashboard")
	}
}

func TestRunLeaderConfigShow(t *testing.T) {
	var buf bytes.Buffer
	err := runLeaderConfigShow(&buf)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Leader Agent Configuration") {
		t.Error("missing header comment")
	}

	// Verify the YAML portion is valid
	// Strip the comment line
	lines := strings.Split(out, "\n")
	var yamlLines []string
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "Warnings:") && !strings.HasPrefix(line, "  -") {
			yamlLines = append(yamlLines, line)
		}
	}
	yamlContent := strings.Join(yamlLines, "\n")

	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &parsed); err != nil {
		t.Fatalf("output is not valid YAML: %v\nContent:\n%s", err, yamlContent)
	}
	if _, ok := parsed["blocker_radar"]; !ok {
		t.Error("missing blocker_radar key in YAML output")
	}
}

func TestPrintBox(t *testing.T) {
	var buf bytes.Buffer
	printBox(&buf, "Test", []string{"line one", "line two"})

	out := buf.String()
	if !strings.Contains(out, "┌─ Test") {
		t.Error("missing top border with title")
	}
	if !strings.Contains(out, "line one") {
		t.Error("missing first line")
	}
	if !strings.Contains(out, "└") {
		t.Error("missing bottom border")
	}
}

func TestPrintBox_Empty(t *testing.T) {
	var buf bytes.Buffer
	printBox(&buf, "Empty", nil)

	out := buf.String()
	if !strings.Contains(out, "┌─ Empty") {
		t.Error("missing top border")
	}
	// Should have an empty content row
	if strings.Count(out, "│") < 2 {
		t.Error("missing content row borders")
	}
}
