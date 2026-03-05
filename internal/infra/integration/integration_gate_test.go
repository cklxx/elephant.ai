package integration

import "testing"

func TestIntegrationSuiteRequiresBuildTag(t *testing.T) {
	t.Skip("integration suite is gated; run with -tags=integration to execute E2E tests")
}
