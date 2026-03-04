package teamruntime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/infra/coding"
	"gopkg.in/yaml.v3"
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

func TestBootstrapManagerEnsure_NilManager(t *testing.T) {
	var mgr *BootstrapManager
	_, err := mgr.Ensure(context.Background(), EnsureRequest{
		SessionID: "s1",
		Template:  "team",
		Goal:      "test nil",
		RoleIDs:   []string{"worker"},
	})
	if err == nil {
		t.Fatal("expected error from nil manager, got nil")
	}
	if err.Error() != "bootstrap manager is nil" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBootstrapManagerEnsure_EmptyRoleIDs(t *testing.T) {
	baseDir := t.TempDir()
	mgr := NewBootstrapManager(baseDir, nil).WithCapabilityTTL(30 * time.Second)
	mgr.tmuxManager = nil

	result, err := mgr.Ensure(context.Background(), EnsureRequest{
		SessionID: "session-empty-roles",
		Template:  "team",
		Goal:      "no roles",
		RoleIDs:   []string{},
	})
	if err != nil {
		t.Fatalf("Ensure with empty RoleIDs failed: %v", err)
	}
	if len(result.RoleBindings) != 0 {
		t.Fatalf("expected 0 role bindings, got %d", len(result.RoleBindings))
	}
	if result.Bootstrap.TeamID == "" {
		t.Fatal("expected non-empty TeamID even with no roles")
	}
}

func TestBootstrapManagerEnsure_CapabilityTTLCacheHit(t *testing.T) {
	baseDir := t.TempDir()
	mgr := NewBootstrapManager(baseDir, nil).WithCapabilityTTL(1 * time.Hour)
	mgr.tmuxManager = nil

	// First call creates the capabilities file via live probe.
	result1, err := mgr.Ensure(context.Background(), EnsureRequest{
		SessionID: "session-cache",
		Template:  "team",
		Goal:      "cache test",
		RoleIDs:   []string{"worker"},
		Targets:   map[string]string{"worker": "nonexistent-binary"},
	})
	if err != nil {
		t.Fatalf("first Ensure failed: %v", err)
	}

	// Seed the capabilities file with known data so the second call must read
	// from cache rather than re-probing.
	capPath := filepath.Join(result1.BaseDir, "capabilities.yaml")
	seeded := CapabilitySnapshot{
		GeneratedAt: time.Now().UTC(),
		TTLSeconds:  3600,
		Capabilities: []coding.DiscoveredCLICapability{
			{ID: "cached-cli", Binary: "cached", Path: "/usr/local/bin/cached", Executable: true},
		},
	}
	data, err := yaml.Marshal(seeded)
	if err != nil {
		t.Fatalf("marshal seeded snapshot: %v", err)
	}
	if err := os.WriteFile(capPath, data, 0o644); err != nil {
		t.Fatalf("write seeded capabilities: %v", err)
	}

	// Second call should hit the TTL cache and return our seeded capability.
	result2, err := mgr.Ensure(context.Background(), EnsureRequest{
		SessionID: "session-cache",
		Template:  "team",
		Goal:      "cache test",
		RoleIDs:   []string{"worker"},
		Targets:   map[string]string{"worker": "cached"},
	})
	if err != nil {
		t.Fatalf("second Ensure failed: %v", err)
	}
	if len(result2.Capabilities) != 1 {
		t.Fatalf("expected 1 cached capability, got %d", len(result2.Capabilities))
	}
	if result2.Capabilities[0].ID != "cached-cli" {
		t.Fatalf("expected cached-cli capability, got %q", result2.Capabilities[0].ID)
	}
}
