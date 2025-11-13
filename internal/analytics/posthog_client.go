package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/posthog/posthog-go"
)

const defaultPostHogHost = "https://app.posthog.com"

// PostHogClient emits events to PostHog.
type PostHogClient struct {
	client posthog.Client
}

// NewPostHogClient creates a PostHog-backed analytics client.
func NewPostHogClient(apiKey string, host string) (Client, error) {
	if apiKey == "" {
		return nil, errors.New("posthog api key is required")
	}

	endpoint := host
	if endpoint == "" {
		endpoint = defaultPostHogHost
	}

	phClient, err := posthog.NewWithConfig(apiKey, posthog.Config{Endpoint: endpoint})
	if err != nil {
		return nil, err
	}

	return &PostHogClient{client: phClient}, nil
}

// Capture sends an event to PostHog.
func (c *PostHogClient) Capture(ctx context.Context, distinctID string, event string, properties map[string]any) error {
	if c == nil || c.client == nil {
		return errors.New("posthog client not initialized")
	}

	if distinctID == "" {
		distinctID = "anonymous"
	}

	props := posthog.NewProperties()
	for key, value := range properties {
		props = props.Set(key, value)
	}

	return c.client.Enqueue(posthog.Capture{
		DistinctId: distinctID,
		Event:      event,
		Properties: props,
		Timestamp:  time.Now(),
	})
}

// Close flushes any buffered events and releases resources.
func (c *PostHogClient) Close() error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Close()
}
