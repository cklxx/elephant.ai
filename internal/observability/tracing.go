package observability

import (
	"context"
	"fmt"

	id "alex/internal/utils/id"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// TracingConfig configures distributed tracing
type TracingConfig struct {
	Enabled        bool    `yaml:"enabled"`
	Exporter       string  `yaml:"exporter"` // otlp, zipkin
	OTLPEndpoint   string  `yaml:"otlp_endpoint"`
	ZipkinEndpoint string  `yaml:"zipkin_endpoint"`
	SampleRate     float64 `yaml:"sample_rate"` // 0.0 to 1.0
	ServiceName    string  `yaml:"service_name"`
	ServiceVersion string  `yaml:"service_version"`
}

// TracerProvider wraps OpenTelemetry tracer
type TracerProvider struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
}

// NewTracerProvider creates a new tracer provider
func NewTracerProvider(config TracingConfig) (*TracerProvider, error) {
	if !config.Enabled {
		// Return noop tracer
		return &TracerProvider{
			tracer: noop.NewTracerProvider().Tracer("alex"),
		}, nil
	}

	// Default service name
	if config.ServiceName == "" {
		config.ServiceName = "alex"
	}

	// Default sample rate
	if config.SampleRate <= 0 || config.SampleRate > 1.0 {
		config.SampleRate = 1.0
	}

	// Create exporter based on config
	var exporter sdktrace.SpanExporter
	var err error

	switch config.Exporter {
	case "otlp":
		endpoint := config.OTLPEndpoint
		if endpoint == "" {
			endpoint = "localhost:4318"
		}
		exporter, err = otlptracehttp.New(
			context.Background(),
			otlptracehttp.WithEndpoint(endpoint),
			otlptracehttp.WithInsecure(),
		)
	case "zipkin":
		endpoint := config.ZipkinEndpoint
		if endpoint == "" {
			endpoint = "http://localhost:9411/api/v2/spans"
		}
		exporter, err = zipkin.New(endpoint)
	default:
		return nil, fmt.Errorf("unsupported exporter: %s", config.Exporter)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create exporter: %w", err)
	}

	// Create resource
	res, err := resource.New(
		context.Background(),
		resource.WithAttributes(
			semconv.ServiceName(config.ServiceName),
			semconv.ServiceVersion(config.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create trace provider
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.TraceIDRatioBased(config.SampleRate)),
	)

	otel.SetTracerProvider(provider)

	return &TracerProvider{
		provider: provider,
		tracer:   provider.Tracer("alex"),
	}, nil
}

// Shutdown gracefully shuts down the tracer provider
func (tp *TracerProvider) Shutdown(ctx context.Context) error {
	if tp.provider != nil {
		return tp.provider.Shutdown(ctx)
	}
	return nil
}

// Tracer returns the tracer
func (tp *TracerProvider) Tracer() trace.Tracer {
	return tp.tracer
}

// StartSpan starts a new span
func (tp *TracerProvider) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	ids := id.IDsFromContext(ctx)
	if ids.SessionID != "" {
		attrs = append(attrs, attribute.String(AttrSessionID, ids.SessionID))
	}
	if ids.TaskID != "" {
		attrs = append(attrs, attribute.String(AttrTaskID, ids.TaskID))
	}
	if ids.ParentTaskID != "" {
		attrs = append(attrs, attribute.String(AttrParentTaskID, ids.ParentTaskID))
	}

	return tp.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// Common span names
const (
	SpanSessionSolveTask = "alex.session.solve_task"
	SpanReactIteration   = "alex.react.iteration"
	SpanToolExecute      = "alex.tool.execute"
	SpanLLMGenerate      = "alex.llm.generate"
	SpanHTTPServer       = "alex.http.request"
	SpanSSEConnection    = "alex.sse.connection"
)

// Common attribute keys
const (
	AttrSessionID    = "alex.session_id"
	AttrTaskID       = "alex.task_id"
	AttrParentTaskID = "alex.parent_task_id"
	AttrToolName     = "alex.tool_name"
	AttrModel        = "alex.llm.model"
	AttrTokenCount   = "alex.llm.token_count"
	AttrInputTokens  = "alex.llm.input_tokens"
	AttrOutputTokens = "alex.llm.output_tokens"
	AttrCost         = "alex.cost"
	AttrIteration    = "alex.iteration"
	AttrStatus       = "alex.status"
	AttrError        = "alex.error"
)

// Helper functions to add common attributes

// SessionAttrs creates session attributes
func SessionAttrs(sessionID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrSessionID, sessionID),
	}
}

// ToolAttrs creates tool attributes
func ToolAttrs(toolName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrToolName, toolName),
	}
}

// LLMAttrs creates LLM attributes
func LLMAttrs(model string, inputTokens, outputTokens int, cost float64) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String(AttrModel, model),
		attribute.Int(AttrInputTokens, inputTokens),
		attribute.Int(AttrOutputTokens, outputTokens),
		attribute.Int(AttrTokenCount, inputTokens+outputTokens),
	}
	if cost > 0 {
		attrs = append(attrs, attribute.Float64(AttrCost, cost))
	}
	return attrs
}

// IterationAttrs creates iteration attributes
func IterationAttrs(iteration int) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int(AttrIteration, iteration),
	}
}

// StatusAttrs creates status attributes
func StatusAttrs(status string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrStatus, status),
	}
}

// ErrorAttrs creates error attributes
func ErrorAttrs(err error) []attribute.KeyValue {
	if err == nil {
		return nil
	}
	return []attribute.KeyValue{
		attribute.Bool(AttrError, true),
		attribute.String("error.message", err.Error()),
	}
}
