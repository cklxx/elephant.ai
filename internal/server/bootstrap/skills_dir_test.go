package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
)

func TestEnsureSkillsDirFromWorkspaceSkipsWhenEnvSet(t *testing.T) {
	t.Setenv(skillsDirEnvVar, "/tmp/existing")

	dir := t.TempDir()
	if ok := ensureSkillsDirFromWorkspace(dir, logging.OrNop(nil)); ok {
		t.Fatalf("expected no override when %s already set", skillsDirEnvVar)
	}
	if got, _ := runtimeconfig.DefaultEnvLookup(skillsDirEnvVar); got != "/tmp/existing" {
		t.Fatalf("expected %s to remain set, got %q", skillsDirEnvVar, got)
	}
}

func TestEnsureSkillsDirFromWorkspaceRequiresWorkspace(t *testing.T) {
	t.Setenv(skillsDirEnvVar, "")
	if ok := ensureSkillsDirFromWorkspace("", logging.OrNop(nil)); ok {
		t.Fatalf("expected no override when workspace is empty")
	}
	if got, _ := runtimeconfig.DefaultEnvLookup(skillsDirEnvVar); got != "" {
		t.Fatalf("expected %s to remain empty, got %q", skillsDirEnvVar, got)
	}
}

func TestEnsureSkillsDirFromWorkspaceSkipsMissingSkillsDir(t *testing.T) {
	t.Setenv(skillsDirEnvVar, "")
	dir := t.TempDir()
	if ok := ensureSkillsDirFromWorkspace(dir, logging.OrNop(nil)); ok {
		t.Fatalf("expected no override when skills dir is missing")
	}
	if got, _ := runtimeconfig.DefaultEnvLookup(skillsDirEnvVar); got != "" {
		t.Fatalf("expected %s to remain empty, got %q", skillsDirEnvVar, got)
	}
}

func TestEnsureSkillsDirFromWorkspaceSetsEnv(t *testing.T) {
	t.Setenv(skillsDirEnvVar, "")
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, "skills")
	if err := os.Mkdir(skillsDir, 0o755); err != nil {
		t.Fatalf("mkdir skills dir: %v", err)
	}

	if ok := ensureSkillsDirFromWorkspace(dir, logging.OrNop(nil)); !ok {
		t.Fatalf("expected skills dir to be set")
	}
	if got, _ := runtimeconfig.DefaultEnvLookup(skillsDirEnvVar); got != skillsDir {
		t.Fatalf("expected %s=%q, got %q", skillsDirEnvVar, skillsDir, got)
	}
}
