package bootstrap

import (
	"context"
	"sync"

	"alex/internal/logging"
)

// Subsystem represents a long-running server component with a managed lifecycle.
type Subsystem interface {
	Name() string
	Start(ctx context.Context) error
	Stop()
}

// SubsystemManager manages the lifecycle of multiple subsystems, providing
// unified context cancellation and ordered shutdown.
type SubsystemManager struct {
	mu         sync.Mutex
	subsystems []managedSubsystem
	logger     logging.Logger
}

type managedSubsystem struct {
	sub    Subsystem
	cancel context.CancelFunc
}

// NewSubsystemManager creates a new subsystem manager.
func NewSubsystemManager(logger logging.Logger) *SubsystemManager {
	return &SubsystemManager{
		logger: logger,
	}
}

// Start starts a subsystem with a derived context. If start fails, it is
// not tracked and the error is returned.
func (m *SubsystemManager) Start(parent context.Context, sub Subsystem) error {
	ctx, cancel := context.WithCancel(parent)
	if err := sub.Start(ctx); err != nil {
		cancel()
		return err
	}
	m.mu.Lock()
	m.subsystems = append(m.subsystems, managedSubsystem{sub: sub, cancel: cancel})
	m.mu.Unlock()
	m.logger.Info("[SubsystemManager] Started subsystem: %s", sub.Name())
	return nil
}

// StopAll stops all managed subsystems in reverse order (LIFO).
func (m *SubsystemManager) StopAll() {
	m.mu.Lock()
	subs := make([]managedSubsystem, len(m.subsystems))
	copy(subs, m.subsystems)
	m.subsystems = nil
	m.mu.Unlock()

	for i := len(subs) - 1; i >= 0; i-- {
		s := subs[i]
		m.logger.Info("[SubsystemManager] Stopping subsystem: %s", s.sub.Name())
		s.cancel()
		s.sub.Stop()
	}
}
