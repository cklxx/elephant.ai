package main

import (
	"fmt"
	"strings"
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
		return c.handleConfig(cmdArgs)

	case "cost", "costs":
		return c.handleCostCommand(cmdArgs)

	case "-i", "--interactive", "interactive":
		if len(cmdArgs) > 0 {
			return fmt.Errorf("interactive mode does not accept additional arguments")
		}
		return RunNativeChatUI(c.container)

	case "index":
		return c.handleIndex(cmdArgs)

	case "search":
		if len(cmdArgs) == 0 {
			return fmt.Errorf("usage: alex search <query>")
		}
		query := strings.Join(cmdArgs, " ")
		return c.handleSearch(query)

	case "mcp":
		return c.handleMCP(cmdArgs)

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
	fmt.Printf(`
ALEX - Agile Light Easy Xpert Code Agent (v%s)

Usage:
  alex <task>                    Execute a task with streaming output
  alex help                      Show this help message
  alex version                   Show version
  alex sessions                  List all sessions
  alex config                    Show current configuration
  alex cost                      Show cost tracking commands
  alex index [--repo PATH]       Index repository for code search
  alex search "query"            Search indexed code
  alex mcp                       MCP (Model Context Protocol) management

Configuration:
  Config file: ~/.alex-config.json
  Environment variables:
    OPENROUTER_API_KEY           API key for OpenRouter/OpenAI
    LLM_PROVIDER                 LLM provider (openrouter, openai, deepseek, ollama, mock)
    LLM_MODEL                    Model name
    ALEX_VERBOSE                 Show full tool output (set to 1 or true)

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
  ✓ Code search and indexing

Architecture: Hexagonal (Ports & Adapters)
Documentation: See docs/architecture/ALEX_DETAILED_ARCHITECTURE.md
`, appVersion())
}

func (c *CLI) handleSessions(args []string) error {
	ctx := c.container.BackgroundContext()

	sessionIDs, err := c.container.Coordinator.ListSessions(ctx)
	if err != nil {
		return err
	}

	if len(sessionIDs) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	fmt.Printf("Found %d session(s):\n\n", len(sessionIDs))

	// Fetch and display detailed metadata for each session
	for i, sid := range sessionIDs {
		session, err := c.container.SessionStore.Get(ctx, sid)
		if err != nil {
			fmt.Printf("  %d. %s (error loading metadata: %v)\n", i+1, sid, err)
			continue
		}

		// Calculate stats
		messageCount := len(session.Messages)
		todoCount := len(session.Todos)

		// Format timestamps
		created := session.CreatedAt.Format("2006-01-02 15:04:05")
		updated := session.UpdatedAt.Format("2006-01-02 15:04:05")

		// Display session info
		fmt.Printf("  %d. %s\n", i+1, sid)
		fmt.Printf("     Created:  %s\n", created)
		fmt.Printf("     Updated:  %s\n", updated)
		fmt.Printf("     Messages: %d\n", messageCount)
		fmt.Printf("     Todos:    %d\n", todoCount)

		// Show metadata if present
		if len(session.Metadata) > 0 {
			fmt.Printf("     Metadata: ")
			first := true
			for key, value := range session.Metadata {
				if !first {
					fmt.Printf(", ")
				}
				fmt.Printf("%s=%s", key, value)
				first = false
			}
			fmt.Println()
		}
		fmt.Println()
	}

	return nil
}

func (c *CLI) handleConfig(args []string) error {
	switch len(args) {
	case 0:
		return runConfigCommand()
	case 1:
		if args[0] != "show" {
			return fmt.Errorf("unknown config subcommand %q", args[0])
		}
		return runConfigCommand()
	default:
		return fmt.Errorf("usage: alex config [show]")
	}
}

func runConfigCommand() error {
	config, err := loadConfig()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	fmt.Println("Current Configuration:")
	fmt.Printf("  Provider:      %s\n", config.LLMProvider)
	fmt.Printf("  Model:         %s\n", config.LLMModel)
	fmt.Printf("  Base URL:      %s\n", config.BaseURL)
	fmt.Printf("  Max Tokens:    %d\n", config.MaxTokens)
	fmt.Printf("  Max Iterations: %d\n", config.MaxIterations)
	fmt.Printf("  Temperature:   %.2f\n", config.Temperature)
	fmt.Printf("  Top P:         %.2f\n", config.TopP)
	fmt.Printf("  Environment:   %s\n", config.Environment)
	fmt.Printf("  Verbose:       %t\n", config.Verbose)
	if len(config.StopSequences) > 0 {
		fmt.Printf("  Stop Seqs:     %s\n", strings.Join(config.StopSequences, ", "))
	} else {
		fmt.Println("  Stop Seqs:     (not set)")
	}
	if config.APIKey != "" {
		fmt.Println("  API Key:       (set)")
	} else {
		fmt.Println("  API Key:       (not set)")
	}

	return nil
}
