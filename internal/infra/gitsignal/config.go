package gitsignal

import "time"

// GitHubConfig configures the GitHub signal provider.
type GitHubConfig struct {
	// Token is the GitHub personal access token or app installation token.
	Token string `json:"token" yaml:"token"`

	// Repos is the list of repositories to monitor (owner/repo format).
	Repos []string `json:"repos" yaml:"repos"`

	// BaseURL overrides the GitHub API base URL for GitHub Enterprise.
	// Leave empty for github.com (defaults to https://api.github.com).
	BaseURL string `json:"base_url" yaml:"base_url"`

	// PollInterval controls how often the provider polls for new events.
	// Defaults to 5 minutes if zero.
	PollInterval time.Duration `json:"poll_interval" yaml:"poll_interval"`

	// ReviewBottleneckThreshold is the duration after which a pending
	// review is flagged as a bottleneck. Defaults to 24 hours if zero.
	ReviewBottleneckThreshold time.Duration `json:"review_bottleneck_threshold" yaml:"review_bottleneck_threshold"`
}

// DefaultGitHubConfig returns a GitHubConfig with sensible defaults.
func DefaultGitHubConfig() GitHubConfig {
	return GitHubConfig{
		BaseURL:                   "https://api.github.com",
		PollInterval:              5 * time.Minute,
		ReviewBottleneckThreshold: 24 * time.Hour,
	}
}

func (c GitHubConfig) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return "https://api.github.com"
}

func (c GitHubConfig) reviewThreshold() time.Duration {
	if c.ReviewBottleneckThreshold > 0 {
		return c.ReviewBottleneckThreshold
	}
	return 24 * time.Hour
}
