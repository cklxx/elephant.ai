package bootstrap

import (
	"net/http"

	"alex/internal/app/di"
	"alex/internal/delivery/server"
	"alex/internal/shared/logging"
)

// buildHooksBridge creates a HooksBridge handler that forwards Claude Code
// hook events to the Lark gateway. Returns nil if prerequisites are not met.
func buildHooksBridge(cfg Config, container *di.Container, logger logging.Logger) http.Handler {
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
