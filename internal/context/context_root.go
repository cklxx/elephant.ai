package context

import (
	"os"
	"path/filepath"
	"strings"
)

func resolveContextConfigRoot() string {
	if envRoot, ok := os.LookupEnv(contextConfigEnvVar); ok {
		if trimmed := strings.TrimSpace(envRoot); trimmed != "" {
			return trimmed
		}
	}
	if resolved := locateExistingContextRoot(); resolved != "" {
		return resolved
	}
	return filepath.Join("configs", "context")
}

func locateExistingContextRoot() string {
	var starts []string
	if wd, err := os.Getwd(); err == nil && wd != "" {
		starts = append(starts, filepath.Clean(wd))
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Clean(filepath.Dir(exe))
		starts = append(starts, exeDir)
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
		if resolved := searchContextRootFromDir(start); resolved != "" {
			return resolved
		}
	}
	return ""
}

func searchContextRootFromDir(start string) string {
	dir := filepath.Clean(start)
	if dir == "" {
		return ""
	}
	for {
		candidate := filepath.Join(dir, "configs", "context")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir || parent == "" {
			break
		}
		dir = parent
	}
	candidate := filepath.Join(dir, "configs", "context")
	if info, err := os.Stat(candidate); err == nil && info.IsDir() {
		return candidate
	}
	return ""
}
