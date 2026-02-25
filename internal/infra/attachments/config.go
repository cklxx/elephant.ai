package attachments

import "strings"

// NormalizeConfig fills attachment store defaults when unset.
func NormalizeConfig(cfg StoreConfig) StoreConfig {
	if strings.TrimSpace(cfg.Dir) == "" {
		cfg.Dir = "~/.alex/attachments"
	}
	if strings.TrimSpace(cfg.Provider) == "" {
		cfg.Provider = ProviderLocal
	}
	return cfg
}
