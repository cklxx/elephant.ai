package config

// Config represents unified application configuration
type Config struct {
	// Core Application Settings
	DefaultLanguage  string   `yaml:"defaultLanguage" json:"defaultLanguage" mapstructure:"defaultLanguage"`
	OutputFormat     string   `yaml:"outputFormat" json:"outputFormat" mapstructure:"outputFormat"`
	AnalysisDepth    int      `yaml:"analysisDepth" json:"analysisDepth" mapstructure:"analysisDepth"`
	BackupOnRefactor bool     `yaml:"backupOnRefactor" json:"backupOnRefactor" mapstructure:"backupOnRefactor"`
	ExcludePatterns  []string `yaml:"excludePatterns" json:"excludePatterns" mapstructure:"excludePatterns"`

	// API Configuration
	APIKey  string `yaml:"api_key" json:"api_key" mapstructure:"api_key"`
	BaseURL string `yaml:"base_url" json:"base_url" mapstructure:"base_url"`
	Model   string `yaml:"model" json:"model" mapstructure:"model"`

	// Tavily API Configuration
	TavilyAPIKey string `yaml:"tavily_api_key" json:"tavily_api_key" mapstructure:"tavily_api_key"`

	// Agent Configuration
	AllowedTools    []string `yaml:"allowedTools" json:"allowedTools" mapstructure:"allowedTools"`
	MaxIterations   int      `yaml:"maxIterations" json:"maxIterations" mapstructure:"maxIterations"`
	Timeout         int      `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	EnableStreaming bool     `yaml:"enableStreaming" json:"enableStreaming" mapstructure:"enableStreaming"`
	EnableMemory    bool     `yaml:"enableMemory" json:"enableMemory" mapstructure:"enableMemory"`

	// LLM Configuration
	Temperature      float64  `yaml:"temperature" json:"temperature" mapstructure:"temperature"`
	MaxTokens        int      `yaml:"maxTokens" json:"maxTokens" mapstructure:"maxTokens"`
	TopP             float64  `yaml:"topP" json:"topP" mapstructure:"topP"`
	FrequencyPenalty float64  `yaml:"frequencyPenalty" json:"frequencyPenalty" mapstructure:"frequencyPenalty"`
	PresencePenalty  float64  `yaml:"presencePenalty" json:"presencePenalty" mapstructure:"presencePenalty"`
	StopSequences    []string `yaml:"stopSequences" json:"stopSequences" mapstructure:"stopSequences"`

	// Storage Configuration
	StorageType string `yaml:"storageType" json:"storageType" mapstructure:"storageType"`
	StoragePath string `yaml:"storagePath" json:"storagePath" mapstructure:"storagePath"`

	// Session Configuration
	SessionTimeout   int    `yaml:"sessionTimeout" json:"sessionTimeout" mapstructure:"sessionTimeout"`
	SessionStorePath string `yaml:"sessionStorePath" json:"sessionStorePath" mapstructure:"sessionStorePath"`
	AutoSaveInterval int    `yaml:"autoSaveInterval" json:"autoSaveInterval" mapstructure:"autoSaveInterval"`

	// Security Configuration
	EnableSandbox   bool     `yaml:"enableSandbox" json:"enableSandbox" mapstructure:"enableSandbox"`
	AllowedPaths    []string `yaml:"allowedPaths" json:"allowedPaths" mapstructure:"allowedPaths"`
	BlockedCommands []string `yaml:"blockedCommands" json:"blockedCommands" mapstructure:"blockedCommands"`
	MaxFileSize     int64    `yaml:"maxFileSize" json:"maxFileSize" mapstructure:"maxFileSize"`

	// Logging Configuration
	LogLevel    string `yaml:"logLevel" json:"logLevel" mapstructure:"logLevel"`
	LogFilePath string `yaml:"logFilePath" json:"logFilePath" mapstructure:"logFilePath"`
	EnableDebug bool   `yaml:"enableDebug" json:"enableDebug" mapstructure:"enableDebug"`

	// UI Configuration
	Theme        string `yaml:"theme" json:"theme" mapstructure:"theme"`
	EnableTUI    bool   `yaml:"enableTUI" json:"enableTUI" mapstructure:"enableTUI"`
	ShowProgress bool   `yaml:"showProgress" json:"showProgress" mapstructure:"showProgress"`
	ColorOutput  bool   `yaml:"colorOutput" json:"colorOutput" mapstructure:"colorOutput"`
}

// LLMConfig represents LLM-specific configuration
type LLMConfig struct {
	Provider         string   `yaml:"provider" json:"provider" mapstructure:"provider"`
	Model            string   `yaml:"model" json:"model" mapstructure:"model"`
	APIKey           string   `yaml:"apiKey" json:"apiKey" mapstructure:"apiKey"`
	BaseURL          string   `yaml:"baseURL" json:"baseURL" mapstructure:"baseURL"`
	Temperature      float64  `yaml:"temperature" json:"temperature" mapstructure:"temperature"`
	MaxTokens        int      `yaml:"maxTokens" json:"maxTokens" mapstructure:"maxTokens"`
	TopP             float64  `yaml:"topP" json:"topP" mapstructure:"topP"`
	FrequencyPenalty float64  `yaml:"frequencyPenalty" json:"frequencyPenalty" mapstructure:"frequencyPenalty"`
	PresencePenalty  float64  `yaml:"presencePenalty" json:"presencePenalty" mapstructure:"presencePenalty"`
	StopSequences    []string `yaml:"stopSequences" json:"stopSequences" mapstructure:"stopSequences"`
	Timeout          int      `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	MaxRetries       int      `yaml:"maxRetries" json:"maxRetries" mapstructure:"maxRetries"`
	EnableStreaming  bool     `yaml:"enableStreaming" json:"enableStreaming" mapstructure:"enableStreaming"`
}

// ToolConfig represents tool-specific configuration
type ToolConfig struct {
	Name        string                 `yaml:"name" json:"name" mapstructure:"name"`
	Enabled     bool                   `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	Timeout     int                    `yaml:"timeout" json:"timeout" mapstructure:"timeout"`
	MaxRetries  int                    `yaml:"maxRetries" json:"maxRetries" mapstructure:"maxRetries"`
	Settings    map[string]interface{} `yaml:"settings" json:"settings" mapstructure:"settings"`
	Permissions []string               `yaml:"permissions" json:"permissions" mapstructure:"permissions"`
}

// NewDefaultConfig creates a new Config with default values
func NewDefaultConfig() *Config {
	return &Config{
		DefaultLanguage:  "en",
		OutputFormat:     "text",
		AnalysisDepth:    3,
		BackupOnRefactor: true,
		ExcludePatterns:  []string{".git", "node_modules", ".vscode", ".idea"},

		AllowedTools:    []string{"file_read", "file_write", "bash", "search"},
		MaxIterations:   10,
		Timeout:         300,
		EnableStreaming: true,
		EnableMemory:    true,

		Temperature:      0.7,
		MaxTokens:        2000,
		TopP:             0.9,
		FrequencyPenalty: 0.0,
		PresencePenalty:  0.0,
		StopSequences:    []string{},

		StorageType:      "file",
		StoragePath:      "~/.alex",
		SessionTimeout:   3600,
		SessionStorePath: "~/.alex/sessions",
		AutoSaveInterval: 300,

		EnableSandbox:   true,
		AllowedPaths:    []string{"."},
		BlockedCommands: []string{"rm -rf", "sudo", "chmod 777"},
		MaxFileSize:     10485760, // 10MB

		LogLevel:    "info",
		LogFilePath: "~/.alex/logs/alex.log",
		EnableDebug: false,

		Theme:        "default",
		EnableTUI:    false,
		ShowProgress: true,
		ColorOutput:  true,
	}
}
