package main

import (
	"context"
	"fmt"
	"strings"

	markdown "github.com/MichaelMure/go-term-markdown"
)

type CLI struct {
	container *Container
}

func NewCLI(container *Container) *CLI {
	return &CLI{container: container}
}

func (c *CLI) Run(args []string) error {
	if len(args) == 0 {
		// Default to interactive mode if no arguments
		return RunInteractive(c.container)
	}

	// Parse command
	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "help", "-h", "--help":
		c.showUsage()
		return nil

	case "version", "-v", "--version":
		fmt.Println("ALEX v2.0 (Hexagonal Architecture)")
		return nil

	case "chat", "interactive", "i":
		return RunInteractive(c.container)

	case "simple", "--simple":
		return RunSimpleREPL(c.container)

	case "tui":
		return RunTUI(c.container)

	case "demo", "--demo-parallel":
		if c.container != nil {
			fmt.Println("Running parallel execution demo...")
			// Demo functionality can be added later
			return nil
		}
		return fmt.Errorf("demo not available")

	case "session", "sessions":
		return c.handleSessions(cmdArgs)

	case "config":
		return c.handleConfig()

	default:
		// Default: treat as task
		task := strings.Join(args, " ")
		return c.handleTask(task, "")
	}
}

func (c *CLI) showUsage() {
	fmt.Print(`
ALEX - Agile Light Easy Xpert Code Agent (v2.0)

Usage:
  alex <task>                    Execute a task
  alex interactive               Start interactive chat mode with readline (default)
  alex simple                    Start simple REPL mode (no readline, guaranteed compatibility)
  alex tui                       Start TUI mode with Bubble Tea framework
  alex --demo-parallel           Run parallel execution demo
  alex help                      Show this help message
  alex version                   Show version
  alex sessions                  List all sessions
  alex config                    Show current configuration

Configuration:
  Config file: ~/.alex-config.json
  Environment variables:
    OPENROUTER_API_KEY           API key for OpenRouter/OpenAI
    LLM_PROVIDER                 LLM provider (openrouter, openai, deepseek, ollama, mock)
    LLM_MODEL                    Model name

Examples:
  alex "list files in current directory"
  alex "analyze the Go project structure"
  alex --demo-parallel

Architecture: Hexagonal (Ports & Adapters)
Documentation: See NEW_ARCHITECTURE.md
`)
}

func (c *CLI) handleDemo() error {
	fmt.Println("Demo functionality - parallel subagent execution")
	fmt.Println("To be implemented...")
	return nil
}

func (c *CLI) handleTask(task string, sessionID string) error {
	fmt.Printf("Executing: %s\n", task)

	result, err := c.container.Coordinator.ExecuteTask(context.Background(), task, sessionID)
	if err != nil {
		return fmt.Errorf("task execution failed: %w", err)
	}

	fmt.Printf("\n✓ Task completed in %d iterations\n", result.Iterations)
	fmt.Printf("Tokens used: %d\n", result.TokensUsed)

	// Render markdown answer
	if result.Answer != "" {
		rendered := renderMarkdownCLI(result.Answer)
		fmt.Printf("\nAnswer:\n%s\n", rendered)
	}

	return nil
}

// renderMarkdownCLI renders markdown content to terminal
func renderMarkdownCLI(content string) string {
	width := 100
	result := markdown.Render(content, width, 0) // No left padding for single task
	return string(result)
}

func (c *CLI) handleSessions(args []string) error {
	sessions, err := c.container.Coordinator.ListSessions(context.Background())
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return nil
	}

	fmt.Printf("Found %d session(s):\n", len(sessions))
	for i, sid := range sessions {
		fmt.Printf("  %d. %s\n", i+1, sid)
	}

	return nil
}

func (c *CLI) handleConfig() error {
	config := loadConfig()

	fmt.Println("Current Configuration:")
	fmt.Printf("  Provider:      %s\n", config.LLMProvider)
	fmt.Printf("  Model:         %s\n", config.LLMModel)
	fmt.Printf("  Base URL:      %s\n", config.BaseURL)
	fmt.Printf("  Max Tokens:    %d\n", config.MaxTokens)
	fmt.Printf("  Max Iterations: %d\n", config.MaxIterations)
	if config.APIKey != "" {
		fmt.Printf("  API Key:       %s...%s\n", config.APIKey[:8], config.APIKey[len(config.APIKey)-4:])
	} else {
		fmt.Println("  API Key:       (not set)")
	}

	return nil
}
