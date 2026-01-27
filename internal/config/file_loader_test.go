package config

import (
	"os"
	"testing"
)

func TestLoadFileConfigExpandsEnv(t *testing.T) {
	data := []byte(`
runtime:
  api_key: "${OPENAI_API_KEY}"
  tool_max_concurrent: 12
server:
  port: "${PORT}"
  enable_mcp: false
  max_task_body_bytes: 1048576
  allowed_origins:
    - "http://${ORIGIN_HOST}"
auth:
  jwt_secret: "${AUTH_JWT_SECRET}"
  access_token_ttl_minutes: "${AUTH_ACCESS_TOKEN_TTL_MINUTES}"
session:
  database_url: "${SESSION_DB}"
analytics:
  posthog_api_key: "${POSTHOG_API_KEY}"
attachments:
  provider: "cloudflare"
  cloudflare_account_id: "${CF_ACCOUNT}"
  presign_ttl: "20m"
web:
  api_url: "http://${WEB_HOST}"
channels:
  wechat:
    enabled: true
    login_mode: "${WECHAT_LOGIN_MODE}"
    hot_login_storage_path: "${WECHAT_STORAGE}"
    session_prefix: "wechat"
    reply_prefix: "${WECHAT_REPLY_PREFIX}"
    mention_only: false
    allow_groups: true
    allow_direct: true
    allowed_conversation_ids:
      - "${WECHAT_ALLOW_1}"
`)

	env := envMap{
		"OPENAI_API_KEY":                "secret",
		"PORT":                          "8081",
		"ORIGIN_HOST":                   "example.com",
		"AUTH_JWT_SECRET":               "jwt-secret",
		"AUTH_ACCESS_TOKEN_TTL_MINUTES": "20",
		"SESSION_DB":                    "postgres://localhost:5432/app",
		"POSTHOG_API_KEY":               "ph-key",
		"CF_ACCOUNT":                    "cf-account",
		"WEB_HOST":                      "localhost:3000",
		"WECHAT_LOGIN_MODE":             "desktop",
		"WECHAT_STORAGE":                "~/.alex/wechat",
		"WECHAT_REPLY_PREFIX":           "[bot] ",
		"WECHAT_ALLOW_1":                "conv-1",
	}

	cfg, path, err := LoadFileConfig(
		WithEnv(env.Lookup),
		WithFileReader(func(string) ([]byte, error) { return data, nil }),
		WithConfigPath("/tmp/config.yaml"),
	)
	if err != nil {
		t.Fatalf("LoadFileConfig error: %v", err)
	}
	if path != "/tmp/config.yaml" {
		t.Fatalf("expected config path, got %q", path)
	}
	if cfg.Runtime == nil || cfg.Runtime.APIKey != "secret" {
		t.Fatalf("expected runtime api key to expand, got %#v", cfg.Runtime)
	}
	if cfg.Runtime == nil || cfg.Runtime.ToolMaxConcurrent == nil || *cfg.Runtime.ToolMaxConcurrent != 12 {
		t.Fatalf("expected tool_max_concurrent to parse, got %#v", cfg.Runtime)
	}
	if cfg.Server == nil || cfg.Server.Port != "8081" || cfg.Server.EnableMCP == nil || *cfg.Server.EnableMCP {
		t.Fatalf("expected server config to expand, got %#v", cfg.Server)
	}
	if cfg.Server == nil || cfg.Server.MaxTaskBodyBytes == nil || *cfg.Server.MaxTaskBodyBytes != 1048576 {
		t.Fatalf("expected max task body bytes to parse, got %#v", cfg.Server)
	}
	if cfg.Server == nil || len(cfg.Server.AllowedOrigins) != 1 || cfg.Server.AllowedOrigins[0] != "http://example.com" {
		t.Fatalf("expected allowed origins to expand, got %#v", cfg.Server)
	}
	if cfg.Auth == nil || cfg.Auth.JWTSecret != "jwt-secret" || cfg.Auth.AccessTokenTTLMinutes != "20" {
		t.Fatalf("expected auth config to expand, got %#v", cfg.Auth)
	}
	if cfg.Session == nil || cfg.Session.DatabaseURL != "postgres://localhost:5432/app" {
		t.Fatalf("expected session config to expand, got %#v", cfg.Session)
	}
	if cfg.Analytics == nil || cfg.Analytics.PostHogAPIKey != "ph-key" {
		t.Fatalf("expected analytics config to expand, got %#v", cfg.Analytics)
	}
	if cfg.Attachments == nil || cfg.Attachments.CloudflareAccountID != "cf-account" || cfg.Attachments.PresignTTL != "20m" {
		t.Fatalf("expected attachments config to expand, got %#v", cfg.Attachments)
	}
	if cfg.Web == nil || cfg.Web.APIURL != "http://localhost:3000" {
		t.Fatalf("expected web config to expand, got %#v", cfg.Web)
	}
	if cfg.Channels == nil || cfg.Channels.WeChat == nil {
		t.Fatalf("expected channels config to expand, got %#v", cfg.Channels)
	}
	if cfg.Channels.WeChat.LoginMode != "desktop" || cfg.Channels.WeChat.HotLoginStoragePath != "~/.alex/wechat" {
		t.Fatalf("expected wechat channel values to expand, got %#v", cfg.Channels.WeChat)
	}
	if cfg.Channels.WeChat.ReplyPrefix != "[bot] " {
		t.Fatalf("expected wechat reply prefix to expand, got %#v", cfg.Channels.WeChat.ReplyPrefix)
	}
	if cfg.Channels.WeChat.AllowedConversationIDs == nil || len(cfg.Channels.WeChat.AllowedConversationIDs) != 1 || cfg.Channels.WeChat.AllowedConversationIDs[0] != "conv-1" {
		t.Fatalf("expected wechat allowlist to expand, got %#v", cfg.Channels.WeChat.AllowedConversationIDs)
	}
}

func TestLoadFileConfigMissingFileReturnsEmpty(t *testing.T) {
	cfg, path, err := LoadFileConfig(
		WithEnv(envMap{}.Lookup),
		WithFileReader(func(string) ([]byte, error) { return nil, os.ErrNotExist }),
		WithConfigPath("/tmp/missing.yaml"),
	)
	if err != nil {
		t.Fatalf("LoadFileConfig error: %v", err)
	}
	if path != "/tmp/missing.yaml" {
		t.Fatalf("expected config path, got %q", path)
	}
	if cfg != (FileConfig{}) {
		t.Fatalf("expected empty config, got %#v", cfg)
	}
}
