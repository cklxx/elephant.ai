package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSkillsRootUsesEnvWithoutSync(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	envRoot := filepath.Join(workspace, "custom-skills")
	if err := os.MkdirAll(filepath.Join(repoRoot, "skills"), 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	if err := os.MkdirAll(envRoot, 0o755); err != nil {
		t.Fatalf("mkdir env skills: %v", err)
	}

	t.Setenv("HOME", homeRoot)
	t.Setenv(skillsDirEnvVar, envRoot)

	root, err := ResolveSkillsRoot()
	if err != nil {
		t.Fatalf("resolve skills root: %v", err)
	}
	if root != envRoot {
		t.Fatalf("expected env root %q, got %q", envRoot, root)
	}

	defaultHome := filepath.Join(homeRoot, ".alex", "skills")
	if _, statErr := os.Stat(defaultHome); !os.IsNotExist(statErr) {
		t.Fatalf("expected no home sync when env is set, got stat error: %v", statErr)
	}
}

func TestResolveSkillsRootDefaultsToHomeAndCopiesMissing(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	if err := os.MkdirAll(repoSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	writeSkillFileForDiscovery(t, repoSkillsRoot, "alpha", "repo alpha")
	writeSkillFileForDiscovery(t, repoSkillsRoot, "beta", "repo beta")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	t.Setenv("HOME", homeRoot)
	t.Setenv(skillsDirEnvVar, "")

	root, err := ResolveSkillsRoot()
	if err != nil {
		t.Fatalf("resolve skills root: %v", err)
	}

	expectedRoot := filepath.Join(homeRoot, ".alex", "skills")
	if root != expectedRoot {
		t.Fatalf("expected home root %q, got %q", expectedRoot, root)
	}

	alphaSkill := filepath.Join(expectedRoot, "alpha", "SKILL.md")
	if _, statErr := os.Stat(alphaSkill); statErr != nil {
		t.Fatalf("expected copied alpha skill: %v", statErr)
	}
	betaSkill := filepath.Join(expectedRoot, "beta", "SKILL.md")
	if _, statErr := os.Stat(betaSkill); statErr != nil {
		t.Fatalf("expected copied beta skill: %v", statErr)
	}
}

func TestEnsureHomeSkillsBackfillsRepoAuthoritativeOnce(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	homeSkillsRoot := filepath.Join(homeRoot, ".alex", "skills")
	if err := os.MkdirAll(repoSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	if err := os.MkdirAll(homeSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir home skills: %v", err)
	}

	writeSkillFileForDiscovery(t, repoSkillsRoot, "alpha", "repo alpha")
	writeSkillFileForDiscovery(t, repoSkillsRoot, "beta", "repo beta")
	writeSkillFileForDiscovery(t, homeSkillsRoot, "alpha", "user alpha")
	writeSkillFileForDiscovery(t, homeSkillsRoot, "custom-only", "user custom only")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills: %v", err)
	}

	alphaContent, err := os.ReadFile(filepath.Join(homeSkillsRoot, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read alpha skill: %v", err)
	}
	if string(alphaContent) != skillMarkdown("alpha", "repo alpha") {
		t.Fatalf("expected existing alpha skill to be backfilled from repo")
	}

	betaContent, err := os.ReadFile(filepath.Join(homeSkillsRoot, "beta", "SKILL.md"))
	if err != nil {
		t.Fatalf("read beta skill: %v", err)
	}
	if string(betaContent) != skillMarkdown("beta", "repo beta") {
		t.Fatalf("expected missing beta skill copied from repo")
	}

	customContent, err := os.ReadFile(filepath.Join(homeSkillsRoot, "custom-only", "SKILL.md"))
	if err != nil {
		t.Fatalf("read custom-only skill: %v", err)
	}
	if string(customContent) != skillMarkdown("custom-only", "user custom only") {
		t.Fatalf("expected non-repo custom skill to be preserved")
	}

	markerPath := filepath.Join(homeSkillsRoot, homeSkillsBackfillMarkerName)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("expected backfill marker to exist: %v", err)
	}
}

func TestEnsureHomeSkillsAfterBackfillCopiesMissingWithoutOverwriting(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	homeSkillsRoot := filepath.Join(homeRoot, ".alex", "skills")
	if err := os.MkdirAll(repoSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	if err := os.MkdirAll(homeSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir home skills: %v", err)
	}

	writeSkillFileForDiscovery(t, repoSkillsRoot, "alpha", "repo alpha")
	writeSkillFileForDiscovery(t, homeSkillsRoot, "alpha", "user alpha")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills first run: %v", err)
	}

	writeSkillFileForDiscovery(t, homeSkillsRoot, "alpha", "user override after backfill")
	writeSkillFileForDiscovery(t, repoSkillsRoot, "beta", "repo beta")

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills second run: %v", err)
	}

	alphaContent, err := os.ReadFile(filepath.Join(homeSkillsRoot, "alpha", "SKILL.md"))
	if err != nil {
		t.Fatalf("read alpha skill: %v", err)
	}
	if string(alphaContent) != skillMarkdown("alpha", "user override after backfill") {
		t.Fatalf("expected second run to skip overriding after marker is written")
	}

	betaContent, err := os.ReadFile(filepath.Join(homeSkillsRoot, "beta", "SKILL.md"))
	if err != nil {
		t.Fatalf("read beta skill: %v", err)
	}
	if string(betaContent) != skillMarkdown("beta", "repo beta") {
		t.Fatalf("expected second run to still copy missing repo skills")
	}
}

func TestEnsureHomeSkillsBackfillsSupportScripts(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	repoScriptsRoot := filepath.Join(repoRoot, "scripts")
	homeSkillsRoot := filepath.Join(homeRoot, ".alex", "skills")
	if err := os.MkdirAll(repoSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	if err := os.MkdirAll(homeSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir home skills: %v", err)
	}

	writeSkillFileForDiscovery(t, repoSkillsRoot, "alpha", "repo alpha")
	writeSupportScriptFileForDiscovery(t, repoScriptsRoot, "skill_runner/env.py", "print('env')")
	writeSupportScriptFileForDiscovery(t, repoScriptsRoot, "cli/tavily/tavily_search.py", "print('tavily')")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills: %v", err)
	}

	homeScriptsRoot := filepath.Join(homeRoot, ".alex", "scripts")
	if _, err := os.Stat(filepath.Join(homeScriptsRoot, "skill_runner", "env.py")); err != nil {
		t.Fatalf("expected skill_runner helper synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeScriptsRoot, "cli", "tavily", "tavily_search.py")); err != nil {
		t.Fatalf("expected cli helper synced: %v", err)
	}
	if _, err := os.Stat(filepath.Join(homeRoot, ".alex", homeSkillsSupportScriptsMarkerName)); err != nil {
		t.Fatalf("expected support scripts marker to exist: %v", err)
	}
}

func TestEnsureHomeSkillsSupportScriptsAfterBackfillCopiesMissingWithoutOverwriting(t *testing.T) {
	workspace := t.TempDir()
	repoRoot := filepath.Join(workspace, "repo")
	homeRoot := filepath.Join(workspace, "home")
	repoSkillsRoot := filepath.Join(repoRoot, "skills")
	repoScriptsRoot := filepath.Join(repoRoot, "scripts")
	homeSkillsRoot := filepath.Join(homeRoot, ".alex", "skills")
	if err := os.MkdirAll(repoSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo skills: %v", err)
	}
	if err := os.MkdirAll(homeSkillsRoot, 0o755); err != nil {
		t.Fatalf("mkdir home skills: %v", err)
	}

	writeSkillFileForDiscovery(t, repoSkillsRoot, "alpha", "repo alpha")
	writeSupportScriptFileForDiscovery(t, repoScriptsRoot, "skill_runner/env.py", "repo v1")
	writeSupportScriptFileForDiscovery(t, repoScriptsRoot, "cli/tavily/tavily_search.py", "repo tavily")

	previousWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previousWD)
	})

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills first run: %v", err)
	}

	homeScriptsRoot := filepath.Join(homeRoot, ".alex", "scripts")
	writeSupportScriptFileForDiscovery(t, homeScriptsRoot, "skill_runner/env.py", "user override")
	writeSupportScriptFileForDiscovery(t, repoScriptsRoot, "cli/new/new_helper.py", "repo new")

	if err := EnsureHomeSkills(homeSkillsRoot); err != nil {
		t.Fatalf("ensure home skills second run: %v", err)
	}

	envContent, err := os.ReadFile(filepath.Join(homeScriptsRoot, "skill_runner", "env.py"))
	if err != nil {
		t.Fatalf("read env helper: %v", err)
	}
	if string(envContent) != "user override" {
		t.Fatalf("expected existing support script to remain after marker, got %q", string(envContent))
	}
	if _, err := os.Stat(filepath.Join(homeScriptsRoot, "cli", "new", "new_helper.py")); err != nil {
		t.Fatalf("expected newly added support script to be copied: %v", err)
	}
}

func writeSkillFileForDiscovery(t *testing.T, root, name, body string) {
	t.Helper()
	skillDir := filepath.Join(root, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMarkdown(name, body)), 0o644); err != nil {
		t.Fatalf("write skill file: %v", err)
	}
}

func writeSupportScriptFileForDiscovery(t *testing.T, scriptsRoot, relativePath, body string) {
	t.Helper()
	fullPath := filepath.Join(scriptsRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir support script dir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write support script file: %v", err)
	}
}

func skillMarkdown(name, body string) string {
	return "---\nname: " + name + "\ndescription: " + name + " description\n---\n# " + name + "\n" + body + "\n"
}
