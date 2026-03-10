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

func expandExternalAgentsFileConfigEnv(lookup EnvLookup, external *ExternalAgentsFileConfig) {
	if external == nil {
		return
	}
	if external.ClaudeCode != nil {
		cc := external.ClaudeCode
		cc.Binary = expandEnvValue(lookup, cc.Binary)
		cc.DefaultModel = expandEnvValue(lookup, cc.DefaultModel)
		cc.DefaultMode = expandEnvValue(lookup, cc.DefaultMode)
		cc.Timeout = expandEnvValue(lookup, cc.Timeout)
		if len(cc.AutonomousAllowedTools) > 0 {
			tools := make([]string, 0, len(cc.AutonomousAllowedTools))
			for _, tool := range cc.AutonomousAllowedTools {
				tools = append(tools, expandEnvValue(lookup, tool))
			}
			cc.AutonomousAllowedTools = tools
		}
		if len(cc.PlanAllowedTools) > 0 {
			tools := make([]string, 0, len(cc.PlanAllowedTools))
			for _, tool := range cc.PlanAllowedTools {
				tools = append(tools, expandEnvValue(lookup, tool))
			}
			cc.PlanAllowedTools = tools
		}
		if len(cc.Env) > 0 {
			expanded := make(map[string]string, len(cc.Env))
			for key, value := range cc.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			cc.Env = expanded
		}
	}
	if external.Codex != nil {
		cx := external.Codex
		cx.Binary = expandEnvValue(lookup, cx.Binary)
		cx.DefaultModel = expandEnvValue(lookup, cx.DefaultModel)
		cx.ApprovalPolicy = expandEnvValue(lookup, cx.ApprovalPolicy)
		cx.Sandbox = expandEnvValue(lookup, cx.Sandbox)
		cx.PlanApprovalPolicy = expandEnvValue(lookup, cx.PlanApprovalPolicy)
		cx.PlanSandbox = expandEnvValue(lookup, cx.PlanSandbox)
		cx.Timeout = expandEnvValue(lookup, cx.Timeout)
		if len(cx.Env) > 0 {
			expanded := make(map[string]string, len(cx.Env))
			for key, value := range cx.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			cx.Env = expanded
		}
	}
	if external.Kimi != nil {
		km := external.Kimi
		km.Binary = expandEnvValue(lookup, km.Binary)
		km.DefaultModel = expandEnvValue(lookup, km.DefaultModel)
		km.ApprovalPolicy = expandEnvValue(lookup, km.ApprovalPolicy)
		km.Sandbox = expandEnvValue(lookup, km.Sandbox)
		km.PlanApprovalPolicy = expandEnvValue(lookup, km.PlanApprovalPolicy)
		km.PlanSandbox = expandEnvValue(lookup, km.PlanSandbox)
		km.Timeout = expandEnvValue(lookup, km.Timeout)
		if len(km.Env) > 0 {
			expanded := make(map[string]string, len(km.Env))
			for key, value := range km.Env {
				expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
			}
			km.Env = expanded
		}
	}
	for i := range external.Teams {
		team := &external.Teams[i]
		team.Name = expandEnvValue(lookup, team.Name)
		team.Description = expandEnvValue(lookup, team.Description)
		for j := range team.Roles {
			role := &team.Roles[j]
			role.Name = expandEnvValue(lookup, role.Name)
			role.AgentType = expandEnvValue(lookup, role.AgentType)
			role.CapabilityProfile = expandEnvValue(lookup, role.CapabilityProfile)
			role.TargetCLI = expandEnvValue(lookup, role.TargetCLI)
			role.PromptTemplate = expandEnvValue(lookup, role.PromptTemplate)
			role.ExecutionMode = expandEnvValue(lookup, role.ExecutionMode)
			role.AutonomyLevel = expandEnvValue(lookup, role.AutonomyLevel)
			role.WorkspaceMode = expandEnvValue(lookup, role.WorkspaceMode)
			if len(role.Config) > 0 {
				expanded := make(map[string]string, len(role.Config))
				for key, value := range role.Config {
					expanded[expandEnvValue(lookup, key)] = expandEnvValue(lookup, value)
				}
				role.Config = expanded
			}
		}
		for j := range team.Stages {
			stage := &team.Stages[j]
			stage.Name = expandEnvValue(lookup, stage.Name)
			if len(stage.Roles) > 0 {
				roles := make([]string, 0, len(stage.Roles))
				for _, roleName := range stage.Roles {
					roles = append(roles, expandEnvValue(lookup, roleName))
				}
				stage.Roles = roles
			}
		}
	}
}

func convertTeamFileConfigs(raw []TeamFileConfig) []TeamConfig {
	if len(raw) == 0 {
		return nil
	}
	teams := make([]TeamConfig, 0, len(raw))
	for _, team := range raw {
		roles := make([]TeamRoleConfig, 0, len(team.Roles))
		for _, role := range team.Roles {
			inheritContext := false
			if role.InheritContext != nil {
				inheritContext = *role.InheritContext
			}
			roles = append(roles, TeamRoleConfig{
				Name:              role.Name,
				AgentType:         role.AgentType,
				CapabilityProfile: role.CapabilityProfile,
				TargetCLI:         role.TargetCLI,
				PromptTemplate:    role.PromptTemplate,
				ExecutionMode:     role.ExecutionMode,
				AutonomyLevel:     role.AutonomyLevel,
				WorkspaceMode:     role.WorkspaceMode,
				Config:            cloneStringMap(role.Config),
				InheritContext:    inheritContext,
			})
		}
		stages := make([]TeamStageConfig, 0, len(team.Stages))
		for _, stage := range team.Stages {
			stages = append(stages, TeamStageConfig{
				Name:  stage.Name,
				Roles: append([]string(nil), stage.Roles...),
			})
		}
		teams = append(teams, TeamConfig{
			Name:        team.Name,
			Description: team.Description,
			Roles:       roles,
			Stages:      stages,
		})
	}
	return teams
}
