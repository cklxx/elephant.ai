package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"alex/internal/config"
	"alex/internal/tools/mcp"
)

// newMCPCommand creates the MCP management command
func newMCPCommand(cli *CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "🔌 MCP server management",
		Long:  `Manage Model Context Protocol (MCP) servers`,
	}

	cmd.AddCommand(newMCPListCommand(cli))
	cmd.AddCommand(newMCPAddCommand(cli))
	cmd.AddCommand(newMCPRemoveCommand(cli))
	cmd.AddCommand(newMCPEnableCommand(cli))
	cmd.AddCommand(newMCPDisableCommand(cli))
	cmd.AddCommand(newMCPStatusCommand(cli))

	return cmd
}

// newMCPListCommand lists available and installed MCP servers
func newMCPListCommand(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List MCP servers",
		Long:  `List available and installed MCP servers`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			fmt.Println(bold("📋 Available Common MCP Servers:"))
			for _, name := range mcp.ListCommonServerConfigs() {
				_, installed := cli.config.GetMCPServerConfig(name)
				status := "❌ Not installed"
				if installed {
					status = "✅ Installed"
				}
				fmt.Printf("  %s - %s\n", cyan(name), status)
			}

			fmt.Println(bold("\n🔌 Installed Servers:"))
			for _, serverConfig := range cli.config.ListMCPServerConfigs() {
				status := "🔴 Disabled"
				if serverConfig.Enabled {
					status = "🟢 Enabled"
				}
				transport := "stdio"
				if strings.Contains(serverConfig.Type, "http") || strings.Contains(serverConfig.Command, "http") {
					transport = "http"
				}
				fmt.Printf("  %s (%s) - %s [%s]\n", cyan(serverConfig.ID), serverConfig.Name, status, transport)
			}

			return nil
		},
	}
}

// newMCPAddCommand adds MCP servers with transport support
func newMCPAddCommand(cli *CLI) *cobra.Command {
	var transport string

	cmd := &cobra.Command{
		Use:   "add [server-name] [url-or-package]",
		Short: "Add MCP server",
		Long:  `Add an MCP server with specified transport (stdio or http)`,
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			serverName := args[0]

			// Check if it's a common server
			if commonConfig, exists := mcp.CommonServerConfigs[serverName]; exists && len(args) == 1 {
				fmt.Printf("📦 Adding common MCP server: %s\n", cyan(serverName))

				// Convert mcp.ServerConfig to config.ServerConfig
				serverConfig := &config.ServerConfig{
					ID:          commonConfig.ID,
					Name:        commonConfig.Name,
					Type:        string(commonConfig.Type),
					Command:     commonConfig.Command,
					Args:        commonConfig.Args,
					Env:         commonConfig.Env,
					WorkDir:     commonConfig.WorkDir,
					AutoStart:   commonConfig.AutoStart,
					AutoRestart: commonConfig.AutoRestart,
					Timeout:     commonConfig.Timeout,
					Enabled:     commonConfig.Enabled,
				}

				if err := cli.config.UpdateMCPServerConfig(serverConfig); err != nil {
					return fmt.Errorf("failed to add server config: %w", err)
				}

				// Install the package via npx if it's an NPX command
				if commonConfig.Type == mcp.SpawnerTypeNPX {
					if err := installNPXPackage(commonConfig.Command); err != nil {
						return fmt.Errorf("failed to install package %s: %w", commonConfig.Command, err)
					}
				}

				fmt.Printf("✅ Successfully added %s\n", green(serverName))
				return nil
			}

			// Custom server with URL (for HTTP transport)
			if len(args) != 2 {
				return fmt.Errorf("for custom servers, both server-name and url/package are required")
			}

			urlOrPackage := args[1]
			fmt.Printf("📦 Adding custom MCP server: %s\n", cyan(serverName))

			var serverConfig *config.ServerConfig

			if transport == "http" || strings.HasPrefix(urlOrPackage, "http") {
				// HTTP transport
				serverConfig = &config.ServerConfig{
					ID:          sanitizeServerID(serverName),
					Name:        fmt.Sprintf("HTTP: %s", serverName),
					Type:        "http",
					Command:     urlOrPackage,
					Args:        []string{},
					Env:         make(map[string]string),
					AutoStart:   true,
					AutoRestart: true,
					Timeout:     30 * time.Second,
					Enabled:     true,
				}
			} else {
				// NPX/package transport
				serverConfig = &config.ServerConfig{
					ID:          sanitizeServerID(serverName),
					Name:        fmt.Sprintf("Custom: %s", serverName),
					Type:        "npx",
					Command:     urlOrPackage,
					Args:        []string{},
					Env:         make(map[string]string),
					AutoStart:   false,
					AutoRestart: true,
					Timeout:     30 * time.Second,
					Enabled:     true,
				}

				// Install the package
				if err := installNPXPackage(urlOrPackage); err != nil {
					return fmt.Errorf("failed to install package %s: %w", urlOrPackage, err)
				}
			}

			if err := cli.config.UpdateMCPServerConfig(serverConfig); err != nil {
				return fmt.Errorf("failed to add server config: %w", err)
			}

			fmt.Printf("✅ Successfully added %s with %s transport\n", green(serverName), transport)
			return nil
		},
	}

	cmd.Flags().StringVar(&transport, "transport", "stdio", "Transport type: stdio or http")
	return cmd
}

// newMCPRemoveCommand removes MCP servers
func newMCPRemoveCommand(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "remove [server-name]",
		Short: "Remove MCP server",
		Long:  `Remove an installed MCP server`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			serverName := args[0]
			_, exists := cli.config.GetMCPServerConfig(serverName)
			if !exists {
				return fmt.Errorf("server %s is not installed", serverName)
			}

			if err := cli.config.RemoveMCPServerConfig(serverName); err != nil {
				return fmt.Errorf("failed to remove server: %w", err)
			}

			fmt.Printf("🗑️ Removed MCP server: %s\n", yellow(serverName))
			return nil
		},
	}
}

// newMCPEnableCommand enables MCP servers
func newMCPEnableCommand(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "enable [server-name]",
		Short: "Enable MCP server",
		Long:  `Enable an installed MCP server`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			serverName := args[0]
			serverConfig, exists := cli.config.GetMCPServerConfig(serverName)
			if !exists {
				return fmt.Errorf("server %s is not installed", serverName)
			}

			serverConfig.Enabled = true
			if err := cli.config.UpdateMCPServerConfig(serverConfig); err != nil {
				return fmt.Errorf("failed to enable server: %w", err)
			}

			fmt.Printf("🟢 Enabled MCP server: %s\n", green(serverName))
			return nil
		},
	}
}

// newMCPDisableCommand disables MCP servers
func newMCPDisableCommand(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "disable [server-name]",
		Short: "Disable MCP server",
		Long:  `Disable an installed MCP server`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			serverName := args[0]
			serverConfig, exists := cli.config.GetMCPServerConfig(serverName)
			if !exists {
				return fmt.Errorf("server %s is not installed", serverName)
			}

			serverConfig.Enabled = false
			if err := cli.config.UpdateMCPServerConfig(serverConfig); err != nil {
				return fmt.Errorf("failed to disable server: %w", err)
			}

			fmt.Printf("🔴 Disabled MCP server: %s\n", yellow(serverName))
			return nil
		},
	}
}

// newMCPStatusCommand shows MCP server status
func newMCPStatusCommand(cli *CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "status [server-name]",
		Short: "Show MCP server status",
		Long:  `Show detailed status of an MCP server`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.initializeConfigOnly(); err != nil {
				return err
			}

			if len(args) == 0 {
				// Show status of all servers
				fmt.Println(bold("📊 MCP Server Status:"))
				for _, serverConfig := range cli.config.ListMCPServerConfigs() {
					showServerStatus(serverConfig)
				}
			} else {
				// Show status of specific server
				serverName := args[0]
				serverConfig, exists := cli.config.GetMCPServerConfig(serverName)
				if !exists {
					return fmt.Errorf("server %s is not installed", serverName)
				}
				showServerStatus(serverConfig)
			}

			return nil
		},
	}
}

// Helper functions

func installNPXPackage(packageName string) error {
	fmt.Printf("🔄 Installing package: %s\n", packageName)

	cmd := exec.Command("npx", "-y", packageName, "--help")
	if err := cmd.Run(); err != nil {
		// If help command fails, try to install the package
		cmd = exec.Command("npm", "install", "-g", packageName)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install package: %w", err)
		}
	}

	return nil
}

func sanitizeServerID(packageName string) string {
	// Remove common NPM package prefixes and convert to valid ID
	id := strings.TrimPrefix(packageName, "@upstash/")
	id = strings.TrimPrefix(id, "@modelcontextprotocol/server-")
	id = strings.TrimPrefix(id, "mcp-")
	id = strings.ReplaceAll(id, "/", "-")
	id = strings.ReplaceAll(id, "@", "")
	return id
}

func showServerStatus(serverConfig *config.ServerConfig) {
	status := "🔴 Disabled"
	if serverConfig.Enabled {
		status = "🟢 Enabled"
	}

	transport := "stdio"
	if serverConfig.Type == "http" || strings.Contains(serverConfig.Command, "http") {
		transport = "http"
	}

	fmt.Printf("\n%s (%s)\n", cyan(serverConfig.ID), serverConfig.Name)
	fmt.Printf("  Status: %s\n", status)
	fmt.Printf("  Type: %s\n", serverConfig.Type)
	fmt.Printf("  Transport: %s\n", transport)
	fmt.Printf("  Command: %s\n", serverConfig.Command)
	if len(serverConfig.Args) > 0 {
		fmt.Printf("  Args: %s\n", strings.Join(serverConfig.Args, " "))
	}
	if len(serverConfig.Env) > 0 {
		fmt.Printf("  Environment:\n")
		for k, v := range serverConfig.Env {
			displayValue := v
			if strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "password") {
				if v != "" && !strings.Contains(v, "your-") {
					displayValue = "***"
				}
			}
			fmt.Printf("    %s=%s\n", k, displayValue)
		}
	}
	fmt.Printf("  Auto Start: %t\n", serverConfig.AutoStart)
	fmt.Printf("  Auto Restart: %t\n", serverConfig.AutoRestart)
	fmt.Printf("  Timeout: %s\n", serverConfig.Timeout)
}
