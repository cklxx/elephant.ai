package bootstrap

import (
	"net/http"
	"time"

	"alex/internal/infra/gitsignal"
	runtimeconfig "alex/internal/shared/config"
)

// GitSignalStage returns a BootstrapStage that initializes the git signal
// provider and wires it into the DI container.
func (f *Foundation) GitSignalStage() BootstrapStage {
	return BootstrapStage{
		Name: "git-signal", Required: false,
		Init: func() error {
			cfg := f.Config.Runtime.Proactive.Scheduler.GitSignal
			if !cfg.Enabled {
				return nil
			}
			provider := cfg.Provider
			if provider == "" {
				provider = "github"
			}

			switch provider {
			case "github":
				return f.initGitHubSignal(cfg)
			default:
				f.Logger.Warn("GitSignal: unsupported provider %q, skipping", provider)
				return nil
			}
		},
	}
}

func (f *Foundation) initGitHubSignal(cfg gitSignalConfig) error {
	pollInterval := time.Duration(cfg.PollIntervalSeconds) * time.Second
	if pollInterval <= 0 {
		pollInterval = 5 * time.Minute
	}
	bottleneckThreshold := time.Duration(cfg.ReviewBottleneckThreshold) * time.Second
	if bottleneckThreshold <= 0 {
		bottleneckThreshold = 24 * time.Hour
	}

	ghCfg := gitsignal.GitHubConfig{
		Token:                     cfg.Token,
		Repos:                     cfg.Repos,
		BaseURL:                   cfg.BaseURL,
		PollInterval:              pollInterval,
		ReviewBottleneckThreshold: bottleneckThreshold,
	}

	client := &http.Client{Timeout: 30 * time.Second}
	provider := gitsignal.NewGitHubProvider(ghCfg, client, f.Logger)
	f.Container.GitSignalProvider = provider

	f.Logger.Info("GitSignal: GitHub provider initialized (repos=%d)", len(cfg.Repos))
	return nil
}

// gitSignalConfig is a type alias to keep the import clean.
type gitSignalConfig = runtimeconfig.GitSignalConfig
