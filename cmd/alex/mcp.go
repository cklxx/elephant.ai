package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/mcp"
)

// handleMCP handles all MCP subcommands
func (c *CLI) handleMCP(args []string) error {
	if len(args) == 0 {
		c.showMCPUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list", "ls":
		return c.handleMCPList()
	case "add":
		return c.handleMCPAdd(subArgs)
	case "remove", "rm":
		return c.handleMCPRemove(subArgs)
	case "tools":
		return c.handleMCPTools(subArgs)
	case "restart":
		return c.handleMCPRestart(subArgs)
	case "help", "-h", "--help":
		c.showMCPUsage()
		return nil
	default:
		return fmt.Errorf("unknown mcp subcommand: %s (run 'alex mcp help' for usage)", subcommand)
	}
}

// handleMCPList lists all configured MCP servers and their status
func (c *CLI) handleMCPList() error {
	servers := c.container.MCPRegistry.ListServers()

	if len(servers) == 0 {
		fmt.Println("No MCP servers configured.")
		fmt.Println("\nTo add a server, run:")
		fmt.Println("  alex mcp add <name> <command> [args...]")
		return nil
	}

	fmt.Printf("MCP Servers (%d):\n\n", len(servers))

	for _, server := range servers {
		fmt.Printf("  %s\n", server.Name)
		fmt.Printf("    Status: %s\n", server.Status)
		fmt.Printf("    Command: %s %s\n", server.Config.Command, strings.Join(server.Config.Args, " "))

		if server.LastError != nil {
			fmt.Printf("    Error: %v\n", server.LastError)
		}

		if server.Status == mcp.StatusRunning {
			fmt.Printf("    Uptime: %s\n", server.Uptime())
			fmt.Printf("    Restarts: %d\n", server.RestartCount)

			if serverInfo := server.GetServerInfo(); serverInfo != nil {
				fmt.Printf("    Server: %s v%s\n", serverInfo.Name, serverInfo.Version)
			}

			if caps := server.GetCapabilities(); caps != nil {
				features := []string{}
				if caps.Tools != nil {
					features = append(features, "tools")
				}
				if caps.Resources != nil {
					features = append(features, "resources")
				}
				if caps.Prompts != nil {
					features = append(features, "prompts")
				}
				if len(features) > 0 {
					fmt.Printf("    Features: %s\n", strings.Join(features, ", "))
				}
			}
		}

		fmt.Println()
	}

	return nil
}

// handleMCPAdd adds a new MCP server configuration
func (c *CLI) handleMCPAdd(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: alex mcp add <name> <command> [args...]")
	}

	name := args[0]
	command := args[1]
	serverArgs := args[2:]

	// Load current configuration
	loader := mcp.NewConfigLoader()
	config, err := loader.Load()
	if err != nil {
		// Create new config if none exists
		config = &mcp.Config{
			MCPServers: make(map[string]mcp.ServerConfig),
		}
	}

	// Check if server already exists
	if _, exists := config.GetServer(name); exists {
		return fmt.Errorf("server '%s' already exists (use 'alex mcp remove %s' first)", name, name)
	}

	// Add server to configuration
	config.AddServer(name, mcp.ServerConfig{
		Command: command,
		Args:    serverArgs,
	})

	// Save to local config (.mcp.json in current directory)
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath := filepath.Join(cwd, ".mcp.json")
	if err := loader.SaveToPath(configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Added MCP server '%s' to %s\n", name, configPath)
	fmt.Println("\nTo start the server, restart ALEX or run:")
	fmt.Printf("  alex mcp restart %s\n", name)

	return nil
}

// handleMCPRemove removes an MCP server configuration
func (c *CLI) handleMCPRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: alex mcp remove <name>")
	}

	name := args[0]

	// Load current configuration
	loader := mcp.NewConfigLoader()
	config, err := loader.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if server exists
	if _, exists := config.GetServer(name); !exists {
		return fmt.Errorf("server '%s' not found", name)
	}

	// Remove server from configuration
	config.RemoveServer(name)

	// Save to local config
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath := filepath.Join(cwd, ".mcp.json")
	if err := loader.SaveToPath(configPath, config); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Removed MCP server '%s' from %s\n", name, configPath)
	fmt.Println("\nRestart ALEX for changes to take effect.")

	return nil
}

// handleMCPTools lists all tools from a specific MCP server
func (c *CLI) handleMCPTools(args []string) error {
	if len(args) == 0 {
		// List all MCP tools
		tools := c.container.MCPRegistry.ListTools()

		if len(tools) == 0 {
			fmt.Println("No MCP tools available.")
			return nil
		}

		fmt.Printf("MCP Tools (%d):\n\n", len(tools))

		// Group tools by server
		serverTools := make(map[string][]*mcp.ToolAdapter)
		for _, tool := range tools {
			metadata := tool.Metadata()
			serverName := extractServerName(metadata.Name)
			serverTools[serverName] = append(serverTools[serverName], tool)
		}

		for serverName, tools := range serverTools {
			fmt.Printf("  Server: %s\n", serverName)
			for _, tool := range tools {
				def := tool.Definition()
				fmt.Printf("    - %s: %s\n", extractToolName(def.Name), def.Description)
			}
			fmt.Println()
		}

		return nil
	}

	// List tools from specific server
	serverName := args[0]

	server, err := c.container.MCPRegistry.GetServer(serverName)
	if err != nil {
		return fmt.Errorf("server not found: %s", serverName)
	}

	if server.Status != mcp.StatusRunning {
		return fmt.Errorf("server '%s' is not running (status: %s)", serverName, server.Status)
	}

	// Get tools for this server
	allTools := c.container.MCPRegistry.ListTools()
	serverTools := []*mcp.ToolAdapter{}
	for _, tool := range allTools {
		metadata := tool.Metadata()
		if extractServerName(metadata.Name) == serverName {
			serverTools = append(serverTools, tool)
		}
	}

	if len(serverTools) == 0 {
		fmt.Printf("Server '%s' has no tools available.\n", serverName)
		return nil
	}

	fmt.Printf("Tools from '%s' (%d):\n\n", serverName, len(serverTools))

	for _, tool := range serverTools {
		def := tool.Definition()
		toolName := extractToolName(def.Name)

		fmt.Printf("  %s\n", toolName)
		fmt.Printf("    Description: %s\n", def.Description)

		if len(def.Parameters.Properties) > 0 {
			fmt.Printf("    Parameters:\n")
			for paramName, param := range def.Parameters.Properties {
				required := ""
				for _, req := range def.Parameters.Required {
					if req == paramName {
						required = " (required)"
						break
					}
				}
				fmt.Printf("      - %s (%s)%s: %s\n", paramName, param.Type, required, param.Description)
			}
		}

		fmt.Println()
	}

	return nil
}

// handleMCPRestart restarts a specific MCP server
func (c *CLI) handleMCPRestart(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: alex mcp restart <name>")
	}

	name := args[0]

	fmt.Printf("Restarting MCP server '%s'...\n", name)

	if err := c.container.MCPRegistry.RestartServer(name); err != nil {
		return fmt.Errorf("failed to restart server: %w", err)
	}

	fmt.Printf("Server '%s' restarted successfully.\n", name)

	return nil
}

// showMCPUsage displays MCP command usage
func (c *CLI) showMCPUsage() {
	fmt.Print(`
ALEX MCP (Model Context Protocol) Management

Usage:
  alex mcp list                       List all MCP servers and their status
  alex mcp add <name> <cmd> [args]    Add a new MCP server
  alex mcp remove <name>              Remove an MCP server
  alex mcp tools [server]             List tools (all or from specific server)
  alex mcp restart <name>             Restart an MCP server
  alex mcp help                       Show this help message

Examples:
  # Add filesystem server
  alex mcp add filesystem mcp-server-filesystem /workspace

  # Add GitHub server with environment variable
  alex mcp add github mcp-server-github

  # List all servers
  alex mcp list

  # List tools from filesystem server
  alex mcp tools filesystem

  # Restart a server
  alex mcp restart filesystem

Configuration:
  MCP servers are configured in .mcp.json files:
    - Local:   ./.mcp.json (current directory)
    - Project: <git-root>/.mcp.json
    - User:    ~/.alex/.mcp.json

  Priority: local > project > user

  Example .mcp.json:
  {
    "mcpServers": {
      "filesystem": {
        "command": "mcp-server-filesystem",
        "args": ["/workspace"]
      },
      "github": {
        "command": "mcp-server-github",
        "env": {
          "GITHUB_TOKEN": "${GITHUB_TOKEN}"
        }
      }
    }
  }

MCP Specification:
  https://modelcontextprotocol.io/specification
`)
}

// extractServerName extracts the server name from an MCP tool name
// Tool names are formatted as: mcp__<server>__<tool>
func extractServerName(toolName string) string {
	parts := strings.Split(toolName, "__")
	if len(parts) >= 2 && parts[0] == "mcp" {
		return parts[1]
	}
	return "unknown"
}

// extractToolName extracts the tool name from an MCP tool name
// Tool names are formatted as: mcp__<server>__<tool>
func extractToolName(fullName string) string {
	parts := strings.Split(fullName, "__")
	if len(parts) >= 3 && parts[0] == "mcp" {
		return parts[2]
	}
	return fullName
}
