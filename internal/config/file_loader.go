package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadFileConfig loads the YAML config file and returns all sections with env interpolation applied.
func LoadFileConfig(opts ...Option) (FileConfig, string, error) {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	configPath := strings.TrimSpace(options.configPath)
	if configPath == "" {
		configPath, _ = ResolveConfigPath(options.envLookup, options.homeDir)
	}
	if configPath == "" {
		return FileConfig{}, "", nil
	}

	data, err := options.readFile(configPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return FileConfig{}, configPath, nil
		}
		return FileConfig{}, configPath, fmt.Errorf("read config file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return FileConfig{}, configPath, nil
	}

	var parsed FileConfig
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		return FileConfig{}, configPath, fmt.Errorf("parse config file: %w", err)
	}

	parsed = expandFileConfigEnv(options.envLookup, parsed)

	return parsed, configPath, nil
}

func expandFileConfigEnv(lookup EnvLookup, parsed FileConfig) FileConfig {
	if parsed.Runtime != nil {
		expanded := expandRuntimeFileConfigEnv(lookup, *parsed.Runtime)
		parsed.Runtime = &expanded
	}
	if parsed.Apps != nil {
		expanded := expandAppsConfigEnv(lookup, *parsed.Apps)
		parsed.Apps = &expanded
	}
	if parsed.Server != nil {
		parsed.Server.Port = expandEnvValue(lookup, parsed.Server.Port)
		if len(parsed.Server.AllowedOrigins) > 0 {
			origins := make([]string, 0, len(parsed.Server.AllowedOrigins))
			for _, origin := range parsed.Server.AllowedOrigins {
				origins = append(origins, expandEnvValue(lookup, origin))
			}
			parsed.Server.AllowedOrigins = origins
		}
	}
	if parsed.Auth != nil {
		parsed.Auth.JWTSecret = expandEnvValue(lookup, parsed.Auth.JWTSecret)
		parsed.Auth.AccessTokenTTLMinutes = expandEnvValue(lookup, parsed.Auth.AccessTokenTTLMinutes)
		parsed.Auth.RefreshTokenTTLDays = expandEnvValue(lookup, parsed.Auth.RefreshTokenTTLDays)
		parsed.Auth.StateTTLMinutes = expandEnvValue(lookup, parsed.Auth.StateTTLMinutes)
		parsed.Auth.RedirectBaseURL = expandEnvValue(lookup, parsed.Auth.RedirectBaseURL)
		parsed.Auth.GoogleClientID = expandEnvValue(lookup, parsed.Auth.GoogleClientID)
		parsed.Auth.GoogleClientSecret = expandEnvValue(lookup, parsed.Auth.GoogleClientSecret)
		parsed.Auth.GoogleAuthURL = expandEnvValue(lookup, parsed.Auth.GoogleAuthURL)
		parsed.Auth.GoogleTokenURL = expandEnvValue(lookup, parsed.Auth.GoogleTokenURL)
		parsed.Auth.GoogleUserInfoURL = expandEnvValue(lookup, parsed.Auth.GoogleUserInfoURL)
		parsed.Auth.WeChatAppID = expandEnvValue(lookup, parsed.Auth.WeChatAppID)
		parsed.Auth.WeChatAuthURL = expandEnvValue(lookup, parsed.Auth.WeChatAuthURL)
		parsed.Auth.DatabaseURL = expandEnvValue(lookup, parsed.Auth.DatabaseURL)
		parsed.Auth.BootstrapEmail = expandEnvValue(lookup, parsed.Auth.BootstrapEmail)
		parsed.Auth.BootstrapPassword = expandEnvValue(lookup, parsed.Auth.BootstrapPassword)
		parsed.Auth.BootstrapDisplayName = expandEnvValue(lookup, parsed.Auth.BootstrapDisplayName)
	}
	if parsed.Session != nil {
		parsed.Session.DatabaseURL = expandEnvValue(lookup, parsed.Session.DatabaseURL)
		parsed.Session.Dir = expandEnvValue(lookup, parsed.Session.Dir)
	}
	if parsed.Analytics != nil {
		parsed.Analytics.PostHogAPIKey = expandEnvValue(lookup, parsed.Analytics.PostHogAPIKey)
		parsed.Analytics.PostHogHost = expandEnvValue(lookup, parsed.Analytics.PostHogHost)
	}
	if parsed.Attachments != nil {
		parsed.Attachments.Provider = expandEnvValue(lookup, parsed.Attachments.Provider)
		parsed.Attachments.Dir = expandEnvValue(lookup, parsed.Attachments.Dir)
		parsed.Attachments.CloudflareAccountID = expandEnvValue(lookup, parsed.Attachments.CloudflareAccountID)
		parsed.Attachments.CloudflareAccessKeyID = expandEnvValue(lookup, parsed.Attachments.CloudflareAccessKeyID)
		parsed.Attachments.CloudflareSecretAccessKey = expandEnvValue(lookup, parsed.Attachments.CloudflareSecretAccessKey)
		parsed.Attachments.CloudflareBucket = expandEnvValue(lookup, parsed.Attachments.CloudflareBucket)
		parsed.Attachments.CloudflarePublicBaseURL = expandEnvValue(lookup, parsed.Attachments.CloudflarePublicBaseURL)
		parsed.Attachments.CloudflareKeyPrefix = expandEnvValue(lookup, parsed.Attachments.CloudflareKeyPrefix)
		parsed.Attachments.PresignTTL = expandEnvValue(lookup, parsed.Attachments.PresignTTL)
	}
	if parsed.Web != nil {
		parsed.Web.APIURL = expandEnvValue(lookup, parsed.Web.APIURL)
	}

	return parsed
}

func expandAppsConfigEnv(lookup EnvLookup, parsed AppsConfig) AppsConfig {
	if len(parsed.Plugins) == 0 {
		return parsed
	}
	plugins := make([]AppPluginConfig, 0, len(parsed.Plugins))
	for _, plugin := range parsed.Plugins {
		plugin.ID = expandEnvValue(lookup, plugin.ID)
		plugin.Name = expandEnvValue(lookup, plugin.Name)
		plugin.Description = expandEnvValue(lookup, plugin.Description)
		plugin.IntegrationNote = expandEnvValue(lookup, plugin.IntegrationNote)
		if len(plugin.Capabilities) > 0 {
			caps := make([]string, 0, len(plugin.Capabilities))
			for _, cap := range plugin.Capabilities {
				caps = append(caps, expandEnvValue(lookup, cap))
			}
			plugin.Capabilities = caps
		}
		if len(plugin.Sources) > 0 {
			sources := make([]string, 0, len(plugin.Sources))
			for _, src := range plugin.Sources {
				sources = append(sources, expandEnvValue(lookup, src))
			}
			plugin.Sources = sources
		}
		plugins = append(plugins, plugin)
	}
	parsed.Plugins = plugins
	return parsed
}
