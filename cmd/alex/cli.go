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
		fmt.Println("ALEX v2.0 (Hexagonal Architecture)")
		return nil

	case "session", "sessions":
		return c.handleSessions(cmdArgs)

	case "config":
		return c.handleConfig()

	case "cost", "costs":
		return c.handleCostCommand(cmdArgs)

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
	fmt.Print(`
ALEX - Agile Light Easy Xpert Code Agent (v2.0)

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
