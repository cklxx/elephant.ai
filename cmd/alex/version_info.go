package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"

	"alex/internal/config"
)

var (
	versionOnce   sync.Once
	cachedVersion string
)

// appVersion returns the best-effort semantic version for the alex binary.
// The lookup order is:
//  1. Explicit ALEX_VERSION environment variable (useful for custom builds)
//  2. Go build information when available (e.g. go install alex@vX)
//  3. Nearby npm package.json files (when distributed via npm)
//  4. A development fallback string
func appVersion() string {
	versionOnce.Do(func() {
		cachedVersion = detectVersion()
	})
	return cachedVersion
}

func detectVersion() string {
	if v, ok := config.DefaultEnvLookup("ALEX_VERSION"); ok {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			return trimmed
		}
	}

	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}

		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && setting.Value != "" {
				return fmt.Sprintf("dev-%s", setting.Value)
			}
		}
	}

	if v := readVersionFromPackageJSON(); v != "" {
		return v
	}

	return "development"
}

func readVersionFromPackageJSON() string {
	exeDir := executableDir()
	searchDirs := []string{}
	if exeDir != "" {
		searchDirs = append(searchDirs,
			exeDir,
			filepath.Clean(filepath.Join(exeDir, "..")),
			filepath.Clean(filepath.Join(exeDir, "..", "..")),
		)
	}

	if wd, err := os.Getwd(); err == nil {
		searchDirs = append(searchDirs, filepath.Clean(wd))
	}

	seen := map[string]struct{}{}
	for _, dir := range searchDirs {
		if dir == "" {
			continue
		}
		if _, ok := seen[dir]; ok {
			continue
		}
		seen[dir] = struct{}{}

		candidates := []string{
			filepath.Join(dir, "package.json"),
			filepath.Join(dir, "npm", "alex-code", "package.json"),
		}

		for _, candidate := range candidates {
			if v := parseVersionFromPackageJSON(candidate); v != "" {
				return v
			}
		}
	}

	return ""
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exePath); err == nil {
		exePath = resolved
	}
	return filepath.Dir(exePath)
}

func parseVersionFromPackageJSON(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var pkg struct {
		Version string `json:"version"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}

	return strings.TrimSpace(pkg.Version)
}
