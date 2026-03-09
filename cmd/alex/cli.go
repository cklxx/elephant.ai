package main

import (
	"fmt"
	"os"
	"strings"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

type CLI struct {
	container *Container
}

func NewCLI(container *Container) *CLI {
	return &CLI{container: container}
}

func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		c.showUsage()
		return nil
	}

	// Parse command
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		c.showUsage()
		return nil

	case "version", "-v", "--version":
		fmt.Println(appVersion())
		return nil

	case "session", "sessions":
		return c.handleSessions(cmdArgs)

	case "config":
		return executeConfigCommand(cmdArgs, os.Stdout)

	case "team":
		return runTeamCommandWithContainer(cmdArgs, c.container)

	case "cost", "costs":
		return c.handleCostCommand(cmdArgs)

	case "model", "models":
		return executeModelCommand(cmdArgs, os.Stdout)
	case "setup":
		return executeSetupCommandWith(cmdArgs, os.Stdin, os.Stdout, runtimeconfig.LoadCLICredentials(), runtimeEnvLookup())

	case "llama-cpp", "llamacpp":
		return executeLlamaCppCommand(cmdArgs, os.Stdout, runtimeEnvLookup())

	case "eval", "evaluation":
		return c.handleEval(cmdArgs)
	case "acp":
		return c.handleACP(cmdArgs)
	case "resume":
		return c.handleResume(cmdArgs)

	default:
		// Default: treat as task and run with stream output
		task := strings.Join(args, " ")
		return RunTaskWithStreamOutput(c.container, task, "")
	}
}

func (c *CLI) showUsage() {
	printUsage()
}

func printUsage() {
	configLine := configUsageLine()
	fmt.Printf(`
elephant.ai - Fragment-to-Fabric Agent Console (v%s)

Usage:
  alex <task>                    Execute a task with streaming output
  alex resume <session-id>       Resume a session from the latest checkpoint
  alex help                      Show this help message
  alex version                   Show version
  alex sessions                  List all sessions
  alex sessions pull <id> [...]  Inspect or export context snapshots
  alex sessions cleanup [...]    Remove historical sessions (see options below)
  alex team [status] [...]       Show latest team-runtime status (CLI capabilities/tmux/events)
  alex team run [...]            Execute team workflow via CLI (template/file/prompt)
  alex team inject [...]         Inject input into a team role tmux pane
  alex config                    Show current configuration
  alex config set <field> <value> Persist a managed override
  alex config clear <field>       Remove a managed override
  alex config validate [--profile <name>] Validate runtime configuration
  alex config path                Show the override file path
  alex setup                     Run first-run setup wizard (runtime + lark + model)
  alex model                     List available subscription models
  alex model use <p/m>           Select a subscription model (e.g. codex/gpt-5.2-codex)
  alex model use                 Select from an interactive picker
  alex model clear               Remove subscription selection
  alex llama-cpp pull <repo> <file>  Download GGUF weights from Hugging Face
  alex cost                      Show cost tracking commands
  alex eval [options]            Run local agent evaluation against SWE-Bench datasets
  alex acp [--initial-message]        Run ACP (Agent Client Protocol) over stdio
  alex acp serve [--port N]           Run ACP over HTTP/SSE (default 127.0.0.1:9000)

Configuration:
%s

Sessions cleanup options:
  --older-than 30d               Delete sessions not updated in the last 30 days
  --keep-latest 20               Always keep the newest N sessions, regardless of age
  --dry-run                      Show what would be deleted without removing files

Examples:
  alex "list files in current directory"
  alex "analyze the authentication flow in this codebase"
  alex "explain how the ReAct engine works"

Features:
  ✓ Real-time streaming output with tool visualization
  ✓ Markdown rendering with syntax highlighting
  ✓ Color-coded tool status and icons
  ✓ Cost tracking and analytics
  ✓ Session management
  ✓ Subscription model selection

Architecture: Hexagonal (Ports & Adapters)
Documentation: AGENTS.md + docs/reference/ARCHITECTURE.md + docs/reference/CONFIG.md
`, appVersion(), configLine)
}

func (c *CLI) handleResume(args []string) error {
	if c == nil || c.container == nil {
		return fmt.Errorf("container not initialized")
	}
	if len(args) == 0 || utils.IsBlank(args[0]) {
		return fmt.Errorf("usage: alex resume <session-id>")
	}
	sessionID := strings.TrimSpace(args[0])

	store := c.container.Container.CheckpointStore
	if store == nil {
		return fmt.Errorf("checkpoint store not configured")
	}

	ctx := cliBaseContext()
	cp, err := store.Load(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("load checkpoint: %w", err)
	}
	if cp == nil {
		return fmt.Errorf("no checkpoint found for session %s", sessionID)
	}

	return RunTaskWithStreamOutput(c.container, "", sessionID)
}

func configUsageLine() string {
	return configUsageLineWith(runtimeEnvLookup(), os.UserHomeDir)
}

func configUsageLineWith(envLookup runtimeconfig.EnvLookup, homeDir func() (string, error)) string {
	path, source := runtimeconfig.ResolveConfigPath(envLookup, homeDir)
	if utils.IsBlank(path) {
		return "  Config file: (unresolved)"
	}
	suffix := ""
	switch source {
	case "ALEX_CONFIG_PATH":
		suffix = " (ALEX_CONFIG_PATH)"
	case "fallback":
		suffix = " (fallback)"
	}
	return fmt.Sprintf("  Config file: %s%s", path, suffix)
}
