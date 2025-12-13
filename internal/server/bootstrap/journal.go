package bootstrap

import (
	"path/filepath"
	"strings"

	"alex/internal/analytics/journal"
	"alex/internal/utils"
)

func BuildJournalReader(sessionDir string, logger *utils.Logger) journal.Reader {
	sessionDir = strings.TrimSpace(sessionDir)
	if sessionDir == "" {
		if logger != nil {
			logger.Warn("Session directory missing; turn replay disabled")
		}
		return nil
	}

	reader, err := journal.NewFileReader(filepath.Join(sessionDir, "journals"))
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to initialize journal reader: %v", err)
		}
		return nil
	}

	return reader
}
