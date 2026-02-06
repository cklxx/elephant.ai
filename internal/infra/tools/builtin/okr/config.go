package okr

import (
	"os"
	"path/filepath"
)

// OKRConfig holds configuration for the OKR tools.
type OKRConfig struct {
	GoalsRoot string // Directory containing goal markdown files
}

// DefaultOKRConfig returns sensible defaults for OKR file storage.
func DefaultOKRConfig() OKRConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return OKRConfig{
		GoalsRoot: filepath.Join(home, ".alex", "goals"),
	}
}

// GoalPath returns the full filesystem path for a given goal ID.
func (c OKRConfig) GoalPath(goalID string) string {
	return filepath.Join(c.GoalsRoot, goalID+".md")
}
