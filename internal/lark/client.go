// Package lark provides a thin typed wrapper around the Lark Open API SDK.
//
// Design decisions:
//   - Thin wrapper, not a facade — delegates to the SDK, adds error mapping and defaults.
//   - Stateless per-call — the underlying *lark.Client is created once and shared.
//   - Sub-service accessors (Calendar(), Task()) return typed helpers.
//   - User access token support via functional options on each call.
package lark

import (
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// Client wraps the Lark SDK client with typed sub-service accessors.
type Client struct {
	raw *lark.Client
}

// New creates a Client from app credentials.
func New(appID, appSecret string, opts ...lark.ClientOptionFunc) *Client {
	return &Client{raw: lark.NewClient(appID, appSecret, opts...)}
}

// Wrap creates a Client from an existing SDK client.
// Useful when the gateway already holds a *lark.Client.
func Wrap(raw *lark.Client) *Client {
	return &Client{raw: raw}
}

// Raw returns the underlying SDK client for advanced use cases.
func (c *Client) Raw() *lark.Client { return c.raw }

// Calendar returns the calendar sub-service.
func (c *Client) Calendar() *CalendarService {
	return &CalendarService{client: c.raw}
}

// Task returns the task sub-service.
func (c *Client) Task() *TaskService {
	return &TaskService{client: c.raw}
}

// --- Shared helpers ---

// CallOption configures a single API call (e.g. user access token).
type CallOption func(*callOptions)

type callOptions struct {
	reqOpts []larkcore.RequestOptionFunc
}

// WithUserToken sets a user access token for the call.
func WithUserToken(token string) CallOption {
	return func(o *callOptions) {
		if token != "" {
			o.reqOpts = append(o.reqOpts, larkcore.WithUserAccessToken(token))
		}
	}
}

func buildOpts(opts []CallOption) []larkcore.RequestOptionFunc {
	var co callOptions
	for _, fn := range opts {
		fn(&co)
	}
	return co.reqOpts
}

// APIError represents a Lark API error with code and message.
type APIError struct {
	Code int
	Msg  string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("lark api error: code=%d msg=%s", e.Code, e.Msg)
}
