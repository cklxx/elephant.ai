package config

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

type envMap map[string]string

func (e envMap) Lookup(key string) (string, bool) {
	val, ok := e[key]
	if !ok || val == "" {
		return "", false
	}
	return val, true
}

func TestLoadDefaults(t *testing.T) {
	cfg, meta, err := Load(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return nil, os.ErrNotExist }),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "mock" {
		t.Fatalf("expected provider to fall back to mock without api key, got %q", cfg.LLMProvider)
	}
	if cfg.LLMSmallProvider != "mock" {
		t.Fatalf("expected small model provider to fall back to mock without api key, got %q", cfg.LLMSmallProvider)
	}
	if cfg.TemperatureProvided {
		t.Fatalf("expected temperature to be marked as not provided")
	}
	if got := meta.Source("llm_provider"); got != SourceDefault {
		t.Fatalf("expected default provider source, got %s", got)
	}
	if cfg.Environment != "development" {
		t.Fatalf("expected default environment 'development', got %q", cfg.Environment)
	}
	if cfg.Verbose {
		t.Fatal("expected verbose default to be false")
	}
	if !cfg.FollowTranscript || !cfg.FollowStream {
		t.Fatalf("expected follow defaults to be true, got transcript=%v stream=%v", cfg.FollowTranscript, cfg.FollowStream)
	}
}

func TestLoadFromFile(t *testing.T) {
	fileData := []byte(`
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  llm_small_provider: "openai"
  llm_small_model: "gpt-4o-mini"
  llm_vision_model: "gpt-4o-mini"
  api_key: "sk-test"
  tavily_api_key: "file-tavily"
  ark_api_key: "file-ark"
  seedream_text_endpoint_id: "file-text-id"
  seedream_image_endpoint_id: "file-image-id"
  seedream_text_model: "file-text-model"
  seedream_image_model: "file-image-model"
  seedream_vision_model: "file-vision-model"
  seedream_video_model: "file-video-model"
  environment: "staging"
  verbose: true
  disable_tui: true
  follow_transcript: false
  follow_stream: false
  temperature: 0
  max_iterations: 200
  stop_sequences:
    - "DONE"
  session_dir: "~/sessions"
  agent_preset: "designer"
  tool_preset: "safe"
`)
	cfg, meta, err := Load(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "openai" || cfg.LLMModel != "gpt-4o" {
		t.Fatalf("unexpected model/provider from file: %#v", cfg)
	}
	if cfg.LLMSmallProvider != "openai" || cfg.LLMSmallModel != "gpt-4o-mini" {
		t.Fatalf("unexpected small model from file: provider=%s model=%s", cfg.LLMSmallProvider, cfg.LLMSmallModel)
	}
	if cfg.LLMVisionModel != "gpt-4o-mini" {
		t.Fatalf("expected llm_vision_model from file, got %q", cfg.LLMVisionModel)
	}
	if !cfg.TemperatureProvided || cfg.Temperature != 0 {
		t.Fatalf("expected explicit zero temperature to be preserved: %+v", cfg)
	}
	if cfg.MaxIterations != 200 {
		t.Fatalf("expected max_iterations=200, got %d", cfg.MaxIterations)
	}
	if len(cfg.StopSequences) != 1 || cfg.StopSequences[0] != "DONE" {
		t.Fatalf("unexpected stop sequences: %#v", cfg.StopSequences)
	}
	if cfg.SessionDir != "~/sessions" {
		t.Fatalf("unexpected session dir: %s", cfg.SessionDir)
	}
	if cfg.TavilyAPIKey != "file-tavily" {
		t.Fatalf("expected tavily key from file, got %q", cfg.TavilyAPIKey)
	}
	if cfg.SeedreamTextEndpointID != "file-text-id" || cfg.SeedreamImageEndpointID != "file-image-id" {
		t.Fatalf("expected seedream endpoints from file, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
	}
	if cfg.ArkAPIKey != "file-ark" {
		t.Fatalf("expected ark API key from file, got %q", cfg.ArkAPIKey)
	}
	if cfg.SeedreamTextModel != "file-text-model" || cfg.SeedreamImageModel != "file-image-model" || cfg.SeedreamVisionModel != "file-vision-model" || cfg.SeedreamVideoModel != "file-video-model" {
		t.Fatalf("expected seedream models from file, got %q/%q/%q/%q", cfg.SeedreamTextModel, cfg.SeedreamImageModel, cfg.SeedreamVisionModel, cfg.SeedreamVideoModel)
	}
	if cfg.Environment != "staging" {
		t.Fatalf("expected environment from file, got %q", cfg.Environment)
	}
	if !cfg.Verbose {
		t.Fatal("expected verbose true from file")
	}
	if !cfg.DisableTUI {
		t.Fatal("expected disable_tui true from file")
	}
	if cfg.FollowTranscript || cfg.FollowStream {
		t.Fatalf("expected follow defaults overridden to false, got transcript=%v stream=%v", cfg.FollowTranscript, cfg.FollowStream)
	}
	if cfg.AgentPreset != "designer" {
		t.Fatalf("expected agent preset from file, got %q", cfg.AgentPreset)
	}
	if cfg.ToolPreset != "safe" {
		t.Fatalf("expected tool preset from file, got %q", cfg.ToolPreset)
	}
	if meta.Source("tavily_api_key") != SourceFile {
		t.Fatalf("expected tavily key source from file, got %s", meta.Source("tavily_api_key"))
	}
	if meta.Source("llm_vision_model") != SourceFile {
		t.Fatalf("expected vision model source from file, got %s", meta.Source("llm_vision_model"))
	}
	if meta.Source("llm_small_provider") != SourceFile || meta.Source("llm_small_model") != SourceFile {
		t.Fatalf("expected small model source from file")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceFile || meta.Source("seedream_image_endpoint_id") != SourceFile {
		t.Fatalf("expected seedream endpoints source from file")
	}
	if meta.Source("seedream_video_model") != SourceFile {
		t.Fatalf("expected seedream video model source from file")
	}
	if meta.Source("agent_preset") != SourceFile || meta.Source("tool_preset") != SourceFile {
		t.Fatalf("expected preset sources from file")
	}
	if meta.Source("temperature") != SourceFile {
		t.Fatalf("expected temperature source to be file, got %s", meta.Source("temperature"))
	}
	if meta.Source("follow_transcript") != SourceFile {
		t.Fatalf("expected follow_transcript source to be file, got %s", meta.Source("follow_transcript"))
	}
	if meta.Source("follow_stream") != SourceFile {
		t.Fatalf("expected follow_stream source to be file, got %s", meta.Source("follow_stream"))
	}
}

func TestLoadHonorsEnvConfigPath(t *testing.T) {
	expectedPath := "/tmp/alex-config-test.yaml"
	fileData := []byte(`
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  api_key: "sk-test"
`)

	cfg, _, err := Load(
		WithEnv(envMap{"ALEX_CONFIG_PATH": expectedPath}.Lookup),
		WithHomeDir(func() (string, error) {
			t.Fatalf("unexpected home dir lookup")
			return "", nil
		}),
		WithFileReader(func(path string) ([]byte, error) {
			if path != expectedPath {
				t.Fatalf("expected config path %q, got %q", expectedPath, path)
			}
			return fileData, nil
		}),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "openai" || cfg.LLMModel != "gpt-4o" {
		t.Fatalf("unexpected config loaded from env path: %#v", cfg)
	}
}

func TestLoadConfigPathOverrideWinsOverEnv(t *testing.T) {
	explicitPath := "/tmp/alex-explicit-config.yaml"
	fileData := []byte(`
runtime:
  llm_provider: "openai"
  llm_model: "gpt-4o"
  api_key: "sk-test"
`)

	_, _, err := Load(
		WithEnv(envMap{"ALEX_CONFIG_PATH": "/tmp/ignored.yaml"}.Lookup),
		WithConfigPath(explicitPath),
		WithFileReader(func(path string) ([]byte, error) {
			if path != explicitPath {
				t.Fatalf("expected explicit config path %q, got %q", explicitPath, path)
			}
			return fileData, nil
		}),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
}

func TestEnvInterpolation(t *testing.T) {
	fileData := []byte(`
runtime:
  api_key: "${API_KEY}"
  tavily_api_key: "${TAVILY_KEY}"
  llm_model: "${MODEL_NAME}"
  llm_vision_model: "vision-${VISION_SUFFIX}"
`)
	cfg, meta, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"API_KEY":       "env-api-key",
			"TAVILY_KEY":    "env-tavily",
			"MODEL_NAME":    "env-model",
			"VISION_SUFFIX": "v1",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.APIKey != "env-api-key" {
		t.Fatalf("expected api key interpolation, got %q", cfg.APIKey)
	}
	if cfg.TavilyAPIKey != "env-tavily" {
		t.Fatalf("expected tavily key interpolation, got %q", cfg.TavilyAPIKey)
	}
	if cfg.LLMModel != "env-model" {
		t.Fatalf("expected llm_model interpolation, got %q", cfg.LLMModel)
	}
	if cfg.LLMVisionModel != "vision-v1" {
		t.Fatalf("expected llm_vision_model interpolation, got %q", cfg.LLMVisionModel)
	}
	if meta.Source("api_key") != SourceFile {
		t.Fatalf("expected file source for api_key, got %s", meta.Source("api_key"))
	}
	if meta.Source("tavily_api_key") != SourceFile {
		t.Fatalf("expected file source for tavily_api_key, got %s", meta.Source("tavily_api_key"))
	}
}

func TestAutoProviderResolvesFromEnv(t *testing.T) {
	fileData := []byte(`
runtime:
  llm_provider: "auto"
  llm_model: "claude-3-5-sonnet"
`)

	cfg, _, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"ANTHROPIC_API_KEY": "anthropic-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Fatalf("expected auto provider to resolve to anthropic, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "anthropic-key" {
		t.Fatalf("expected api key to resolve from env, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected anthropic base url default, got %q", cfg.BaseURL)
	}
}

func TestCLIProviderResolvesFromCodexCLI(t *testing.T) {
	fileData := []byte(`
runtime:
  llm_provider: "cli"
`)

	readFile := func(path string) ([]byte, error) {
		switch path {
		case "/tmp/config.yaml":
			return fileData, nil
		case "/home/test/.codex/auth.json":
			return []byte(`{"tokens":{"access_token":"codex-token"}}`), nil
		case "/home/test/.codex/config.toml":
			return []byte(`model = "gpt-5.2-codex"`), nil
		default:
			return nil, os.ErrNotExist
		}
	}

	cfg, meta, err := Load(
		WithConfigPath("/tmp/config.yaml"),
		WithFileReader(readFile),
		WithHomeDir(func() (string, error) { return "/home/test", nil }),
		WithEnv(envMap{}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "codex" {
		t.Fatalf("expected provider codex, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "codex-token" {
		t.Fatalf("expected codex token from CLI, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != codexCLIBaseURL {
		t.Fatalf("expected codex base url, got %q", cfg.BaseURL)
	}
	if cfg.LLMModel != "gpt-5.2-codex" {
		t.Fatalf("expected codex model from CLI, got %q", cfg.LLMModel)
	}
	if meta.Source("api_key") != SourceCodexCLI {
		t.Fatalf("expected codex cli source for api_key, got %s", meta.Source("api_key"))
	}
	if meta.Source("llm_provider") != SourceCodexCLI {
		t.Fatalf("expected codex cli source for llm_provider, got %s", meta.Source("llm_provider"))
	}
}

func TestAnthropicOAuthTokenPreferred(t *testing.T) {
	fileData := []byte(`
runtime:
  llm_provider: "auto"
`)

	cfg, meta, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"CLAUDE_CODE_OAUTH_TOKEN": "oauth-token",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Fatalf("expected auto provider to resolve to anthropic, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "oauth-token" {
		t.Fatalf("expected oauth token to resolve from env, got %q", cfg.APIKey)
	}
	if meta.Source("api_key") != SourceClaudeCLI {
		t.Fatalf("expected claude cli source for api_key, got %s", meta.Source("api_key"))
	}
}

func TestAnthropicProviderUsesEnvKey(t *testing.T) {
	fileData := []byte(`
runtime:
  llm_provider: "anthropic"
  llm_model: "claude-3-5-sonnet"
`)

	cfg, _, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"ANTHROPIC_API_KEY": "anthropic-key",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMProvider != "anthropic" {
		t.Fatalf("expected provider to stay anthropic, got %q", cfg.LLMProvider)
	}
	if cfg.APIKey != "anthropic-key" {
		t.Fatalf("expected api key to resolve from env, got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://api.anthropic.com/v1" {
		t.Fatalf("expected anthropic base url default, got %q", cfg.BaseURL)
	}
}

func TestEnvOverridesFile(t *testing.T) {
	fileData := []byte(`
runtime:
  temperature: 0.1
  tavily_api_key: "file-key"
`)
	cfg, meta, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"LLM_TEMPERATURE":            "0",
			"LLM_MODEL":                  "env-model",
			"LLM_VISION_MODEL":           "env-vision-model",
			"TAVILY_API_KEY":             "env-tavily",
			"ARK_API_KEY":                "env-ark",
			"SEEDREAM_TEXT_ENDPOINT_ID":  "env-text",
			"SEEDREAM_IMAGE_ENDPOINT_ID": "env-image",
			"SEEDREAM_TEXT_MODEL":        "env-text-model",
			"SEEDREAM_IMAGE_MODEL":       "env-image-model",
			"SEEDREAM_VISION_MODEL":      "env-vision-model",
			"SEEDREAM_VIDEO_MODEL":       "env-video-model",
			"ALEX_ENV":                   "production",
			"ALEX_VERBOSE":               "yes",
			"ALEX_NO_TUI":                "true",
			"ALEX_TUI_FOLLOW_TRANSCRIPT": "false",
			"ALEX_TUI_FOLLOW_STREAM":     "false",
			"ALEX_REASONING_STREAM":      "true",
			"AGENT_PRESET":               "designer",
			"TOOL_PRESET":                "full",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMModel != "env-model" {
		t.Fatalf("expected env model override, got %s", cfg.LLMModel)
	}
	if cfg.LLMVisionModel != "env-vision-model" {
		t.Fatalf("expected env vision model override, got %s", cfg.LLMVisionModel)
	}
	if cfg.Temperature != 0 || !cfg.TemperatureProvided {
		t.Fatalf("expected env zero temperature override, got %+v", cfg)
	}
	if cfg.TavilyAPIKey != "env-tavily" {
		t.Fatalf("expected tavily key from env, got %q", cfg.TavilyAPIKey)
	}
	if cfg.SeedreamTextEndpointID != "env-text" || cfg.SeedreamImageEndpointID != "env-image" {
		t.Fatalf("expected seedream endpoints from env, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
	}
	if cfg.ArkAPIKey != "env-ark" {
		t.Fatalf("expected ark api key from env, got %q", cfg.ArkAPIKey)
	}
	if cfg.SeedreamTextModel != "env-text-model" || cfg.SeedreamImageModel != "env-image-model" || cfg.SeedreamVisionModel != "env-vision-model" || cfg.SeedreamVideoModel != "env-video-model" {
		t.Fatalf("expected seedream models from env, got %q/%q/%q/%q", cfg.SeedreamTextModel, cfg.SeedreamImageModel, cfg.SeedreamVisionModel, cfg.SeedreamVideoModel)
	}
	if cfg.Environment != "production" {
		t.Fatalf("expected environment from env, got %q", cfg.Environment)
	}
	if !cfg.Verbose {
		t.Fatal("expected verbose true from env override")
	}
	if !cfg.DisableTUI {
		t.Fatal("expected disable TUI true from env override")
	}
	if cfg.FollowTranscript || cfg.FollowStream {
		t.Fatalf("expected follow toggles false from env override, got transcript=%v stream=%v", cfg.FollowTranscript, cfg.FollowStream)
	}
	if cfg.AgentPreset != "designer" || cfg.ToolPreset != "full" {
		t.Fatalf("expected presets from env, got %q/%q", cfg.AgentPreset, cfg.ToolPreset)
	}
	if meta.Source("tavily_api_key") != SourceEnv {
		t.Fatalf("expected env source for tavily key, got %s", meta.Source("tavily_api_key"))
	}
	if meta.Source("llm_vision_model") != SourceEnv {
		t.Fatalf("expected env source for vision model, got %s", meta.Source("llm_vision_model"))
	}
	if meta.Source("ark_api_key") != SourceEnv {
		t.Fatalf("expected env source for ark api key")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceEnv || meta.Source("seedream_image_endpoint_id") != SourceEnv {
		t.Fatalf("expected env source for seedream endpoints")
	}
	if meta.Source("seedream_text_model") != SourceEnv || meta.Source("seedream_image_model") != SourceEnv || meta.Source("seedream_vision_model") != SourceEnv || meta.Source("seedream_video_model") != SourceEnv {
		t.Fatalf("expected env source for seedream models")
	}
	if meta.Source("temperature") != SourceEnv {
		t.Fatalf("expected env source for temperature, got %s", meta.Source("temperature"))
	}
	if meta.Source("environment") != SourceEnv {
		t.Fatalf("expected env source for environment, got %s", meta.Source("environment"))
	}
	if meta.Source("verbose") != SourceEnv {
		t.Fatalf("expected env source for verbose, got %s", meta.Source("verbose"))
	}
	if meta.Source("disable_tui") != SourceEnv {
		t.Fatalf("expected env source for disable_tui, got %s", meta.Source("disable_tui"))
	}
	if meta.Source("follow_transcript") != SourceEnv {
		t.Fatalf("expected env source for follow_transcript, got %s", meta.Source("follow_transcript"))
	}
	if meta.Source("follow_stream") != SourceEnv {
		t.Fatalf("expected env source for follow_stream, got %s", meta.Source("follow_stream"))
	}
	if meta.Source("agent_preset") != SourceEnv || meta.Source("tool_preset") != SourceEnv {
		t.Fatalf("expected env source for presets")
	}
}

func TestLoadNormalizesRuntimeConfig(t *testing.T) {
	overrides := Overrides{
		LLMProvider:   ptrString(" openai "),
		LLMModel:      ptrString(" gpt-4o "),
		APIKey:        ptrString(" sk-test "),
		StopSequences: ptrStringSlice([]string{" STOP ", "STOP", " ", "", "\nDONE\n", "DONE"}),
		AgentPreset:   ptrString(" coder "),
		ToolPreset:    ptrString(" safe "),
	}

	cfg, _, err := Load(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return nil, os.ErrNotExist }),
		WithOverrides(overrides),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.LLMProvider != "openai" {
		t.Fatalf("expected trimmed provider, got %q", cfg.LLMProvider)
	}
	if cfg.LLMModel != "gpt-4o" {
		t.Fatalf("expected trimmed model, got %q", cfg.LLMModel)
	}
	if cfg.APIKey != "sk-test" {
		t.Fatalf("expected trimmed API key, got %q", cfg.APIKey)
	}
	if cfg.AgentPreset != "coder" || cfg.ToolPreset != "safe" {
		t.Fatalf("expected trimmed presets, got agent=%q tool=%q", cfg.AgentPreset, cfg.ToolPreset)
	}
	if len(cfg.StopSequences) != 2 || cfg.StopSequences[0] != "STOP" || cfg.StopSequences[1] != "DONE" {
		t.Fatalf("expected cleaned stop sequences, got %#v", cfg.StopSequences)
	}
}

func TestOverridesTakePriority(t *testing.T) {
	overrideTemp := 1.0
	overrideModel := "override-model"
	overrideVisionModel := "override-vision-model"
	overrideTavily := "override-tavily"
	overrideArk := "override-ark"
	overrideSeedreamText := "override-text"
	overrideSeedreamImage := "override-image"
	overrideSeedreamTextModel := "override-text-model"
	overrideSeedreamImageModel := "override-image-model"
	overrideSeedreamVisionModel := "override-vision-model"
	overrideSeedreamVideoModel := "override-video-model"
	overrideEnv := "qa"
	overrideVerbose := true
	overrideFollowTranscript := false
	overrideFollowStream := false
	overrideAgentPreset := "designer"
	overrideToolPreset := "read-only"
	cfg, meta, err := Load(
		WithEnv(envMap{"LLM_MODEL": "env-model"}.Lookup),
		WithOverrides(Overrides{
			LLMModel:                &overrideModel,
			LLMVisionModel:          &overrideVisionModel,
			Temperature:             &overrideTemp,
			TavilyAPIKey:            &overrideTavily,
			ArkAPIKey:               &overrideArk,
			SeedreamTextEndpointID:  &overrideSeedreamText,
			SeedreamImageEndpointID: &overrideSeedreamImage,
			SeedreamTextModel:       &overrideSeedreamTextModel,
			SeedreamImageModel:      &overrideSeedreamImageModel,
			SeedreamVisionModel:     &overrideSeedreamVisionModel,
			SeedreamVideoModel:      &overrideSeedreamVideoModel,
			Environment:             &overrideEnv,
			Verbose:                 &overrideVerbose,
			FollowTranscript:        &overrideFollowTranscript,
			FollowStream:            &overrideFollowStream,
			AgentPreset:             &overrideAgentPreset,
			ToolPreset:              &overrideToolPreset,
		}),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMModel != "override-model" {
		t.Fatalf("expected override model, got %s", cfg.LLMModel)
	}
	if cfg.LLMVisionModel != overrideVisionModel {
		t.Fatalf("expected override vision model, got %s", cfg.LLMVisionModel)
	}
	if cfg.Temperature != 1.0 || !cfg.TemperatureProvided {
		t.Fatalf("expected override temperature 1.0, got %+v", cfg)
	}
	if cfg.TavilyAPIKey != "override-tavily" {
		t.Fatalf("expected override tavily key, got %q", cfg.TavilyAPIKey)
	}
	if cfg.ArkAPIKey != overrideArk {
		t.Fatalf("expected override ark api key, got %q", cfg.ArkAPIKey)
	}
	if cfg.SeedreamTextEndpointID != overrideSeedreamText || cfg.SeedreamImageEndpointID != overrideSeedreamImage {
		t.Fatalf("expected override seedream endpoints, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
	}
	if cfg.SeedreamTextModel != overrideSeedreamTextModel || cfg.SeedreamImageModel != overrideSeedreamImageModel || cfg.SeedreamVisionModel != overrideSeedreamVisionModel || cfg.SeedreamVideoModel != overrideSeedreamVideoModel {
		t.Fatalf("expected override seedream models, got %q/%q/%q/%q", cfg.SeedreamTextModel, cfg.SeedreamImageModel, cfg.SeedreamVisionModel, cfg.SeedreamVideoModel)
	}
	if cfg.Environment != "qa" {
		t.Fatalf("expected override environment, got %q", cfg.Environment)
	}
	if !cfg.Verbose {
		t.Fatal("expected override verbose true")
	}
	if cfg.FollowTranscript || cfg.FollowStream {
		t.Fatalf("expected override follow toggles false, got transcript=%v stream=%v", cfg.FollowTranscript, cfg.FollowStream)
	}
	if cfg.AgentPreset != overrideAgentPreset || cfg.ToolPreset != overrideToolPreset {
		t.Fatalf("expected override presets, got %q/%q", cfg.AgentPreset, cfg.ToolPreset)
	}
	if meta.Source("tavily_api_key") != SourceOverride {
		t.Fatalf("expected override source for tavily key, got %s", meta.Source("tavily_api_key"))
	}
	if meta.Source("llm_model") != SourceOverride {
		t.Fatalf("expected override source for model, got %s", meta.Source("llm_model"))
	}
	if meta.Source("llm_vision_model") != SourceOverride {
		t.Fatalf("expected override source for vision model, got %s", meta.Source("llm_vision_model"))
	}
	if meta.Source("environment") != SourceOverride {
		t.Fatalf("expected override source for environment, got %s", meta.Source("environment"))
	}
	if meta.Source("verbose") != SourceOverride {
		t.Fatalf("expected override source for verbose, got %s", meta.Source("verbose"))
	}
	if meta.Source("follow_transcript") != SourceOverride {
		t.Fatalf("expected override source for follow_transcript, got %s", meta.Source("follow_transcript"))
	}
	if meta.Source("follow_stream") != SourceOverride {
		t.Fatalf("expected override source for follow_stream, got %s", meta.Source("follow_stream"))
	}
	if meta.Source("ark_api_key") != SourceOverride {
		t.Fatalf("expected override source for ark api key")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceOverride || meta.Source("seedream_image_endpoint_id") != SourceOverride {
		t.Fatalf("expected override source for seedream endpoints")
	}
	if meta.Source("seedream_text_model") != SourceOverride || meta.Source("seedream_image_model") != SourceOverride || meta.Source("seedream_vision_model") != SourceOverride {
		t.Fatalf("expected override source for seedream models")
	}
	if meta.Source("agent_preset") != SourceOverride || meta.Source("tool_preset") != SourceOverride {
		t.Fatalf("expected override source for presets")
	}
}

func ptrString(value string) *string {
	return &value
}

func ptrStringSlice(value []string) *[]string {
	return &value
}

func TestLoadFromFileSupportsSnakeCaseArkKey(t *testing.T) {
	fileData := []byte(`
runtime:
  ark_api_key: "snake-ark"
`)
	cfg, meta, err := Load(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ArkAPIKey != "snake-ark" {
		t.Fatalf("expected ark api key from snake_case entry, got %q", cfg.ArkAPIKey)
	}
	if meta.Source("ark_api_key") != SourceFile {
		t.Fatalf("expected ark api key source to be file, got %s", meta.Source("ark_api_key"))
	}
}

func TestInvalidEnvReturnsError(t *testing.T) {
	_, _, err := Load(
		WithEnv(envMap{"LLM_TEMPERATURE": "abc"}.Lookup),
	)
	if err == nil {
		t.Fatal("expected error when temperature env is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "LLM_TEMPERATURE") {
		t.Fatalf("expected error mentioning LLM_TEMPERATURE, got %v", err)
	}
}

func TestInvalidVerboseReturnsError(t *testing.T) {
	_, _, err := Load(
		WithEnv(envMap{"ALEX_VERBOSE": "sometimes"}.Lookup),
	)
	if err == nil {
		t.Fatal("expected error when verbose env is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "ALEX_VERBOSE") {
		t.Fatalf("expected error mentioning ALEX_VERBOSE, got %v", err)
	}
}

func TestInvalidDisableTUIReturnsError(t *testing.T) {
	_, _, err := Load(
		WithEnv(envMap{"ALEX_NO_TUI": "sometimes"}.Lookup),
	)
	if err == nil {
		t.Fatal("expected error when disable TUI env is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "ALEX_NO_TUI") {
		t.Fatalf("expected error mentioning ALEX_NO_TUI, got %v", err)
	}
}

func TestInvalidFollowTranscriptReturnsError(t *testing.T) {
	_, _, err := Load(
		WithEnv(envMap{"ALEX_TUI_FOLLOW_TRANSCRIPT": "maybe"}.Lookup),
	)
	if err == nil {
		t.Fatal("expected error when follow transcript env is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "ALEX_TUI_FOLLOW_TRANSCRIPT") {
		t.Fatalf("expected error mentioning ALEX_TUI_FOLLOW_TRANSCRIPT, got %v", err)
	}
}

func TestInvalidFollowStreamReturnsError(t *testing.T) {
	_, _, err := Load(
		WithEnv(envMap{"ALEX_TUI_FOLLOW_STREAM": "sometimes"}.Lookup),
	)
	if err == nil {
		t.Fatal("expected error when follow stream env is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "ALEX_TUI_FOLLOW_STREAM") {
		t.Fatalf("expected error mentioning ALEX_TUI_FOLLOW_STREAM, got %v", err)
	}
}

func TestFollowEnvironmentOverrides(t *testing.T) {
	cfg, _, err := Load(
		WithEnv(envMap{
			"ALEX_TUI_FOLLOW_TRANSCRIPT": "false",
			"ALEX_TUI_FOLLOW_STREAM":     "true",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FollowTranscript {
		t.Fatal("expected env to disable transcript follow")
	}
	if !cfg.FollowStream {
		t.Fatal("expected env to enable stream follow")
	}
}

func TestInvalidYAMLReturnsError(t *testing.T) {
	fileData := []byte("invalid: [")
	_, _, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
	)
	if err == nil {
		t.Fatal("expected error when config YAML is invalid")
	}
	if got := fmt.Sprintf("%v", err); !strings.Contains(got, "parse config file") {
		t.Fatalf("expected parse error, got %v", err)
	}
}
