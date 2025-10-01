package observability

import (
	"context"
	"fmt"
)

// Observability manages all observability components
type Observability struct {
	Logger  *Logger
	Metrics *MetricsCollector
	Tracer  *TracerProvider
	config  Config
}

// New creates a new observability instance
func New(configPath string) (*Observability, error) {
	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load observability config: %w", err)
	}

	// Initialize logger
	logger := NewLogger(LogConfig{
		Level:  config.Logging.Level,
		Format: config.Logging.Format,
	})

	// Initialize metrics
	metrics, err := NewMetricsCollector(config.Metrics)
	if err != nil {
		logger.Error("Failed to initialize metrics", "error", err)
		// Don't fail, continue without metrics
		metrics = &MetricsCollector{}
	}

	// Initialize tracing
	tracer, err := NewTracerProvider(config.Tracing)
	if err != nil {
		logger.Error("Failed to initialize tracing", "error", err)
		// Don't fail, use noop tracer
		tracer = &TracerProvider{}
	}

	logger.Info("Observability initialized",
		"log_level", config.Logging.Level,
		"metrics_enabled", config.Metrics.Enabled,
		"tracing_enabled", config.Tracing.Enabled,
	)

	return &Observability{
		Logger:  logger,
		Metrics: metrics,
		Tracer:  tracer,
		config:  config,
	}, nil
}

// Shutdown gracefully shuts down all observability components
func (o *Observability) Shutdown(ctx context.Context) error {
	o.Logger.Info("Shutting down observability")

	// Shutdown metrics
	if err := o.Metrics.Shutdown(ctx); err != nil {
		o.Logger.Error("Failed to shutdown metrics", "error", err)
	}

	// Shutdown tracing
	if err := o.Tracer.Shutdown(ctx); err != nil {
		o.Logger.Error("Failed to shutdown tracing", "error", err)
	}

	return nil
}

// Config returns the current configuration
func (o *Observability) Config() Config {
	return o.config
}
