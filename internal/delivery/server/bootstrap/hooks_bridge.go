package bootstrap

import (
	"alex/internal/app/di"
	"alex/internal/delivery/server"
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
