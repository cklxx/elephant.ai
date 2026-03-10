package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"

	"gopkg.in/yaml.v3"
)

const (
	leaderUsage        = "usage: alex leader {status|dashboard|config} [options]"
	leaderStatusUsage  = "usage: alex leader status [--json] [--url <server-url>]"
	leaderDashUsage    = "usage: alex leader dashboard [--json] [--url <server-url>]"
	leaderConfigUsage  = "usage: alex leader config {show}"

	defaultDashboardURL     = "http://localhost:8080/api/leader/dashboard"
	leaderRequestTimeout    = 5 * time.Second
)

// ---------- local DTO types (mirroring server response, no import coupling) ----------

type leaderDashboardResponse struct {
	TasksByStatus  leaderTaskStatusCounts `json:"tasks_by_status"`
	RecentBlockers []leaderBlockerAlert   `json:"recent_blockers"`
	DailySummary   *leaderDailySummary    `json:"daily_summary,omitempty"`
	ScheduledJobs  []leaderScheduledJob   `json:"scheduled_jobs,omitempty"`
}

type leaderTaskStatusCounts struct {
	Pending    int `json:"pending"`
	InProgress int `json:"in_progress"`
	Blocked    int `json:"blocked"`
	Completed  int `json:"completed"`
}

type leaderBlockerAlert struct {
	TaskID      string `json:"task_id"`
	Description string `json:"description"`
	Reason      string `json:"reason"`
	Detail      string `json:"detail"`
	Status      string `json:"status"`
}

type leaderDailySummary struct {
	NewTasks       int     `json:"new_tasks"`
	Completed      int     `json:"completed"`
	InProgress     int     `json:"in_progress"`
	Blocked        int     `json:"blocked"`
	CompletionRate float64 `json:"completion_rate"`
}

type leaderScheduledJob struct {
	Name     string    `json:"name"`
	CronExpr string   `json:"cron_expr"`
	Status   string    `json:"status"`
	NextRun  time.Time `json:"next_run,omitempty"`
	LastRun  time.Time `json:"last_run,omitempty"`
}

// ---------- command dispatch ----------

func runLeaderCommand(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stdout, leaderUsage)
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "status":
		return runLeaderStatus(args[1:])
	case "dashboard", "dash":
		return runLeaderDashboard(args[1:])
	case "config":
		return runLeaderConfig(args[1:])
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown leader subcommand %q (expected: status|dashboard|config)", args[0])}
	}
}

// ---------- flag parsing helpers ----------

type leaderFlags struct {
	jsonOutput bool
	url        string
}

func parseLeaderFlags(args []string) (leaderFlags, error) {
	f := leaderFlags{url: defaultDashboardURL}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			f.jsonOutput = true
		case "--url":
			if i+1 < len(args) {
				i++
				f.url = args[i]
			} else {
				return f, fmt.Errorf("--url requires a value")
			}
		}
	}
	return f, nil
}

// ---------- status subcommand ----------

func runLeaderStatus(args []string) error {
	flags, err := parseLeaderFlags(args)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	resp, err := fetchLeaderDashboard(flags.url)
	if err != nil {
		if flags.jsonOutput {
			return printLeaderJSON(os.Stdout, &leaderDashboardResponse{})
		}
		fmt.Fprintf(os.Stdout, "Leader Agent Status\n===================\n\n")
		fmt.Fprintf(os.Stdout, "  Server at %s is not reachable.\n", flags.url)
		fmt.Fprintf(os.Stdout, "  %v\n", err)
		fmt.Fprintf(os.Stdout, "\nHint: is the server running? Try: alex dev start\n")
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("server unreachable")}
	}

	if flags.jsonOutput {
		return printLeaderJSON(os.Stdout, resp)
	}
	printLeaderStatus(os.Stdout, resp)
	return nil
}

// ---------- dashboard subcommand ----------

func runLeaderDashboard(args []string) error {
	flags, err := parseLeaderFlags(args)
	if err != nil {
		return &ExitCodeError{Code: 2, Err: err}
	}

	resp, err := fetchLeaderDashboard(flags.url)
	if err != nil {
		if flags.jsonOutput {
			return printLeaderJSON(os.Stdout, &leaderDashboardResponse{})
		}
		fmt.Fprintf(os.Stdout, "Server at %s is not reachable: %v\n", flags.url, err)
		return &ExitCodeError{Code: 1, Err: fmt.Errorf("server unreachable")}
	}

	if flags.jsonOutput {
		return printLeaderJSON(os.Stdout, resp)
	}
	printLeaderDashboard(os.Stdout, resp)
	return nil
}

// ---------- config subcommand ----------

func runLeaderConfig(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(os.Stdout, leaderConfigUsage)
		return nil
	}

	switch strings.ToLower(args[0]) {
	case "show":
		return runLeaderConfigShow(os.Stdout)
	default:
		return &ExitCodeError{Code: 2, Err: fmt.Errorf("unknown leader config subcommand %q (expected: show)", args[0])}
	}
}

func runLeaderConfigShow(w io.Writer) error {
	cfg := runtimeconfig.DefaultLeaderConfig()

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal leader config: %w", err)
	}
	fmt.Fprintf(w, "# Leader Agent Configuration (defaults)\n")
	fmt.Fprintf(w, "%s", string(data))

	if errs := cfg.Validate(); len(errs) > 0 {
		fmt.Fprintf(w, "\nWarnings:\n")
		for _, e := range errs {
			fmt.Fprintf(w, "  - %s: %s\n", e.Field, e.Message)
		}
	}
	return nil
}

// ---------- HTTP client ----------

func fetchLeaderDashboard(url string) (*leaderDashboardResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), leaderRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	client := &http.Client{Timeout: leaderRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var dr leaderDashboardResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return &dr, nil
}

// ---------- JSON output ----------

func printLeaderJSON(w io.Writer, resp *leaderDashboardResponse) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(resp)
}

// ---------- human-friendly status ----------

func printLeaderStatus(w io.Writer, resp *leaderDashboardResponse) {
	fmt.Fprintf(w, "Leader Agent Status\n===================\n\n")

	// Tasks
	t := resp.TasksByStatus
	fmt.Fprintf(w, "Tasks:\n")
	fmt.Fprintf(w, "  Pending:      %d\n", t.Pending)
	fmt.Fprintf(w, "  In Progress:  %d\n", t.InProgress)
	fmt.Fprintf(w, "  Blocked:      %d\n", t.Blocked)
	fmt.Fprintf(w, "  Completed:    %d\n", t.Completed)

	// Blockers
	fmt.Fprintf(w, "\nActive Blockers: (%d)\n", len(resp.RecentBlockers))
	for _, b := range resp.RecentBlockers {
		desc := b.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}
		fmt.Fprintf(w, "  %-10s %-18s %q — %s\n", b.TaskID, b.Reason, desc, b.Detail)
	}

	// Scheduled Jobs
	fmt.Fprintf(w, "\nScheduled Jobs: (%d)\n", len(resp.ScheduledJobs))
	for _, j := range resp.ScheduledJobs {
		if j.Status == "disabled" {
			fmt.Fprintf(w, "  %-16s %-16s %s\n", j.Name, j.CronExpr, j.Status)
		} else {
			next := ""
			if !j.NextRun.IsZero() {
				next = "next: " + j.NextRun.Format("2006-01-02 15:04")
			}
			fmt.Fprintf(w, "  %-16s %-16s %-10s %s\n", j.Name, j.CronExpr, j.Status, next)
		}
	}
}

// ---------- compact dashboard ----------

func printLeaderDashboard(w io.Writer, resp *leaderDashboardResponse) {
	// Tasks box
	t := resp.TasksByStatus
	taskLine := fmt.Sprintf("Pending: %d  In Progress: %d  Blocked: %d  Done: %d", t.Pending, t.InProgress, t.Blocked, t.Completed)
	printBox(w, "Tasks", []string{taskLine})

	// Blockers box
	blockerTitle := fmt.Sprintf("Blockers (%d)", len(resp.RecentBlockers))
	var blockerLines []string
	for _, b := range resp.RecentBlockers {
		desc := b.Description
		if len(desc) > 30 {
			desc = desc[:27] + "..."
		}
		blockerLines = append(blockerLines, fmt.Sprintf("%-10s %-16s %q", b.TaskID, b.Reason, desc))
	}
	printBox(w, blockerTitle, blockerLines)

	// Jobs box
	jobsTitle := fmt.Sprintf("Jobs (%d)", len(resp.ScheduledJobs))
	var jobLines []string
	for _, j := range resp.ScheduledJobs {
		if j.Status == "disabled" {
			jobLines = append(jobLines, fmt.Sprintf("%-16s %-16s (disabled)", j.Name, j.CronExpr))
		} else {
			next := ""
			if !j.NextRun.IsZero() {
				next = "next: " + j.NextRun.Format("15:04")
			}
			jobLines = append(jobLines, fmt.Sprintf("%-16s %-16s %s", j.Name, j.CronExpr, next))
		}
	}
	printBox(w, jobsTitle, jobLines)
}

func printBox(w io.Writer, title string, lines []string) {
	// Calculate width
	minWidth := len(title) + 6 // "┌─ " + title + " ─┐" padding
	width := minWidth
	for _, l := range lines {
		if len(l)+4 > width { // "│ " + content + " │"
			width = len(l) + 4
		}
	}
	if width < 50 {
		width = 50
	}

	// Top border
	topPad := width - len(title) - 4 // "┌─ " + title + " " + "─...─┐"
	if topPad < 1 {
		topPad = 1
	}
	fmt.Fprintf(w, "┌─ %s %s┐\n", title, strings.Repeat("─", topPad))

	// Content lines
	for _, l := range lines {
		pad := width - len(l) - 4
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(w, "│ %s%s │\n", l, strings.Repeat(" ", pad))
	}
	if len(lines) == 0 {
		pad := width - 4
		if pad < 0 {
			pad = 0
		}
		fmt.Fprintf(w, "│ %s │\n", strings.Repeat(" ", pad))
	}

	// Bottom border
	fmt.Fprintf(w, "└%s┘\n", strings.Repeat("─", width-2))
}
