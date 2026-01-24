package llm

import (
	"context"
	"strings"

	"alex/internal/agent/ports"
)

type toolCallParsingClient struct {
	underlying ports.LLMClient
	parser     ports.FunctionCallParser
}

var (
	_ ports.LLMClient          = (*toolCallParsingClient)(nil)
	_ ports.StreamingLLMClient = (*toolCallParsingClient)(nil)
)

func WrapWithToolCallParsing(client ports.LLMClient, parser ports.FunctionCallParser) ports.LLMClient {
	if client == nil || parser == nil {
		return client
	}
	if _, ok := client.(*toolCallParsingClient); ok {
		return client
	}
	return &toolCallParsingClient{underlying: client, parser: parser}
}

func (c *toolCallParsingClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	resp, err := c.underlying.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	return c.withParsedToolCalls(req, resp), nil
}

func (c *toolCallParsingClient) StreamComplete(
	ctx context.Context,
	req ports.CompletionRequest,
	callbacks ports.CompletionStreamCallbacks,
) (*ports.CompletionResponse, error) {
	streaming, ok := c.underlying.(ports.StreamingLLMClient)
	if !ok {
		resp, err := c.underlying.Complete(ctx, req)
		if err != nil {
			return resp, err
		}
		if callbacks.OnContentDelta != nil {
			if resp != nil && resp.Content != "" {
				callbacks.OnContentDelta(ports.ContentDelta{Delta: resp.Content})
			}
			callbacks.OnContentDelta(ports.ContentDelta{Final: true})
		}
		return c.withParsedToolCalls(req, resp), nil
	}

	resp, err := streaming.StreamComplete(ctx, req, callbacks)
	if err != nil {
		return resp, err
	}
	return c.withParsedToolCalls(req, resp), nil
}

func (c *toolCallParsingClient) Model() string {
	return c.underlying.Model()
}

func (c *toolCallParsingClient) SetUsageCallback(callback func(usage ports.TokenUsage, model string, provider string)) {
	if trackingClient, ok := c.underlying.(ports.UsageTrackingClient); ok {
		trackingClient.SetUsageCallback(callback)
	}
}

func (c *toolCallParsingClient) withParsedToolCalls(req ports.CompletionRequest, resp *ports.CompletionResponse) *ports.CompletionResponse {
	if resp == nil || c == nil || c.parser == nil {
		return resp
	}
	if len(resp.ToolCalls) > 0 {
		return resp
	}
	if len(req.Tools) == 0 {
		return resp
	}
	if strings.TrimSpace(resp.Content) == "" {
		return resp
	}

	parsed, err := c.parser.Parse(resp.Content)
	if err != nil || len(parsed) == 0 {
		return resp
	}

	allowed := allowedToolNames(req.Tools)
	if len(allowed) == 0 {
		return resp
	}

	filtered := make([]ports.ToolCall, 0, len(parsed))
	for _, call := range parsed {
		name := strings.TrimSpace(call.Name)
		if name == "" {
			continue
		}
		if _, ok := allowed[name]; !ok {
			continue
		}
		filtered = append(filtered, call)
	}

	if len(filtered) == 0 {
		return resp
	}

	resp.ToolCalls = filtered
	return resp
}

func allowedToolNames(tools []ports.ToolDefinition) map[string]struct{} {
	if len(tools) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" {
			continue
		}
		allowed[name] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}
