package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"alex/internal/app/di"
	"alex/internal/runtime"
	"alex/internal/runtime/adapter"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/pool"
	"alex/internal/shared/logging"
)

// startRuntimeSubsystem wires the Runtime, AdapterFactory, StallDetector,
// LeaderAgent, and PanePool inside the Lark process and starts them as
// background goroutines.
// It returns the *runtime.Runtime so callers can attach HTTP handlers.
func startRuntimeSubsystem(
	ctx context.Context,
	bus hooks.Bus,
	container *di.Container,
	logger logging.Logger,
) *runtime.Runtime {
	log := logging.OrNop(logger)

	storeDir := "_runtime"
	if container != nil {
		storeDir = filepath.Join(container.SessionDir(), "_runtime")
	}

	rt, err := runtime.New(storeDir, runtime.Config{Bus: bus})
	if err != nil {
		log.Warn("runtime: failed to create runtime subsystem: %v — runtime features disabled", err)
		return nil
	}

	// Wire the adapter factory so StartSession actually launches CC in a Kaku pane.
	pm, err := panel.NewManager()
	if err != nil {
		log.Warn("runtime: panel manager unavailable: %v — CC pane launch disabled", err)
	} else {
		fac := adapter.NewFactory(pm, rt, "http://localhost:9090", nil)
		rt.SetFactory(fac)
		log.Info("runtime: adapter factory wired (CC pane launch enabled)")
	}

	// PanePool: create an empty pool; panes are registered via POST /api/runtime/pool.
	panePool := pool.New()
	rt.SetPool(panePool)

	// StallDetector: scans every 10 s, stall threshold 60 s.
	detector := hooks.NewStallDetector(rt, bus, 60*time.Second, 10*time.Second)
	go detector.Run(ctx)

	// LeaderAgent: handles stalled / needs-input / child-completed events via LLM.
	// The leader uses a dedicated ephemeral session ID per stall call to avoid
	// accumulating unbounded history in a single session.
	if container != nil && container.AgentCoordinator != nil {
		coord := container.AgentCoordinator
		executeFunc := func(ctx context.Context, prompt, sessionID string) (string, error) {
			if sessionID == "" {
				sessionID = fmt.Sprintf("leader-ephemeral-%d", time.Now().UnixNano())
			}
			result, err := coord.ExecuteTask(ctx, prompt, sessionID, nil)
			if err != nil {
				return "", err
			}
			return result.Answer, nil
		}
		la := leader.New(rt, bus, executeFunc)
		go la.Run(ctx)
		log.Info("runtime: LeaderAgent started")
	} else {
		log.Info("runtime: LeaderAgent skipped (no AgentCoordinator)")
	}

	log.Info("runtime: subsystem started storeDir=%s", storeDir)
	return rt
}
