// Package scheduler provides agent tools for managing scheduled jobs at
// runtime. The tools communicate with the scheduler subsystem through the
// schedulerapi.Service interface, avoiding a direct import of the
// internal/scheduler package and the resulting import cycle.
package scheduler

import (
	"alex/internal/schedulerapi"
	"alex/internal/tools/builtin/shared"
	"context"
)

// getService extracts the schedulerapi.Service from context.
func getService(ctx context.Context) schedulerapi.Service {
	v := shared.SchedulerFromContext(ctx)
	if v == nil {
		return nil
	}
	s, ok := v.(schedulerapi.Service)
	if !ok {
		return nil
	}
	return s
}
