package config

func expandRuntimeFileConfigEnv(lookup EnvLookup, parsed RuntimeFileConfig) RuntimeFileConfig {
	parsed.LLMProvider = expandEnvValue(lookup, parsed.LLMProvider)
	parsed.LLMModel = expandEnvValue(lookup, parsed.LLMModel)
	parsed.LLMVisionModel = expandEnvValue(lookup, parsed.LLMVisionModel)
	parsed.APIKey = expandEnvValue(lookup, parsed.APIKey)
	parsed.ArkAPIKey = expandEnvValue(lookup, parsed.ArkAPIKey)
	parsed.BaseURL = expandEnvValue(lookup, parsed.BaseURL)
	parsed.TavilyAPIKey = expandEnvValue(lookup, parsed.TavilyAPIKey)
	parsed.MoltbookAPIKey = expandEnvValue(lookup, parsed.MoltbookAPIKey)
	parsed.MoltbookBaseURL = expandEnvValue(lookup, parsed.MoltbookBaseURL)
	parsed.Profile = expandEnvValue(lookup, parsed.Profile)
	parsed.Environment = expandEnvValue(lookup, parsed.Environment)
	parsed.SessionDir = expandEnvValue(lookup, parsed.SessionDir)
	parsed.CostDir = expandEnvValue(lookup, parsed.CostDir)
	parsed.SessionStaleAfter = expandEnvValue(lookup, parsed.SessionStaleAfter)
	parsed.AgentPreset = expandEnvValue(lookup, parsed.AgentPreset)
	parsed.ToolPreset = expandEnvValue(lookup, parsed.ToolPreset)
	parsed.Toolset = expandEnvValue(lookup, parsed.Toolset)
	if parsed.Browser != nil {
		parsed.Browser.CDPURL = expandEnvValue(lookup, parsed.Browser.CDPURL)
		parsed.Browser.ChromePath = expandEnvValue(lookup, parsed.Browser.ChromePath)
		parsed.Browser.UserDataDir = expandEnvValue(lookup, parsed.Browser.UserDataDir)
	}
	if parsed.Proactive != nil {
		expandProactiveFileConfigEnv(lookup, parsed.Proactive)
	}
	if parsed.ExternalAgents != nil {
		expandExternalAgentsFileConfigEnv(lookup, parsed.ExternalAgents)
	}

	if len(parsed.StopSequences) > 0 {
		expanded := make([]string, 0, len(parsed.StopSequences))
		for _, seq := range parsed.StopSequences {
			expanded = append(expanded, expandEnvValue(lookup, seq))
		}
		parsed.StopSequences = expanded
	}

	return parsed
}

func expandExternalAgentsFileConfigEnv(_ EnvLookup, _ *ExternalAgentsFileConfig) {}
