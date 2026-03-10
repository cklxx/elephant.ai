package bootstrap

import (
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

func applyServerHTTPConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Server == nil {
		return
	}
	applyTrimmedString(&cfg.Port, file.Server.Port)
	applyTrimmedString(&cfg.DebugPort, file.Server.DebugPort)
	applyTrimmedString(&cfg.DebugBindHost, file.Server.DebugBindHost)
	applyPositiveInt64(&cfg.MaxTaskBodyBytes, file.Server.MaxTaskBodyBytes)
	applyTrimmedString(&cfg.LeaderAPIToken, file.Server.LeaderAPIToken)
	applyPositiveDuration(&cfg.NonStreamTimeout, file.Server.NonStreamTimeoutSeconds, time.Second)
	applyStreamGuardConfig(&cfg.StreamGuard, file.Server)
	applyRateLimitConfig(&cfg.RateLimit, file.Server)
	applyTaskExecutionConfig(&cfg.TaskExecution, file.Server)
	applyEventHistoryConfig(&cfg.EventHistory, file.Server)
	if file.Server.AllowedOrigins != nil {
		cfg.AllowedOrigins = normalizeAllowedOrigins(file.Server.AllowedOrigins)
	}
	if len(file.Server.TrustedProxies) > 0 {
		cfg.RateLimit.TrustedProxies = append([]string(nil), file.Server.TrustedProxies...)
	}
}

func applyStreamGuardConfig(dst *StreamGuardConfig, srv *runtimeconfig.ServerConfig) {
	applyPositiveDuration(&dst.MaxDuration, srv.StreamMaxDurationSeconds, time.Second)
	applyPositiveInt64(&dst.MaxBytes, srv.StreamMaxBytes)
	applyPositiveInt(&dst.MaxConcurrent, srv.StreamMaxConcurrent)
}

func applyRateLimitConfig(dst *RateLimitConfig, srv *runtimeconfig.ServerConfig) {
	applyPositiveInt(&dst.RequestsPerMinute, srv.RateLimitRequestsPerMinute)
	applyPositiveInt(&dst.Burst, srv.RateLimitBurst)
}

func applyTaskExecutionConfig(dst *TaskExecutionConfig, srv *runtimeconfig.ServerConfig) {
	applyTrimmedString(&dst.OwnerID, srv.TaskExecutionOwnerID)
	applyPositiveDuration(&dst.LeaseTTL, srv.TaskExecutionLeaseTTLSeconds, time.Second)
	applyPositiveDuration(&dst.LeaseRenewInterval, srv.TaskExecutionLeaseRenewIntervalSeconds, time.Second)
	applyNonNegativeInt(&dst.MaxInFlight, srv.TaskExecutionMaxInFlight)
	applyPositiveInt(&dst.ResumeClaimBatchSize, srv.TaskExecutionResumeClaimBatchSize)
}

func applyEventHistoryConfig(dst *EventHistoryConfig, srv *runtimeconfig.ServerConfig) {
	applyNonNegativeDuration(&dst.Retention, srv.EventHistoryRetentionDays, 24*time.Hour)
	applyNonNegativeInt(&dst.MaxSessions, srv.EventHistoryMaxSessions)
	applyNonNegativeDuration(&dst.SessionTTL, srv.EventHistorySessionTTL, time.Second)
	applyNonNegativeInt(&dst.MaxEvents, srv.EventHistoryMaxEvents)
}

func applySessionConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Session == nil {
		return
	}
	if dir := strings.TrimSpace(file.Session.Dir); dir != "" {
		cfg.Session.Dir = dir
	}
}

func applyAnalyticsConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Analytics == nil {
		return
	}
	cfg.Analytics = runtimeconfig.AnalyticsConfig{
		PostHogAPIKey: strings.TrimSpace(file.Analytics.PostHogAPIKey),
		PostHogHost:   strings.TrimSpace(file.Analytics.PostHogHost),
	}
}

func applyAttachmentConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Attachments == nil {
		return
	}
	a := file.Attachments
	applyTrimmedString(&cfg.Attachment.Provider, a.Provider)
	applyTrimmedString(&cfg.Attachment.Dir, a.Dir)
	applyTrimmedString(&cfg.Attachment.CloudflareAccountID, a.CloudflareAccountID)
	applyTrimmedString(&cfg.Attachment.CloudflareAccessKeyID, a.CloudflareAccessKeyID)
	applyTrimmedString(&cfg.Attachment.CloudflareSecretAccessKey, a.CloudflareSecretAccessKey)
	applyTrimmedString(&cfg.Attachment.CloudflareBucket, a.CloudflareBucket)
	applyTrimmedString(&cfg.Attachment.CloudflarePublicBaseURL, a.CloudflarePublicBaseURL)
	applyTrimmedString(&cfg.Attachment.CloudflareKeyPrefix, a.CloudflareKeyPrefix)
	if ttlRaw := strings.TrimSpace(a.PresignTTL); ttlRaw != "" {
		if parsed, err := time.ParseDuration(ttlRaw); err == nil && parsed > 0 {
			cfg.Attachment.PresignTTL = parsed
		}
	}
}

func normalizeAllowedOrigins(values []string) []string {
	return utils.TrimDedupeStrings(values)
}

func lookupFirstNonEmptyEnv(lookup runtimeconfig.EnvLookup, keys ...string) string {
	if lookup == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := lookup(key); ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func applyTrimmedString(dst *string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalTrimmedString(dst *string, value *string) {
	if value == nil {
		return
	}
	*dst = strings.TrimSpace(*value)
}

func applyTrimmedLowerString(dst *string, value string) {
	trimmed := utils.TrimLower(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalBool(dst *bool, value *bool) {
	if value != nil {
		*dst = *value
	}
}

func applyPositiveInt(dst *int, value *int) {
	if value != nil && *value > 0 {
		*dst = *value
	}
}

func applyPositiveDuration(dst *time.Duration, value *int, unit time.Duration) {
	if value != nil && *value > 0 {
		*dst = time.Duration(*value) * unit
	}
}

func applyPositiveInt64(dst *int64, value *int64) {
	if value != nil && *value > 0 {
		*dst = *value
	}
}

func applyNonNegativeInt(dst *int, value *int) {
	if value == nil {
		return
	}
	if *value <= 0 {
		*dst = 0
	} else {
		*dst = *value
	}
}

func applyNonNegativeDuration(dst *time.Duration, value *int, unit time.Duration) {
	if value == nil {
		return
	}
	if *value <= 0 {
		*dst = 0
	} else {
		*dst = time.Duration(*value) * unit
	}
}
