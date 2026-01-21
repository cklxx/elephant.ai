package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ValueSource describes where a configuration value originated from.
type ValueSource string

const (
	SourceDefault        ValueSource = "default"
	SourceFile           ValueSource = "file"
	SourceEnv            ValueSource = "environment"
	SourceOverride       ValueSource = "override"
	SourceCodexCLI       ValueSource = "codex_cli"
	SourceClaudeCLI      ValueSource = "claude_cli"
	SourceAntigravityCLI ValueSource = "antigravity_cli"
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
	DefaultACPHost     = "127.0.0.1"
	DefaultACPPort     = 9000
	DefaultACPPortFile = ".pids/acp.port"
)

// RuntimeConfig captures user-configurable settings shared across binaries.
type RuntimeConfig struct {
	LLMProvider                string   `json:"llm_provider" yaml:"llm_provider"`
	LLMModel                   string   `json:"llm_model" yaml:"llm_model"`
	LLMSmallProvider           string   `json:"llm_small_provider" yaml:"llm_small_provider"`
	LLMSmallModel              string   `json:"llm_small_model" yaml:"llm_small_model"`
	LLMVisionModel             string   `json:"llm_vision_model" yaml:"llm_vision_model"`
	APIKey                     string   `json:"api_key" yaml:"api_key"`
	ArkAPIKey                  string   `json:"ark_api_key" yaml:"ark_api_key"`
	BaseURL                    string   `json:"base_url" yaml:"base_url"`
	SandboxBaseURL             string   `json:"sandbox_base_url" yaml:"sandbox_base_url"`
	ACPExecutorAddr            string   `json:"acp_executor_addr" yaml:"acp_executor_addr"`
	ACPExecutorCWD             string   `json:"acp_executor_cwd" yaml:"acp_executor_cwd"`
	ACPExecutorMode            string   `json:"acp_executor_mode" yaml:"acp_executor_mode"`
	ACPExecutorAutoApprove     bool     `json:"acp_executor_auto_approve" yaml:"acp_executor_auto_approve"`
	ACPExecutorMaxCLICalls     int      `json:"acp_executor_max_cli_calls" yaml:"acp_executor_max_cli_calls"`
	ACPExecutorMaxDuration     int      `json:"acp_executor_max_duration_seconds" yaml:"acp_executor_max_duration_seconds"`
	ACPExecutorRequireManifest bool     `json:"acp_executor_require_manifest" yaml:"acp_executor_require_manifest"`
	TavilyAPIKey               string   `json:"tavily_api_key" yaml:"tavily_api_key"`
	SeedreamTextEndpointID     string   `json:"seedream_text_endpoint_id" yaml:"seedream_text_endpoint_id"`
	SeedreamImageEndpointID    string   `json:"seedream_image_endpoint_id" yaml:"seedream_image_endpoint_id"`
	SeedreamTextModel          string   `json:"seedream_text_model" yaml:"seedream_text_model"`
	SeedreamImageModel         string   `json:"seedream_image_model" yaml:"seedream_image_model"`
	SeedreamVisionModel        string   `json:"seedream_vision_model" yaml:"seedream_vision_model"`
	SeedreamVideoModel         string   `json:"seedream_video_model" yaml:"seedream_video_model"`
	Environment                string   `json:"environment" yaml:"environment"`
	Verbose                    bool     `json:"verbose" yaml:"verbose"`
	DisableTUI                 bool     `json:"disable_tui" yaml:"disable_tui"`
	FollowTranscript           bool     `json:"follow_transcript" yaml:"follow_transcript"`
	FollowStream               bool     `json:"follow_stream" yaml:"follow_stream"`
	MaxIterations              int      `json:"max_iterations" yaml:"max_iterations"`
	MaxTokens                  int      `json:"max_tokens" yaml:"max_tokens"`
	UserRateLimitRPS           float64  `json:"user_rate_limit_rps" yaml:"user_rate_limit_rps"`
	UserRateLimitBurst         int      `json:"user_rate_limit_burst" yaml:"user_rate_limit_burst"`
	Temperature                float64  `json:"temperature" yaml:"temperature"`
	TemperatureProvided        bool     `json:"temperature_provided" yaml:"temperature_provided"`
	TopP                       float64  `json:"top_p" yaml:"top_p"`
	StopSequences              []string `json:"stop_sequences" yaml:"stop_sequences"`
	SessionDir                 string   `json:"session_dir" yaml:"session_dir"`
	CostDir                    string   `json:"cost_dir" yaml:"cost_dir"`
	AgentPreset                string   `json:"agent_preset" yaml:"agent_preset"`
	ToolPreset                 string   `json:"tool_preset" yaml:"tool_preset"`
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
	LLMProvider                *string   `json:"llm_provider,omitempty" yaml:"llm_provider,omitempty"`
	LLMModel                   *string   `json:"llm_model,omitempty" yaml:"llm_model,omitempty"`
	LLMSmallProvider           *string   `json:"llm_small_provider,omitempty" yaml:"llm_small_provider,omitempty"`
	LLMSmallModel              *string   `json:"llm_small_model,omitempty" yaml:"llm_small_model,omitempty"`
	LLMVisionModel             *string   `json:"llm_vision_model,omitempty" yaml:"llm_vision_model,omitempty"`
	APIKey                     *string   `json:"api_key,omitempty" yaml:"api_key,omitempty"`
	ArkAPIKey                  *string   `json:"ark_api_key,omitempty" yaml:"ark_api_key,omitempty"`
	BaseURL                    *string   `json:"base_url,omitempty" yaml:"base_url,omitempty"`
	SandboxBaseURL             *string   `json:"sandbox_base_url,omitempty" yaml:"sandbox_base_url,omitempty"`
	ACPExecutorAddr            *string   `json:"acp_executor_addr,omitempty" yaml:"acp_executor_addr,omitempty"`
	ACPExecutorCWD             *string   `json:"acp_executor_cwd,omitempty" yaml:"acp_executor_cwd,omitempty"`
	ACPExecutorMode            *string   `json:"acp_executor_mode,omitempty" yaml:"acp_executor_mode,omitempty"`
	ACPExecutorAutoApprove     *bool     `json:"acp_executor_auto_approve,omitempty" yaml:"acp_executor_auto_approve,omitempty"`
	ACPExecutorMaxCLICalls     *int      `json:"acp_executor_max_cli_calls,omitempty" yaml:"acp_executor_max_cli_calls,omitempty"`
	ACPExecutorMaxDuration     *int      `json:"acp_executor_max_duration_seconds,omitempty" yaml:"acp_executor_max_duration_seconds,omitempty"`
	ACPExecutorRequireManifest *bool     `json:"acp_executor_require_manifest,omitempty" yaml:"acp_executor_require_manifest,omitempty"`
	TavilyAPIKey               *string   `json:"tavily_api_key,omitempty" yaml:"tavily_api_key,omitempty"`
	SeedreamTextEndpointID     *string   `json:"seedream_text_endpoint_id,omitempty" yaml:"seedream_text_endpoint_id,omitempty"`
	SeedreamImageEndpointID    *string   `json:"seedream_image_endpoint_id,omitempty" yaml:"seedream_image_endpoint_id,omitempty"`
	SeedreamTextModel          *string   `json:"seedream_text_model,omitempty" yaml:"seedream_text_model,omitempty"`
	SeedreamImageModel         *string   `json:"seedream_image_model,omitempty" yaml:"seedream_image_model,omitempty"`
	SeedreamVisionModel        *string   `json:"seedream_vision_model,omitempty" yaml:"seedream_vision_model,omitempty"`
	SeedreamVideoModel         *string   `json:"seedream_video_model,omitempty" yaml:"seedream_video_model,omitempty"`
	Environment                *string   `json:"environment,omitempty" yaml:"environment,omitempty"`
	Verbose                    *bool     `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	DisableTUI                 *bool     `json:"disable_tui,omitempty" yaml:"disable_tui,omitempty"`
	FollowTranscript           *bool     `json:"follow_transcript,omitempty" yaml:"follow_transcript,omitempty"`
	FollowStream               *bool     `json:"follow_stream,omitempty" yaml:"follow_stream,omitempty"`
	MaxIterations              *int      `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty"`
	MaxTokens                  *int      `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty"`
	UserRateLimitRPS           *float64  `json:"user_rate_limit_rps,omitempty" yaml:"user_rate_limit_rps,omitempty"`
	UserRateLimitBurst         *int      `json:"user_rate_limit_burst,omitempty" yaml:"user_rate_limit_burst,omitempty"`
	Temperature                *float64  `json:"temperature,omitempty" yaml:"temperature,omitempty"`
	TopP                       *float64  `json:"top_p,omitempty" yaml:"top_p,omitempty"`
	StopSequences              *[]string `json:"stop_sequences,omitempty" yaml:"stop_sequences,omitempty"`
	SessionDir                 *string   `json:"session_dir,omitempty" yaml:"session_dir,omitempty"`
	CostDir                    *string   `json:"cost_dir,omitempty" yaml:"cost_dir,omitempty"`
	AgentPreset                *string   `json:"agent_preset,omitempty" yaml:"agent_preset,omitempty"`
	ToolPreset                 *string   `json:"tool_preset,omitempty" yaml:"tool_preset,omitempty"`
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

// Load constructs the runtime configuration by merging defaults, file, env, and overrides.
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
		LLMProvider:                DefaultLLMProvider,
		LLMModel:                   DefaultLLMModel,
		LLMSmallProvider:           DefaultLLMProvider,
		LLMSmallModel:              DefaultLLMModel,
		BaseURL:                    DefaultLLMBaseURL,
		SandboxBaseURL:             "http://localhost:18086",
		ACPExecutorAddr:            defaultACPExecutorAddr(options.envLookup),
		ACPExecutorCWD:             defaultACPExecutorCWD(),
		ACPExecutorMode:            "full",
		ACPExecutorAutoApprove:     true,
		ACPExecutorMaxCLICalls:     12,
		ACPExecutorMaxDuration:     900,
		ACPExecutorRequireManifest: true,
		SeedreamTextModel:          DefaultSeedreamTextModel,
		SeedreamImageModel:         DefaultSeedreamImageModel,
		SeedreamVisionModel:        DefaultSeedreamVisionModel,
		SeedreamVideoModel:         DefaultSeedreamVideoModel,
		Environment:                "development",
		FollowTranscript:           true,
		FollowStream:               true,
		MaxIterations:              150,
		MaxTokens:                  DefaultMaxTokens,
		UserRateLimitRPS:           1.0,
		UserRateLimitBurst:         3,
		Temperature:                0.7,
		TopP:                       1.0,
		SessionDir:                 "~/.alex-sessions",
		CostDir:                    "~/.alex-costs",
	}

	// Helper to set provenance only when a value actually changes precedence.
	setSource := func(field string, source ValueSource) {
		meta.sources[field] = source
	}

	// Load from config file if present.
	if err := applyFile(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	// Apply environment overrides next.
	if err := applyEnv(&cfg, &meta, options); err != nil {
		return RuntimeConfig{}, Metadata{}, err
	}

	// Apply caller overrides last.
	applyOverrides(&cfg, &meta, options.overrides)

	normalizeRuntimeConfig(&cfg)
	cliCreds := CLICredentials{}
	if shouldLoadCLICredentials(cfg) {
		cliCreds = LoadCLICredentials(
			WithEnv(options.envLookup),
			WithFileReader(options.readFile),
			WithHomeDir(options.homeDir),
		)
	}
	resolveAutoProvider(&cfg, &meta, options.envLookup, cliCreds)
	resolveProviderCredentials(&cfg, &meta, options.envLookup, cliCreds)
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
	cfg.ACPExecutorAddr = strings.TrimSpace(cfg.ACPExecutorAddr)
	cfg.ACPExecutorCWD = strings.TrimSpace(cfg.ACPExecutorCWD)
	cfg.ACPExecutorMode = strings.TrimSpace(cfg.ACPExecutorMode)
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

func defaultACPExecutorAddr(lookup EnvLookup) string {
	host := defaultACPHost(lookup)
	if port, ok := acpPortFromEnv(lookup); ok {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	if port, ok := readACPPortFile(); ok {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	return fmt.Sprintf("http://%s:%d", host, DefaultACPPort)
}

func defaultACPExecutorCWD() string {
	const sandboxDir = "/workspace"
	if info, err := os.Stat(sandboxDir); err == nil && info.IsDir() {
		return sandboxDir
	}
	if wd, err := os.Getwd(); err == nil {
		wd = strings.TrimSpace(wd)
		if wd != "" {
			return wd
		}
	}
	return sandboxDir
}

func defaultACPHost(lookup EnvLookup) string {
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	if value, ok := lookup("ACP_HOST"); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return DefaultACPHost
}

func acpPortFromEnv(lookup EnvLookup) (int, bool) {
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	value, ok := lookup("ACP_PORT")
	if !ok {
		return 0, false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func readACPPortFile() (int, bool) {
	wd, err := os.Getwd()
	if err != nil || strings.TrimSpace(wd) == "" {
		return 0, false
	}
	path := filepath.Join(wd, DefaultACPPortFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func shouldLoadCLICredentials(cfg RuntimeConfig) bool {
	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	switch provider {
	case "auto", "cli":
		return true
	}

	if strings.TrimSpace(cfg.APIKey) != "" {
		return false
	}

	switch provider {
	case "codex", "openai-responses", "responses", "anthropic", "claude", "antigravity":
		return true
	default:
		return false
	}
}
func applyFile(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	configPath := strings.TrimSpace(opts.configPath)
	if configPath == "" {
		configPath, _ = ResolveConfigPath(opts.envLookup, opts.homeDir)
	}
	if configPath == "" {
		return nil
	}

	data, err := opts.readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("read config file: %w", err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}

	parsed, err := parseRuntimeConfigYAML(data)
	if err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	lookup := opts.envLookup
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	parsed = expandRuntimeFileConfigEnv(lookup, parsed)

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
	if parsed.ACPExecutorAddr != "" {
		cfg.ACPExecutorAddr = parsed.ACPExecutorAddr
		meta.sources["acp_executor_addr"] = SourceFile
	}
	if parsed.ACPExecutorCWD != "" {
		cfg.ACPExecutorCWD = parsed.ACPExecutorCWD
		meta.sources["acp_executor_cwd"] = SourceFile
	}
	if parsed.ACPExecutorMode != "" {
		cfg.ACPExecutorMode = parsed.ACPExecutorMode
		meta.sources["acp_executor_mode"] = SourceFile
	}
	if parsed.ACPExecutorAutoApprove != nil {
		cfg.ACPExecutorAutoApprove = *parsed.ACPExecutorAutoApprove
		meta.sources["acp_executor_auto_approve"] = SourceFile
	}
	if parsed.ACPExecutorMaxCLICalls != nil {
		cfg.ACPExecutorMaxCLICalls = *parsed.ACPExecutorMaxCLICalls
		meta.sources["acp_executor_max_cli_calls"] = SourceFile
	}
	if parsed.ACPExecutorMaxDuration != nil {
		cfg.ACPExecutorMaxDuration = *parsed.ACPExecutorMaxDuration
		meta.sources["acp_executor_max_duration_seconds"] = SourceFile
	}
	if parsed.ACPExecutorRequireManifest != nil {
		cfg.ACPExecutorRequireManifest = *parsed.ACPExecutorRequireManifest
		meta.sources["acp_executor_require_manifest"] = SourceFile
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
	if parsed.DisableTUI != nil {
		cfg.DisableTUI = *parsed.DisableTUI
		meta.sources["disable_tui"] = SourceFile
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
	if value, ok := lookup("ACP_EXECUTOR_ADDR"); ok && value != "" {
		cfg.ACPExecutorAddr = value
		meta.sources["acp_executor_addr"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_CWD"); ok && value != "" {
		cfg.ACPExecutorCWD = value
		meta.sources["acp_executor_cwd"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MODE"); ok && value != "" {
		cfg.ACPExecutorMode = value
		meta.sources["acp_executor_mode"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_AUTO_APPROVE"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_AUTO_APPROVE: %w", err)
		}
		cfg.ACPExecutorAutoApprove = parsed
		meta.sources["acp_executor_auto_approve"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MAX_CLI_CALLS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_MAX_CLI_CALLS: %w", err)
		}
		cfg.ACPExecutorMaxCLICalls = parsed
		meta.sources["acp_executor_max_cli_calls"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_MAX_DURATION_SECONDS"); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_MAX_DURATION_SECONDS: %w", err)
		}
		cfg.ACPExecutorMaxDuration = parsed
		meta.sources["acp_executor_max_duration_seconds"] = SourceEnv
	}
	if value, ok := lookup("ACP_EXECUTOR_REQUIRE_MANIFEST"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ACP_EXECUTOR_REQUIRE_MANIFEST: %w", err)
		}
		cfg.ACPExecutorRequireManifest = parsed
		meta.sources["acp_executor_require_manifest"] = SourceEnv
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

type autoProviderCandidate struct {
	Provider    string
	APIKeyEnv   string
	BaseURLEnv  string
	DefaultBase string
	Source      ValueSource
}

func applyAutoProviderCandidate(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cand autoProviderCandidate) bool {
	key, ok := lookup(cand.APIKeyEnv)
	key = strings.TrimSpace(key)
	if !ok || key == "" {
		return false
	}

	cfg.LLMProvider = cand.Provider
	meta.sources["llm_provider"] = cand.Source
	if cfg.APIKey == "" {
		cfg.APIKey = key
		meta.sources["api_key"] = cand.Source
	}

	if cfg.LLMSmallProvider == "" || strings.EqualFold(cfg.LLMSmallProvider, "auto") || strings.EqualFold(cfg.LLMSmallProvider, "cli") {
		cfg.LLMSmallProvider = cand.Provider
		meta.sources["llm_small_provider"] = cand.Source
	}

	if base, ok := lookup(cand.BaseURLEnv); ok && strings.TrimSpace(base) != "" {
		cfg.BaseURL = strings.TrimSpace(base)
		meta.sources["base_url"] = SourceEnv
	} else if cand.DefaultBase != "" && meta.Source("base_url") == SourceDefault {
		cfg.BaseURL = cand.DefaultBase
		meta.sources["base_url"] = cand.Source
	}

	return true
}

func applyCLICandidates(cfg *RuntimeConfig, meta *Metadata, candidates ...CLICredential) bool {
	for _, cand := range candidates {
		if strings.TrimSpace(cand.APIKey) == "" {
			continue
		}
		cfg.LLMProvider = cand.Provider
		meta.sources["llm_provider"] = cand.Source
		if cfg.APIKey == "" {
			cfg.APIKey = cand.APIKey
			meta.sources["api_key"] = cand.Source
		}
		if cfg.LLMSmallProvider == "" || strings.EqualFold(cfg.LLMSmallProvider, "auto") || strings.EqualFold(cfg.LLMSmallProvider, "cli") {
			cfg.LLMSmallProvider = cand.Provider
			meta.sources["llm_small_provider"] = cand.Source
		}
		if cand.BaseURL != "" && meta.Source("base_url") == SourceDefault {
			cfg.BaseURL = cand.BaseURL
			meta.sources["base_url"] = cand.Source
		}
		if cand.Model != "" && meta.Source("llm_model") == SourceDefault {
			cfg.LLMModel = cand.Model
			meta.sources["llm_model"] = cand.Source
		}
		return true
	}
	return false
}

func resolveAutoProvider(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cli CLICredentials) {
	if cfg == nil {
		return
	}
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if provider != "auto" && provider != "cli" {
		return
	}

	candidates := []autoProviderCandidate{
		{
			Provider:    "anthropic",
			APIKeyEnv:   "CLAUDE_CODE_OAUTH_TOKEN",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceClaudeCLI,
		},
		{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_AUTH_TOKEN",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceClaudeCLI,
		},
		{
			Provider:    "anthropic",
			APIKeyEnv:   "ANTHROPIC_API_KEY",
			BaseURLEnv:  "ANTHROPIC_BASE_URL",
			DefaultBase: "https://api.anthropic.com/v1",
			Source:      SourceEnv,
		},
		{
			Provider:    "codex",
			APIKeyEnv:   "CODEX_API_KEY",
			BaseURLEnv:  "CODEX_BASE_URL",
			DefaultBase: codexCLIBaseURL,
			Source:      SourceEnv,
		},
		{
			Provider:   "antigravity",
			APIKeyEnv:  "ANTIGRAVITY_API_KEY",
			BaseURLEnv: "ANTIGRAVITY_BASE_URL",
			Source:     SourceEnv,
		},
		{
			Provider:   "openai",
			APIKeyEnv:  "OPENAI_API_KEY",
			BaseURLEnv: "OPENAI_BASE_URL",
			Source:     SourceEnv,
		},
	}

	applyEnv := func() bool {
		for _, cand := range candidates {
			if applyAutoProviderCandidate(cfg, meta, lookup, cand) {
				return true
			}
		}
		return false
	}
	applyCLI := func() bool {
		return applyCLICandidates(cfg, meta, cli.Codex, cli.Antigravity, cli.Claude)
	}

	if provider == "cli" {
		if applyCLI() {
			return
		}
		_ = applyEnv()
		return
	}

	if applyEnv() {
		return
	}
	_ = applyCLI()
}

func resolveProviderCredentials(cfg *RuntimeConfig, meta *Metadata, lookup EnvLookup, cli CLICredentials) {
	if cfg == nil {
		return
	}
	if lookup == nil {
		lookup = DefaultEnvLookup
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.LLMProvider))
	if provider == "" {
		return
	}

	if cfg.APIKey == "" {
		switch provider {
		case "anthropic", "claude":
			if cli.Claude.APIKey != "" {
				cfg.APIKey = strings.TrimSpace(cli.Claude.APIKey)
				meta.sources["api_key"] = cli.Claude.Source
				break
			}
			if key, ok := lookup("ANTHROPIC_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		case "openai-responses", "responses", "codex":
			if cli.Codex.APIKey != "" {
				cfg.APIKey = strings.TrimSpace(cli.Codex.APIKey)
				meta.sources["api_key"] = cli.Codex.Source
				break
			}
			if key, ok := lookup("CODEX_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
				break
			}
			if key, ok := lookup("OPENAI_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		case "antigravity":
			if cli.Antigravity.APIKey != "" {
				cfg.APIKey = strings.TrimSpace(cli.Antigravity.APIKey)
				meta.sources["api_key"] = cli.Antigravity.Source
				break
			}
			if key, ok := lookup("ANTIGRAVITY_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		case "openai", "openrouter", "deepseek":
			if key, ok := lookup("OPENAI_API_KEY"); ok && strings.TrimSpace(key) != "" {
				cfg.APIKey = strings.TrimSpace(key)
				meta.sources["api_key"] = SourceEnv
			}
		}
	}

	if meta.Source("llm_model") == SourceDefault {
		switch provider {
		case "codex":
			if cli.Codex.Model != "" {
				cfg.LLMModel = cli.Codex.Model
				meta.sources["llm_model"] = cli.Codex.Source
			}
		case "antigravity":
			if cli.Antigravity.Model != "" {
				cfg.LLMModel = cli.Antigravity.Model
				meta.sources["llm_model"] = cli.Antigravity.Source
			}
		}
	}

	if meta.Source("base_url") == SourceDefault {
		switch provider {
		case "anthropic", "claude":
			if base, ok := lookup("ANTHROPIC_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			} else {
				cfg.BaseURL = "https://api.anthropic.com/v1"
				meta.sources["base_url"] = SourceEnv
			}
		case "openai-responses", "responses", "codex":
			if cli.Codex.BaseURL != "" {
				cfg.BaseURL = cli.Codex.BaseURL
				meta.sources["base_url"] = cli.Codex.Source
				break
			}
			if base, ok := lookup("CODEX_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
				break
			}
			if provider == "codex" {
				cfg.BaseURL = codexCLIBaseURL
				meta.sources["base_url"] = SourceEnv
			}
		case "antigravity":
			if cli.Antigravity.BaseURL != "" {
				cfg.BaseURL = cli.Antigravity.BaseURL
				meta.sources["base_url"] = cli.Antigravity.Source
				break
			}
			if base, ok := lookup("ANTIGRAVITY_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			}
		case "openai", "openrouter", "deepseek":
			if base, ok := lookup("OPENAI_BASE_URL"); ok && strings.TrimSpace(base) != "" {
				cfg.BaseURL = strings.TrimSpace(base)
				meta.sources["base_url"] = SourceEnv
			}
		}
	}
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
	if overrides.ACPExecutorAddr != nil {
		cfg.ACPExecutorAddr = *overrides.ACPExecutorAddr
		meta.sources["acp_executor_addr"] = SourceOverride
	}
	if overrides.ACPExecutorCWD != nil {
		cfg.ACPExecutorCWD = *overrides.ACPExecutorCWD
		meta.sources["acp_executor_cwd"] = SourceOverride
	}
	if overrides.ACPExecutorMode != nil {
		cfg.ACPExecutorMode = *overrides.ACPExecutorMode
		meta.sources["acp_executor_mode"] = SourceOverride
	}
	if overrides.ACPExecutorAutoApprove != nil {
		cfg.ACPExecutorAutoApprove = *overrides.ACPExecutorAutoApprove
		meta.sources["acp_executor_auto_approve"] = SourceOverride
	}
	if overrides.ACPExecutorMaxCLICalls != nil {
		cfg.ACPExecutorMaxCLICalls = *overrides.ACPExecutorMaxCLICalls
		meta.sources["acp_executor_max_cli_calls"] = SourceOverride
	}
	if overrides.ACPExecutorMaxDuration != nil {
		cfg.ACPExecutorMaxDuration = *overrides.ACPExecutorMaxDuration
		meta.sources["acp_executor_max_duration_seconds"] = SourceOverride
	}
	if overrides.ACPExecutorRequireManifest != nil {
		cfg.ACPExecutorRequireManifest = *overrides.ACPExecutorRequireManifest
		meta.sources["acp_executor_require_manifest"] = SourceOverride
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

type runtimeFile struct {
	Runtime *RuntimeFileConfig `yaml:"runtime"`
}

func parseRuntimeConfigYAML(data []byte) (RuntimeFileConfig, error) {
	var wrapped runtimeFile
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return RuntimeFileConfig{}, err
	}
	if wrapped.Runtime != nil {
		return *wrapped.Runtime, nil
	}

	var parsed RuntimeFileConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return RuntimeFileConfig{}, err
	}
	return parsed, nil
}

func expandRuntimeFileConfigEnv(lookup EnvLookup, parsed RuntimeFileConfig) RuntimeFileConfig {
	parsed.LLMProvider = expandEnvValue(lookup, parsed.LLMProvider)
	parsed.LLMModel = expandEnvValue(lookup, parsed.LLMModel)
	parsed.LLMSmallProvider = expandEnvValue(lookup, parsed.LLMSmallProvider)
	parsed.LLMSmallModel = expandEnvValue(lookup, parsed.LLMSmallModel)
	parsed.LLMVisionModel = expandEnvValue(lookup, parsed.LLMVisionModel)
	parsed.APIKey = expandEnvValue(lookup, parsed.APIKey)
	parsed.ArkAPIKey = expandEnvValue(lookup, parsed.ArkAPIKey)
	parsed.BaseURL = expandEnvValue(lookup, parsed.BaseURL)
	parsed.SandboxBaseURL = expandEnvValue(lookup, parsed.SandboxBaseURL)
	parsed.ACPExecutorAddr = expandEnvValue(lookup, parsed.ACPExecutorAddr)
	parsed.ACPExecutorCWD = expandEnvValue(lookup, parsed.ACPExecutorCWD)
	parsed.ACPExecutorMode = expandEnvValue(lookup, parsed.ACPExecutorMode)
	parsed.TavilyAPIKey = expandEnvValue(lookup, parsed.TavilyAPIKey)
	parsed.SeedreamTextEndpointID = expandEnvValue(lookup, parsed.SeedreamTextEndpointID)
	parsed.SeedreamImageEndpointID = expandEnvValue(lookup, parsed.SeedreamImageEndpointID)
	parsed.SeedreamTextModel = expandEnvValue(lookup, parsed.SeedreamTextModel)
	parsed.SeedreamImageModel = expandEnvValue(lookup, parsed.SeedreamImageModel)
	parsed.SeedreamVisionModel = expandEnvValue(lookup, parsed.SeedreamVisionModel)
	parsed.SeedreamVideoModel = expandEnvValue(lookup, parsed.SeedreamVideoModel)
	parsed.Environment = expandEnvValue(lookup, parsed.Environment)
	parsed.SessionDir = expandEnvValue(lookup, parsed.SessionDir)
	parsed.CostDir = expandEnvValue(lookup, parsed.CostDir)
	parsed.AgentPreset = expandEnvValue(lookup, parsed.AgentPreset)
	parsed.ToolPreset = expandEnvValue(lookup, parsed.ToolPreset)

	if len(parsed.StopSequences) > 0 {
		expanded := make([]string, 0, len(parsed.StopSequences))
		for _, seq := range parsed.StopSequences {
			expanded = append(expanded, expandEnvValue(lookup, seq))
		}
		parsed.StopSequences = expanded
	}

	return parsed
}
