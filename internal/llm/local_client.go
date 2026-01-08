package llm

import (
	"context"

	"alex/internal/agent/ports"
	"alex/internal/localmodel"
	"alex/internal/logging"
)

type localClient struct {
	underlying ports.LLMClient
	manager    *localmodel.ServerManager
	logger     logging.Logger
	baseURL    string
}

var (
	_ ports.LLMClient          = (*localClient)(nil)
	_ ports.StreamingLLMClient = (*localClient)(nil)
)

func NewLocalClient(model string, config Config) (ports.LLMClient, error) {
	if config.BaseURL == "" {
		config.BaseURL = localmodel.BaseURL
	}

	underlying, err := NewOpenAIClient(model, config)
	if err != nil {
		return nil, err
	}

	return &localClient{
		underlying: underlying,
		manager:    localmodel.DefaultManager(),
		logger:     logging.NewComponentLogger("local-llm"),
		baseURL:    config.BaseURL,
	}, nil
}

func (c *localClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if err := c.manager.Ensure(ctx, c.logger, c.baseURL); err != nil {
		return nil, err
	}
	return c.underlying.Complete(ctx, req)
}

func (c *localClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	if err := c.manager.Ensure(ctx, c.logger, c.baseURL); err != nil {
		return nil, err
	}
	if streaming, ok := c.underlying.(ports.StreamingLLMClient); ok {
		return streaming.StreamComplete(ctx, req, callbacks)
	}
	resp, err := c.underlying.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if cb := callbacks.OnContentDelta; cb != nil {
		if resp != nil && resp.Content != "" {
			cb(ports.ContentDelta{Delta: resp.Content})
		}
		cb(ports.ContentDelta{Final: true})
	}
	return resp, nil
}

func (c *localClient) Model() string {
	return c.underlying.Model()
}

func (c *localClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	if trackingClient, ok := c.underlying.(ports.UsageTrackingClient); ok {
		trackingClient.SetUsageCallback(callback)
	}
}
