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
"strconv"
"strings"
"time"

runtimeconfig "alex/internal/config"
configadmin "alex/internal/config/admin"
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
		return c.handleConfig(cmdArgs)

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
  alex config set <field> <value> Persist a managed override
  alex config clear <field>       Remove a managed override
  alex config path                Show the override file path
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

func (c *CLI) handleConfig(args []string) error {
	return runConfigCommand(args)
}

func runConfigCommand(args []string) error {
	return executeConfigCommand(args, os.Stdout)
}

func executeConfigCommand(args []string, out io.Writer) error {
	envLookup := runtimeEnvLookup()
	overridesPath := managedOverridesPath(envLookup)
	subcommand := ""
	if len(args) > 0 {
		subcommand = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch subcommand {
	case "", "show", "list":
		return printConfigSummary(out, overridesPath)
	case "set":
		key, value, err := parseSetArgs(args[1:])
		if err != nil {
			return err
		}
		if err := mutateOverrides(envLookup, key, value, setOverrideField); err != nil {
			return err
		}
		fmt.Fprintf(out, "已更新 %s (写入 %s)\n\n", normalizeOverrideKey(key), overridesPath)
		return printConfigSummary(out, overridesPath)
	case "clear", "unset", "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: alex config clear <field>")
		}
		key := strings.TrimSpace(args[1])
		if key == "" {
			return fmt.Errorf("usage: alex config clear <field>")
		}
		if err := mutateOverrides(envLookup, key, "", clearOverrideField); err != nil {
			return err
		}
		fmt.Fprintf(out, "已清除 %s (写入 %s)\n\n", normalizeOverrideKey(key), overridesPath)
		return printConfigSummary(out, overridesPath)
	case "path", "file":
		fmt.Fprintln(out, overridesPath)
		return nil
	case "help", "-h", "--help":
		printConfigUsage(out)
		return nil
	default:
		printConfigUsage(out)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

type overrideMutation func(*runtimeconfig.Overrides, string, string) error

func mutateOverrides(envLookup runtimeconfig.EnvLookup, key, value string, fn overrideMutation) error {
	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return fmt.Errorf("load managed overrides: %w", err)
	}
	if err := fn(&overrides, key, value); err != nil {
		return err
	}
	if err := saveManagedOverrides(envLookup, overrides); err != nil {
		return fmt.Errorf("save managed overrides: %w", err)
	}
	return nil
}

func printConfigSummary(out io.Writer, overridesPath string) error {
	cfg, meta, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}
	fmt.Fprintln(out, "Current Configuration:")
	fmt.Fprintf(out, "  Provider:       %s\n", cfg.LLMProvider)
	fmt.Fprintf(out, "  Model:          %s\n", cfg.LLMModel)
	fmt.Fprintf(out, "  Base URL:       %s\n", cfg.BaseURL)
	fmt.Fprintf(out, "  Max Tokens:     %d\n", cfg.MaxTokens)
	fmt.Fprintf(out, "  Max Iterations: %d\n", cfg.MaxIterations)
	fmt.Fprintf(out, "  Temperature:    %.2f\n", cfg.Temperature)
	fmt.Fprintf(out, "  Top P:          %.2f\n", cfg.TopP)
	fmt.Fprintf(out, "  Environment:    %s\n", cfg.Environment)
	fmt.Fprintf(out, "  Verbose:        %t\n", cfg.Verbose)
	if len(cfg.StopSequences) > 0 {
		fmt.Fprintf(out, "  Stop Seqs:      %s\n", strings.Join(cfg.StopSequences, ", "))
	} else {
		fmt.Fprintln(out, "  Stop Seqs:      (not set)")
	}
	if cfg.APIKey != "" {
		fmt.Fprintln(out, "  API Key:        (set)")
	} else {
		fmt.Fprintln(out, "  API Key:        (not set)")
	}
	fmt.Fprintf(out, "  Loaded At:      %s\n", meta.LoadedAt().Format(time.RFC3339))
	fmt.Fprintf(out, "\nManaged overrides file: %s\n", overridesPath)
	fmt.Fprintln(out, "就绪检查:")
	fmt.Fprintln(out, readinessSummary(configadmin.DeriveReadinessTasks(cfg)))
	return nil
}

func printConfigUsage(out io.Writer) {
	fmt.Fprintln(out, "Config command usage:")
	fmt.Fprintln(out, "  alex config                       Show current configuration snapshot")
	fmt.Fprintln(out, "  alex config set <field> <value>   Persist a managed override (e.g. llm_model gpt-4o-mini)")
	fmt.Fprintln(out, "  alex config set field=value       Alternate set syntax")
	fmt.Fprintln(out, "  alex config clear <field>         Remove an override")
	fmt.Fprintln(out, "  alex config path                  Print the overrides file location")
	fmt.Fprintln(out, "\nSupported fields: llm_provider, llm_model, base_url, api_key, ark_api_key, tavily_api_key, sandbox_base_url, environment, max_tokens, max_iterations, temperature, top_p, verbose, stop_sequences, agent_preset, tool_preset, and Seedream model/endpoints.")
}

func parseSetArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	if len(args) == 1 {
		if strings.Contains(args[0], "=") {
			parts := strings.SplitN(args[0], "=", 2)
			key := strings.TrimSpace(parts[0])
			value := ""
			if len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}
			if key == "" || value == "" {
				return "", "", fmt.Errorf("usage: alex config set <field>=<value>")
			}
			return key, value, nil
		}
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	key := strings.TrimSpace(args[0])
	value := strings.TrimSpace(strings.Join(args[1:], " "))
	if key == "" || value == "" {
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	return key, value, nil
}

func setOverrideField(overrides *runtimeconfig.Overrides, key, value string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	key = normalizeOverrideKey(key)
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("value for %s cannot be empty", key)
	}
	switch key {
	case "llm_provider":
		overrides.LLMProvider = stringPtr(value)
	case "llm_model":
		overrides.LLMModel = stringPtr(value)
	case "api_key":
		overrides.APIKey = stringPtr(value)
	case "ark_api_key":
		overrides.ArkAPIKey = stringPtr(value)
	case "base_url":
		overrides.BaseURL = stringPtr(value)
	case "tavily_api_key":
		overrides.TavilyAPIKey = stringPtr(value)
	case "seedream_text_endpoint_id":
		overrides.SeedreamTextEndpointID = stringPtr(value)
	case "seedream_image_endpoint_id":
		overrides.SeedreamImageEndpointID = stringPtr(value)
	case "seedream_text_model":
		overrides.SeedreamTextModel = stringPtr(value)
	case "seedream_image_model":
		overrides.SeedreamImageModel = stringPtr(value)
	case "seedream_vision_model":
		overrides.SeedreamVisionModel = stringPtr(value)
	case "seedream_video_model":
		overrides.SeedreamVideoModel = stringPtr(value)
	case "sandbox_base_url":
		overrides.SandboxBaseURL = stringPtr(value)
	case "environment":
		overrides.Environment = stringPtr(value)
	case "session_dir":
		overrides.SessionDir = stringPtr(value)
	case "cost_dir":
		overrides.CostDir = stringPtr(value)
	case "agent_preset":
		overrides.AgentPreset = stringPtr(value)
	case "tool_preset":
		overrides.ToolPreset = stringPtr(value)
	case "max_tokens":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("max_tokens must be an integer: %w", err)
		}
		if parsed <= 0 {
			return fmt.Errorf("max_tokens must be greater than zero")
		}
		overrides.MaxTokens = intPtr(parsed)
	case "max_iterations":
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("max_iterations must be an integer: %w", err)
		}
		if parsed <= 0 {
			return fmt.Errorf("max_iterations must be greater than zero")
		}
		overrides.MaxIterations = intPtr(parsed)
	case "temperature":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("temperature must be a float: %w", err)
		}
		overrides.Temperature = floatPtr(parsed)
	case "top_p":
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("top_p must be a float: %w", err)
		}
		overrides.TopP = floatPtr(parsed)
	case "verbose":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("verbose must be a boolean: %w", err)
		}
		overrides.Verbose = boolPtr(parsed)
	case "disable_tui":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("disable_tui must be a boolean: %w", err)
		}
		overrides.DisableTUI = boolPtr(parsed)
	case "follow_transcript":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("follow_transcript must be a boolean: %w", err)
		}
		overrides.FollowTranscript = boolPtr(parsed)
	case "follow_stream":
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("follow_stream must be a boolean: %w", err)
		}
		overrides.FollowStream = boolPtr(parsed)
	case "stop_sequences":
		seqs := splitListValue(value)
		if len(seqs) == 0 {
			return fmt.Errorf("stop_sequences requires at least one entry")
		}
		overrides.StopSequences = &seqs
	default:
		return fmt.Errorf("unsupported override field %q", key)
	}
	return nil
}

func clearOverrideField(overrides *runtimeconfig.Overrides, key, _ string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	switch normalizeOverrideKey(key) {
	case "llm_provider":
		overrides.LLMProvider = nil
	case "llm_model":
		overrides.LLMModel = nil
	case "api_key":
		overrides.APIKey = nil
	case "ark_api_key":
		overrides.ArkAPIKey = nil
	case "base_url":
		overrides.BaseURL = nil
	case "tavily_api_key":
		overrides.TavilyAPIKey = nil
	case "seedream_text_endpoint_id":
		overrides.SeedreamTextEndpointID = nil
	case "seedream_image_endpoint_id":
		overrides.SeedreamImageEndpointID = nil
	case "seedream_text_model":
		overrides.SeedreamTextModel = nil
	case "seedream_image_model":
		overrides.SeedreamImageModel = nil
	case "seedream_vision_model":
		overrides.SeedreamVisionModel = nil
	case "seedream_video_model":
		overrides.SeedreamVideoModel = nil
	case "sandbox_base_url":
		overrides.SandboxBaseURL = nil
	case "environment":
		overrides.Environment = nil
	case "session_dir":
		overrides.SessionDir = nil
	case "cost_dir":
		overrides.CostDir = nil
	case "agent_preset":
		overrides.AgentPreset = nil
	case "tool_preset":
		overrides.ToolPreset = nil
	case "max_tokens":
		overrides.MaxTokens = nil
	case "max_iterations":
		overrides.MaxIterations = nil
	case "temperature":
		overrides.Temperature = nil
	case "top_p":
		overrides.TopP = nil
	case "verbose":
		overrides.Verbose = nil
	case "disable_tui":
		overrides.DisableTUI = nil
	case "follow_transcript":
		overrides.FollowTranscript = nil
	case "follow_stream":
		overrides.FollowStream = nil
	case "stop_sequences":
		overrides.StopSequences = nil
	default:
		return fmt.Errorf("unsupported override field %q", key)
	}
	return nil
}

var overrideKeyAliases = map[string]string{
	"provider":           "llm_provider",
	"model":              "llm_model",
	"key":                "api_key",
	"openai_api_key":     "api_key",
	"openrouter_api_key": "api_key",
	"baseurl":            "base_url",
	"sandbox_url":        "sandbox_base_url",
	"env":                "environment",
}

func normalizeOverrideKey(key string) string {
	trimmed := strings.TrimSpace(strings.ToLower(key))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	if canonical, ok := overrideKeyAliases[trimmed]; ok {
		return canonical
	}
	return trimmed
}

func splitListValue(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
	var result []string
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func stringPtr(value string) *string {
	v := value
	return &v
}

func boolPtr(value bool) *bool {
	v := value
	return &v
}

func intPtr(value int) *int {
	v := value
	return &v
}

func floatPtr(value float64) *float64 {
	v := value
	return &v
}
func readinessSummary(tasks []configadmin.ReadinessTask) string {
        if len(tasks) == 0 {
                return "  ✓ 所有关键配置均已就绪"
        }
        var builder strings.Builder
        for _, task := range tasks {
                fmt.Fprintf(&builder, "  [%s] %s\n", strings.ToUpper(string(task.Severity)), task.Label)
                if hint := strings.TrimSpace(task.Hint); hint != "" {
                        fmt.Fprintf(&builder, "      ↳ %s\n", hint)
                }
        }
        return strings.TrimRight(builder.String(), "\n")
}
