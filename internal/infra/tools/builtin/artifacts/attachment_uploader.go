package artifacts

import (
	"strings"
	"time"

	"alex/internal/domain/materials"
	"alex/internal/infra/attachments"
	"alex/internal/infra/httpclient"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// BuildAttachmentStoreMigrator creates a migrator that persists inline attachments.
func BuildAttachmentStoreMigrator(loggerName string, timeout time.Duration) (*materials.AttachmentStoreMigrator, error) {
	fileCfg, _, err := config.LoadFileConfig(config.WithEnv(config.DefaultEnvLookup))
	if err != nil {
		return nil, err
	}
	if fileCfg.Attachments == nil {
		return nil, nil
	}

	raw := fileCfg.Attachments
	cfg := attachments.StoreConfig{
		Provider:                  strings.TrimSpace(raw.Provider),
		Dir:                       strings.TrimSpace(raw.Dir),
		CloudflareAccountID:       strings.TrimSpace(raw.CloudflareAccountID),
		CloudflareAccessKeyID:     strings.TrimSpace(raw.CloudflareAccessKeyID),
		CloudflareSecretAccessKey: strings.TrimSpace(raw.CloudflareSecretAccessKey),
		CloudflareBucket:          strings.TrimSpace(raw.CloudflareBucket),
		CloudflarePublicBaseURL:   strings.TrimSpace(raw.CloudflarePublicBaseURL),
		CloudflareKeyPrefix:       strings.TrimSpace(raw.CloudflareKeyPrefix),
	}
	if ttlRaw := strings.TrimSpace(raw.PresignTTL); ttlRaw != "" {
		if parsed, err := time.ParseDuration(ttlRaw); err == nil && parsed > 0 {
			cfg.PresignTTL = parsed
		}
	}
	cfg = attachments.NormalizeConfig(cfg)

	store, err := attachments.NewStore(cfg)
	if err != nil {
		return nil, err
	}

	logger := logging.NewComponentLogger(loggerName)
	client := httpclient.New(timeout, logger)
	return materials.NewAttachmentStoreMigrator(store, client, cfg.CloudflarePublicBaseURL, logger), nil
}
