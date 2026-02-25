package attachments

import (
	"alex/internal/shared/utils"
)

// NormalizeConfig fills attachment store defaults when unset.
func NormalizeConfig(cfg StoreConfig) StoreConfig {
	if utils.IsBlank(cfg.Dir) {
		cfg.Dir = "~/.alex/attachments"
	}
	if utils.IsBlank(cfg.Provider) {
		cfg.Provider = ProviderLocal
	}
	return cfg
}
