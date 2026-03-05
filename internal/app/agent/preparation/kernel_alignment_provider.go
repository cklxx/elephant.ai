package preparation

import (
	"alex/internal/shared/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultKernelStateRootDir = "~/.alex/kernel"
	defaultSoulPath           = "~/.alex/memory/SOUL.md"
	defaultUserProfilePath    = "~/.alex/memory/USER.md"
	defaultKernelGoalFileName = "GOAL.md"
	defaultServiceUser        = "cklxx"
)

// KernelAlignmentContextProvider generates kernel alignment context that is
// injected into the system prompt.
type KernelAlignmentContextProvider func() string

// KernelAlignmentContextConfig controls where kernel alignment context is loaded from.
type KernelAlignmentContextConfig struct {
	KernelID    string
	ServiceUser string
	StateRoot   string
	SoulPath    string
	UserPath    string
}

// NewKernelAlignmentContextProvider creates a provider that always sources
// soul/user/goal context from canonical files (not runtime prompt config).
func NewKernelAlignmentContextProvider(cfg KernelAlignmentContextConfig) KernelAlignmentContextProvider {
	kernelID := strings.TrimSpace(cfg.KernelID)
	if kernelID == "" {
		kernelID = "default"
	}
	serviceUser := strings.TrimSpace(cfg.ServiceUser)
	if serviceUser == "" {
		serviceUser = defaultServiceUser
	}
	stateRoot := strings.TrimSpace(cfg.StateRoot)
	if stateRoot == "" {
		stateRoot = defaultKernelStateRootDir
	}
	soulPath := strings.TrimSpace(cfg.SoulPath)
	if soulPath == "" {
		soulPath = defaultSoulPath
	}
	userPath := strings.TrimSpace(cfg.UserPath)
	if userPath == "" {
		userPath = defaultUserProfilePath
	}

	goalPath := filepath.Join(resolveHomePath(stateRoot), kernelID, defaultKernelGoalFileName)
	soulResolved := resolveHomePath(soulPath)
	userResolved := resolveHomePath(userPath)

	return func() string {
		goal := strings.TrimSpace(readTextFile(goalPath))

		var b strings.Builder
		b.WriteString(fmt.Sprintf("Service user: %s\n", serviceUser))
		b.WriteString("Mission source: file-driven alignment (non-config)\n")
		b.WriteString(fmt.Sprintf("Kernel goal file: %s\n", goalPath))
		b.WriteString(fmt.Sprintf("SOUL source: %s\n", soulResolved))
		b.WriteString(fmt.Sprintf("USER source: %s\n", userResolved))
		b.WriteString("\n## Kernel Objective\n")
		if goal == "" {
			b.WriteString("(none)\n")
		} else {
			b.WriteString(goal)
			b.WriteString("\n")
		}
		b.WriteString("\n## Soul Values\n")
		b.WriteString(fmt.Sprintf("[Loaded via Identity/Memory sections — use read_file %s for full content]\n", soulResolved))
		b.WriteString("\n## User Service Settings\n")
		b.WriteString(fmt.Sprintf("[Loaded via Identity/Memory sections — use read_file %s for full content]\n", userResolved))
		b.WriteString("\n## Self-Heal Channel\n")
		b.WriteString("- On blocking engineering issues, dispatch via `team-cli` skill + `alex team run`.\n")
		b.WriteString("- Prefer agent_type=`codex`; fallback to `claude_code` if unavailable.\n")
		b.WriteString("- Monitor progress via `alex team status --json` and read the generated .status sidecar.\n")
		return strings.TrimSpace(b.String())
	}
}

func readTextFile(path string) string {
	if utils.IsBlank(path) {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

func resolveHomePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "~/") {
		home, err := os.UserHomeDir()
		if err == nil && utils.HasContent(home) {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}
