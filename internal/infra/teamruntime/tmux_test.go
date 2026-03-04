package teamruntime

import (
	"testing"
)

func TestTmuxTeamSessionName(t *testing.T) {
	tests := []struct {
		name   string
		teamID string
		want   string
	}{
		{
			name:   "simple ID",
			teamID: "team-abc",
			want:   "elephant-team-team-abc",
		},
		{
			name:   "dots replaced",
			teamID: "v1.2.3",
			want:   "elephant-team-v1-2-3",
		},
		{
			name:   "colons replaced",
			teamID: "host:port",
			want:   "elephant-team-host-port",
		},
		{
			name:   "slashes replaced",
			teamID: "a/b/c",
			want:   "elephant-team-a-b-c",
		},
		{
			name:   "spaces replaced",
			teamID: "my team",
			want:   "elephant-team-my-team",
		},
		{
			name:   "mixed special chars",
			teamID: "team.v1:run/01 x",
			want:   "elephant-team-team-v1-run-01-x",
		},
		{
			name:   "empty string defaults to unknown",
			teamID: "",
			want:   "elephant-team-unknown",
		},
		{
			name:   "whitespace only defaults to unknown",
			teamID: "   ",
			want:   "elephant-team-unknown",
		},
		{
			name:   "already safe ID",
			teamID: "safe-id-123",
			want:   "elephant-team-safe-id-123",
		},
		{
			name:   "leading/trailing whitespace trimmed",
			teamID: "  team-1  ",
			want:   "elephant-team-team-1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tmuxTeamSessionName(tt.teamID)
			if got != tt.want {
				t.Errorf("tmuxTeamSessionName(%q) = %q, want %q", tt.teamID, got, tt.want)
			}
		})
	}
}

func TestPaneBootstrapCommand(t *testing.T) {
	tests := []struct {
		name    string
		roleID  string
		teamID  string
		binding RoleBinding
		want    string
	}{
		{
			name:   "no log path - export only",
			roleID: "planner",
			teamID: "team-01",
			binding: RoleBinding{
				SelectedCLI:       "claude",
				CapabilityProfile: "planning",
			},
			want: "export ROLE_ID=planner TEAM_ID=team-01 TARGET_CLI=claude CAP_PROFILE=planning",
		},
		{
			name:   "with log path - export plus tail",
			roleID: "executor",
			teamID: "team-02",
			binding: RoleBinding{
				SelectedCLI:       "codex",
				CapabilityProfile: "execution",
				RoleLogPath:       "/tmp/logs/executor.log",
			},
			want: "export ROLE_ID=executor TEAM_ID=team-02 TARGET_CLI=codex CAP_PROFILE=execution; mkdir -p '/tmp/logs'; touch '/tmp/logs/executor.log'; tail -n +1 -f '/tmp/logs/executor.log'",
		},
		{
			name:   "empty values use quoted empty strings",
			roleID: "",
			teamID: "",
			binding: RoleBinding{
				SelectedCLI:       "",
				CapabilityProfile: "",
			},
			want: `export ROLE_ID="" TEAM_ID="" TARGET_CLI="" CAP_PROFILE=""`,
		},
		{
			name:   "spaces in values are replaced with underscores",
			roleID: "my role",
			teamID: "my team",
			binding: RoleBinding{
				SelectedCLI:       "my cli",
				CapabilityProfile: "my profile",
			},
			want: "export ROLE_ID=my_role TEAM_ID=my_team TARGET_CLI=my_cli CAP_PROFILE=my_profile",
		},
		{
			name:   "whitespace-only log path treated as no log path",
			roleID: "worker",
			teamID: "t1",
			binding: RoleBinding{
				SelectedCLI:       "codex",
				CapabilityProfile: "execution",
				RoleLogPath:       "   ",
			},
			want: "export ROLE_ID=worker TEAM_ID=t1 TARGET_CLI=codex CAP_PROFILE=execution",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := paneBootstrapCommand(tt.roleID, tt.teamID, tt.binding)
			if got != tt.want {
				t.Errorf("paneBootstrapCommand() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestShellSafeEnv(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "simple", raw: "codex", want: "codex"},
		{name: "spaces to underscores", raw: "my cli", want: "my_cli"},
		{name: "empty", raw: "", want: `""`},
		{name: "whitespace only", raw: "   ", want: `""`},
		{name: "leading trailing", raw: " hello ", want: "hello"},
		{name: "multiple spaces", raw: "a b c", want: "a_b_c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellSafeEnv(tt.raw)
			if got != tt.want {
				t.Errorf("shellSafeEnv(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestShellSafePath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "simple path", raw: "/tmp/file.log", want: "'/tmp/file.log'"},
		{name: "empty", raw: "", want: "''"},
		{name: "path with spaces", raw: "/tmp/my dir/file.log", want: "'/tmp/my dir/file.log'"},
		{name: "path with single quotes", raw: "/tmp/it's/here", want: "'/tmp/it'\\''s/here'"},
		{name: "multiple single quotes", raw: "a'b'c", want: "'a'\\''b'\\''c'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellSafePath(tt.raw)
			if got != tt.want {
				t.Errorf("shellSafePath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestShellSafePathDir(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "regular path", raw: "/tmp/logs/file.log", want: "'/tmp/logs'"},
		{name: "nested path", raw: "/a/b/c/d.txt", want: "'/a/b/c'"},
		{name: "no slash", raw: "file.log", want: "'file.log'"},
		{name: "root file", raw: "/file.log", want: "'/file.log'"},
		{name: "empty", raw: "", want: "'.'"},
		{name: "whitespace only", raw: "   ", want: "'.'"},
		{name: "dir with single quote", raw: "/tmp/it's/dir/file.log", want: "'/tmp/it'\\''s/dir'"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shellSafePathDir(tt.raw)
			if got != tt.want {
				t.Errorf("shellSafePathDir(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestAssignRolePanes(t *testing.T) {
	tests := []struct {
		name     string
		bindings map[string]RoleBinding
		panes    []string
		want     map[string]string
	}{
		{
			name: "exact match",
			bindings: map[string]RoleBinding{
				"alpha": {},
				"beta":  {},
			},
			panes: []string{"%0", "%1"},
			want:  map[string]string{"alpha": "%0", "beta": "%1"},
		},
		{
			name: "more roles than panes",
			bindings: map[string]RoleBinding{
				"a": {},
				"b": {},
				"c": {},
			},
			panes: []string{"%0", "%1"},
			want:  map[string]string{"a": "%0", "b": "%1"},
		},
		{
			name: "more panes than roles",
			bindings: map[string]RoleBinding{
				"x": {},
			},
			panes: []string{"%0", "%1", "%2"},
			want:  map[string]string{"x": "%0"},
		},
		{
			name:     "no bindings",
			bindings: map[string]RoleBinding{},
			panes:    []string{"%0"},
			want:     map[string]string{},
		},
		{
			name: "no panes",
			bindings: map[string]RoleBinding{
				"a": {},
			},
			panes: []string{},
			want:  map[string]string{},
		},
		{
			name:     "both empty",
			bindings: map[string]RoleBinding{},
			panes:    []string{},
			want:     map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := assignRolePanes(tt.bindings, tt.panes)
			if len(got) != len(tt.want) {
				t.Fatalf("assignRolePanes() returned %d entries, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("assignRolePanes()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestSortedRoleIDs(t *testing.T) {
	tests := []struct {
		name     string
		bindings map[string]RoleBinding
		want     []string
	}{
		{
			name: "sorted output",
			bindings: map[string]RoleBinding{
				"zebra":    {},
				"alpha":    {},
				"middle":   {},
				"beta":     {},
			},
			want: []string{"alpha", "beta", "middle", "zebra"},
		},
		{
			name:     "empty map",
			bindings: map[string]RoleBinding{},
			want:     []string{},
		},
		{
			name: "single entry",
			bindings: map[string]RoleBinding{
				"solo": {},
			},
			want: []string{"solo"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedRoleIDs(tt.bindings)
			if len(got) != len(tt.want) {
				t.Fatalf("sortedRoleIDs() returned %d items, want %d", len(got), len(tt.want))
			}
			for i, v := range tt.want {
				if got[i] != v {
					t.Errorf("sortedRoleIDs()[%d] = %q, want %q", i, got[i], v)
				}
			}
		})
	}
}

func TestAssignRolePanes_DeterministicOrder(t *testing.T) {
	bindings := map[string]RoleBinding{
		"charlie": {},
		"alpha":   {},
		"bravo":   {},
	}
	panes := []string{"%0", "%1", "%2"}

	// Run multiple times to confirm deterministic assignment via sorted keys.
	for i := 0; i < 10; i++ {
		got := assignRolePanes(bindings, panes)
		if got["alpha"] != "%0" || got["bravo"] != "%1" || got["charlie"] != "%2" {
			t.Fatalf("iteration %d: non-deterministic assignment: %v", i, got)
		}
	}
}
