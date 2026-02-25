package main

import (
	"context"
	"sync"

	id "alex/internal/shared/utils/id"
)

var (
	cliBaseOnce   sync.Once
	cliBaseCtx    context.Context
	cliBaseCancel context.CancelFunc
)

func cliBaseContext() context.Context {
	cliBaseOnce.Do(initCLIBaseContext)
	ctx := cliBaseCtx
	ctx, _ = id.EnsureLogID(ctx, id.NewLogID)
	return ctx
}

func cancelCLIBaseContext() {
	cliBaseOnce.Do(initCLIBaseContext)
	if cliBaseCancel != nil {
		cliBaseCancel()
	}
}

func initCLIBaseContext() {
	ctx, cancel := context.WithCancel(context.Background())
	cliBaseCtx = ctx
	cliBaseCancel = cancel
}
