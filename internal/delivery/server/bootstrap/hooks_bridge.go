package bootstrap

import (
	"context"
	"fmt"

	"alex/internal/app/di"
	"alex/internal/delivery/server"
	"alex/internal/runtime/hooks"
	"alex/internal/shared/logging"
)

// LarkNotifier is the minimal interface needed to send a completion message.
type LarkNotifier interface {
	SendNotification(ctx context.Context, chatID, text string) error
}

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

// startRuntimeCompletionNotifier subscribes to all runtime events and sends a
// Feishu (Lark) notification when a session completes or fails. It runs in a
// non-blocking goroutine mirroring the pattern used by startRuntimeBusLogger.
// If chatID is empty the notifier starts but silently skips every notification.
func startRuntimeCompletionNotifier(ctx context.Context, bus hooks.Bus, lark LarkNotifier, chatID string, logger logging.Logger) {
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
				var text string
				switch ev.Type {
				case hooks.EventCompleted:
					text = fmt.Sprintf("✅ Runtime session `%s` completed", ev.SessionID)
				case hooks.EventFailed:
					text = fmt.Sprintf("❌ Runtime session `%s` failed", ev.SessionID)
				default:
					continue
				}
				if chatID == "" {
					continue
				}
				if err := lark.SendNotification(ctx, chatID, text); err != nil {
					logger.Warn("runtime_completion_notifier: send failed session_id=%s type=%s err=%v",
						ev.SessionID, string(ev.Type), err,
					)
				}
			}
		}
	}()
}
