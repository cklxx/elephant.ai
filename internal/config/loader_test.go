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
		t.Fatalf("expected mock provider when API key missing, got %q", cfg.LLMProvider)
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
	if cfg.SeedreamHost != DefaultSeedreamHost || cfg.SeedreamRegion != DefaultSeedreamRegion {
		t.Fatalf("expected default seedream host/region, got %q/%q", cfg.SeedreamHost, cfg.SeedreamRegion)
	}
	if cfg.Verbose {
		t.Fatal("expected verbose default to be false")
	}
	if !cfg.FollowTranscript || !cfg.FollowStream {
		t.Fatalf("expected follow defaults to be true, got transcript=%v stream=%v", cfg.FollowTranscript, cfg.FollowStream)
	}
}

func TestLoadFromFile(t *testing.T) {
	fileData := []byte(`{
                "llm_provider": "openai",
                "llm_model": "gpt-4o",
                "api_key": "sk-test",
                "tavilyApiKey": "file-tavily",
                "volcAccessKey": "file-ak",
                "volcSecretKey": "file-sk",
                "seedreamHost": "file-host",
                "seedreamRegion": "file-region",
                "seedreamTextEndpointId": "file-text-id",
                "seedreamImageEndpointId": "file-image-id",
                "environment": "staging",
                "verbose": true,
                "follow_transcript": false,
                "follow_stream": false,
                "temperature": 0,
                "max_iterations": 200,
                "stop_sequences": ["DONE"],
                "session_dir": "~/sessions",
                "agent_preset": "designer",
                "tool_preset": "safe"
        }`)
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
	if cfg.VolcAccessKey != "file-ak" || cfg.VolcSecretKey != "file-sk" {
		t.Fatalf("expected volc credentials from file, got %q/%q", cfg.VolcAccessKey, cfg.VolcSecretKey)
	}
	if cfg.SeedreamHost != "file-host" || cfg.SeedreamRegion != "file-region" {
		t.Fatalf("expected seedream host/region from file, got %q/%q", cfg.SeedreamHost, cfg.SeedreamRegion)
	}
	if cfg.SeedreamTextEndpointID != "file-text-id" || cfg.SeedreamImageEndpointID != "file-image-id" {
		t.Fatalf("expected seedream endpoints from file, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
	}
	if cfg.Environment != "staging" {
		t.Fatalf("expected environment from file, got %q", cfg.Environment)
	}
	if !cfg.Verbose {
		t.Fatal("expected verbose true from file")
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
	if meta.Source("volc_access_key") != SourceFile || meta.Source("volc_secret_key") != SourceFile {
		t.Fatalf("expected volc credentials source from file")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceFile || meta.Source("seedream_image_endpoint_id") != SourceFile {
		t.Fatalf("expected seedream endpoints source from file")
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

func TestEnvOverridesFile(t *testing.T) {
	fileData := []byte(`{"temperature": 0.1, "tavilyApiKey": "file-key"}`)
	cfg, meta, err := Load(
		WithFileReader(func(string) ([]byte, error) { return fileData, nil }),
		WithEnv(envMap{
			"LLM_TEMPERATURE":            "0",
			"LLM_MODEL":                  "env-model",
			"TAVILY_API_KEY":             "env-tavily",
			"VOLC_ACCESSKEY":             "env-ak",
			"VOLC_SECRETKEY":             "env-sk",
			"SEEDREAM_HOST":              "env-host",
			"SEEDREAM_REGION":            "env-region",
			"SEEDREAM_TEXT_ENDPOINT_ID":  "env-text",
			"SEEDREAM_IMAGE_ENDPOINT_ID": "env-image",
			"ALEX_ENV":                   "production",
			"ALEX_VERBOSE":               "yes",
			"ALEX_NO_TUI":                "true",
			"ALEX_TUI_FOLLOW_TRANSCRIPT": "false",
			"ALEX_TUI_FOLLOW_STREAM":     "false",
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
	if cfg.Temperature != 0 || !cfg.TemperatureProvided {
		t.Fatalf("expected env zero temperature override, got %+v", cfg)
	}
	if cfg.TavilyAPIKey != "env-tavily" {
		t.Fatalf("expected tavily key from env, got %q", cfg.TavilyAPIKey)
	}
	if cfg.VolcAccessKey != "env-ak" || cfg.VolcSecretKey != "env-sk" {
		t.Fatalf("expected volc credentials from env, got %q/%q", cfg.VolcAccessKey, cfg.VolcSecretKey)
	}
	if cfg.SeedreamHost != "env-host" || cfg.SeedreamRegion != "env-region" {
		t.Fatalf("expected seedream host/region from env, got %q/%q", cfg.SeedreamHost, cfg.SeedreamRegion)
	}
	if cfg.SeedreamTextEndpointID != "env-text" || cfg.SeedreamImageEndpointID != "env-image" {
		t.Fatalf("expected seedream endpoints from env, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
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
	if meta.Source("volc_access_key") != SourceEnv || meta.Source("volc_secret_key") != SourceEnv {
		t.Fatalf("expected env source for volc credentials")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceEnv || meta.Source("seedream_image_endpoint_id") != SourceEnv {
		t.Fatalf("expected env source for seedream endpoints")
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

func TestOverridesTakePriority(t *testing.T) {
	overrideTemp := 1.0
	overrideModel := "override-model"
	overrideTavily := "override-tavily"
	overrideVolcAK := "override-ak"
	overrideVolcSK := "override-sk"
	overrideSeedreamHost := "override-host"
	overrideSeedreamRegion := "override-region"
	overrideSeedreamText := "override-text"
	overrideSeedreamImage := "override-image"
	overrideEnv := "qa"
	overrideVerbose := true
	overrideFollowTranscript := false
	overrideFollowStream := false
	overrideAgentPreset := "designer"
	overrideToolPreset := "web-only"
	cfg, meta, err := Load(
		WithEnv(envMap{"LLM_MODEL": "env-model"}.Lookup),
		WithOverrides(Overrides{
			LLMModel:                &overrideModel,
			Temperature:             &overrideTemp,
			TavilyAPIKey:            &overrideTavily,
			VolcAccessKey:           &overrideVolcAK,
			VolcSecretKey:           &overrideVolcSK,
			SeedreamHost:            &overrideSeedreamHost,
			SeedreamRegion:          &overrideSeedreamRegion,
			SeedreamTextEndpointID:  &overrideSeedreamText,
			SeedreamImageEndpointID: &overrideSeedreamImage,
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
	if cfg.Temperature != 1.0 || !cfg.TemperatureProvided {
		t.Fatalf("expected override temperature 1.0, got %+v", cfg)
	}
	if cfg.TavilyAPIKey != "override-tavily" {
		t.Fatalf("expected override tavily key, got %q", cfg.TavilyAPIKey)
	}
	if cfg.VolcAccessKey != overrideVolcAK || cfg.VolcSecretKey != overrideVolcSK {
		t.Fatalf("expected override volc credentials, got %q/%q", cfg.VolcAccessKey, cfg.VolcSecretKey)
	}
	if cfg.SeedreamHost != overrideSeedreamHost || cfg.SeedreamRegion != overrideSeedreamRegion {
		t.Fatalf("expected override seedream host/region, got %q/%q", cfg.SeedreamHost, cfg.SeedreamRegion)
	}
	if cfg.SeedreamTextEndpointID != overrideSeedreamText || cfg.SeedreamImageEndpointID != overrideSeedreamImage {
		t.Fatalf("expected override seedream endpoints, got %q/%q", cfg.SeedreamTextEndpointID, cfg.SeedreamImageEndpointID)
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
	if meta.Source("volc_access_key") != SourceOverride || meta.Source("volc_secret_key") != SourceOverride {
		t.Fatalf("expected override source for volc credentials")
	}
	if meta.Source("seedream_text_endpoint_id") != SourceOverride || meta.Source("seedream_image_endpoint_id") != SourceOverride {
		t.Fatalf("expected override source for seedream endpoints")
	}
	if meta.Source("agent_preset") != SourceOverride || meta.Source("tool_preset") != SourceOverride {
		t.Fatalf("expected override source for presets")
	}
}

func TestAliasLookup(t *testing.T) {
	baseEnv := envMap{"ALEX_MODEL_NAME": "alias-model"}
	cfg, _, err := Load(
		WithEnv(AliasEnvLookup(baseEnv.Lookup, map[string][]string{
			"LLM_MODEL": {"ALEX_MODEL_NAME"},
		})),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.LLMModel != "alias-model" {
		t.Fatalf("expected alias model, got %s", cfg.LLMModel)
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

func TestFollowAliasEnvironmentOverrides(t *testing.T) {
	cfg, _, err := Load(
		WithEnv(envMap{
			"ALEX_FOLLOW_TRANSCRIPT": "false",
			"ALEX_FOLLOW_STREAM":     "true",
		}.Lookup),
	)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.FollowTranscript {
		t.Fatal("expected alias env to disable transcript follow")
	}
	if !cfg.FollowStream {
		t.Fatal("expected alias env to enable stream follow")
	}
}
