package bootstrap

import (
	"path/filepath"
	"strings"

	"alex/internal/analytics/journal"
	"alex/internal/logging"
)

func BuildJournalReader(sessionDir string, logger logging.Logger) journal.Reader {
	logger = logging.OrNop(logger)
	sessionDir = strings.TrimSpace(sessionDir)
	if sessionDir == "" {
		logger.Warn("Session directory missing; turn replay disabled")
		return nil
	}

	reader, err := journal.NewFileReader(filepath.Join(sessionDir, "journals"))
	if err != nil {
		logger.Warn("Failed to initialize journal reader: %v", err)
		return nil
	}

	return reader
}
