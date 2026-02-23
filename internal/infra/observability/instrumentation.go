package observability

import (
	"context"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	llm "alex/internal/domain/agent/ports/llm"
	tools "alex/internal/domain/agent/ports/tools"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// InstrumentedLLMClient wraps an LLM client with observability
type InstrumentedLLMClient struct {
	inner llm.LLMClient
	obs   *Observability
}

// NewInstrumentedLLMClient creates an instrumented LLM client
func NewInstrumentedLLMClient(client llm.LLMClient, obs *Observability) llm.LLMClient {
	return &InstrumentedLLMClient{
		inner: client,
		obs:   obs,
	}
}

func (c *InstrumentedLLMClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	// Start span
	ctx, span := c.obs.Tracer.StartSpan(ctx, SpanLLMGenerate,
		attribute.String(AttrModel, c.inner.Model()),
	)
	defer span.End()

	// Log request
	c.obs.Logger.InfoContext(ctx, "LLM request started",
		"model", c.inner.Model(),
		"messages", len(req.Messages),
		"tools", len(req.Tools),
	)

	// Measure latency
	start := time.Now()
	resp, err := c.inner.Complete(ctx, req)
	latency := time.Since(start)

	// Handle error
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		c.obs.Logger.ErrorContext(ctx, "LLM request failed",
			"error", err,
			"latency", latency,
		)
		c.obs.Metrics.RecordLLMRequest(ctx, c.inner.Model(), "error", latency, 0, 0, 0)
		return nil, err
	}

	// Calculate cost
	cost := EstimateCost(c.inner.Model(), resp.Usage.PromptTokens, resp.Usage.CompletionTokens)

	// Record metrics
	c.obs.Metrics.RecordLLMRequest(
		ctx,
		c.inner.Model(),
		"success",
		latency,
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		cost,
	)

	// Add span attributes
	span.SetAttributes(LLMAttrs(
		c.inner.Model(),
		resp.Usage.PromptTokens,
		resp.Usage.CompletionTokens,
		cost,
	)...)
	span.SetAttributes(attribute.String("stop_reason", resp.StopReason))

	// Log success
	c.obs.Logger.InfoContext(ctx, "LLM request completed",
		"model", c.inner.Model(),
		"latency", latency,
		"input_tokens", resp.Usage.PromptTokens,
		"output_tokens", resp.Usage.CompletionTokens,
		"total_tokens", resp.Usage.TotalTokens,
		"cost", cost,
	)

	return resp, nil
}

func (c *InstrumentedLLMClient) Model() string {
	return c.inner.Model()
}

// InstrumentedToolExecutor wraps a tool executor with observability
type InstrumentedToolExecutor struct {
	inner tools.ToolExecutor
	obs   *Observability
}

// NewInstrumentedToolExecutor creates an instrumented tool executor
func NewInstrumentedToolExecutor(executor tools.ToolExecutor, obs *Observability) tools.ToolExecutor {
	return &InstrumentedToolExecutor{
		inner: executor,
		obs:   obs,
	}
}

func (t *InstrumentedToolExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	toolName := call.Name

	// Start span
	ctx, span := t.obs.Tracer.StartSpan(ctx, SpanToolExecute, ToolAttrs(toolName)...)
	defer span.End()

	// Log sanitized parameters
	sanitizedArgs := sanitizeToolArguments(call.Arguments)
	t.obs.Logger.InfoContext(ctx, "Tool execution started",
		"tool", toolName,
		"call_id", call.ID,
		"args", sanitizedArgs,
	)

	// Measure duration
	start := time.Now()
	result, err := t.inner.Execute(ctx, call)
	duration := time.Since(start)

	// Handle error
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		t.obs.Logger.ErrorContext(ctx, "Tool execution failed",
			"tool", toolName,
			"error", err,
			"duration", duration,
		)
		t.obs.Metrics.RecordToolExecution(ctx, toolName, "error", duration)
		return result, err
	}

	// Check result error
	status := "success"
	if result != nil && result.Error != nil {
		status = "error"
		span.SetStatus(codes.Error, result.Error.Error())
		span.RecordError(result.Error)
	}

	// Record metrics
	t.obs.Metrics.RecordToolExecution(ctx, toolName, status, duration)

	// Log success
	t.obs.Logger.InfoContext(ctx, "Tool execution completed",
		"tool", toolName,
		"duration", duration,
		"status", status,
		"result_length", len(result.Content),
	)

	return result, nil
}

func (t *InstrumentedToolExecutor) Definition() ports.ToolDefinition {
	return t.inner.Definition()
}

func (t *InstrumentedToolExecutor) Metadata() ports.ToolMetadata {
	return t.inner.Metadata()
}

// InstrumentedToolRegistry wraps a tool registry with observability
type InstrumentedToolRegistry struct {
	inner tools.ToolRegistry
	obs   *Observability
}

// NewInstrumentedToolRegistry creates an instrumented tool registry
func NewInstrumentedToolRegistry(registry tools.ToolRegistry, obs *Observability) tools.ToolRegistry {
	return &InstrumentedToolRegistry{
		inner: registry,
		obs:   obs,
	}
}

func (r *InstrumentedToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	tool, err := r.inner.Get(name)
	if err != nil {
		return nil, err
	}
	// Wrap with instrumentation
	return NewInstrumentedToolExecutor(tool, r.obs), nil
}

func (r *InstrumentedToolRegistry) List() []ports.ToolDefinition {
	return r.inner.List()
}

func (r *InstrumentedToolRegistry) Register(tool tools.ToolExecutor) error {
	// Wrap before registering
	instrumented := NewInstrumentedToolExecutor(tool, r.obs)
	return r.inner.Register(instrumented)
}

func (r *InstrumentedToolRegistry) Unregister(name string) error {
	return r.inner.Unregister(name)
}

// sanitizeToolArguments removes sensitive data from tool arguments
func sanitizeToolArguments(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}

	sanitized := make(map[string]any, len(args))
	for key, value := range args {
		if isSensitiveArgumentKey(key) {
			sanitized[key] = "***REDACTED***"
			continue
		}
		sanitized[key] = sanitizeToolArgumentValue(key, value)
	}
	return sanitized
}

func sanitizeToolArgumentValue(key string, value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return sanitizeToolArguments(typed)
	case map[string]string:
		converted := make(map[string]any, len(typed))
		for nestedKey, nestedValue := range typed {
			converted[nestedKey] = nestedValue
		}
		return sanitizeToolArguments(converted)
	case []any:
		sanitized := make([]any, len(typed))
		for i, item := range typed {
			sanitized[i] = sanitizeToolArgumentValue(key, item)
		}
		return sanitized
	case []string:
		sanitized := make([]any, len(typed))
		for i, item := range typed {
			sanitized[i] = sanitizeToolArgumentValue(key, item)
		}
		return sanitized
	case string:
		if isSensitiveArgumentKey(key) {
			return "***REDACTED***"
		}
		if len(typed) > 20 && (containsSensitiveKeyword(key) || looksLikeAPIKey(typed)) {
			return SanitizeAPIKey(typed)
		}
		return typed
	default:
		return value
	}
}

func isSensitiveArgumentKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	switch lower {
	case "api_key", "apikey", "password", "token", "secret", "credentials", "credential",
		"authorization", "x-api-key", "access_token", "refresh_token", "id_token",
		"client_secret", "cookie", "set-cookie":
		return true
	default:
		return containsSensitiveKeyword(lower)
	}
}

func containsSensitiveKeyword(s string) bool {
	keywords := []string{"token", "secret", "password", "credential", "auth", "api_key", "apikey", "x-api-key", "cookie"}
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func looksLikeAPIKey(s string) bool {
	// Simple heuristic: long alphanumeric string with dashes or underscores
	if len(s) < 20 {
		return false
	}

	alphanumCount := 0
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			alphanumCount++
		}
	}

	// If >80% alphanumeric, likely an API key
	return float64(alphanumCount)/float64(len(s)) > 0.8
}
