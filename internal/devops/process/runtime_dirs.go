package process

import (
	"fmt"
	"os"
)

// EnsureRuntimeDirs creates the runtime pid/log directories used by dev services.
func EnsureRuntimeDirs(pidDir, logDir string) error {
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		return fmt.Errorf("create pid dir %s: %w", pidDir, err)
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return fmt.Errorf("create log dir %s: %w", logDir, err)
	}
	return nil
}
