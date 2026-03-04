package teamruntime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBootstrapManagerEnsure_WritesArtifacts(t *testing.T) {
	baseDir := t.TempDir()
	mgr := NewBootstrapManager(baseDir, nil).WithCapabilityTTL(30 * time.Second)
	// Avoid invoking tmux in unit test.
	mgr.tmuxManager = nil

	result, err := mgr.Ensure(context.Background(), EnsureRequest{
		SessionID: "session-test",
		Template:  "research_team",
		Goal:      "improve execution",
		RoleIDs:   []string{"planner", "executor"},
		Profiles: map[string]string{
			"planner":  "planning",
			"executor": "execution",
		},
		Targets: map[string]string{
			"planner":  "claude",
			"executor": "codex",
		},
	})
	if err != nil {
		t.Fatalf("Ensure failed: %v", err)
	}

	paths := []string{
		filepath.Join(result.BaseDir, "bootstrap.yaml"),
		filepath.Join(result.BaseDir, "capabilities.yaml"),
		filepath.Join(result.BaseDir, "role_registry.yaml"),
		filepath.Join(result.BaseDir, "runtime_state.yaml"),
		result.EventLogPath,
	}
	for _, p := range paths {
		if _, statErr := os.Stat(p); statErr != nil {
			t.Fatalf("expected artifact file %s: %v", p, statErr)
		}
	}
	if result.Bootstrap.TeamID == "" || result.Bootstrap.SessionID == "" {
		t.Fatalf("unexpected bootstrap metadata: %+v", result.Bootstrap)
	}
	if len(result.RoleBindings) != 2 {
		t.Fatalf("expected 2 role bindings, got %d", len(result.RoleBindings))
	}
}
