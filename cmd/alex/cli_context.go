package main

import (
	"context"

	id "alex/internal/utils/id"
)

func cliBaseContext() context.Context {
	ctx, _ := id.EnsureLogID(context.Background(), id.NewLogID)
	return ctx
}
