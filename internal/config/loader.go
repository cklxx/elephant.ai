package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ValueSource describes where a configuration value originated from.
type ValueSource string

const (
	SourceDefault  ValueSource = "default"
	SourceFile     ValueSource = "file"
	SourceEnv      ValueSource = "environment"
	SourceOverride ValueSource = "override"
)

// Seedream defaults target the public Volcano Engine Ark deployment in mainland China.
const (
	DefaultSeedreamTextModel   = "doubao-seedream-4-5-251128"
	DefaultSeedreamImageModel  = "doubao-seedream-4-5-251128"
	DefaultSeedreamVisionModel = "doubao-seed-1-6-vision-250815"
	DefaultSeedreamVideoModel  = "doubao-seedance-1-0-pro-fast-251015"
)

const (
	DefaultLLMProvider = "openai"
	DefaultLLMModel    = "gpt-4o-mini"
	DefaultLLMBaseURL  = "https://api.openai.com/v1"
	DefaultMaxTokens   = 8192
)

// RuntimeConfig captures user-configurable settings shared across binaries.
type RuntimeConfig struct {
	LLMProvider             string   `json:"llm_provider"`
	LLMModel                string   `json:"llm_model"`
	LLMSmallProvider        string   `json:"llm_small_provider"`
	LLMSmallModel           string   `json:"llm_small_model"`
	LLMVisionModel          string   `json:"llm_vision_model"`
	APIKey                  string   `json:"api_key"`
	ArkAPIKey               string   `json:"ark_api_key"`
	BaseURL                 string   `json:"base_url"`
	SandboxBaseURL          string   `json:"sandbox_base_url"`
	TavilyAPIKey            string   `json:"tavily_api_key"`
	SeedreamTextEndpointID  string   `json:"seedream_text_endpoint_id"`
	SeedreamImageEndpointID string   `json:"seedream_image_endpoint_id"`
	SeedreamTextModel       string   `json:"seedream_text_model"`
	SeedreamImageModel      string   `json:"seedream_image_model"`
	SeedreamVisionModel     string   `json:"seedream_vision_model"`
	SeedreamVideoModel      string   `json:"seedream_video_model"`
	Environment             string   `json:"environment"`
	Verbose                 bool     `json:"verbose"`
	DisableTUI              bool     `json:"disable_tui"`
	FollowTranscript        bool     `json:"follow_transcript"`
	FollowStream            bool     `json:"follow_stream"`
	MaxIterations           int      `json:"max_iterations"`
	MaxTokens               int      `json:"max_tokens"`
	UserRateLimitRPS        float64  `json:"user_rate_limit_rps"`
	UserRateLimitBurst      int      `json:"user_rate_limit_burst"`
	Temperature             float64  `json:"temperature"`
	TemperatureProvided     bool     `json:"temperature_provided"`
	TopP                    float64  `json:"top_p"`
	StopSequences           []string `json:"stop_sequences"`
	SessionDir              string   `json:"session_dir"`
	CostDir                 string   `json:"cost_dir"`
	AgentPreset             string   `json:"agent_preset"`
	ToolPreset              string   `json:"tool_preset"`
}

// Metadata contains provenance details for loaded configuration.
type Metadata struct {
	sources  map[string]ValueSource
	loadedAt time.Time
}

// Sources returns a copy of the provenance map for JSON serialization.
func (m Metadata) Sources() map[string]ValueSource {
	if m.sources == nil {
		return map[string]ValueSource{}
	}
	copy := make(map[string]ValueSource, len(m.sources))
	for key, value := range m.sources {
		copy[key] = value
	}
	return copy
}

// Source returns the origin for the given configuration field.
func (m Metadata) Source(field string) ValueSource {
	if m.sources == nil {
		return SourceDefault
	}
	if src, ok := m.sources[field]; ok {
		return src
	}
	return SourceDefault
}

// LoadedAt returns the timestamp when the configuration was constructed.
func (m Metadata) LoadedAt() time.Time {
	return m.loadedAt
}

// Overrides conveys caller-specified values that should win over env/file sources.
type Overrides struct {
	LLMProvider             *string   `json:"llm_provider,omitempty"`
	LLMModel                *string   `json:"llm_model,omitempty"`
	LLMSmallProvider        *string   `json:"llm_small_provider,omitempty"`
	LLMSmallModel           *string   `json:"llm_small_model,omitempty"`
	LLMVisionModel          *string   `json:"llm_vision_model,omitempty"`
	APIKey                  *string   `json:"api_key,omitempty"`
	ArkAPIKey               *string   `json:"ark_api_key,omitempty"`
	BaseURL                 *string   `json:"base_url,omitempty"`
	SandboxBaseURL          *string   `json:"sandbox_base_url,omitempty"`
	TavilyAPIKey            *string   `json:"tavily_api_key,omitempty"`
	SeedreamTextEndpointID  *string   `json:"seedream_text_endpoint_id,omitempty"`
	SeedreamImageEndpointID *string   `json:"seedream_image_endpoint_id,omitempty"`
	SeedreamTextModel       *string   `json:"seedream_text_model,omitempty"`
	SeedreamImageModel      *string   `json:"seedream_image_model,omitempty"`
	SeedreamVisionModel     *string   `json:"seedream_vision_model,omitempty"`
	SeedreamVideoModel      *string   `json:"seedream_video_model,omitempty"`
	Environment             *string   `json:"environment,omitempty"`
	Verbose                 *bool     `json:"verbose,omitempty"`
	DisableTUI              *bool     `json:"disable_tui,omitempty"`
	FollowTranscript        *bool     `json:"follow_transcript,omitempty"`
	FollowStream            *bool     `json:"follow_stream,omitempty"`
	MaxIterations           *int      `json:"max_iterations,omitempty"`
	MaxTokens               *int      `json:"max_tokens,omitempty"`
	UserRateLimitRPS        *float64  `json:"user_rate_limit_rps,omitempty"`
	UserRateLimitBurst      *int      `json:"user_rate_limit_burst,omitempty"`
	Temperature             *float64  `json:"temperature,omitempty"`
	TopP                    *float64  `json:"top_p,omitempty"`
	StopSequences           *[]string `json:"stop_sequences,omitempty"`
	SessionDir              *string   `json:"session_dir,omitempty"`
	CostDir                 *string   `json:"cost_dir,omitempty"`
	AgentPreset             *string   `json:"agent_preset,omitempty"`
	ToolPreset              *string   `json:"tool_preset,omitempty"`
}

// EnvLookup resolves the value for an environment variable.
type EnvLookup func(string) (string, bool)

// Option customises the loader behaviour.
type Option func(*loadOptions)

type loadOptions struct {
	envLookup  EnvLookup
	readFile   func(string) ([]byte, error)
	homeDir    func() (string, error)
	overrides  Overrides
	configPath string
}

// WithEnv supplies a custom environment lookup implementation.
func WithEnv(lookup EnvLookup) Option {
	return func(o *loadOptions) {
		o.envLookup = lookup
	}
}

// WithOverrides applies caller overrides that take highest precedence.
func WithOverrides(overrides Overrides) Option {
	return func(o *loadOptions) {
		o.overrides = overrides
	}
}

// WithConfigPath forces the loader to read configuration from a specific file.
func WithConfigPath(path string) Option {
	return func(o *loadOptions) {
		o.configPath = path
	}
}

// WithFileReader injects a custom reader, used primarily for tests.
func WithFileReader(reader func(string) ([]byte, error)) Option {
	return func(o *loadOptions) {
		o.readFile = reader
	}
}

// WithHomeDir overrides how the loader resolves the user's home directory.
func WithHomeDir(resolver func() (string, error)) Option {
	return func(o *loadOptions) {
		o.homeDir = resolver
	}
}

// DefaultEnvLookup delegates to os.LookupEnv.
func DefaultEnvLookup(key string) (string, bool) {
	return os.LookupEnv(key)
}

// Load constructs the runtime configuration by merging defaults, file, env and overrides.
func Load(opts ...Option) (RuntimeConfig, Metadata, error) {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	meta := Metadata{sources: map[string]ValueSource{}, loadedAt: time.Now()}

	cfg := RuntimeConfig{
		LLMProvider:         DefaultLLMProvider,
		LLMModel:            DefaultLLMModel,
		LLMSmallProvider:    DefaultLLMProvider,
		LLMSmallModel:       DefaultLLMModel,
		BaseURL:             DefaultLLMBaseURL,
		SandboxBaseURL:      "http://localhost:18086",
		SeedreamTextModel:   DefaultSeedreamTextModel,
		SeedreamImageModel:  DefaultSeedreamImageModel,
		SeedreamVisionModel: DefaultSeedreamVisionModel,
		SeedreamVideoModel:  DefaultSeedreamVideoModel,
		Environment:         "development",
		FollowTranscript:    true,
		FollowStream:        true,
		MaxIterations:       150,
		MaxTokens:           DefaultMaxTokens,
		UserRateLimitRPS:    1.0,
		UserRateLimitBurst:  3,
		Temperature:         0.7,
		TopP:                1.0,
		SessionDir:          "~/.alex-sessions",
		CostDir:             "~/.alex-costs",
	}

	// Helper to set provenance only when a value actually changes precedence.
	setSource := func(field string, source ValueSource) {
		meta.sources[field] = source
	}

	// Load from config file if present.
	if err := applyFile(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	// Apply environment overrides.
	if err := applyEnv(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	// Apply caller overrides last.
	applyOverrides(&cfg, &meta, options.overrides)

	normalizeRuntimeConfig(&cfg)
	// If API key remains unset, default to mock provider (unless ollama).
	if cfg.APIKey == "" && cfg.LLMProvider != "mock" && cfg.LLMProvider != "ollama" {
		cfg.LLMProvider = "mock"
		if cfg.LLMSmallProvider != "mock" {
			cfg.LLMSmallProvider = "mock"
			setSource("llm_small_provider", SourceDefault)
		}
		if cfg.LLMSmallModel != "mock" {
			cfg.LLMSmallModel = "mock"
			setSource("llm_small_model", SourceDefault)
		}
		setSource("llm_provider", SourceDefault)
	}

	return cfg, meta, nil
}

func normalizeRuntimeConfig(cfg *RuntimeConfig) {
	cfg.LLMProvider = strings.TrimSpace(cfg.LLMProvider)
	cfg.LLMModel = strings.TrimSpace(cfg.LLMModel)
	cfg.LLMSmallProvider = strings.TrimSpace(cfg.LLMSmallProvider)
	cfg.LLMSmallModel = strings.TrimSpace(cfg.LLMSmallModel)
	cfg.LLMVisionModel = strings.TrimSpace(cfg.LLMVisionModel)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.ArkAPIKey = strings.TrimSpace(cfg.ArkAPIKey)
	cfg.BaseURL = strings.TrimSpace(cfg.BaseURL)
	cfg.SandboxBaseURL = strings.TrimSpace(cfg.SandboxBaseURL)
	cfg.TavilyAPIKey = strings.TrimSpace(cfg.TavilyAPIKey)
	cfg.SeedreamTextEndpointID = strings.TrimSpace(cfg.SeedreamTextEndpointID)
	cfg.SeedreamImageEndpointID = strings.TrimSpace(cfg.SeedreamImageEndpointID)
	cfg.SeedreamTextModel = strings.TrimSpace(cfg.SeedreamTextModel)
	cfg.SeedreamImageModel = strings.TrimSpace(cfg.SeedreamImageModel)
	cfg.SeedreamVisionModel = strings.TrimSpace(cfg.SeedreamVisionModel)
	cfg.SeedreamVideoModel = strings.TrimSpace(cfg.SeedreamVideoModel)
	cfg.Environment = strings.TrimSpace(cfg.Environment)
	cfg.SessionDir = strings.TrimSpace(cfg.SessionDir)
	cfg.CostDir = strings.TrimSpace(cfg.CostDir)
	cfg.AgentPreset = strings.TrimSpace(cfg.AgentPreset)
	cfg.ToolPreset = strings.TrimSpace(cfg.ToolPreset)

	if len(cfg.StopSequences) > 0 {
		filtered := cfg.StopSequences[:0]
		seen := make(map[string]struct{}, len(cfg.StopSequences))
		for _, seq := range cfg.StopSequences {
			trimmed := strings.TrimSpace(seq)
			if trimmed == "" {
				continue
			}
			if _, exists := seen[trimmed]; exists {
				continue
			}
			seen[trimmed] = struct{}{}
			filtered = append(filtered, trimmed)
		}
		cfg.StopSequences = filtered
	}
}

type fileConfig struct {
	LLMProvider             string   `json:"llm_provider"`
	LLMModel                string   `json:"llm_model"`
	LLMSmallProvider        string   `json:"llm_small_provider"`
	LLMSmallModel           string   `json:"llm_small_model"`
	LLMVisionModel          string   `json:"llm_vision_model"`
	APIKey                  string   `json:"api_key"`
	ArkAPIKey               string   `json:"ark_api_key"`
	BaseURL                 string   `json:"base_url"`
	SandboxBaseURL          string   `json:"sandbox_base_url"`
	TavilyAPIKey            string   `json:"tavily_api_key"`
	SeedreamTextEndpointID  string   `json:"seedream_text_endpoint_id"`
	SeedreamImageEndpointID string   `json:"seedream_image_endpoint_id"`
	SeedreamTextModel       string   `json:"seedream_text_model"`
	SeedreamImageModel      string   `json:"seedream_image_model"`
	SeedreamVisionModel     string   `json:"seedream_vision_model"`
	SeedreamVideoModel      string   `json:"seedream_video_model"`
	Environment             string   `json:"environment"`
	Verbose                 *bool    `json:"verbose"`
	FollowTranscript        *bool    `json:"follow_transcript"`
	FollowStream            *bool    `json:"follow_stream"`
	MaxIterations           *int     `json:"max_iterations"`
	MaxTokens               *int     `json:"max_tokens"`
	UserRateLimitRPS        *float64 `json:"user_rate_limit_rps"`
	UserRateLimitBurst      *int     `json:"user_rate_limit_burst"`
	Temperature             *float64 `json:"temperature"`
	TopP                    *float64 `json:"top_p"`
	StopSequences           []string `json:"stop_sequences"`
	SessionDir              string   `json:"session_dir"`
	CostDir                 string   `json:"cost_dir"`
	AgentPreset             string   `json:"agent_preset"`
	ToolPreset              string   `json:"tool_preset"`
}

func applyFile(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	configPath := strings.TrimSpace(opts.configPath)
	if configPath == "" {
		lookup := opts.envLookup
		if lookup == nil {
			lookup = DefaultEnvLookup
		}
		if value, ok := lookup("ALEX_CONFIG_PATH"); ok {
			configPath = strings.TrimSpace(value)
		}
	}
	if configPath == "" {
		home, err := opts.homeDir()
		if err != nil {
			return nil
		}
		configPath = filepath.Join(home, ".alex-config.json")
	}

	data, err := opts.readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	var parsed fileConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}

	if parsed.APIKey != "" {
		cfg.APIKey = parsed.APIKey
		meta.sources["api_key"] = SourceFile
	}
	if parsed.ArkAPIKey != "" {
		cfg.ArkAPIKey = parsed.ArkAPIKey
		meta.sources["ark_api_key"] = SourceFile
	}
	if parsed.LLMProvider != "" {
		cfg.LLMProvider = parsed.LLMProvider
		meta.sources["llm_provider"] = SourceFile
	}
	if parsed.LLMModel != "" {
		cfg.LLMModel = parsed.LLMModel
		meta.sources["llm_model"] = SourceFile
	}
	if parsed.LLMSmallProvider != "" {
		cfg.LLMSmallProvider = parsed.LLMSmallProvider
		meta.sources["llm_small_provider"] = SourceFile
	}
	if parsed.LLMSmallModel != "" {
		cfg.LLMSmallModel = parsed.LLMSmallModel
		meta.sources["llm_small_model"] = SourceFile
	}
	if parsed.LLMVisionModel != "" {
		cfg.LLMVisionModel = parsed.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceFile
	}
	if parsed.BaseURL != "" {
		cfg.BaseURL = parsed.BaseURL
		meta.sources["base_url"] = SourceFile
	}
	if parsed.SandboxBaseURL != "" {
		cfg.SandboxBaseURL = parsed.SandboxBaseURL
		meta.sources["sandbox_base_url"] = SourceFile
	}
	if parsed.TavilyAPIKey != "" {
		cfg.TavilyAPIKey = parsed.TavilyAPIKey
		meta.sources["tavily_api_key"] = SourceFile
	}
	if parsed.SeedreamTextEndpointID != "" {
		cfg.SeedreamTextEndpointID = parsed.SeedreamTextEndpointID
		meta.sources["seedream_text_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamImageEndpointID != "" {
		cfg.SeedreamImageEndpointID = parsed.SeedreamImageEndpointID
		meta.sources["seedream_image_endpoint_id"] = SourceFile
	}
	if parsed.SeedreamTextModel != "" {
		cfg.SeedreamTextModel = parsed.SeedreamTextModel
		meta.sources["seedream_text_model"] = SourceFile
	}
	if parsed.SeedreamImageModel != "" {
		cfg.SeedreamImageModel = parsed.SeedreamImageModel
		meta.sources["seedream_image_model"] = SourceFile
	}
	if parsed.SeedreamVisionModel != "" {
		cfg.SeedreamVisionModel = parsed.SeedreamVisionModel
		meta.sources["seedream_vision_model"] = SourceFile
	}
	if parsed.SeedreamVideoModel != "" {
		cfg.SeedreamVideoModel = parsed.SeedreamVideoModel
		meta.sources["seedream_video_model"] = SourceFile
	}
	if parsed.Environment != "" {
		cfg.Environment = parsed.Environment
		meta.sources["environment"] = SourceFile
	}
	if parsed.Verbose != nil {
		cfg.Verbose = *parsed.Verbose
		meta.sources["verbose"] = SourceFile
	}
	if parsed.FollowTranscript != nil {
		cfg.FollowTranscript = *parsed.FollowTranscript
		meta.sources["follow_transcript"] = SourceFile
	}
	if parsed.FollowStream != nil {
		cfg.FollowStream = *parsed.FollowStream
		meta.sources["follow_stream"] = SourceFile
	}
	if parsed.MaxIterations != nil {
		cfg.MaxIterations = *parsed.MaxIterations
		meta.sources["max_iterations"] = SourceFile
	}
	if parsed.MaxTokens != nil {
		cfg.MaxTokens = *parsed.MaxTokens
		meta.sources["max_tokens"] = SourceFile
	}
	if parsed.UserRateLimitRPS != nil {
		cfg.UserRateLimitRPS = *parsed.UserRateLimitRPS
		meta.sources["user_rate_limit_rps"] = SourceFile
	}
	if parsed.UserRateLimitBurst != nil {
		cfg.UserRateLimitBurst = *parsed.UserRateLimitBurst
		meta.sources["user_rate_limit_burst"] = SourceFile
	}
	if parsed.Temperature != nil {
		cfg.Temperature = *parsed.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceFile
	}
	if parsed.TopP != nil {
		cfg.TopP = *parsed.TopP
		meta.sources["top_p"] = SourceFile
	}
	if len(parsed.StopSequences) > 0 {
		cfg.StopSequences = append([]string(nil), parsed.StopSequences...)
		meta.sources["stop_sequences"] = SourceFile
	}
	if parsed.SessionDir != "" {
		cfg.SessionDir = parsed.SessionDir
		meta.sources["session_dir"] = SourceFile
	}
	if parsed.CostDir != "" {
		cfg.CostDir = parsed.CostDir
		meta.sources["cost_dir"] = SourceFile
	}
	if parsed.AgentPreset != "" {
		cfg.AgentPreset = parsed.AgentPreset
		meta.sources["agent_preset"] = SourceFile
	}
	if parsed.ToolPreset != "" {
		cfg.ToolPreset = parsed.ToolPreset
		meta.sources["tool_preset"] = SourceFile
	}

	return nil
}

func applyEnv(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	lookup := opts.envLookup
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	if value, ok := lookup("OPENAI_API_KEY"); ok && value != "" {
		cfg.APIKey = value
		meta.sources["api_key"] = SourceEnv
	}
	if value, ok := lookup("ARK_API_KEY"); ok && value != "" {
		cfg.ArkAPIKey = value
		meta.sources["ark_api_key"] = SourceEnv
	}
	if value, ok := lookup("LLM_PROVIDER"); ok && value != "" {
		cfg.LLMProvider = value
		meta.sources["llm_provider"] = SourceEnv
	}
	if value, ok := lookup("LLM_MODEL"); ok && value != "" {
		cfg.LLMModel = value
		meta.sources["llm_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_SMALL_PROVIDER"); ok && value != "" {
		cfg.LLMSmallProvider = value
		meta.sources["llm_small_provider"] = SourceEnv
	}
	if value, ok := lookup("LLM_SMALL_MODEL"); ok && value != "" {
		cfg.LLMSmallModel = value
		meta.sources["llm_small_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_VISION_MODEL"); ok && value != "" {
		cfg.LLMVisionModel = value
		meta.sources["llm_vision_model"] = SourceEnv
	}
	if value, ok := lookup("LLM_BASE_URL"); ok && value != "" {
		cfg.BaseURL = value
		meta.sources["base_url"] = SourceEnv
	}
	if value, ok := lookup("SANDBOX_BASE_URL"); ok && value != "" {
		cfg.SandboxBaseURL = value
		meta.sources["sandbox_base_url"] = SourceEnv
	}
	if value, ok := lookup("TAVILY_API_KEY"); ok && value != "" {
		cfg.TavilyAPIKey = value
		meta.sources["tavily_api_key"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamTextEndpointID = value
		meta.sources["seedream_text_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamImageEndpointID = value
		meta.sources["seedream_image_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_MODEL"); ok && value != "" {
		cfg.SeedreamTextModel = value
		meta.sources["seedream_text_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_MODEL"); ok && value != "" {
		cfg.SeedreamImageModel = value
		meta.sources["seedream_image_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_VISION_MODEL"); ok && value != "" {
		cfg.SeedreamVisionModel = value
		meta.sources["seedream_vision_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_VIDEO_MODEL"); ok && value != "" {
		cfg.SeedreamVideoModel = value
		meta.sources["seedream_video_model"] = SourceEnv
	}
	if value, ok := lookup("AGENT_PRESET"); ok && value != "" {
		cfg.AgentPreset = value
		meta.sources["agent_preset"] = SourceEnv
	}
	if value, ok := lookup("TOOL_PRESET"); ok && value != "" {
		cfg.ToolPreset = value
		meta.sources["tool_preset"] = SourceEnv
	}
	if value, ok := lookup("ALEX_ENV"); ok && value != "" {
		cfg.Environment = value
		meta.sources["environment"] = SourceEnv
	}
	if value, ok := lookup("ALEX_VERBOSE"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_VERBOSE: %w", err)
		}
		cfg.Verbose = parsed
		meta.sources["verbose"] = SourceEnv
	}
	if value, ok := lookup("ALEX_NO_TUI"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_NO_TUI: %w", err)
		}
		cfg.DisableTUI = parsed
		meta.sources["disable_tui"] = SourceEnv
	}
	if value, ok := lookup("ALEX_TUI_FOLLOW_TRANSCRIPT"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_TUI_FOLLOW_TRANSCRIPT: %w", err)
		}
		cfg.FollowTranscript = parsed
		meta.sources["follow_transcript"] = SourceEnv
	}

	if value, ok := lookup("ALEX_TUI_FOLLOW_STREAM"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_TUI_FOLLOW_STREAM: %w", err)
		}
		cfg.FollowStream = parsed
		meta.sources["follow_stream"] = SourceEnv
	}
	if value, ok := lookup("LLM_MAX_ITERATIONS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_MAX_ITERATIONS: %w", err)
		}
		cfg.MaxIterations = parsed
		meta.sources["max_iterations"] = SourceEnv
	}
	if value, ok := lookup("LLM_MAX_TOKENS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse LLM_MAX_TOKENS: %w", err)
		}
		cfg.MaxTokens = parsed
		meta.sources["max_tokens"] = SourceEnv
	}
	if value, ok := lookup("USER_LLM_RPS"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse USER_LLM_RPS: %w", err)
		}
		cfg.UserRateLimitRPS = parsed
		meta.sources["user_rate_limit_rps"] = SourceEnv
	}
	if value, ok := lookup("USER_LLM_BURST"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse USER_LLM_BURST: %w", err)
		}
		cfg.UserRateLimitBurst = parsed
		meta.sources["user_rate_limit_burst"] = SourceEnv
	}
	if value, ok := lookup("LLM_TEMPERATURE"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse LLM_TEMPERATURE: %w", err)
		}
		cfg.Temperature = parsed
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceEnv
	}
	if value, ok := lookup("LLM_TOP_P"); ok && value != "" {
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("parse LLM_TOP_P: %w", err)
		}
		cfg.TopP = parsed
		meta.sources["top_p"] = SourceEnv
	}
	if value, ok := lookup("LLM_STOP"); ok && value != "" {
		parts := strings.FieldsFunc(value, func(r rune) bool {
			switch r {
			case ',', ';', ' ', '\n', '\t':
				return true
			default:
				return false
			}
		})
		filtered := parts[:0]
		for _, token := range parts {
			trimmed := strings.TrimSpace(token)
			if trimmed != "" {
				filtered = append(filtered, trimmed)
			}
		}
		cfg.StopSequences = append([]string(nil), filtered...)
		meta.sources["stop_sequences"] = SourceEnv
	}
	if value, ok := lookup("ALEX_SESSION_DIR"); ok && value != "" {
		cfg.SessionDir = value
		meta.sources["session_dir"] = SourceEnv
	}
	if value, ok := lookup("ALEX_COST_DIR"); ok && value != "" {
		cfg.CostDir = value
		meta.sources["cost_dir"] = SourceEnv
	}

	return nil
}

func applyOverrides(cfg *RuntimeConfig, meta *Metadata, overrides Overrides) {
	if overrides.LLMProvider != nil {
		cfg.LLMProvider = *overrides.LLMProvider
		meta.sources["llm_provider"] = SourceOverride
	}
	if overrides.LLMModel != nil {
		cfg.LLMModel = *overrides.LLMModel
		meta.sources["llm_model"] = SourceOverride
	}
	if overrides.LLMSmallProvider != nil {
		cfg.LLMSmallProvider = *overrides.LLMSmallProvider
		meta.sources["llm_small_provider"] = SourceOverride
	}
	if overrides.LLMSmallModel != nil {
		cfg.LLMSmallModel = *overrides.LLMSmallModel
		meta.sources["llm_small_model"] = SourceOverride
	}
	if overrides.LLMVisionModel != nil {
		cfg.LLMVisionModel = *overrides.LLMVisionModel
		meta.sources["llm_vision_model"] = SourceOverride
	}
	if overrides.APIKey != nil {
		cfg.APIKey = *overrides.APIKey
		meta.sources["api_key"] = SourceOverride
	}
	if overrides.ArkAPIKey != nil {
		cfg.ArkAPIKey = *overrides.ArkAPIKey
		meta.sources["ark_api_key"] = SourceOverride
	}
	if overrides.BaseURL != nil {
		cfg.BaseURL = *overrides.BaseURL
		meta.sources["base_url"] = SourceOverride
	}
	if overrides.SandboxBaseURL != nil {
		cfg.SandboxBaseURL = *overrides.SandboxBaseURL
		meta.sources["sandbox_base_url"] = SourceOverride
	}
	if overrides.TavilyAPIKey != nil {
		cfg.TavilyAPIKey = *overrides.TavilyAPIKey
		meta.sources["tavily_api_key"] = SourceOverride
	}
	if overrides.SeedreamTextEndpointID != nil {
		cfg.SeedreamTextEndpointID = *overrides.SeedreamTextEndpointID
		meta.sources["seedream_text_endpoint_id"] = SourceOverride
	}
	if overrides.SeedreamImageEndpointID != nil {
		cfg.SeedreamImageEndpointID = *overrides.SeedreamImageEndpointID
		meta.sources["seedream_image_endpoint_id"] = SourceOverride
	}
	if overrides.SeedreamTextModel != nil {
		cfg.SeedreamTextModel = *overrides.SeedreamTextModel
		meta.sources["seedream_text_model"] = SourceOverride
	}
	if overrides.SeedreamImageModel != nil {
		cfg.SeedreamImageModel = *overrides.SeedreamImageModel
		meta.sources["seedream_image_model"] = SourceOverride
	}
	if overrides.SeedreamVisionModel != nil {
		cfg.SeedreamVisionModel = *overrides.SeedreamVisionModel
		meta.sources["seedream_vision_model"] = SourceOverride
	}
	if overrides.SeedreamVideoModel != nil {
		cfg.SeedreamVideoModel = *overrides.SeedreamVideoModel
		meta.sources["seedream_video_model"] = SourceOverride
	}
	if overrides.Environment != nil {
		cfg.Environment = *overrides.Environment
		meta.sources["environment"] = SourceOverride
	}
	if overrides.Verbose != nil {
		cfg.Verbose = *overrides.Verbose
		meta.sources["verbose"] = SourceOverride
	}
	if overrides.DisableTUI != nil {
		cfg.DisableTUI = *overrides.DisableTUI
		meta.sources["disable_tui"] = SourceOverride
	}
	if overrides.FollowTranscript != nil {
		cfg.FollowTranscript = *overrides.FollowTranscript
		meta.sources["follow_transcript"] = SourceOverride
	}
	if overrides.FollowStream != nil {
		cfg.FollowStream = *overrides.FollowStream
		meta.sources["follow_stream"] = SourceOverride
	}
	if overrides.MaxIterations != nil {
		cfg.MaxIterations = *overrides.MaxIterations
		meta.sources["max_iterations"] = SourceOverride
	}
	if overrides.MaxTokens != nil {
		cfg.MaxTokens = *overrides.MaxTokens
		meta.sources["max_tokens"] = SourceOverride
	}
	if overrides.UserRateLimitRPS != nil {
		cfg.UserRateLimitRPS = *overrides.UserRateLimitRPS
		meta.sources["user_rate_limit_rps"] = SourceOverride
	}
	if overrides.UserRateLimitBurst != nil {
		cfg.UserRateLimitBurst = *overrides.UserRateLimitBurst
		meta.sources["user_rate_limit_burst"] = SourceOverride
	}
	if overrides.Temperature != nil {
		cfg.Temperature = *overrides.Temperature
		cfg.TemperatureProvided = true
		meta.sources["temperature"] = SourceOverride
	}
	if overrides.TopP != nil {
		cfg.TopP = *overrides.TopP
		meta.sources["top_p"] = SourceOverride
	}
	if overrides.StopSequences != nil {
		cfg.StopSequences = append([]string(nil), *overrides.StopSequences...)
		meta.sources["stop_sequences"] = SourceOverride
	}
	if overrides.SessionDir != nil {
		cfg.SessionDir = *overrides.SessionDir
		meta.sources["session_dir"] = SourceOverride
	}
	if overrides.CostDir != nil {
		cfg.CostDir = *overrides.CostDir
		meta.sources["cost_dir"] = SourceOverride
	}
	if overrides.AgentPreset != nil {
		cfg.AgentPreset = *overrides.AgentPreset
		meta.sources["agent_preset"] = SourceOverride
	}
	if overrides.ToolPreset != nil {
		cfg.ToolPreset = *overrides.ToolPreset
		meta.sources["tool_preset"] = SourceOverride
	}
}

func parseBoolEnv(value string) (bool, error) {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean value %q", value)
	}
}
