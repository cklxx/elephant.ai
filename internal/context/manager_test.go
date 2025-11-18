package context

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestSelectWorldPrefersExplicitKey(t *testing.T) {
	worlds := map[string]ports.WorldProfile{
		"prod":    {ID: "prod", Environment: "production"},
		"staging": {ID: "staging", Environment: "staging"},
	}
	session := &ports.Session{Metadata: map[string]string{"world": "staging"}}

	world := selectWorld("prod", session, worlds)
	if world.ID != "prod" {
		t.Fatalf("expected explicit world key to win, got %q", world.ID)
	}

	world = selectWorld("", session, worlds)
	if world.ID != "staging" {
		t.Fatalf("expected session metadata world, got %q", world.ID)
	}

	world = selectWorld("", &ports.Session{}, map[string]ports.WorldProfile{})
	if world.ID != "default" {
		t.Fatalf("expected default world fallback, got %q", world.ID)
	}
}

func TestBuildWindowIncludesWorldProfile(t *testing.T) {
	root := buildStaticContextTree(t)
	mgr := NewManager(WithConfigRoot(root))

	session := &ports.Session{ID: "sess-1", Metadata: map[string]string{"world": "fallback"}}
	window, err := mgr.BuildWindow(context.Background(), session, ports.ContextWindowConfig{WorldKey: "prod"})
	if err != nil {
		t.Fatalf("BuildWindow returned error: %v", err)
	}

	if window.Static.World.ID != "prod" {
		t.Fatalf("expected prod world, got %q", window.Static.World.ID)
	}
	if window.Static.World.Environment != "production" {
		t.Fatalf("unexpected environment: %q", window.Static.World.Environment)
	}
	if len(window.Static.World.Capabilities) != 2 {
		t.Fatalf("expected 2 capabilities, got %d", len(window.Static.World.Capabilities))
	}
}

func TestCompressInjectsStructuredSummary(t *testing.T) {
	mgr := &manager{}
	messages := []ports.Message{{
		Role:    "system",
		Source:  ports.MessageSourceSystemPrompt,
		Content: "base system",
	}}
	for i := 0; i < 12; i++ {
		messages = append(messages, ports.Message{Role: "user", Content: fmt.Sprintf("Need help with feature %d", i)})
		messages = append(messages, ports.Message{Role: "assistant", Content: fmt.Sprintf("Working on feature %d", i)})
	}
	target := mgr.EstimateTokens(messages) - 1
	compressed, err := mgr.Compress(messages, target)
	if err != nil {
		t.Fatalf("compress returned error: %v", err)
	}
	if len(compressed) != 12 {
		t.Fatalf("expected 12 messages (head + summary + last 10), got %d", len(compressed))
	}
	summary := compressed[1]
	if summary.Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected summary to be marked as system prompt, got %v", summary.Source)
	}
	if summary.Role != "system" {
		t.Fatalf("expected summary role system, got %s", summary.Role)
	}
	if !strings.Contains(summary.Content, "Earlier conversation had") {
		t.Fatalf("expected structured summary content, got %q", summary.Content)
	}
	if strings.Contains(summary.Content, "Previous conversation compressed") {
		t.Fatalf("legacy placeholder should be removed, got %q", summary.Content)
	}
}

func buildStaticContextTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeContextFile(t, root, "personas", "default.yaml", `id: default
tone: balanced
risk_profile: moderate
decision_style: deliberate
voice: neutral`)
	writeContextFile(t, root, "goals", "default.yaml", `id: default
long_term:
  - Deliver value`)
	writeContextFile(t, root, "policies", "default.yaml", `id: default
hard_constraints:
  - Always follow company policies`)
	writeContextFile(t, root, "knowledge", "default.yaml", `id: default
description: base knowledge`)
	writeContextFile(t, root, "worlds", "prod.yaml", `id: prod
environment: production
capabilities:
  - deploy
  - monitor
limits:
  - No destructive actions without approval
cost_model:
  - Standard token budget`)
	return root
}

func writeContextFile(t *testing.T, root, subdir, name, body string) {
	t.Helper()
	dir := filepath.Join(root, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
