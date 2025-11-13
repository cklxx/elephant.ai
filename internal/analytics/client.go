package analytics

import "context"

// Client captures product analytics events.
type Client interface {
	Capture(ctx context.Context, distinctID string, event string, properties map[string]any) error
	Close() error
}

type noopClient struct{}

// NewNoopClient returns a client that drops all events.
func NewNoopClient() Client {
	return noopClient{}
}

func (noopClient) Capture(ctx context.Context, distinctID string, event string, properties map[string]any) error {
	return nil
}

func (noopClient) Close() error {
	return nil
}
