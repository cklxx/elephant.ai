package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"alex/internal/devops/supervisor"
)

func runDevLarkCommand(args []string) error {
	cmd := "status"
	if len(args) > 0 {
		cmd = args[0]
		args = args[1:]
	}

	switch cmd {
	case "supervise", "run":
		return larkSupervise()
	case "start":
		return larkStart()
	case "stop":
		return larkStop()
	case "status":
		return larkStatus()
	case "logs":
		return larkLogs()
	case "help", "-h", "--help":
		printLarkUsage()
		return nil
	default:
		return fmt.Errorf("unknown lark command: %s", cmd)
	}
}

func larkSupervise() error {
	cfg, err := buildSupervisorConfig()
	if err != nil {
		return err
	}

	sup := supervisor.New(cfg)
	registerLarkComponents(sup, cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	return sup.Run(ctx)
}

func larkStart() error {
	cfg, err := buildSupervisorConfig()
	if err != nil {
		return err
	}

	// Check if already running
	pidFile := filepath.Join(cfg.PIDDir, "lark-supervisor.pid")
	if data, err := os.ReadFile(pidFile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && syscall.Kill(pid, 0) == nil {
			fmt.Printf("Supervisor already running (PID: %d)\n", pid)
			return nil
		}
	}

	// Launch supervisor in background
	alexBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logFile := filepath.Join(cfg.LogDir, "lark-supervisor.log")
	os.MkdirAll(filepath.Dir(logFile), 0o755)

	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	cmd := exec.Command(alexBin, "dev", "lark", "supervise")
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		f.Close()
		return fmt.Errorf("start supervisor: %w", err)
	}

	os.MkdirAll(filepath.Dir(pidFile), 0o755)
	os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o644)
	f.Close()

	time.Sleep(1 * time.Second)
	if syscall.Kill(cmd.Process.Pid, 0) == nil {
		fmt.Printf("Supervisor started (PID: %d)\n", cmd.Process.Pid)
		return nil
	}

	return fmt.Errorf("supervisor failed to start (see %s)", logFile)
}

func larkStop() error {
	cfg, err := buildSupervisorConfig()
	if err != nil {
		return err
	}

	pidFile := filepath.Join(cfg.PIDDir, "lark-supervisor.pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Supervisor is not running")
		return nil
	}

	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid <= 0 || syscall.Kill(pid, 0) != nil {
		os.Remove(pidFile)
		fmt.Println("Supervisor is not running (stale PID file cleaned)")
		return nil
	}

	fmt.Printf("Stopping supervisor (PID: %d)...\n", pid)
	syscall.Kill(pid, syscall.SIGTERM)

	// Wait for exit
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}

	if syscall.Kill(pid, 0) == nil {
		syscall.Kill(pid, syscall.SIGKILL)
	}

	os.Remove(pidFile)
	fmt.Println("Supervisor stopped")
	return nil
}

func larkStatus() error {
	cfg, err := buildSupervisorConfig()
	if err != nil {
		return err
	}

	statusPath := filepath.Join(cfg.TmpDir, "lark-supervisor.status.json")
	sf := supervisor.NewStatusFile(statusPath)
	status, err := sf.Read()
	if err != nil {
		fmt.Println("No supervisor status found")
		return nil
	}

	// Check if supervisor process is alive
	pidFile := filepath.Join(cfg.PIDDir, "lark-supervisor.pid")
	supervisorState := "stopped"
	if data, err := os.ReadFile(pidFile); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
		if pid > 0 && syscall.Kill(pid, 0) == nil {
			supervisorState = "running"
		}
	}

	fmt.Printf("supervisor: %s\n", supervisorState)
	fmt.Printf("mode: %s\n", status.Mode)
	for name, comp := range status.Components {
		fmt.Printf("%s: %s pid=%d sha=%s\n", name, comp.Health, comp.PID, comp.DeployedSHA)
	}
	fmt.Printf("restart_count_window: %d\n", status.RestartCountWindow)
	fmt.Printf("autofix_state: %s\n", status.Autofix.State)
	fmt.Printf("autofix_runs_window: %d\n", status.Autofix.RunsWindow)
	return nil
}

func larkLogs() error {
	cfg, err := buildSupervisorConfig()
	if err != nil {
		return err
	}

	logFiles := []string{
		filepath.Join(cfg.LogDir, "lark-supervisor.log"),
		filepath.Join(cfg.MainRoot, "logs", "lark-main.log"),
		filepath.Join(cfg.TestRoot, "logs", "lark-test.log"),
		filepath.Join(cfg.TestRoot, "logs", "lark-loop.log"),
	}

	// Touch files to ensure they exist
	for _, f := range logFiles {
		os.MkdirAll(filepath.Dir(f), 0o755)
		if _, err := os.Stat(f); os.IsNotExist(err) {
			os.WriteFile(f, nil, 0o644)
		}
	}

	cmd := exec.Command("tail", append([]string{"-n", "200", "-f"}, logFiles...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()

	return cmd.Run()
}

func buildSupervisorConfig() (supervisor.Config, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return supervisor.Config{}, err
	}

	// Resolve main root via git worktree
	mainRoot := resolveMainRoot(projectDir)
	testRoot := filepath.Join(mainRoot, ".worktrees", "test")

	pidDir := filepath.Join(testRoot, ".pids")
	logDir := filepath.Join(testRoot, "logs")
	tmpDir := filepath.Join(testRoot, "tmp")

	// Ensure directories
	os.MkdirAll(pidDir, 0o755)
	os.MkdirAll(logDir, 0o755)
	os.MkdirAll(tmpDir, 0o755)

	return supervisor.Config{
		TickInterval:       envDuration("LARK_SUPERVISOR_TICK_SECONDS", 5*time.Second),
		RestartMaxInWindow: envInt("LARK_RESTART_MAX_IN_WINDOW", 5),
		RestartWindow:      envDuration("LARK_RESTART_WINDOW_SECONDS", 600*time.Second),
		CooldownDuration:   envDuration("LARK_COOLDOWN_SECONDS", 300*time.Second),
		MainRoot:           mainRoot,
		TestRoot:           testRoot,
		PIDDir:             pidDir,
		LogDir:             logDir,
		TmpDir:             tmpDir,
		AutofixConfig: supervisor.AutofixConfig{
			Enabled:       envBool("LARK_SUPERVISOR_AUTOFIX_ENABLED", true),
			Trigger:       envString("LARK_SUPERVISOR_AUTOFIX_TRIGGER", "cooldown"),
			Timeout:       envDuration("LARK_SUPERVISOR_AUTOFIX_TIMEOUT_SECONDS", 1800*time.Second),
			MaxInWindow:   envInt("LARK_SUPERVISOR_AUTOFIX_MAX_IN_WINDOW", 3),
			Window:        envDuration("LARK_SUPERVISOR_AUTOFIX_WINDOW_SECONDS", 3600*time.Second),
			Cooldown:      envDuration("LARK_SUPERVISOR_AUTOFIX_COOLDOWN_SECONDS", 900*time.Second),
			Scope:         envString("LARK_SUPERVISOR_AUTOFIX_SCOPE", "repo"),
			ScriptPath:    filepath.Join(mainRoot, "scripts", "lark", "autofix.sh"),
			HistoryFile:   filepath.Join(tmpDir, "lark-autofix.history"),
			SignatureFile: filepath.Join(tmpDir, "lark-autofix.last-signature"),
			AppliedFile:   filepath.Join(tmpDir, "lark-autofix.applied"),
			StateFile:     filepath.Join(tmpDir, "lark-autofix.state.json"),
			LockDir:       filepath.Join(tmpDir, "lark-autofix.lock"),
		},
	}, nil
}

func registerLarkComponents(sup *supervisor.Supervisor, cfg supervisor.Config) {
	mainSH := filepath.Join(cfg.MainRoot, "scripts", "lark", "main.sh")
	testSH := filepath.Join(cfg.MainRoot, "scripts", "lark", "test.sh")
	loopSH := filepath.Join(cfg.MainRoot, "scripts", "lark", "loop-agent.sh")

	sup.RegisterComponent(&supervisor.Component{
		Name: "main",
		StartFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, mainSH, "restart")
			return cmd.Run()
		},
		StopFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, mainSH, "stop")
			return cmd.Run()
		},
		HealthFn: func() string {
			return checkPIDHealth(filepath.Join(cfg.MainRoot, ".pids", "lark-main.pid"))
		},
		PIDFile: filepath.Join(cfg.MainRoot, ".pids", "lark-main.pid"),
		SHAFile: filepath.Join(cfg.MainRoot, ".pids", "lark-main.sha"),
	})

	sup.RegisterComponent(&supervisor.Component{
		Name: "test",
		StartFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, testSH, "restart")
			return cmd.Run()
		},
		StopFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, testSH, "stop")
			return cmd.Run()
		},
		HealthFn: func() string {
			return checkPIDHealth(filepath.Join(cfg.TestRoot, ".pids", "lark-test.pid"))
		},
		PIDFile: filepath.Join(cfg.TestRoot, ".pids", "lark-test.pid"),
		SHAFile: filepath.Join(cfg.TestRoot, ".pids", "lark-test.sha"),
	})

	sup.RegisterComponent(&supervisor.Component{
		Name: "loop",
		StartFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, loopSH, "restart")
			return cmd.Run()
		},
		StopFn: func(ctx context.Context) error {
			cmd := exec.CommandContext(ctx, loopSH, "stop")
			return cmd.Run()
		},
		HealthFn: func() string {
			pidFile := filepath.Join(cfg.TestRoot, ".pids", "lark-loop.pid")
			data, err := os.ReadFile(pidFile)
			if err != nil {
				return "down"
			}
			pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			if pid > 0 && syscall.Kill(pid, 0) == nil {
				return "alive"
			}
			return "down"
		},
		PIDFile: filepath.Join(cfg.TestRoot, ".pids", "lark-loop.pid"),
	})
}

func checkPIDHealth(pidFile string) string {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return "down"
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid > 0 && syscall.Kill(pid, 0) == nil {
		return "healthy"
	}
	return "down"
}

func resolveMainRoot(projectDir string) string {
	if v := os.Getenv("LARK_MAIN_ROOT"); v != "" {
		return v
	}
	// Try git worktree for refs/heads/main
	cmd := exec.Command("git", "-C", projectDir, "worktree", "list", "--porcelain")
	out, err := cmd.Output()
	if err == nil {
		var currentWorktree string
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "worktree ") {
				currentWorktree = strings.TrimPrefix(line, "worktree ")
			}
			if line == "branch refs/heads/main" && currentWorktree != "" {
				return currentWorktree
			}
		}
	}

	// Fallback to repo root
	cmd = exec.Command("git", "-C", projectDir, "rev-parse", "--show-toplevel")
	out, err = cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}

	return projectDir
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if secs, err := strconv.Atoi(v); err == nil {
		return time.Duration(secs) * time.Second
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return def
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return n
	}
	return def
}

func envString(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	}
	return def
}

func printLarkUsage() {
	fmt.Print(`alex dev lark — Lark supervisor management

Usage:
  alex dev lark [command]

Commands:
  supervise|run    Run supervisor in foreground
  start            Start supervisor in background
  stop             Stop supervisor
  status           Show supervisor status
  logs             Tail all Lark logs
  help             Show this help
`)
}
