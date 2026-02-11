package skills

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const skillsDirEnvVar = "ALEX_SKILLS_DIR"

const (
	homeSkillsBackfillMarkerName       = ".repo_backfill_version"
	homeSkillsBackfillVersion          = "repo-authoritative-v1"
	homeSkillsSupportScriptsMarkerName = ".repo_support_scripts_version"
	homeSkillsSupportScriptsVersion    = "repo-authoritative-v1"
)

var homeSupportScriptDirs = []string{"skill_runner", "cli"}

// LocateDefaultDir resolves the runtime skills root.
func LocateDefaultDir() string {
	root, err := ResolveSkillsRoot()
	if err != nil {
		return root
	}
	return root
}

// ResolveSkillsRoot resolves the skills root with this precedence:
//  1. ALEX_SKILLS_DIR (if set) without auto-copy
//  2. ~/.alex/skills with repo sync (one-time repo-authoritative backfill, then missing-only sync)
func ResolveSkillsRoot() (string, error) {
	if root, ok := skillsRootFromEnv(); ok {
		return root, nil
	}
	homeRoot, err := defaultHomeSkillsDir()
	if err != nil {
		if repoRoot := locateRepositorySkillsRoot(); repoRoot != "" {
			return repoRoot, nil
		}
		return "", err
	}
	if err := EnsureHomeSkills(homeRoot); err != nil {
		return homeRoot, err
	}
	return homeRoot, nil
}

// EnsureHomeSkills syncs repository skills into homeRoot.
// Behavior:
//   - First run for backfill version: repo-authoritative full overwrite for repo-known skills.
//   - Subsequent runs: copy missing skills only (preserve user edits).
func EnsureHomeSkills(homeRoot string) error {
	homeRoot = filepath.Clean(strings.TrimSpace(homeRoot))
	if homeRoot == "" || homeRoot == "." {
		return nil
	}
	if err := os.MkdirAll(homeRoot, 0o755); err != nil {
		return err
	}

	repoRoot := locateRepositorySkillsRoot()
	if repoRoot == "" {
		return nil
	}

	backfillDone, err := isHomeBackfillVersionApplied(homeRoot)
	if err != nil {
		return err
	}
	if !backfillDone {
		if err := copyRepoSkills(repoRoot, homeRoot, true); err != nil {
			return err
		}
		if err := writeHomeBackfillVersion(homeRoot); err != nil {
			return err
		}
	} else {
		if err := copyMissingSkills(repoRoot, homeRoot); err != nil {
			return err
		}
	}

	return ensureHomeSupportScripts(homeRoot, repoRoot)
}

func skillsRootFromEnv() (string, bool) {
	value, ok := os.LookupEnv(skillsDirEnvVar)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	root := filepath.Clean(trimmed)
	if root == "" || root == "." {
		return "", false
	}
	return root, true
}

func defaultHomeSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	home = strings.TrimSpace(home)
	if home == "" {
		return "", errors.New("home directory is empty")
	}
	return filepath.Join(filepath.Clean(home), ".alex", "skills"), nil
}

func locateRepositorySkillsRoot() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil && wd != "" {
		starts = append(starts, filepath.Clean(wd))
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		starts = append(starts, filepath.Clean(filepath.Dir(exe)))
	}

	seen := make(map[string]struct{}, len(starts))
	for _, start := range starts {
		if start == "" {
			continue
		}
		if _, ok := seen[start]; ok {
			continue
		}
		seen[start] = struct{}{}
		if resolved := searchSkillsRootFromDir(start); resolved != "" {
			return resolved
		}
	}

	return ""
}

func searchSkillsRootFromDir(start string) string {
	dir := filepath.Clean(start)
	if dir == "" {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "skills")
		if hasSkillFiles(candidate) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func hasSkillFiles(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return false
	}
	paths, err := discoverSkillFiles(dir)
	return err == nil && len(paths) > 0
}

func copyMissingSkills(sourceRoot, targetRoot string) error {
	return copyRepoSkills(sourceRoot, targetRoot, false)
}

func copyRepoSkills(sourceRoot, targetRoot string, overwriteExisting bool) error {
	entries, err := os.ReadDir(sourceRoot)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		sourceSkillDir := filepath.Join(sourceRoot, entry.Name())
		if !hasSkillDefinition(sourceSkillDir) {
			continue
		}

		targetSkillDir := filepath.Join(targetRoot, entry.Name())
		if overwriteExisting {
			if err := os.RemoveAll(targetSkillDir); err != nil {
				return err
			}
		} else {
			_, statErr := os.Stat(targetSkillDir)
			if statErr == nil {
				continue
			}
			if !errors.Is(statErr, fs.ErrNotExist) {
				return statErr
			}
		}
		if err := copyDirectory(sourceSkillDir, targetSkillDir); err != nil {
			return err
		}
	}

	return nil
}

func isHomeBackfillVersionApplied(homeRoot string) (bool, error) {
	markerPath := filepath.Join(homeRoot, homeSkillsBackfillMarkerName)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(data)) == homeSkillsBackfillVersion, nil
}

func writeHomeBackfillVersion(homeRoot string) error {
	markerPath := filepath.Join(homeRoot, homeSkillsBackfillMarkerName)
	return os.WriteFile(markerPath, []byte(homeSkillsBackfillVersion+"\n"), 0o644)
}

func ensureHomeSupportScripts(homeSkillsRoot, repoSkillsRoot string) error {
	alexRoot := filepath.Clean(filepath.Dir(homeSkillsRoot))
	repoScriptsRoot := filepath.Join(filepath.Dir(repoSkillsRoot), "scripts")
	homeScriptsRoot := filepath.Join(alexRoot, "scripts")

	info, err := os.Stat(repoScriptsRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	if err := os.MkdirAll(homeScriptsRoot, 0o755); err != nil {
		return err
	}

	backfillDone, err := isHomeSupportScriptsVersionApplied(alexRoot)
	if err != nil {
		return err
	}
	if !backfillDone {
		if err := copyRepoSupportScripts(repoScriptsRoot, homeScriptsRoot, true); err != nil {
			return err
		}
		return writeHomeSupportScriptsVersion(alexRoot)
	}
	return copyRepoSupportScripts(repoScriptsRoot, homeScriptsRoot, false)
}

func copyRepoSupportScripts(sourceRoot, targetRoot string, overwriteExisting bool) error {
	for _, dirName := range homeSupportScriptDirs {
		sourceDir := filepath.Join(sourceRoot, dirName)
		info, err := os.Stat(sourceDir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return err
		}
		if !info.IsDir() {
			continue
		}

		targetDir := filepath.Join(targetRoot, dirName)
		if overwriteExisting {
			if err := os.RemoveAll(targetDir); err != nil {
				return err
			}
		}
		if err := copyDirectory(sourceDir, targetDir); err != nil {
			return err
		}
	}
	return nil
}

func isHomeSupportScriptsVersionApplied(alexRoot string) (bool, error) {
	markerPath := filepath.Join(alexRoot, homeSkillsSupportScriptsMarkerName)
	data, err := os.ReadFile(markerPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return strings.TrimSpace(string(data)) == homeSkillsSupportScriptsVersion, nil
}

func writeHomeSupportScriptsVersion(alexRoot string) error {
	markerPath := filepath.Join(alexRoot, homeSkillsSupportScriptsMarkerName)
	return os.WriteFile(markerPath, []byte(homeSkillsSupportScriptsVersion+"\n"), 0o644)
}

func hasSkillDefinition(dir string) bool {
	for _, candidate := range []string{"SKILL.md", "SKILL.mdx"} {
		path := filepath.Join(dir, candidate)
		info, err := os.Stat(path)
		if err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}

func copyDirectory(sourceDir, targetDir string) error {
	return filepath.WalkDir(sourceDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relativePath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, relativePath)

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return os.MkdirAll(targetPath, info.Mode().Perm())
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(targetPath); err == nil {
			return nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, info.Mode().Perm())
	})
}
