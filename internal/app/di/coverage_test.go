package di

import (
	"context"
	"testing"
	"time"

	"alex/internal/app/lifecycle"
	"alex/internal/domain/agent/presets"
	runtimeconfig "alex/internal/shared/config"
)

func TestContainerUtilityMethods(t *testing.T) {
	drained := false
	container := &Container{
		config: Config{
			LLMProvider:   " openai ",
			LLMModel:      " gpt-5-mini ",
			APIKey:        " secret ",
			BaseURL:       " https://example.invalid ",
			SessionDir:    "/tmp/alex-sessions",
			StopSequences: []string{"STOP"},
		},
		Drainables: []lifecycle.Drainable{
			lifecycle.DrainFunc{
				DrainName: "test-drainable",
				Fn: func(ctx context.Context) {
					drained = true
				},
			},
		},
	}

	if container.HasLLMFactory() {
		t.Fatal("HasLLMFactory() = true, want false")
	}
	if factory := container.LLMFactory(); factory != nil {
		t.Fatalf("LLMFactory() = %#v, want nil", factory)
	}
	if health := container.GetModelHealth(); health != nil {
		t.Fatalf("GetModelHealth() = %#v, want nil", health)
	}
	healthy, message := container.AggregateModelHealth()
	if !healthy || message != "No models tracked yet" {
		t.Fatalf("AggregateModelHealth() = (%v, %q), want (true, %q)", healthy, message, "No models tracked yet")
	}
	if sanitized := container.SanitizedModelHealth(); sanitized != nil {
		t.Fatalf("SanitizedModelHealth() = %#v, want nil", sanitized)
	}
	if got := container.DefaultLLMProfile(); got.Provider != "openai" || got.Model != "gpt-5-mini" || got.APIKey != "secret" || got.BaseURL != "https://example.invalid" {
		t.Fatalf("DefaultLLMProfile() = %#v, want trimmed config values", got)
	}
	if got := container.SessionDir(); got != "/tmp/alex-sessions" {
		t.Fatalf("SessionDir() = %q, want /tmp/alex-sessions", got)
	}
	if err := container.Drain(context.Background()); err != nil {
		t.Fatalf("Drain() error = %v", err)
	}
	if !drained {
		t.Fatal("Drain() did not invoke drainable")
	}
	if err := container.Shutdown(); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	alt := &AlternateCoordinator{}
	if err := alt.Shutdown(); err != nil {
		t.Fatalf("AlternateCoordinator.Shutdown() error = %v", err)
	}
}

func TestConfigFromRuntimeConfig(t *testing.T) {
	runtime := runtimeconfig.RuntimeConfig{
		LLMSettings: runtimeconfig.LLMSettings{
			LLMProvider:        "anthropic",
			LLMModel:           "claude-sonnet",
			LLMVisionModel:     "vision-model",
			APIKey:             "api-key",
			ArkAPIKey:          "ark-key",
			BaseURL:            "https://llm.example",
			MaxTokens:          4096,
			LLMCacheSize:       32,
			LLMCacheTTLSeconds: 90,
			Temperature:        0.4,
			TemperatureProvided: true,
			TopP:               0.8,
			StopSequences:      []string{"DONE"},
		},
		RateLimitSettings: runtimeconfig.RateLimitSettings{
			UserRateLimitRPS:   1.5,
			UserRateLimitBurst: 3,
			KimiRateLimitRPS:   2.5,
			KimiRateLimitBurst: 5,
		},
		IntegrationKeys: runtimeconfig.IntegrationKeys{
			TavilyAPIKey: "tavily-key",
		},
		StorageSettings: runtimeconfig.StorageSettings{
			SessionDir:               "/tmp/sessions",
			CostDir:                  "/tmp/costs",
			SessionStaleAfterSeconds: 7200,
		},
		MaxIterations:     12,
		ToolMaxConcurrent: 4,
		AgentPreset:       "planner",
		ToolPreset:        "safe",
		Toolset:           "coding",
		Profile:           "prod",
		Browser: runtimeconfig.BrowserConfig{
			Connector:      "cdp",
			CDPURL:         "ws://browser.example",
			ChromePath:     "/Applications/Chrome",
			Headless:       true,
			UserDataDir:    "/tmp/browser",
			TimeoutSeconds: 17,
			BridgeListen:   "127.0.0.1:1111",
			BridgeToken:    "bridge-token",
		},
	}

	cfg := ConfigFromRuntimeConfig(runtime)
	if cfg.LLMProvider != "anthropic" || cfg.LLMModel != "claude-sonnet" || cfg.LLMVisionModel != "vision-model" {
		t.Fatalf("ConfigFromRuntimeConfig() basic LLM fields = %#v", cfg)
	}
	if cfg.ToolMode != string(presets.ToolModeCLI) {
		t.Fatalf("ToolMode = %q, want %q", cfg.ToolMode, string(presets.ToolModeCLI))
	}
	if cfg.BrowserConfig.Timeout != 17*time.Second {
		t.Fatalf("Browser timeout = %v, want %v", cfg.BrowserConfig.Timeout, 17*time.Second)
	}
	if cfg.SessionStaleAfter != 2*time.Hour {
		t.Fatalf("SessionStaleAfter = %v, want 2h", cfg.SessionStaleAfter)
	}
	if len(cfg.StopSequences) != 1 || cfg.StopSequences[0] != "DONE" {
		t.Fatalf("StopSequences = %#v, want copied slice", cfg.StopSequences)
	}

	runtime.StopSequences[0] = "MUTATED"
	if cfg.StopSequences[0] != "DONE" {
		t.Fatalf("StopSequences should be cloned, got %#v", cfg.StopSequences)
	}
}

func TestConvertTeamConfigsAndBuildAgentAppConfig(t *testing.T) {
	teams := convertTeamConfigs([]runtimeconfig.TeamConfig{
		{
			Name:        "incident",
			Description: "Incident response team",
			Roles: []runtimeconfig.TeamRoleConfig{
				{
					Name:              "investigator",
					AgentType:         "codex",
					CapabilityProfile: "debug",
					TargetCLI:         "codex",
					PromptTemplate:    "Investigate",
					ExecutionMode:     "autonomous",
					AutonomyLevel:     "high",
					WorkspaceMode:     "shared",
					Config:            map[string]string{"key": "value"},
					InheritContext:    true,
				},
			},
			Stages: []runtimeconfig.TeamStageConfig{
				{Name: "investigate", Roles: []string{"investigator"}},
			},
		},
	})
	if len(teams) != 1 || len(teams[0].Roles) != 1 || len(teams[0].Stages) != 1 {
		t.Fatalf("convertTeamConfigs() = %#v, want one team with one role and stage", teams)
	}
	if teams[0].Roles[0].Name != "investigator" || teams[0].Roles[0].Config["key"] != "value" {
		t.Fatalf("converted roles = %#v, want copied role fields", teams[0].Roles)
	}

	builder := newContainerBuilder(Config{
		LLMProvider:                "openai",
		LLMModel:                   "gpt-5",
		LLMVisionModel:             "gpt-5-vision",
		APIKey:                     "api-key",
		BaseURL:                    "https://api.example",
		MaxTokens:                  2048,
		MaxIterations:              9,
		ToolMaxConcurrent:          3,
		Temperature:                0.2,
		TemperatureProvided:        true,
		TopP:                       0.9,
		StopSequences:              []string{"STOP"},
		AgentPreset:                "assistant",
		ToolPreset:                 "safe",
		ToolMode:                   "cli",
		EnvironmentSummary:         "summary",
		EnvironmentSummaryProvider: func() string { return "summary-provider" },
		SessionStaleAfter:          15 * time.Minute,
		ToolPolicy:                 runtimeconfig.RuntimeConfig{}.ToolPolicy,
		Proactive:                  runtimeconfig.ProactiveConfig{},
		ExternalAgents:             runtimeconfig.ExternalAgentsConfig{MaxParallelAgents: 7},
	})

	appCfg := builder.buildAgentAppConfig()
	if appCfg.LLMProvider != "openai" || appCfg.LLMModel != "gpt-5" || appCfg.MaxBackgroundTasks != 7 {
		t.Fatalf("buildAgentAppConfig() = %#v, want mapped config values", appCfg)
	}
	if len(appCfg.StopSequences) != 1 || appCfg.StopSequences[0] != "STOP" {
		t.Fatalf("StopSequences = %#v, want copied slice", appCfg.StopSequences)
	}
	builder.config.StopSequences[0] = "MUTATED"
	if appCfg.StopSequences[0] != "STOP" {
		t.Fatalf("StopSequences should be cloned, got %#v", appCfg.StopSequences)
	}
	if appCfg.ToolMode != "cli" || appCfg.EnvironmentSummaryProvider() != "summary-provider" {
		t.Fatalf("buildAgentAppConfig() did not preserve tool mode/provider: %#v", appCfg)
	}
}
