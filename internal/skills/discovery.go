package skills

import (
	"os"
	"path/filepath"
	"strings"
)

const skillsDirEnvVar = "ALEX_SKILLS_DIR"

// LocateDefaultDir tries to locate a skills directory by checking:
//  1. ALEX_SKILLS_DIR (if set)
//  2. ./skills relative to the current working directory or executable path (walking upwards)
func LocateDefaultDir() string {
	if value, ok := os.LookupEnv(skillsDirEnvVar); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			if dir := filepath.Clean(trimmed); dir != "" {
				return dir
			}
		}
	}

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
