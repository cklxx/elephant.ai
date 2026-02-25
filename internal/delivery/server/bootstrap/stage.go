package bootstrap

import (
	"fmt"
	"sync"

	"alex/internal/shared/logging"
)

// BootstrapStage represents a single initialization step during server startup.
type BootstrapStage struct {
	Name     string       // Human-readable stage name (e.g., "observability", "config")
	Required bool         // If true, failure aborts startup; otherwise recorded as degraded
	Init     func() error // Initialization function
}

// DegradedComponents tracks components that failed optional initialization
// but did not prevent server startup.
type DegradedComponents struct {
	mu         sync.RWMutex
	components map[string]string // component name → error description
}

// NewDegradedComponents creates a new degraded component tracker.
func NewDegradedComponents() *DegradedComponents {
	return &DegradedComponents{
		components: make(map[string]string),
	}
}

// Record marks a component as degraded with an error description.
func (d *DegradedComponents) Record(name, reason string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.components[name] = reason
}

// Map returns a snapshot of all degraded components.
func (d *DegradedComponents) Map() map[string]string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make(map[string]string, len(d.components))
	for k, v := range d.components {
		out[k] = v
	}
	return out
}

// IsEmpty reports whether any components are degraded.
func (d *DegradedComponents) IsEmpty() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.components) == 0
}

// RunStages executes stages in order. Required stages abort on error;
// optional stages are recorded as degraded and execution continues.
func RunStages(stages []BootstrapStage, degraded *DegradedComponents, logger logging.Logger) error {
	for _, stage := range stages {
		logger.Info("[Bootstrap] Running stage: %s (required=%v)", stage.Name, stage.Required)
		if err := stage.Init(); err != nil {
			if stage.Required {
				return fmt.Errorf("required stage %q failed: %w", stage.Name, err)
			}
			logger.Warn("[Bootstrap] Optional stage %q failed: %v (continuing in degraded mode)", stage.Name, err)
			if degraded != nil {
				degraded.Record(stage.Name, err.Error())
			}
		}
	}
	return nil
}
