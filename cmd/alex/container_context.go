package main

import (
	"context"

	id "alex/internal/utils/id"
)

func (c *Container) UserID() string {
	return c.userID
}

func (c *Container) BackgroundContext() context.Context {
	return id.WithUserID(context.Background(), c.userID)
}

func (c *Container) WithUser(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return id.WithUserID(ctx, c.userID)
}
