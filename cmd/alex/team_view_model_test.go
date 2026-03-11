package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/infra/teamruntime"
)

func TestBuildTeamRunView_ProducesUserFacingSummary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_team_runtime")
	now := time.Now().UTC().Truncate(time.Second)
	writeTeamRuntimeFixture(t, root, "session-view", "team-view", now)

	statuses, err := loadTeamRuntimeStatus(teamStatusOptions{
		runtimeRoot: root,
		eventsTail:  5,
	})
	if err != nil {
		t.Fatalf("loadTeamRuntimeStatus failed: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	view := statuses[0].View
	if view.SessionID != "session-view" || view.TeamID != "team-view" {
		t.Fatalf("unexpected identity: %+v", view)
	}
	if view.OverallStatus != "completed" {
		t.Fatalf("expected completed status, got %q", view.OverallStatus)
	}
	if view.FocusRoleID != "executor" {
		t.Fatalf("expected executor focus role, got %q", view.FocusRoleID)
	}
	if len(view.Roles) != 1 {
		t.Fatalf("expected 1 role view, got %d", len(view.Roles))
	}
	if view.Roles[0].ShortSummary != "Role completed" {
		t.Fatalf("expected completed summary, got %q", view.Roles[0].ShortSummary)
	}
	if len(view.RecentEvents) == 0 {
		t.Fatal("expected recent events in view")
	}
	if len(view.Artifacts) < 2 {
		t.Fatalf("expected runtime artifacts to be surfaced, got %+v", view.Artifacts)
	}
}

func TestSelectPreferredTerminalRole_PrefersBlockedOrFailedRole(t *testing.T) {
	entry := teamRuntimeStatus{
		SessionID: "session-priority",
		TeamID:    "team-priority",
		Roles: []teamruntime.RoleBinding{
			{RoleID: "planner", TmuxPane: "%1"},
			{RoleID: "reviewer", TmuxPane: "%2"},
			{RoleID: "executor", TmuxPane: "%3"},
		},
		View: teamRunView{
			Roles: []teamRoleView{
				{RoleID: "planner", Status: "completed", LastActivityAt: time.Now().Add(-2 * time.Minute)},
				{RoleID: "reviewer", Status: "running", LastActivityAt: time.Now().Add(-1 * time.Minute)},
				{RoleID: "executor", Status: "failed", LastActivityAt: time.Now()},
			},
		},
	}

	role, autoSelected, err := selectPreferredTerminalRole(entry, "")
	if err != nil {
		t.Fatalf("selectPreferredTerminalRole failed: %v", err)
	}
	if !autoSelected {
		t.Fatal("expected auto selection for multi-role team")
	}
	if role.RoleID != "executor" {
		t.Fatalf("expected failed role to be focused, got %q", role.RoleID)
	}
}

func TestRenderTerminalSnapshotView_UsesUserFacingLanguage(t *testing.T) {
	view := teamTerminalSnapshotView{
		Title:               "Live Terminal",
		RoleID:              "executor",
		SelectedAgent:       "codex",
		Status:              "running",
		Summary:             "Used shell_exec",
		LastActivityAt:      time.Date(2026, 3, 11, 11, 30, 0, 0, time.UTC),
		Mode:                "stream",
		Lines:               120,
		Content:             "step 1\nstep 2",
		OpenInteractiveHint: "alex team terminal --session-id s1 --team-id t1 --role-id executor --mode attach",
		FollowUpHint:        "alex team inject --session-id s1 --team-id t1 --role-id executor --message \"<follow-up>\"",
	}

	rendered := renderTerminalSnapshotView(view)
	for _, needle := range []string{
		"Live Terminal",
		"Role: executor",
		"Agent: codex",
		"Status: running",
		"Summary: Used shell_exec",
		"Open Interactive View:",
		"Send Follow-up:",
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("expected %q in rendered output:\n%s", needle, rendered)
		}
	}
}

func TestRenderTeamRunView_IncludesRoleCommandsAndArtifacts(t *testing.T) {
	view := teamRunView{
		Goal:          "Ship team terminal UX",
		OverallStatus: "running",
		StartedAt:     time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 3, 11, 10, 5, 0, 0, time.UTC),
		SessionID:     "session-42",
		TeamID:        "team-42",
		FocusRoleID:   "executor",
		Roles: []teamRoleView{
			{
				RoleID:            "executor",
				SelectedAgent:     "codex",
				Status:            "running",
				ShortSummary:      "Used shell_exec",
				LastActivityAt:    time.Date(2026, 3, 11, 10, 5, 0, 0, time.UTC),
				TerminalAvailable: true,
				FollowUpAvailable: true,
			},
		},
		RecentEvents: []teamActivityView{
			{Timestamp: time.Date(2026, 3, 11, 10, 4, 0, 0, time.UTC), RoleID: "executor", Summary: "Used shell_exec"},
		},
		Artifacts: []teamArtifactView{
			{Kind: "terminal_log", Label: "executor recent output", Path: "/tmp/executor.log", RoleID: "executor"},
		},
	}

	rendered := renderTeamRunView(view)
	for _, needle := range []string{
		"Goal: Ship team terminal UX",
		"Team status: running",
		"Roles:",
		"Recent Output: alex team terminal --session-id session-42 --team-id team-42 --role-id executor --mode capture --lines 120",
		"Open Interactive View: alex team terminal --session-id session-42 --team-id team-42 --role-id executor --mode attach",
		"Send Follow-up: alex team inject --session-id session-42 --team-id team-42 --role-id executor --message \"<follow-up>\"",
		"Artifacts:",
		"/tmp/executor.log",
	} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("expected %q in rendered output:\n%s", needle, rendered)
		}
	}
}
