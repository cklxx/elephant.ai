package devops

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// DevConfig holds all configuration for the development environment.
type DevConfig struct {
	// Backend
	ServerPort int    `env:"SERVER_PORT" yaml:"server_port" default:"8080"`
	ServerBin  string `yaml:"server_bin" default:"./alex-server"`

	// Web
	WebPort int    `env:"WEB_PORT" yaml:"web_port" default:"3000"`
	WebDir  string `yaml:"web_dir" default:"./web"`

	// Sandbox
	SandboxPort           int    `env:"SANDBOX_PORT" yaml:"sandbox_port" default:"18086"`
	SandboxImage          string `env:"SANDBOX_IMAGE" yaml:"sandbox_image" default:"ghcr.io/agent-infra/sandbox:latest"`
	SandboxContainer      string `env:"SANDBOX_CONTAINER_NAME" yaml:"sandbox_container" default:"alex-sandbox"`
	SandboxBaseURL        string `env:"SANDBOX_BASE_URL" yaml:"sandbox_base_url"`
	SandboxAutoInstallCLI bool   `env:"SANDBOX_AUTO_INSTALL_CLI" yaml:"sandbox_auto_install_cli" default:"true"`
	SandboxConfigPath     string `yaml:"sandbox_config_path" default:"/root/.alex/config.yaml"`

	// ACP
	ACPPort             int    `env:"ACP_PORT" yaml:"acp_port" default:"0"`
	ACPHost             string `env:"ACP_HOST" yaml:"acp_host" default:"127.0.0.1"`
	ACPRunMode          string `env:"ACP_RUN_MODE" yaml:"acp_run_mode" default:"sandbox"`
	StartACPWithSandbox bool   `env:"START_ACP_WITH_SANDBOX" yaml:"start_acp_with_sandbox" default:"true"`

	// Auth DB
	AuthDatabaseURL string `env:"AUTH_DATABASE_URL" yaml:"auth_database_url"`
	AuthJWTSecret   string `env:"AUTH_JWT_SECRET" yaml:"auth_jwt_secret" default:"dev-secret-change-me"`
	SkipLocalAuthDB bool   `env:"SKIP_LOCAL_AUTH_DB" yaml:"skip_local_auth_db"`

	// Auto-management
	AutoStopConflictingPorts bool   `env:"AUTO_STOP_CONFLICTING_PORTS" yaml:"auto_stop_conflicting_ports" default:"true"`
	CGOMode                  string `env:"ALEX_CGO_MODE" yaml:"cgo_mode" default:"auto"`

	// Directories
	ProjectDir string `yaml:"-"` // Set at runtime, not from config
	PIDDir     string `yaml:"pid_dir" default:".pids"`
	LogDir     string `yaml:"log_dir" default:"logs"`

	// Supervisor (Lark)
	Supervisor SupervisorConfig `yaml:"supervisor"`
}

// SupervisorConfig holds Lark supervisor settings.
type SupervisorConfig struct {
	TickInterval       time.Duration `yaml:"tick_interval" default:"5s"`
	RestartMaxInWindow int           `yaml:"restart_max_in_window" default:"5"`
	RestartWindow      time.Duration `yaml:"restart_window" default:"10m"`
	CooldownDuration   time.Duration `yaml:"cooldown_duration" default:"5m"`
	AutofixEnabled     bool          `yaml:"autofix_enabled" default:"true"`
	AutofixTrigger     string        `yaml:"autofix_trigger" default:"cooldown"`
	AutofixTimeout     time.Duration `yaml:"autofix_timeout" default:"30m"`
	AutofixMaxInWindow int           `yaml:"autofix_max_in_window" default:"3"`
	AutofixWindow      time.Duration `yaml:"autofix_window" default:"1h"`
	AutofixCooldown    time.Duration `yaml:"autofix_cooldown" default:"15m"`
	AutofixScope       string        `yaml:"autofix_scope" default:"repo"`
}

// LoadDevConfig loads configuration with the priority:
// code defaults -> config file -> .env -> environment -> overrides.
func LoadDevConfig(configPath string) (*DevConfig, error) {
	cfg := &DevConfig{}
	applyDefaults(cfg)

	if configPath != "" {
		if err := loadYAML(configPath, cfg); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("load config %s: %w", configPath, err)
			}
		}
	}

	applyEnv(cfg)

	if cfg.ProjectDir == "" {
		dir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		cfg.ProjectDir = dir
	}

	if cfg.SandboxBaseURL == "" {
		cfg.SandboxBaseURL = fmt.Sprintf("http://localhost:%d", cfg.SandboxPort)
	}

	cfg.PIDDir = cfg.resolvePath(cfg.PIDDir)
	cfg.LogDir = cfg.resolvePath(cfg.LogDir)
	cfg.ServerBin = cfg.resolvePath(cfg.ServerBin)
	cfg.WebDir = cfg.resolvePath(cfg.WebDir)

	return cfg, nil
}

func (c *DevConfig) resolvePath(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(c.ProjectDir, p)
}

func loadYAML(path string, cfg *DevConfig) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse yaml: %w", err)
	}

	devSection, ok := raw["devops"]
	if !ok {
		return nil
	}

	devData, err := yaml.Marshal(devSection)
	if err != nil {
		return fmt.Errorf("re-marshal devops section: %w", err)
	}

	return yaml.Unmarshal(devData, cfg)
}

func applyDefaults(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)

		if !fv.CanSet() {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
			applyDefaults(fv.Addr().Interface())
			continue
		}

		tag := field.Tag.Get("default")
		if tag == "" {
			continue
		}

		if fv.IsZero() {
			setFieldFromString(fv, field.Type, tag)
		}
	}
}

func applyEnv(v any) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	rt := rv.Type()

	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fv := rv.Field(i)

		if !fv.CanSet() {
			continue
		}

		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(time.Duration(0)) {
			applyEnv(fv.Addr().Interface())
			continue
		}

		envKey := field.Tag.Get("env")
		if envKey == "" {
			continue
		}

		envVal, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		setFieldFromString(fv, field.Type, envVal)
	}
}

func setFieldFromString(fv reflect.Value, ft reflect.Type, val string) {
	switch ft.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int:
		if n, err := strconv.Atoi(val); err == nil {
			fv.SetInt(int64(n))
		}
	case reflect.Bool:
		switch strings.ToLower(val) {
		case "true", "1", "yes":
			fv.SetBool(true)
		case "false", "0", "no":
			fv.SetBool(false)
		}
	case reflect.Int64:
		if ft == reflect.TypeOf(time.Duration(0)) {
			if d, err := time.ParseDuration(val); err == nil {
				fv.SetInt(int64(d))
			}
		} else {
			if n, err := strconv.ParseInt(val, 10, 64); err == nil {
				fv.SetInt(n)
			}
		}
	}
}
