package config

import (
	"strconv"
	"strings"
)

// RuntimeEnvLookup returns an EnvLookup that surfaces resolved runtime configuration values
// as synthetic environment variables. When a key is not present in the runtime snapshot the
// lookup falls back to the supplied base implementation.
func RuntimeEnvLookup(cfg RuntimeConfig, base EnvLookup) EnvLookup {
	values := runtimeEnvValues(cfg)

	return func(key string) (string, bool) {
		if value, ok := values[key]; ok && value != "" {
			return value, true
		}
		if base != nil {
			if value, ok := base(key); ok && value != "" {
				return value, true
			}
		}
		return "", false
	}
}

func runtimeEnvValues(cfg RuntimeConfig) map[string]string {
	values := map[string]string{}

	set := func(key, value string) {
		if value == "" {
			return
		}
		values[key] = value
	}

	set("OPENAI_API_KEY", cfg.APIKey)
	set("OPENROUTER_API_KEY", cfg.APIKey)

	set("LLM_PROVIDER", cfg.LLMProvider)
	set("ALEX_LLM_PROVIDER", cfg.LLMProvider)

	set("LLM_MODEL", cfg.LLMModel)
	set("ALEX_LLM_MODEL", cfg.LLMModel)
	set("ALEX_MODEL_NAME", cfg.LLMModel)

	set("LLM_BASE_URL", cfg.BaseURL)
	set("ALEX_BASE_URL", cfg.BaseURL)

	set("SANDBOX_BASE_URL", cfg.SandboxBaseURL)
	set("ALEX_SANDBOX_BASE_URL", cfg.SandboxBaseURL)

	set("TAVILY_API_KEY", cfg.TavilyAPIKey)
	set("ALEX_TAVILY_API_KEY", cfg.TavilyAPIKey)

	set("VOLC_ACCESSKEY", cfg.VolcAccessKey)
	set("ALEX_VOLC_ACCESSKEY", cfg.VolcAccessKey)
	set("VOLC_SECRETKEY", cfg.VolcSecretKey)
	set("ALEX_VOLC_SECRETKEY", cfg.VolcSecretKey)
	set("SEEDREAM_HOST", cfg.SeedreamHost)
	set("ALEX_SEEDREAM_HOST", cfg.SeedreamHost)
	set("SEEDREAM_REGION", cfg.SeedreamRegion)
	set("ALEX_SEEDREAM_REGION", cfg.SeedreamRegion)
	set("SEEDREAM_TEXT_ENDPOINT_ID", cfg.SeedreamTextEndpointID)
	set("ALEX_SEEDREAM_TEXT_ENDPOINT_ID", cfg.SeedreamTextEndpointID)
	set("SEEDREAM_IMAGE_ENDPOINT_ID", cfg.SeedreamImageEndpointID)
	set("ALEX_SEEDREAM_IMAGE_ENDPOINT_ID", cfg.SeedreamImageEndpointID)

	if cfg.Environment != "" {
		set("ALEX_ENV", cfg.Environment)
	}

	followTranscript := strconv.FormatBool(cfg.FollowTranscript)
	followStream := strconv.FormatBool(cfg.FollowStream)

	set("ALEX_VERBOSE", strconv.FormatBool(cfg.Verbose))
	set("ALEX_NO_TUI", strconv.FormatBool(cfg.DisableTUI))
	set("ALEX_TUI_FOLLOW_TRANSCRIPT", followTranscript)
	set("ALEX_TUI_FOLLOW_STREAM", followStream)
	set("ALEX_FOLLOW_TRANSCRIPT", followTranscript)
	set("ALEX_FOLLOW_STREAM", followStream)

	if cfg.MaxIterations > 0 {
		set("LLM_MAX_ITERATIONS", strconv.Itoa(cfg.MaxIterations))
		set("ALEX_LLM_MAX_ITERATIONS", strconv.Itoa(cfg.MaxIterations))
	}

	if cfg.MaxTokens > 0 {
		set("LLM_MAX_TOKENS", strconv.Itoa(cfg.MaxTokens))
		set("ALEX_LLM_MAX_TOKENS", strconv.Itoa(cfg.MaxTokens))
	}

	if cfg.TemperatureProvided || cfg.Temperature != 0 {
		set("LLM_TEMPERATURE", formatFloat(cfg.Temperature))
		set("ALEX_MODEL_TEMPERATURE", formatFloat(cfg.Temperature))
	}

	if cfg.TopP != 0 {
		set("LLM_TOP_P", formatFloat(cfg.TopP))
	}

	if len(cfg.StopSequences) > 0 {
		set("LLM_STOP", strings.Join(cfg.StopSequences, ","))
	}

	set("ALEX_SESSION_DIR", cfg.SessionDir)
	set("ALEX_COST_DIR", cfg.CostDir)
	set("ALEX_CRAFT_MIRROR_DIR", cfg.CraftMirrorDir)

	set("AGENT_PRESET", cfg.AgentPreset)
	set("ALEX_AGENT_PRESET", cfg.AgentPreset)
	set("TOOL_PRESET", cfg.ToolPreset)
	set("ALEX_TOOL_PRESET", cfg.ToolPreset)

	return values
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
