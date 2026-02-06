package oauth

import (
	"context"
	"errors"
	"fmt"
)

var ErrTokenNotFound = errors.New("lark oauth token not found")

// TokenStore persists Lark OAuth tokens keyed by open_id.
type TokenStore interface {
	EnsureSchema(ctx context.Context) error
	Get(ctx context.Context, openID string) (Token, error)
	Upsert(ctx context.Context, token Token) error
	Delete(ctx context.Context, openID string) error
}

// NeedUserAuthError indicates the user must complete OAuth authorization.
type NeedUserAuthError struct {
	AuthURL string
}

func (e *NeedUserAuthError) Error() string {
	if e == nil || e.AuthURL == "" {
		return "lark user authorization required"
	}
	return fmt.Sprintf("lark user authorization required: %s", e.AuthURL)
}
