package modelregistry

// ModelInfo contains metadata about an LLM model fetched from models.dev.
type ModelInfo struct {
	ID             string
	Provider       string
	ContextWindow  int     // limit.context from models.dev
	InputPer1M     float64 // pricing.input  (USD per 1M tokens)
	OutputPer1M    float64 // pricing.output (USD per 1M tokens)
	SupportsTools  bool
	SupportsVision bool
}
