package main

import (
"bytes"
"context"
"encoding/json"
"errors"
"flag"
"fmt"
"io"
"os"
"sort"
"strings"
"time"

	sessionstate "alex/internal/session/state_store"
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
  alex sessions pull <id> [...]  Inspect or export context snapshots
  alex sessions cleanup [...]    Remove historical sessions (see options below)
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
  ✓ Code search and indexing

Architecture: Hexagonal (Ports & Adapters)
Documentation: See docs/architecture/ALEX_DETAILED_ARCHITECTURE.md
`, appVersion())
}

func (c *CLI) handleSessions(args []string) error {
	ctx := context.Background()

	if len(args) == 0 || args[0] == "list" {
		return c.listSessions(ctx)
	}

	switch args[0] {
	case "cleanup", "clean", "prune":
		return c.cleanupSessions(ctx, args[1:])
	case "pull":
		return c.pullSessionSnapshots(ctx, args[1:])
	default:
		return fmt.Errorf("unknown sessions subcommand: %s", args[0])
	}
}

func (c *CLI) pullSessionSnapshots(ctx context.Context, args []string) error {
	return c.pullSessionSnapshotsWithWriter(ctx, args, os.Stdout)
}

const (
	defaultSnapshotListLimit = 20
	llmTurnSearchPageSize    = 50
	flagUsageLine            = "usage: alex sessions pull <session-id> [--turn N|--llm-turn N]"
)

func (c *CLI) pullSessionSnapshotsWithWriter(ctx context.Context, args []string, out io.Writer) error {
	if c == nil || c.container == nil {
		return fmt.Errorf("container not initialized")
	}
	store := c.container.StateStore
	if store == nil {
		return fmt.Errorf("session state store not configured")
	}

	fs := flag.NewFlagSet("sessions pull", flag.ContinueOnError)
	var flagBuf bytes.Buffer
	fs.SetOutput(&flagBuf)
	turnFlag := fs.Int("turn", -1, "Fetch a specific turn_id snapshot")
	llmTurnFlag := fs.Int("llm-turn", -1, "Fetch the snapshot matching llm_turn_seq")
	limitFlag := fs.Int("limit", defaultSnapshotListLimit, "Number of snapshots to list when no turn specified")
	cursorFlag := fs.String("cursor", "", "Pagination cursor for listing (default newest)")
	rawFlag := fs.Bool("raw", false, "Print raw JSON payload")
	outputFlag := fs.String("output", "", "Write JSON snapshot to the provided file path")

	var positionalID string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positionalID = strings.TrimSpace(args[0])
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(flagBuf.String()))
	}
	sessionID := positionalID
	positional := fs.Args()
if sessionID == "" {
if len(positional) == 0 {
return errors.New(flagUsageLine)
}
		sessionID = strings.TrimSpace(positional[0])
	} else if len(positional) > 0 {
		// When both a leading positional ID and trailing args are supplied, treat the
		// next positional token as an error to avoid ambiguous input.
		return fmt.Errorf("multiple session identifiers provided; %s", flagUsageLine)
	}
	if sessionID == "" {
		return fmt.Errorf("session id required")
	}

	if *turnFlag >= 0 {
		snapshot, err := store.GetSnapshot(ctx, sessionID, *turnFlag)
		if err != nil {
			return err
		}
		return c.outputSnapshot(out, snapshot, *rawFlag, *outputFlag)
	}

	if *llmTurnFlag >= 0 {
		snapshot, err := c.findSnapshotByLLMTurn(ctx, store, sessionID, *llmTurnFlag)
		if err != nil {
			return err
		}
		return c.outputSnapshot(out, snapshot, *rawFlag, *outputFlag)
	}

	if *outputFlag != "" {
		return fmt.Errorf("--output is only supported with --turn or --llm-turn")
	}
	limit := *limitFlag
	if limit <= 0 {
		limit = defaultSnapshotListLimit
	}
	metas, nextCursor, err := store.ListSnapshots(ctx, sessionID, *cursorFlag, limit)
	if err != nil {
		return err
	}
	return c.printSnapshotMetadata(out, sessionID, metas, nextCursor)
}

func (c *CLI) findSnapshotByLLMTurn(ctx context.Context, store sessionstate.Store, sessionID string, llmTurn int) (sessionstate.Snapshot, error) {
	cursor := ""
	for {
		metas, nextCursor, err := store.ListSnapshots(ctx, sessionID, cursor, llmTurnSearchPageSize)
		if err != nil {
			return sessionstate.Snapshot{}, err
		}
		for _, meta := range metas {
			if meta.LLMTurnSeq == llmTurn {
				return store.GetSnapshot(ctx, sessionID, meta.TurnID)
			}
		}
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	return sessionstate.Snapshot{}, fmt.Errorf("no snapshot found for llm_turn_seq=%d", llmTurn)
}

func (c *CLI) printSnapshotMetadata(out io.Writer, sessionID string, metas []sessionstate.SnapshotMetadata, nextCursor string) error {
if len(metas) == 0 {
return writeLine(out, "No snapshots found for session %s\n", sessionID)
}
if err := writeLine(out, "Snapshots for session %s (showing %d):\n", sessionID, len(metas)); err != nil {
return err
}
for _, meta := range metas {
timestamp := meta.CreatedAt.UTC().Format(time.RFC3339)
summary := strings.TrimSpace(meta.Summary)
if summary == "" {
summary = "(no summary)"
}
if err := writeLine(out, "  - Turn %d (LLM %d) @ %s :: %s\n", meta.TurnID, meta.LLMTurnSeq, timestamp, summary); err != nil {
return err
}
}
if nextCursor != "" {
if err := writeLine(out, "Next cursor: %s\n", nextCursor); err != nil {
return err
}
}
return nil
}

func (c *CLI) outputSnapshot(out io.Writer, snapshot sessionstate.Snapshot, raw bool, outputPath string) error {
	var encoded []byte
	var err error
	if raw || outputPath != "" {
		encoded, err = json.MarshalIndent(snapshot, "", "  ")
		if err != nil {
			return fmt.Errorf("encode snapshot: %w", err)
		}
	}
if outputPath != "" {
if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
return fmt.Errorf("write snapshot: %w", err)
}
if err := writeLine(out, "Snapshot saved to %s\n", outputPath); err != nil {
return err
}
}
if raw {
return writeLine(out, "%s\n", encoded)
}
created := "(unknown)"
if !snapshot.CreatedAt.IsZero() {
created = snapshot.CreatedAt.UTC().Format(time.RFC3339)
}
if err := writeLine(out, "Turn %d (LLM turn %d) captured %s\n", snapshot.TurnID, snapshot.LLMTurnSeq, created); err != nil {
return err
}
if strings.TrimSpace(snapshot.Summary) != "" {
if err := writeLine(out, "  Summary: %s\n", strings.TrimSpace(snapshot.Summary)); err != nil {
return err
}
}
if err := writeLine(out, "  Plans: %d | Beliefs: %d | Messages: %d | Feedback: %d\n", len(snapshot.Plans), len(snapshot.Beliefs), len(snapshot.Messages), len(snapshot.Feedback)); err != nil {
return err
}

// TODO(context): surface structured diff/plan output once the runtime populates these fields.
if worldKeys := sortedKeys(snapshot.World); len(worldKeys) > 0 {
if err := writeLine(out, "  World keys: %s\n", strings.Join(worldKeys, ", ")); err != nil {
return err
}
}
if diffKeys := sortedKeys(snapshot.Diff); len(diffKeys) > 0 {
if err := writeLine(out, "  Diff keys: %s\n", strings.Join(diffKeys, ", ")); err != nil {
return err
}
}
if len(snapshot.KnowledgeRefs) > 0 {
var refs []string
for _, ref := range snapshot.KnowledgeRefs {
if ref.ID != "" {
refs = append(refs, ref.ID)
}
}
if len(refs) > 0 {
if err := writeLine(out, "  Knowledge refs: %s\n", strings.Join(refs, ", ")); err != nil {
return err
}
}
}
return c.printSnapshotGaps(out, snapshot)
}

func sortedKeys(input map[string]any) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (c *CLI) printSnapshotGaps(out io.Writer, snapshot sessionstate.Snapshot) error {
if out == nil {
return nil
}
note := summarizeSnapshotGaps(snapshot)
if note == "" {
return nil
}
return writeLine(out, "  TODO: %s\n", note)
}

func writeLine(out io.Writer, format string, args ...any) error {
if out == nil {
return nil
}
if _, err := fmt.Fprintf(out, format, args...); err != nil {
return fmt.Errorf("write output: %w", err)
}
return nil
}

func summarizeSnapshotGaps(snapshot sessionstate.Snapshot) string {
	var missing []string
	if len(snapshot.Diff) == 0 {
		missing = append(missing, "state diff")
	}
	if len(snapshot.World) == 0 {
		missing = append(missing, "world state")
	}
	if len(snapshot.Feedback) == 0 {
		missing = append(missing, "feedback signals")
	}
	if len(missing) == 0 {
		return ""
	}
	return fmt.Sprintf("snapshots currently omit %s; see docs/status/context_framework_status.md", strings.Join(missing, ", "))
}

func (c *CLI) listSessions(ctx context.Context) error {
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

func (c *CLI) handleConfig() error {
	return runConfigCommand()
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
