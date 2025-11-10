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

// DefaultSandboxBaseURL provides the local sandbox endpoint used when no value is configured.
const DefaultSandboxBaseURL = "http://localhost:8090"

// Seedream defaults target the public Volcano Engine Ark deployment in mainland China.
const (
	DefaultSeedreamHost        = "maas-api.ml-platform-cn-beijing.volces.com"
	DefaultSeedreamRegion      = "cn-beijing"
	DefaultSeedreamTextModel   = "doubao-seedream-3-0-t2i-250415"
	DefaultSeedreamImageModel  = "doubao-seedream-4-0-250828"
	DefaultSeedreamVisionModel = "doubao-seed-1-6-vision-250815"
)

// RuntimeConfig captures user-configurable settings shared across binaries.
type RuntimeConfig struct {
	LLMProvider             string
	LLMModel                string
	APIKey                  string
	ArkAPIKey               string
	BaseURL                 string
	TavilyAPIKey            string
	VolcAccessKey           string
	VolcSecretKey           string
	SeedreamHost            string
	SeedreamRegion          string
	SeedreamTextEndpointID  string
	SeedreamImageEndpointID string
	SeedreamTextModel       string
	SeedreamImageModel      string
	SeedreamVisionModel     string
	SandboxBaseURL          string
	Environment             string
	Verbose                 bool
	DisableTUI              bool
	FollowTranscript        bool
	FollowStream            bool
	MaxIterations           int
	MaxTokens               int
	Temperature             float64
	TemperatureProvided     bool
	TopP                    float64
	StopSequences           []string
	SessionDir              string
	CostDir                 string
	CraftMirrorDir          string
	AgentPreset             string
	ToolPreset              string
}

// Metadata contains provenance details for loaded configuration.
type Metadata struct {
	sources  map[string]ValueSource
	loadedAt time.Time
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
	LLMProvider             *string
	LLMModel                *string
	APIKey                  *string
	ArkAPIKey               *string
	BaseURL                 *string
	TavilyAPIKey            *string
	VolcAccessKey           *string
	VolcSecretKey           *string
	SeedreamHost            *string
	SeedreamRegion          *string
	SeedreamTextEndpointID  *string
	SeedreamImageEndpointID *string
	SeedreamTextModel       *string
	SeedreamImageModel      *string
	SeedreamVisionModel     *string
	SandboxBaseURL          *string
	Environment             *string
	Verbose                 *bool
	DisableTUI              *bool
	FollowTranscript        *bool
	FollowStream            *bool
	MaxIterations           *int
	MaxTokens               *int
	Temperature             *float64
	TopP                    *float64
	StopSequences           *[]string
	SessionDir              *string
	CostDir                 *string
	CraftMirrorDir          *string
	AgentPreset             *string
	ToolPreset              *string
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

// AliasEnvLookup wraps an EnvLookup with additional alias keys.
func AliasEnvLookup(base EnvLookup, aliases map[string][]string) EnvLookup {
	return func(key string) (string, bool) {
		if base == nil {
			base = DefaultEnvLookup
		}
		if value, ok := base(key); ok && value != "" {
			return value, true
		}
		if list, ok := aliases[key]; ok {
			for _, alias := range list {
				if value, ok := base(alias); ok && value != "" {
					return value, true
				}
			}
		}
		return "", false
	}
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
		LLMProvider:         "openrouter",
		LLMModel:            "deepseek/deepseek-chat",
		BaseURL:             "https://openrouter.ai/api/v1",
		SandboxBaseURL:      DefaultSandboxBaseURL,
		SeedreamHost:        DefaultSeedreamHost,
		SeedreamRegion:      DefaultSeedreamRegion,
		SeedreamTextModel:   DefaultSeedreamTextModel,
		SeedreamImageModel:  DefaultSeedreamImageModel,
		SeedreamVisionModel: DefaultSeedreamVisionModel,
		Environment:         "development",
		FollowTranscript:    true,
		FollowStream:        true,
		MaxIterations:       150,
		MaxTokens:           100000,
		Temperature:         0.7,
		TopP:                1.0,
		SessionDir:          "~/.alex-sessions",
		CostDir:             "~/.alex-costs",
		CraftMirrorDir:      "~/.alex-crafts",
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

	// If API key remains unset, default to mock provider.
	if cfg.APIKey == "" && cfg.LLMProvider != "mock" {
		cfg.LLMProvider = "mock"
		setSource("llm_provider", SourceDefault)
	}

	return cfg, meta, nil
}

type fileConfig struct {
	LLMProvider             string                 `json:"llm_provider"`
	LLMModel                string                 `json:"llm_model"`
	Model                   string                 `json:"model"`
	APIKey                  string                 `json:"api_key"`
	ArkAPIKey               string                 `json:"arkApiKey"`
	BaseURL                 string                 `json:"base_url"`
	TavilyAPIKey            string                 `json:"tavilyApiKey"`
	VolcAccessKey           string                 `json:"volcAccessKey"`
	VolcSecretKey           string                 `json:"volcSecretKey"`
	SeedreamHost            string                 `json:"seedreamHost"`
	SeedreamRegion          string                 `json:"seedreamRegion"`
	SeedreamTextEndpointID  string                 `json:"seedreamTextEndpointId"`
	SeedreamImageEndpointID string                 `json:"seedreamImageEndpointId"`
	SeedreamTextModel       string                 `json:"seedreamTextModel"`
	SeedreamImageModel      string                 `json:"seedreamImageModel"`
	SeedreamVisionModel     string                 `json:"seedreamVisionModel"`
	SandboxBaseURL          string                 `json:"sandbox_base_url"`
	Environment             string                 `json:"environment"`
	Verbose                 *bool                  `json:"verbose"`
	FollowTranscript        *bool                  `json:"follow_transcript"`
	FollowStream            *bool                  `json:"follow_stream"`
	MaxIterations           *int                   `json:"max_iterations"`
	MaxTokens               *int                   `json:"max_tokens"`
	Temperature             *float64               `json:"temperature"`
	TopP                    *float64               `json:"top_p"`
	StopSequences           []string               `json:"stop_sequences"`
	SessionDir              string                 `json:"session_dir"`
	CostDir                 string                 `json:"cost_dir"`
	CraftMirrorDir          string                 `json:"craft_mirror_dir"`
	Models                  map[string]modelConfig `json:"models"`
	AgentPreset             string                 `json:"agent_preset"`
	ToolPreset              string                 `json:"tool_preset"`
}

type modelConfig struct {
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func applyFile(cfg *RuntimeConfig, meta *Metadata, opts loadOptions) error {
	configPath := opts.configPath
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
	} else if parsed.Model != "" {
		cfg.LLMModel = parsed.Model
		meta.sources["llm_model"] = SourceFile
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
	if parsed.VolcAccessKey != "" {
		cfg.VolcAccessKey = parsed.VolcAccessKey
		meta.sources["volc_access_key"] = SourceFile
	}
	if parsed.VolcSecretKey != "" {
		cfg.VolcSecretKey = parsed.VolcSecretKey
		meta.sources["volc_secret_key"] = SourceFile
	}
	if parsed.SeedreamHost != "" {
		cfg.SeedreamHost = parsed.SeedreamHost
		meta.sources["seedream_host"] = SourceFile
	}
	if parsed.SeedreamRegion != "" {
		cfg.SeedreamRegion = parsed.SeedreamRegion
		meta.sources["seedream_region"] = SourceFile
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
	if parsed.CraftMirrorDir != "" {
		cfg.CraftMirrorDir = parsed.CraftMirrorDir
		meta.sources["craft_mirror_dir"] = SourceFile
	}
	if parsed.AgentPreset != "" {
		cfg.AgentPreset = parsed.AgentPreset
		meta.sources["agent_preset"] = SourceFile
	}
	if parsed.ToolPreset != "" {
		cfg.ToolPreset = parsed.ToolPreset
		meta.sources["tool_preset"] = SourceFile
	}
	if parsed.Models != nil {
		if basic, ok := parsed.Models["basic"]; ok {
			if basic.APIKey != "" && cfg.APIKey == "" {
				cfg.APIKey = basic.APIKey
				meta.sources["api_key"] = SourceFile
			}
			if basic.Model != "" {
				cfg.LLMModel = basic.Model
				meta.sources["llm_model"] = SourceFile
			}
			if basic.BaseURL != "" {
				cfg.BaseURL = basic.BaseURL
				meta.sources["base_url"] = SourceFile
			}
		}
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
	if value, ok := lookup("OPENROUTER_API_KEY"); ok && value != "" {
		cfg.APIKey = value
		meta.sources["api_key"] = SourceEnv
	}
	if value, ok := lookup("ARK_API_KEY"); ok && value != "" {
		cfg.ArkAPIKey = value
		meta.sources["ark_api_key"] = SourceEnv
	} else if value, ok := lookup("ALEX_ARK_API_KEY"); ok && value != "" {
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
	if value, ok := lookup("VOLC_ACCESSKEY"); ok && value != "" {
		cfg.VolcAccessKey = value
		meta.sources["volc_access_key"] = SourceEnv
	} else if value, ok := lookup("ALEX_VOLC_ACCESSKEY"); ok && value != "" {
		cfg.VolcAccessKey = value
		meta.sources["volc_access_key"] = SourceEnv
	}
	if value, ok := lookup("VOLC_SECRETKEY"); ok && value != "" {
		cfg.VolcSecretKey = value
		meta.sources["volc_secret_key"] = SourceEnv
	} else if value, ok := lookup("ALEX_VOLC_SECRETKEY"); ok && value != "" {
		cfg.VolcSecretKey = value
		meta.sources["volc_secret_key"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_HOST"); ok && value != "" {
		cfg.SeedreamHost = value
		meta.sources["seedream_host"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_HOST"); ok && value != "" {
		cfg.SeedreamHost = value
		meta.sources["seedream_host"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_REGION"); ok && value != "" {
		cfg.SeedreamRegion = value
		meta.sources["seedream_region"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_REGION"); ok && value != "" {
		cfg.SeedreamRegion = value
		meta.sources["seedream_region"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamTextEndpointID = value
		meta.sources["seedream_text_endpoint_id"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_TEXT_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamTextEndpointID = value
		meta.sources["seedream_text_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamImageEndpointID = value
		meta.sources["seedream_image_endpoint_id"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_IMAGE_ENDPOINT_ID"); ok && value != "" {
		cfg.SeedreamImageEndpointID = value
		meta.sources["seedream_image_endpoint_id"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_TEXT_MODEL"); ok && value != "" {
		cfg.SeedreamTextModel = value
		meta.sources["seedream_text_model"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_TEXT_MODEL"); ok && value != "" {
		cfg.SeedreamTextModel = value
		meta.sources["seedream_text_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_IMAGE_MODEL"); ok && value != "" {
		cfg.SeedreamImageModel = value
		meta.sources["seedream_image_model"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_IMAGE_MODEL"); ok && value != "" {
		cfg.SeedreamImageModel = value
		meta.sources["seedream_image_model"] = SourceEnv
	}
	if value, ok := lookup("SEEDREAM_VISION_MODEL"); ok && value != "" {
		cfg.SeedreamVisionModel = value
		meta.sources["seedream_vision_model"] = SourceEnv
	} else if value, ok := lookup("ALEX_SEEDREAM_VISION_MODEL"); ok && value != "" {
		cfg.SeedreamVisionModel = value
		meta.sources["seedream_vision_model"] = SourceEnv
	}
	if value, ok := lookup("AGENT_PRESET"); ok && value != "" {
		cfg.AgentPreset = value
		meta.sources["agent_preset"] = SourceEnv
	} else if value, ok := lookup("ALEX_AGENT_PRESET"); ok && value != "" {
		cfg.AgentPreset = value
		meta.sources["agent_preset"] = SourceEnv
	}
	if value, ok := lookup("TOOL_PRESET"); ok && value != "" {
		cfg.ToolPreset = value
		meta.sources["tool_preset"] = SourceEnv
	} else if value, ok := lookup("ALEX_TOOL_PRESET"); ok && value != "" {
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
	} else if value, ok := lookup("ALEX_FOLLOW_TRANSCRIPT"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_FOLLOW_TRANSCRIPT: %w", err)
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
	} else if value, ok := lookup("ALEX_FOLLOW_STREAM"); ok && value != "" {
		parsed, err := parseBoolEnv(value)
		if err != nil {
			return fmt.Errorf("parse ALEX_FOLLOW_STREAM: %w", err)
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
	if value, ok := lookup("ALEX_CRAFT_MIRROR_DIR"); ok && value != "" {
		cfg.CraftMirrorDir = value
		meta.sources["craft_mirror_dir"] = SourceEnv
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
	if overrides.VolcAccessKey != nil {
		cfg.VolcAccessKey = *overrides.VolcAccessKey
		meta.sources["volc_access_key"] = SourceOverride
	}
	if overrides.VolcSecretKey != nil {
		cfg.VolcSecretKey = *overrides.VolcSecretKey
		meta.sources["volc_secret_key"] = SourceOverride
	}
	if overrides.SeedreamHost != nil {
		cfg.SeedreamHost = *overrides.SeedreamHost
		meta.sources["seedream_host"] = SourceOverride
	}
	if overrides.SeedreamRegion != nil {
		cfg.SeedreamRegion = *overrides.SeedreamRegion
		meta.sources["seedream_region"] = SourceOverride
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
	if overrides.CraftMirrorDir != nil {
		cfg.CraftMirrorDir = *overrides.CraftMirrorDir
		meta.sources["craft_mirror_dir"] = SourceOverride
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
