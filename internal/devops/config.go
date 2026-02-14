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

	// ACP
	ACPPort    int    `env:"ACP_PORT" yaml:"acp_port" default:"0"`
	ACPHost    string `env:"ACP_HOST" yaml:"acp_host" default:"127.0.0.1"`
	ACPRunMode string `env:"ACP_RUN_MODE" yaml:"acp_run_mode" default:"sandbox"`

	// Auto-management
	AutoStopConflictingPorts bool   `env:"AUTO_STOP_CONFLICTING_PORTS" yaml:"auto_stop_conflicting_ports" default:"true"`
	CGOMode                  string `env:"ALEX_CGO_MODE" yaml:"cgo_mode" default:"auto"`
	LarkMode                 bool   `env:"ALEX_DEV_LARK" yaml:"lark_mode"`

	// Directories
	ProjectDir string `yaml:"-"` // Set at runtime, not from config
	PIDDir     string `env:"LARK_PID_DIR" yaml:"pid_dir" default:".pids"`
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

	if strings.TrimSpace(cfg.PIDDir) == ".pids" {
		cfg.PIDDir = defaultSharedPIDDir(configPath, cfg.ProjectDir)
	}

	cfg.PIDDir = cfg.resolvePath(cfg.PIDDir)
	cfg.LogDir = cfg.resolvePath(cfg.LogDir)
	cfg.ServerBin = cfg.resolvePath(cfg.ServerBin)
	cfg.WebDir = cfg.resolvePath(cfg.WebDir)

	return cfg, nil
}

func defaultSharedPIDDir(configPath, projectDir string) string {
	path := strings.TrimSpace(configPath)
	if path == "" {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			path = filepath.Join(home, ".alex", "config.yaml")
		} else {
			path = filepath.Join(projectDir, "config.yaml")
		}
	}

	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	if !filepath.IsAbs(path) {
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
	}

	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil && strings.TrimSpace(resolved) != "" {
		path = resolved
	}

	return filepath.Join(filepath.Dir(path), "pids")
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
