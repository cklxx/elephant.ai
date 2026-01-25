package config

// Config captures runtime defaults for coordinator execution and preparation.
type Config struct {
	LLMProvider         string
	LLMModel            string
	LLMSmallProvider    string
	LLMSmallModel       string
	LLMVisionModel      string
	APIKey              string
	BaseURL             string
	MaxTokens           int
	MaxIterations       int
	ToolMaxConcurrent   int
	Temperature         float64
	TemperatureProvided bool
	TopP                float64
	StopSequences       []string
	AgentPreset         string // Agent persona preset (default, code-expert, etc.)
	ToolPreset          string // Tool access preset (full, read-only, safe, architect)
	ToolMode            string // Tool access mode (web or cli)
	EnvironmentSummary  string
}
