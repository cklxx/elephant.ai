package llm

import (
	"context"

	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/shared/utils"
)

const defaultLlamaCppBaseURL = "http://127.0.0.1:8082/v1"

var (
	_ portsllm.StreamingLLMClient  = (*llamaCppClient)(nil)
	_ portsllm.UsageTrackingClient = (*llamaCppClient)(nil)
)

// llamaCppClient speaks the OpenAI-compatible API exposed by llama.cpp
// (typically via `llama-server`). It reuses the OpenAI-compatible request/stream
// handling but reports provider usage as "llama.cpp".
type llamaCppClient struct {
	inner         *openaiClient
	usageCallback func(usage ports.TokenUsage, model string, provider string)
}

func NewLlamaCppClient(model string, config Config) (portsllm.LLMClient, error) {
	inner := &openaiClient{
		baseClient: newBaseClient(model, config, baseClientOpts{
			defaultBaseURL: defaultLlamaCppBaseURL,
			logCategory:    utils.LogCategoryLLM,
			logComponent:   "llama.cpp",
		}),
	}
	return &llamaCppClient{inner: inner}, nil
}

func (c *llamaCppClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	resp, err := c.inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	c.fireUsageCallback(resp)
	return resp, nil
}

func (c *llamaCppClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	resp, err := c.inner.StreamComplete(ctx, req, callbacks)
	if err != nil {
		return resp, err
	}
	c.fireUsageCallback(resp)
	return resp, nil
}

func (c *llamaCppClient) Model() string {
	if c == nil || c.inner == nil {
		return ""
	}
	return c.inner.Model()
}

func (c *llamaCppClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	if c == nil {
		return
	}
	c.usageCallback = callback
}

func (c *llamaCppClient) fireUsageCallback(resp *ports.CompletionResponse) {
	if c == nil || c.usageCallback == nil || resp == nil {
		return
	}
	c.usageCallback(resp.Usage, c.Model(), "llama.cpp")
}
