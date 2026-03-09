package bootstrap

import (
	"context"

	"alex/internal/app/di"
	"alex/internal/delivery/server"
	"alex/internal/runtime/hooks"
	"alex/internal/shared/logging"
)

// buildHooksBridge creates a HooksBridge handler that forwards Claude Code
// hook events to the Lark gateway. Always wired when Lark is enabled.
func buildHooksBridge(cfg Config, container *di.Container, logger logging.Logger) *server.HooksBridge {
	if container == nil || container.LarkGateway == nil {
		return nil
	}

	var noticeLoader server.NoticeLoader
	if loaderFn := container.LarkGateway.NoticeLoader(); loaderFn != nil {
		noticeLoader = server.NoticeLoaderFunc(loaderFn)
	}

	return server.NewHooksBridge(
		container.LarkGateway,
		noticeLoader,
		cfg.HooksBridge.Token,
		cfg.HooksBridge.DefaultChatID,
		logger,
	)
}

// buildRuntimeHooksHandler creates a RuntimeHooksHandler that translates
// Claude Code hook events into runtime lifecycle events on an in-process bus.
// Returns the handler and the bus (so callers can subscribe to events).
func buildRuntimeHooksHandler(logger logging.Logger) (*server.RuntimeHooksHandler, hooks.Bus) {
	bus := hooks.NewInProcessBus()
	return server.NewRuntimeHooksHandler(bus, logger), bus
}

// startRuntimeBusLogger subscribes to all runtime events and logs them.
// This provides observability for the runtime hooks flow in production.
func startRuntimeBusLogger(ctx context.Context, bus hooks.Bus, logger logging.Logger) {
	ch, cancel := bus.SubscribeAll()
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-ch:
				if !ok {
					return
				}
				logger.Info("runtime_bus_event type=%s session_id=%s at=%s",
					string(ev.Type), ev.SessionID, ev.At.Format("15:04:05"),
				)
			}
		}
	}()
}
