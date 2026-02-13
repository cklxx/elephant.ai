package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrepareEvalRuntimeEnvironmentUsesWorkspaceDirFromConfig(t *testing.T) {
	repoRoot, datasetPath := makeEvalRepoFixture(t)
	configPath := filepath.Join(t.TempDir(), "alex-config.yaml")
	writeEvalTestFile(t, configPath, "channels:\n  lark:\n    workspace_dir: "+repoRoot+"\n")

	t.Setenv("ALEX_CONFIG_PATH", configPath)
	t.Setenv("ALEX_CONTEXT_CONFIG_DIR", "")
	t.Setenv("ALEX_DOTENV_PATH", "")
	unsetEnv(t, "OPENAI_API_KEY")
	t.Chdir(t.TempDir())

	resolved, err := prepareEvalRuntimeEnvironment("evaluation/swe_bench/real_instances.json")
	if err != nil {
		t.Fatalf("prepareEvalRuntimeEnvironment returned error: %v", err)
	}

	wantDataset := filepath.Join(repoRoot, "evaluation", "swe_bench", "real_instances.json")
	if resolved != wantDataset {
		t.Fatalf("resolved dataset = %q, want %q", resolved, wantDataset)
	}

	if got := mustLookupEnv(t, "ALEX_CONTEXT_CONFIG_DIR"); got != filepath.Join(repoRoot, "configs", "context") {
		t.Fatalf("ALEX_CONTEXT_CONFIG_DIR = %q, want %q", got, filepath.Join(repoRoot, "configs", "context"))
	}
	if got := mustLookupEnv(t, "OPENAI_API_KEY"); got != "sk-live-e2e" {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, "sk-live-e2e")
	}

	if datasetPath == "" {
		t.Fatal("datasetPath should not be empty")
	}
}

func TestPrepareEvalRuntimeEnvironmentPreservesExistingContextConfigDir(t *testing.T) {
	_, datasetPath := makeEvalRepoFixture(t)

	t.Setenv("ALEX_CONFIG_PATH", "")
	t.Setenv("ALEX_DOTENV_PATH", "")
	t.Setenv("ALEX_CONTEXT_CONFIG_DIR", "/tmp/custom-context")
	unsetEnv(t, "OPENAI_API_KEY")
	t.Chdir(t.TempDir())

	resolved, err := prepareEvalRuntimeEnvironment(datasetPath)
	if err != nil {
		t.Fatalf("prepareEvalRuntimeEnvironment returned error: %v", err)
	}
	if resolved != datasetPath {
		t.Fatalf("resolved dataset = %q, want %q", resolved, datasetPath)
	}
	if got := mustLookupEnv(t, "ALEX_CONTEXT_CONFIG_DIR"); got != "/tmp/custom-context" {
		t.Fatalf("ALEX_CONTEXT_CONFIG_DIR overwritten: got %q", got)
	}
}

func TestPrepareEvalRuntimeEnvironmentFindsRootFromAbsoluteDatasetPath(t *testing.T) {
	repoRoot, datasetPath := makeEvalRepoFixture(t)

	t.Setenv("ALEX_CONFIG_PATH", "")
	t.Setenv("ALEX_DOTENV_PATH", "")
	t.Setenv("ALEX_CONTEXT_CONFIG_DIR", "")
	unsetEnv(t, "OPENAI_API_KEY")
	t.Chdir(t.TempDir())

	resolved, err := prepareEvalRuntimeEnvironment(datasetPath)
	if err != nil {
		t.Fatalf("prepareEvalRuntimeEnvironment returned error: %v", err)
	}
	if resolved != datasetPath {
		t.Fatalf("resolved dataset = %q, want %q", resolved, datasetPath)
	}
	if got := mustLookupEnv(t, "ALEX_CONTEXT_CONFIG_DIR"); got != filepath.Join(repoRoot, "configs", "context") {
		t.Fatalf("ALEX_CONTEXT_CONFIG_DIR = %q, want %q", got, filepath.Join(repoRoot, "configs", "context"))
	}
	if got := mustLookupEnv(t, "OPENAI_API_KEY"); got != "sk-live-e2e" {
		t.Fatalf("OPENAI_API_KEY = %q, want %q", got, "sk-live-e2e")
	}
}

func makeEvalRepoFixture(t *testing.T) (string, string) {
	t.Helper()

	repoRoot := t.TempDir()
	writeEvalTestFile(t, filepath.Join(repoRoot, "go.mod"), "module alex\n\ngo 1.24.0\n")
	writeEvalTestFile(t, filepath.Join(repoRoot, ".env"), "OPENAI_API_KEY=sk-live-e2e\n")
	writeEvalTestFile(t, filepath.Join(repoRoot, "configs", "context", "personas", "default.yaml"), "id: default\nvoice: test\n")
	datasetPath := filepath.Join(repoRoot, "evaluation", "swe_bench", "real_instances.json")
	writeEvalTestFile(t, datasetPath, "[]\n")

	return repoRoot, datasetPath
}

func writeEvalTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	previous, exists := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if exists {
			_ = os.Setenv(key, previous)
			return
		}
		_ = os.Unsetenv(key)
	})
}

func mustLookupEnv(t *testing.T, key string) string {
	t.Helper()
	value, ok := os.LookupEnv(key)
	if !ok {
		t.Fatalf("%s is not set", key)
	}
	return value
}
