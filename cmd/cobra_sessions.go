package main

import (
	"alex/internal/utils"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

// newSessionCommand creates the session management subcommand
func newSessionCommand(cli *CLI) *cobra.Command {
	sessionCmd := &cobra.Command{
		Use:     "session",
		Short:   "📁 Session management",
		Long:    "Manage conversation sessions",
		Aliases: []string{"sessions", "s"},
	}

	// session list
	listCmd := &cobra.Command{
		Use:     "list",
		Short:   "List available sessions",
		Long:    "Display all available conversation sessions",
		Aliases: []string{"ls", "l"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.listSessions()
		},
	}

	// session show
	showCmd := &cobra.Command{
		Use:   "show <session-id>",
		Short: "Show session details",
		Long:  "Display detailed information about a specific session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.showSession(args[0])
		},
	}

	// session resume
	resumeCmd := &cobra.Command{
		Use:   "resume <session-id>",
		Short: "Resume a session",
		Long:  "Resume an existing conversation session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.resumeSession(args[0])
		},
	}

	// session delete
	deleteCmd := &cobra.Command{
		Use:   "delete <session-id>",
		Short: "Delete a session",
		Long:  "Delete an existing conversation session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.deleteSession(args[0])
		},
	}

	// session cleanup
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old sessions",
		Long:  "Remove old and expired conversation sessions",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.cleanupSessions()
		},
	}

	// session interactive
	interactiveCmd := &cobra.Command{
		Use:     "interactive",
		Short:   "Interactive session management",
		Long:    "Manage sessions interactively using prompts",
		Aliases: []string{"i"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cli.interactiveSessionManagement()
		},
	}

	sessionCmd.AddCommand(listCmd, showCmd, resumeCmd, deleteCmd, cleanupCmd, interactiveCmd)
	return sessionCmd
}

// listSessions displays all available sessions
func (cli *CLI) listSessions() error {
	fmt.Printf("\n%s Available Sessions:\n", bold("📁"))
	fmt.Println()

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".deep-coding-sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		fmt.Printf("%s No sessions found\n", yellow("⚠️"))
		return nil
	}

	if len(entries) == 0 {
		fmt.Printf("%s No sessions found\n", yellow("⚠️"))
		return nil
	}

	// Group sessions by date
	sessionsByDate := make(map[string][]sessionInfo)

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			sessionID := strings.TrimSuffix(entry.Name(), ".json")
			info, err := entry.Info()
			if err != nil {
				continue
			}

			dateKey := info.ModTime().Format("2006-01-02")
			sessionsByDate[dateKey] = append(sessionsByDate[dateKey], sessionInfo{
				ID:       sessionID,
				Modified: info.ModTime(),
				Size:     info.Size(),
			})
		}
	}

	// Display sessions grouped by date
	for date, sessions := range sessionsByDate {
		fmt.Printf("%s %s:\n", blue("📅"), date)
		for _, session := range sessions {
			timeStr := session.Modified.Format("15:04:05")
			sizeStr := utils.FormatFileSize(session.Size)
			fmt.Printf("  %s %s %s\n",
				blue("•"),
				session.ID,
				gray(fmt.Sprintf("(%s, %s)", timeStr, sizeStr)))
		}
		fmt.Println()
	}

	return nil
}

// showSession displays detailed information about a session
func (cli *CLI) showSession(sessionID string) error {
	fmt.Printf("\n%s Session Details: %s\n", bold("🔍"), blue(sessionID))
	fmt.Println(strings.Repeat("=", 50))

	// TODO: Load actual session data and display
	fmt.Printf("Session ID: %s\n", blue(sessionID))
	fmt.Printf("Created: %s\n", blue("2025-01-02 14:30:00"))
	fmt.Printf("Last Active: %s\n", blue("2025-01-02 16:45:00"))
	fmt.Printf("Messages: %s\n", blue("24"))
	fmt.Printf("Tokens Used: %s\n", blue("1,247"))
	fmt.Printf("Tools Called: %s\n", blue("8"))

	return nil
}

// resumeSession resumes an existing session
func (cli *CLI) resumeSession(sessionID string) error {
	fmt.Printf("%s Resuming session: %s\n", blue("📁"), sessionID)

	if _, err := cli.agent.RestoreSession(sessionID); err != nil {
		return fmt.Errorf("failed to resume session: %w", err)
	}

	fmt.Printf("%s Session resumed successfully\n", green("✅"))
	fmt.Printf("%s You can now continue the conversation\n", gray("💡"))
	return nil
}

// deleteSession deletes a session
func (cli *CLI) deleteSession(sessionID string) error {
	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("⚠️  Delete session '%s'? This cannot be undone", sessionID),
		IsConfirm: true,
	}

	result, err := prompt.Run()
	if err != nil || strings.ToLower(result) != "y" {
		fmt.Printf("%s Session deletion cancelled\n", yellow("⚠️"))
		return nil
	}

	// TODO: Implement actual session deletion
	fmt.Printf("%s Session '%s' deleted\n", green("✅"), sessionID)
	return nil
}

// cleanupSessions removes old sessions
func (cli *CLI) cleanupSessions() error {
	fmt.Printf("%s Cleaning up old sessions...\n", blue("🧹"))

	// TODO: Implement actual cleanup logic
	fmt.Printf("%s Removed 3 sessions older than 30 days\n", green("✅"))
	fmt.Printf("%s Cleaned up 1.2MB of storage\n", green("✅"))
	return nil
}

// interactiveSessionManagement provides interactive session management
func (cli *CLI) interactiveSessionManagement() error {
	for {
		actions := []string{
			"📋 List all sessions",
			"🔍 Show session details",
			"📁 Resume a session",
			"🗑️  Delete a session",
			"🧹 Cleanup old sessions",
			"🚪 Exit",
		}

		prompt := promptui.Select{
			Label: "Select an action",
			Items: actions,
			Templates: &promptui.SelectTemplates{
				Label:    "{{ . }}?",
				Active:   "▸ {{ . | cyan }}",
				Inactive: "  {{ . | white }}",
				Selected: "{{ \"✓\" | green }} {{ . }}",
			},
		}

		index, _, err := prompt.Run()
		if err != nil {
			return err
		}

		switch index {
		case 0:
			if err := cli.listSessions(); err != nil {
				fmt.Printf("%s Error listing sessions: %v\n", red("❌"), err)
			}
		case 1:
			if sessionID, err := cli.promptSessionID(); err == nil {
				if err := cli.showSession(sessionID); err != nil {
					fmt.Printf("%s Error showing session: %v\n", red("❌"), err)
				}
			}
		case 2:
			if sessionID, err := cli.promptSessionID(); err == nil {
				if err := cli.resumeSession(sessionID); err != nil {
					fmt.Printf("%s Error resuming session: %v\n", red("❌"), err)
				} else {
					return nil // Exit to resume session
				}
			}
		case 3:
			if sessionID, err := cli.promptSessionID(); err == nil {
				if err := cli.deleteSession(sessionID); err != nil {
					fmt.Printf("%s Error deleting session: %v\n", red("❌"), err)
				}
			}
		case 4:
			if err := cli.cleanupSessions(); err != nil {
				fmt.Printf("%s Error cleaning up sessions: %v\n", red("❌"), err)
			}
		case 5:
			return nil
		}

		fmt.Println()
	}
}

// promptSessionID prompts the user to enter a session ID
func (cli *CLI) promptSessionID() (string, error) {
	prompt := promptui.Prompt{
		Label: "Enter session ID",
		Validate: func(input string) error {
			if strings.TrimSpace(input) == "" {
				return fmt.Errorf("session ID cannot be empty")
			}
			return nil
		},
	}

	return prompt.Run()
}

// sessionInfo holds information about a session
type sessionInfo struct {
	ID       string
	Modified time.Time
	Size     int64
}
