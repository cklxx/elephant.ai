package bootstrap

import (
	"os"
	"path/filepath"
	"strings"

	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
)

const skillsDirEnvVar = "ALEX_SKILLS_DIR"

func ensureSkillsDirFromWorkspace(workspaceDir string, logger logging.Logger) bool {
	if value, ok := runtimeconfig.DefaultEnvLookup(skillsDirEnvVar); ok && strings.TrimSpace(value) != "" {
		return false
	}

	workspaceDir = strings.TrimSpace(workspaceDir)
	if workspaceDir == "" {
		return false
	}

	skillsDir := filepath.Join(workspaceDir, "skills")
	info, err := os.Stat(skillsDir)
	if err != nil || !info.IsDir() {
		return false
	}

	if err := os.Setenv(skillsDirEnvVar, skillsDir); err != nil {
		logging.OrNop(logger).Warn("Failed to set %s: %v", skillsDirEnvVar, err)
		return false
	}

	logging.OrNop(logger).Debug("Set %s=%s", skillsDirEnvVar, skillsDir)
	return true
}
