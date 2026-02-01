package shared

import "time"

// FileToolConfig is reserved for optional configuration in file-based tools.
type FileToolConfig struct{}

// ShellToolConfig is reserved for optional configuration in shell-based tools.
type ShellToolConfig struct{}

// WebFetchConfig configures the web_fetch tool cache behavior.
type WebFetchConfig struct {
	CacheTTL             time.Duration `yaml:"cache_ttl"`
	CacheMaxEntries      int           `yaml:"cache_max_entries"`
	CacheMaxContentBytes int           `yaml:"cache_max_content_bytes"`
	MaxResponseBytes     int           `yaml:"max_response_bytes"`
}
