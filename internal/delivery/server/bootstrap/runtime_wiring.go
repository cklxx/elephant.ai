package bootstrap

import (
	"context"
	"path/filepath"
	"time"

	"alex/internal/app/di"
	"alex/internal/runtime"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/shared/logging"
)

// startRuntimeSubsystem wires the Runtime, StallDetector, and LeaderAgent
// inside the Lark process and starts them as background goroutines.
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

	// StallDetector: scans every 10 s, stall threshold 60 s.
	detector := hooks.NewStallDetector(rt, bus, 60*time.Second, 10*time.Second)
	go detector.Run(ctx)

	// LeaderAgent: handles stalled / needs-input events via LLM.
	if container != nil && container.AgentCoordinator != nil {
		coord := container.AgentCoordinator
		executeFunc := func(ctx context.Context, prompt string) (string, error) {
			result, err := coord.ExecuteTask(ctx, prompt, "leader-agent", nil)
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
