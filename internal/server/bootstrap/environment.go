package bootstrap

import (
	"context"
	"time"

	"alex/internal/environment"
	"alex/internal/tools"
	"alex/internal/utils"
)

func CaptureHostEnvironment(maxFileEntries int) (map[string]string, string) {
	hostSummary := environment.CollectLocalSummary(maxFileEntries)
	return environment.SummaryMap(hostSummary), environment.FormatSummary(hostSummary)
}

func CaptureSandboxEnvironment(
	ctx context.Context,
	manager *tools.SandboxManager,
	maxFileEntries int,
	logger *utils.Logger,
) (map[string]string, string, time.Time, bool) {
	if manager == nil {
		return nil, "", time.Time{}, false
	}

	summary, err := environment.CollectSandboxSummary(ctx, manager, maxFileEntries)
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to capture sandbox environment summary: %v", err)
		}
		return nil, "", time.Time{}, false
	}

	return environment.SummaryMap(summary), environment.FormatSummary(summary), time.Now().UTC(), true
}
