package bootstrap

import (
	"alex/internal/infra/environment"
)

func CaptureHostEnvironment(maxFileEntries int) (map[string]string, string) {
	hostSummary := environment.CollectLocalSummary(maxFileEntries)
	return environment.SummaryMap(hostSummary), environment.FormatSummary(hostSummary)
}
