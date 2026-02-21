package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"alex/internal/devops"
	"alex/internal/devops/services"
)

func runDevCommand(args []string) error {
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	switch cmd {
	case "up", "start":
		larkRequested := hasFlag(args, "--lark")
		return devUp(larkRequested)
	case "down", "stop":
		return devDown(args...)
	case "status":
		return devStatus()
	case "logs":
		target := "all"
		if len(args) > 0 {
			target = args[0]
		}
		return devLogs(target)
	case "restart":
		return devRestart(args...)
	case "test":
		return devTest()
	case "lint":
		return devLint()
	case "lark":
		return runDevLarkCommand(args)
	case "logs-ui", "log-ui", "analyze-logs":
		return devLogsUI()
	case "help", "-h", "--help":
		printDevUsage()
		return nil
	default:
		return fmt.Errorf("unknown dev command: %s (run 'alex dev help')", cmd)
	}
}

func devUp(larkRequested bool) error {
	orch, err := buildOrchestrator()
	if err != nil {
		return err
	}

	larkMode := larkRequested || orch.Config().LarkMode

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := ensureLocalBootstrap(orch.Config().ProjectDir); err != nil {
		orch.Section().Warn("Bootstrap: %v", err)
	}

	if err := orch.Up(ctx); err != nil {
		return err
	}

	// Start lark supervisor if requested
	if larkMode {
		orch.Section().Section("Lark Supervisor")
		if err := larkStart(); err != nil {
			orch.Section().Warn("Lark supervisor: %v", err)
		}
	}

	printDevSummary(orch, larkMode)
	return nil
}

func printDevSummary(orch *devops.Orchestrator, larkMode bool) {
	cfg := orch.Config()
	sec := orch.Section()
	ctx := context.Background()

	// Services section
	sec.Section("Services")
	serviceURLs := map[string]string{
		"backend": fmt.Sprintf("http://localhost:%d", cfg.ServerPort),
		"web":     fmt.Sprintf("http://localhost:%d", cfg.WebPort),
	}
	for _, s := range orch.Status(ctx) {
		url := serviceURLs[s.Name]
		if s.Healthy {
			if url != "" {
				sec.Success("%-10s %s", s.Name, url)
			} else {
				sec.Success("%-10s ready", s.Name)
			}
		} else {
			sec.Warn("%-10s %s", s.Name, s.State)
		}
	}

	// Lark supervisor section (if enabled)
	if larkMode {
		printLarkSummary(sec)
	}

	// Dev tools section
	webBase := fmt.Sprintf("http://localhost:%d", cfg.WebPort)
	type devTool struct {
		name string
		path string
	}
	tools := []devTool{
		{"Evaluation", "/evaluation"},
		{"Sessions", "/sessions"},
		{"Conversation Debug", "/dev/conversation-debug"},
		{"Log Analyzer", "/dev/log-analyzer"},
		{"Context Window", "/dev/context-window"},
		{"Context Config", "/dev/context-config"},
		{"Config Inspector", "/dev/config"},
		{"Apps Config", "/dev/apps-config"},
	}
	sec.Section("Dev Tools")
	for _, t := range tools {
		sec.Info("%-20s %s%s", t.name, webBase, t.path)
	}
}

func devDown(flags ...string) error {
	stopAll := hasFlag(flags, "--all")

	orch, err := buildOrchestrator()
	if err != nil {
		return err
	}

	// Always try to stop lark supervisor if running
	if err := larkStop(); err != nil {
		orch.Section().Warn("Lark stop: %v", err)
	}

	ctx := context.Background()

	if stopAll {
		removeBootstrapMarker(orch.Config().PIDDir)
	}

	return orch.Down(ctx)
}

func devStatus() error {
	orch, err := buildOrchestrator()
	if err != nil {
		return err
	}

	sec := orch.Section()
	ctx := context.Background()
	statuses := orch.Status(ctx)
	for _, s := range statuses {
		if s.Healthy {
			sec.Success("%s: %s (PID: %d) %s", s.Name, s.State, s.PID, s.Message)
		} else {
			sec.Warn("%s: %s %s", s.Name, s.State, s.Message)
		}
	}

	// Show lark supervisor status if running
	printLarkSummary(sec)
	return nil
}

func devLogs(target string) error {
	orch, err := buildOrchestrator()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return orch.Logs(ctx, target, true)
}

func devRestart(names ...string) error {
	orch, err := buildOrchestrator()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return orch.Restart(ctx, names...)
}

func devTest() error {
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	goToolchain := filepath.Join(projectDir, "scripts", "go-with-toolchain.sh")
	cmd := exec.Command(goToolchain, "test", "-race", "-covermode=atomic", "-coverprofile=coverage.out", "./...")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set CGO mode
	env := os.Environ()
	if _, ok := os.LookupEnv("CGO_ENABLED"); !ok {
		env = append(env, "CGO_ENABLED=0")
	}
	cmd.Env = env

	return cmd.Run()
}

func devLint() error {
	projectDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Go lint
	fmt.Println("Running Go lint...")
	lintScript := filepath.Join(projectDir, "scripts", "run-golangci-lint.sh")
	cmd := exec.Command(lintScript, "run", "./...")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go lint: %w", err)
	}

	// Web lint
	fmt.Println("Running web lint...")
	webDir := filepath.Join(projectDir, "web")
	npmPath, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found: %w", err)
	}
	cmd = exec.Command(npmPath, "--prefix", webDir, "run", "lint")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func devLogsUI() error {
	// Start all services first, then open log analyzer
	if err := devUp(false); err != nil {
		return err
	}

	cfg, err := loadDevConfig()
	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://localhost:%d/dev/log-analyzer", cfg.WebPort)
	fmt.Printf("Log analyzer ready: %s\n", url)

	// Try to open browser
	openCmd, _ := exec.LookPath("open")
	if openCmd == "" {
		openCmd, _ = exec.LookPath("xdg-open")
	}
	if openCmd != "" {
		if err := exec.Command(openCmd, url).Start(); err != nil {
			fmt.Printf("Failed to open browser: %v\n", err)
		}
	}
	return nil
}

func buildOrchestrator() (*devops.Orchestrator, error) {
	cfg, err := loadDevConfig()
	if err != nil {
		return nil, err
	}

	orch := devops.NewOrchestrator(cfg)
	backendSvc := buildBackendService(orch)
	webSvc := buildWebService(orch)
	orch.RegisterServices(backendSvc, webSvc)
	return orch, nil
}

func buildBackendService(orch *devops.Orchestrator) *services.BackendService {
	cfg := orch.Config()
	return services.NewBackendService(
		orch.ProcessManager(),
		orch.Ports(),
		orch.Health(),
		orch.Section(),
		services.BackendConfig{
			Port:       cfg.ServerPort,
			OutputBin:  cfg.ServerBin,
			ProjectDir: cfg.ProjectDir,
			LogDir:     cfg.LogDir,
			CGOMode:    cfg.CGOMode,
			AutoStop:   cfg.AutoStopConflictingPorts,
		},
	)
}

func buildWebService(orch *devops.Orchestrator) *services.WebService {
	cfg := orch.Config()
	return services.NewWebService(
		orch.ProcessManager(),
		orch.Ports(),
		orch.Health(),
		orch.Section(),
		services.WebConfig{
			Port:       cfg.WebPort,
			WebDir:     cfg.WebDir,
			ServerPort: cfg.ServerPort,
			AutoStop:   cfg.AutoStopConflictingPorts,
		},
	)
}

func loadDevConfig() (*devops.DevConfig, error) {
	home, _ := os.UserHomeDir()
	configPath := ""
	if raw, ok := os.LookupEnv("ALEX_CONFIG_PATH"); ok {
		configPath = strings.TrimSpace(raw)
	}
	if configPath == "" {
		configPath = filepath.Join(home, ".alex", "config.yaml")
	}
	return devops.LoadDevConfig(configPath)
}

const bootstrapMarker = "bootstrap.done"

func ensureLocalBootstrap(projectDir string) error {
	pidDir, err := resolveSharedDevPIDDir(projectDir)
	if err != nil {
		return err
	}
	marker := filepath.Join(pidDir, bootstrapMarker)
	if _, err := os.Stat(marker); err == nil {
		return nil // already bootstrapped
	}

	script := filepath.Join(projectDir, "scripts", "setup_local_runtime.sh")
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil
	}

	cmd := exec.Command(script)
	cmd.Dir = projectDir
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("bootstrap: %s: %w", string(out), err)
	}

	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		return fmt.Errorf("create bootstrap pid dir: %w", err)
	}
	if err := os.WriteFile(marker, []byte(time.Now().Format(time.RFC3339)), 0o644); err != nil {
		return fmt.Errorf("write bootstrap marker: %w", err)
	}
	return nil
}

func removeBootstrapMarker(pidDir string) {
	os.Remove(filepath.Join(pidDir, bootstrapMarker))
}

func resolveSharedDevPIDDir(projectDir string) (string, error) {
	if raw, ok := os.LookupEnv("LARK_PID_DIR"); ok {
		if value := strings.TrimSpace(raw); value != "" {
			if filepath.IsAbs(value) {
				return filepath.Clean(value), nil
			}
			abs, err := filepath.Abs(value)
			if err != nil {
				return "", fmt.Errorf("resolve LARK_PID_DIR: %w", err)
			}
			return filepath.Clean(abs), nil
		}
	}

	home, _ := os.UserHomeDir()
	configPath := ""
	if raw, ok := os.LookupEnv("ALEX_CONFIG_PATH"); ok {
		configPath = strings.TrimSpace(raw)
	}
	if configPath == "" {
		if home != "" {
			configPath = filepath.Join(home, ".alex", "config.yaml")
		} else {
			configPath = filepath.Join(projectDir, "config.yaml")
		}
	}

	if !filepath.IsAbs(configPath) {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return "", fmt.Errorf("resolve config path %s: %w", configPath, err)
		}
		configPath = abs
	}
	if resolved, err := filepath.EvalSymlinks(configPath); err == nil && strings.TrimSpace(resolved) != "" {
		configPath = resolved
	}

	return filepath.Join(filepath.Dir(filepath.Clean(configPath)), "pids"), nil
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func printDevUsage() {
	fmt.Print(`alex dev — Development environment manager

Usage:
  alex dev [command]

Commands:
  up|start [--lark]  Start dev services (backend + web)
  down|stop [--all]  Stop services (--all resets bootstrap marker)
  status             Show status of all services
  logs [service]     Tail logs (server|web|all)
  restart [service]  Restart specified service(s) or all
  test               Run Go tests (CI parity)
  lint               Run Go + web lint
  logs-ui            Start services and open log analyzer
  help               Show this help
`)
}
