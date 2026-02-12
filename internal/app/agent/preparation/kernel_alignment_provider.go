package preparation

import (
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
		soul := strings.TrimSpace(readTextFile(soulResolved))
		if soul == "" {
			// Fallback to canonical repo reference when personal SOUL.md is absent.
			soul = strings.TrimSpace(readTextFile("docs/reference/SOUL.md"))
		}
		userPrefs := strings.TrimSpace(readTextFile(userResolved))
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
		if soul == "" {
			b.WriteString("(missing)\n")
		} else {
			b.WriteString(soul)
			b.WriteString("\n")
		}
		b.WriteString("\n## User Service Settings\n")
		if userPrefs == "" {
			b.WriteString("(missing)\n")
		} else {
			b.WriteString(userPrefs)
			b.WriteString("\n")
		}
		b.WriteString("\n## Self-Heal Channel\n")
		b.WriteString("- On blocking engineering issues, dispatch coding repair tasks via `bg_dispatch`.\n")
		b.WriteString("- Prefer agent_type=`codex`; fallback to `claude_code` if unavailable.\n")
		b.WriteString("- Track via `bg_status`, collect via `bg_collect`, then merge via `ext_merge`.\n")
		return strings.TrimSpace(b.String())
	}
}

func readTextFile(path string) string {
	if strings.TrimSpace(path) == "" {
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
		if err == nil && strings.TrimSpace(home) != "" {
			return filepath.Join(home, strings.TrimPrefix(trimmed, "~/"))
		}
	}
	return trimmed
}
