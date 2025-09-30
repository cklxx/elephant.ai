package llm

import (
	"net/http"
)

// Provider interface defines different LLM providers
type Provider interface {
	// Name returns the provider name
	Name() string

	// CreateClient creates a new client for this provider
	CreateClient(config *Config) (Client, error)

	// ValidateConfig validates the configuration for this provider
	ValidateConfig(config *Config) error
}

// HTTPClient interface for HTTP-based LLM clients
type HTTPClient interface {
	Client

	// SetHTTPClient sets a custom HTTP client
	SetHTTPClient(client *http.Client)

	// GetHTTPClient returns the current HTTP client
	GetHTTPClient() *http.Client
}

// StreamingClient interface for streaming-capable LLM clients
type StreamingClient interface {
	Client

	// SupportsStreaming returns true if the client supports streaming
	SupportsStreaming() bool

	// SetStreamingEnabled enables or disables streaming
	SetStreamingEnabled(enabled bool)
}

// MetricsCollector interface for collecting LLM metrics
type MetricsCollector interface {
	// RecordRequest records a request metric
	RecordRequest(model string, tokens int)

	// RecordResponse records a response metric
	RecordResponse(model string, tokens int, duration int64)

	// RecordError records an error metric
	RecordError(model string, errorType string)
}

// ClientFactory interface for creating LLM clients
type ClientFactory interface {
	// CreateHTTPClient creates an HTTP-mode client
	CreateHTTPClient(config *Config) (HTTPClient, error)

	// CreateStreamingClient creates a streaming-mode client
	CreateStreamingClient(config *Config) (StreamingClient, error)

	// GetSupportedProviders returns list of supported providers
	GetSupportedProviders() []string
}
