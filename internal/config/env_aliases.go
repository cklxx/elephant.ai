package config

// DefaultEnvAliases returns the canonical alias map used across binaries to resolve
// legacy environment variable names.
func DefaultEnvAliases() map[string][]string {
	aliases := map[string][]string{
		"LLM_PROVIDER":               {"ALEX_LLM_PROVIDER"},
		"LLM_MODEL":                  {"ALEX_LLM_MODEL"},
		"LLM_BASE_URL":               {"ALEX_BASE_URL"},
		"LLM_MAX_TOKENS":             {"ALEX_LLM_MAX_TOKENS"},
		"LLM_MAX_ITERATIONS":         {"ALEX_LLM_MAX_ITERATIONS"},
		"LLM_TEMPERATURE":            {"ALEX_LLM_TEMPERATURE"},
		"LLM_TOP_P":                  {"ALEX_LLM_TOP_P"},
		"LLM_STOP":                   {"ALEX_LLM_STOP"},
		"USER_LLM_RPS":               {"ALEX_USER_LLM_RPS"},
		"USER_LLM_BURST":             {"ALEX_USER_LLM_BURST"},
		"TAVILY_API_KEY":             {"ALEX_TAVILY_API_KEY"},
		"ARK_API_KEY":                {"ALEX_ARK_API_KEY"},
		"SEEDREAM_TEXT_ENDPOINT_ID":  {"ALEX_SEEDREAM_TEXT_ENDPOINT_ID"},
		"SEEDREAM_IMAGE_ENDPOINT_ID": {"ALEX_SEEDREAM_IMAGE_ENDPOINT_ID"},
		"SEEDREAM_TEXT_MODEL":        {"ALEX_SEEDREAM_TEXT_MODEL"},
		"SEEDREAM_IMAGE_MODEL":       {"ALEX_SEEDREAM_IMAGE_MODEL"},
		"SEEDREAM_VISION_MODEL":      {"ALEX_SEEDREAM_VISION_MODEL"},
		"SEEDREAM_VIDEO_MODEL":       {"ALEX_SEEDREAM_VIDEO_MODEL"},
		"SANDBOX_BASE_URL":           {"ALEX_SANDBOX_BASE_URL"},
		"AGENT_PRESET":               {"ALEX_AGENT_PRESET"},
		"TOOL_PRESET":                {"ALEX_TOOL_PRESET"},
		"PORT":                       {"ALEX_SERVER_PORT"},
		"ENABLE_MCP":                 {"ALEX_ENABLE_MCP"},
		"POSTHOG_API_KEY":            {"ALEX_POSTHOG_API_KEY", "NEXT_PUBLIC_POSTHOG_KEY"},
		"POSTHOG_HOST":               {"ALEX_POSTHOG_HOST", "NEXT_PUBLIC_POSTHOG_HOST"},
		"CORS_ALLOWED_ORIGINS":       {"ALEX_ALLOWED_ORIGINS", "ALEX_CORS_ALLOWED_ORIGINS"},
		"CONFIG_ADMIN_STORE_PATH":    {"ALEX_CONFIG_STORE_PATH"},
		"CONFIG_ADMIN_CACHE_TTL":     {"ALEX_CONFIG_CACHE_TTL"},
		"ALEX_ENV":                   {"ENVIRONMENT", "NODE_ENV"},
		"ALEX_VERBOSE":               {"VERBOSE"},
	}

	copy := make(map[string][]string, len(aliases))
	for key, list := range aliases {
		copy[key] = append([]string(nil), list...)
	}
	return copy
}

// DefaultEnvLookupWithAliases composes DefaultEnvLookup with DefaultEnvAliases.
func DefaultEnvLookupWithAliases() EnvLookup {
	return AliasEnvLookup(DefaultEnvLookup, DefaultEnvAliases())
}
