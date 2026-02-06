package bootstrap

import (
	"context"
	"time"

	"alex/internal/logging"
	"alex/internal/observability"
)

// InitObservability best-effort initializes observability and returns a cleanup hook.
func InitObservability(configPath string, logger logging.Logger) (*observability.Observability, func()) {
	obs, err := observability.New(configPath)
	if err != nil {
		logging.OrNop(logger).Warn("Observability disabled: %v", err)
		return nil, nil
	}

	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := obs.Shutdown(ctx); err != nil {
			logging.OrNop(logger).Warn("Observability shutdown error: %v", err)
		}
	}

	return obs, cleanup
}
