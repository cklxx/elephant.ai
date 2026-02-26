package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	sessionstate "alex/internal/infra/session/state_store"
	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
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
		return c.handleConfig(cmdArgs)

	case "cost", "costs":
		return c.handleCostCommand(cmdArgs)

	case "model", "models":
		return c.handleModel(cmdArgs)
	case "setup":
		return c.handleSetup(cmdArgs)

	case "llama-cpp", "llamacpp":
		return c.handleLlamaCpp(cmdArgs)

	case "mcp":
		return c.handleMCP(cmdArgs)

	case "eval", "evaluation":
		return c.handleEval(cmdArgs)
	case "acp":
		return c.handleACP(cmdArgs)
	case "mcp-permission-server":
		return runMCPPermissionServer(cmdArgs)
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
  alex lark scenario run [...]   Run Lark scenario suite (YAML-driven; http/mock)
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
  alex mcp                       MCP (Model Context Protocol) management
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
Documentation: AGENTS.md + docs/reference/ARCHITECTURE_AGENT_FLOW.md + docs/reference/CONFIG.md
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

	store := c.container.CheckpointStore
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

func (c *CLI) handleSessions(args []string) error {
	ctx := cliBaseContext()

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

	fs, flagBuf := newBufferedFlagSet("sessions pull")
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
		return formatBufferedFlagParseError(err, flagBuf)
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
	if utils.HasContent(snapshot.Summary) {
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
	sessionIDs, err := c.listAllSessions(ctx)
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

func (c *CLI) listAllSessions(ctx context.Context) ([]string, error) {
	const pageSize = 200
	var sessionIDs []string
	offset := 0
	for {
		ids, err := c.container.AgentCoordinator.ListSessions(ctx, pageSize, offset)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			break
		}
		sessionIDs = append(sessionIDs, ids...)
		if len(ids) < pageSize {
			break
		}
		offset += len(ids)
	}
	return sessionIDs, nil
}

func (c *CLI) handleConfig(args []string) error {
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
		if _, err := fmt.Fprintf(out, "已更新 %s (写入 %s#overrides)\n\n", normalizeOverrideKey(key), overridesPath); err != nil {
			return fmt.Errorf("write update message: %w", err)
		}
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
		if _, err := fmt.Fprintf(out, "已清除 %s (写入 %s#overrides)\n\n", normalizeOverrideKey(key), overridesPath); err != nil {
			return fmt.Errorf("write clear message: %w", err)
		}
		return printConfigSummary(out, overridesPath)
	case "path", "file":
		if _, err := fmt.Fprintln(out, overridesPath); err != nil {
			return fmt.Errorf("write overrides path: %w", err)
		}
		return nil
	case "validate", "check":
		return validateRuntimeConfiguration(args[1:], out)
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
	if _, err := fmt.Fprintln(out, "Current Configuration:"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Provider:       %s\n", cfg.LLMProvider); err != nil {
		return fmt.Errorf("write provider: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Model:          %s\n", cfg.LLMModel); err != nil {
		return fmt.Errorf("write model: %w", err)
	}
	if cfg.LLMVisionModel != "" {
		if _, err := fmt.Fprintf(out, "  Vision Model:   %s\n", cfg.LLMVisionModel); err != nil {
			return fmt.Errorf("write vision model: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "  Base URL:       %s\n", cfg.BaseURL); err != nil {
		return fmt.Errorf("write base url: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Max Tokens:     %d\n", cfg.MaxTokens); err != nil {
		return fmt.Errorf("write max tokens: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Max Iterations: %d\n", cfg.MaxIterations); err != nil {
		return fmt.Errorf("write max iterations: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Temperature:    %.2f\n", cfg.Temperature); err != nil {
		return fmt.Errorf("write temperature: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Top P:          %.2f\n", cfg.TopP); err != nil {
		return fmt.Errorf("write top p: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Profile:        %s\n", runtimeconfig.NormalizeRuntimeProfile(cfg.Profile)); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Environment:    %s\n", cfg.Environment); err != nil {
		return fmt.Errorf("write environment: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Verbose:        %t\n", cfg.Verbose); err != nil {
		return fmt.Errorf("write verbose: %w", err)
	}
	if len(cfg.StopSequences) > 0 {
		if _, err := fmt.Fprintf(out, "  Stop Seqs:      %s\n", strings.Join(cfg.StopSequences, ", ")); err != nil {
			return fmt.Errorf("write stop sequences: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(out, "  Stop Seqs:      (not set)"); err != nil {
			return fmt.Errorf("write stop sequences missing: %w", err)
		}
	}
	if cfg.APIKey != "" {
		if _, err := fmt.Fprintln(out, "  API Key:        (set)"); err != nil {
			return fmt.Errorf("write api key set: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(out, "  API Key:        (not set)"); err != nil {
			return fmt.Errorf("write api key missing: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "  Loaded At:      %s\n", meta.LoadedAt().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("write loaded at: %w", err)
	}
	if _, err := fmt.Fprintf(out, "\nConfig file: %s\n", overridesPath); err != nil {
		return fmt.Errorf("write config file path: %w", err)
	}
	if _, err := fmt.Fprintln(out, "就绪检查:"); err != nil {
		return fmt.Errorf("write readiness heading: %w", err)
	}
	if _, err := fmt.Fprintln(out, readinessSummary(configadmin.DeriveReadinessTasks(cfg))); err != nil {
		return fmt.Errorf("write readiness summary: %w", err)
	}
	return nil
}

func printConfigUsage(out io.Writer) {
	lines := []string{
		"Config command usage:",
		"  alex config                       Show current configuration snapshot",
		"  alex config set <field> <value>   Persist a managed override (e.g. llm_model gpt-4o-mini)",
		"  alex config set field=value       Alternate set syntax",
		"  alex config clear <field>         Remove an override",
		"  alex config validate [--profile]  Validate runtime configuration",
		"  alex config path                  Print the runtime config file location",
		"",
		"Supported fields: llm_provider, llm_model, llm_vision_model, base_url, api_key, ark_api_key, tavily_api_key, profile, environment, max_tokens, max_iterations, temperature, top_p, verbose, stop_sequences, agent_preset, tool_preset, and Seedream model/endpoints.",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			fmt.Fprintf(os.Stderr, "print config usage: %v\n", err)
			return
		}
	}
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

type overrideFieldHandler struct {
	set   func(*runtimeconfig.Overrides, string) error
	clear func(*runtimeconfig.Overrides)
}

var overrideFieldHandlers = map[string]overrideFieldHandler{
	"llm_provider":              stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMProvider = v }),
	"llm_model":                 stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMModel = v }),
	"llm_vision_model":          stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.LLMVisionModel = v }),
	"api_key":                   stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.APIKey = v }),
	"ark_api_key":               stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.ArkAPIKey = v }),
	"base_url":                  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.BaseURL = v }),
	"tavily_api_key":            stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.TavilyAPIKey = v }),
	"seedream_text_endpoint_id": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamTextEndpointID = v }),
	"seedream_image_endpoint_id": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) {
		o.SeedreamImageEndpointID = v
	}),
	"seedream_text_model":   stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamTextModel = v }),
	"seedream_image_model":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamImageModel = v }),
	"seedream_vision_model": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamVisionModel = v }),
	"seedream_video_model":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SeedreamVideoModel = v }),
	"profile": normalizedStringOverrideField(
		runtimeconfig.NormalizeRuntimeProfile,
		func(o *runtimeconfig.Overrides, v *string) { o.Profile = v },
	),
	"environment":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.Environment = v }),
	"session_dir":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.SessionDir = v }),
	"cost_dir":     stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.CostDir = v }),
	"agent_preset": stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.AgentPreset = v }),
	"tool_preset":  stringOverrideField(func(o *runtimeconfig.Overrides, v *string) { o.ToolPreset = v }),
	"max_tokens":   positiveIntOverrideField("max_tokens", func(o *runtimeconfig.Overrides, v *int) { o.MaxTokens = v }),
	"max_iterations": positiveIntOverrideField(
		"max_iterations",
		func(o *runtimeconfig.Overrides, v *int) { o.MaxIterations = v },
	),
	"temperature": floatOverrideField("temperature", func(o *runtimeconfig.Overrides, v *float64) { o.Temperature = v }),
	"top_p":       floatOverrideField("top_p", func(o *runtimeconfig.Overrides, v *float64) { o.TopP = v }),
	"verbose":     boolOverrideField("verbose", func(o *runtimeconfig.Overrides, v *bool) { o.Verbose = v }),
	"disable_tui": boolOverrideField("disable_tui", func(o *runtimeconfig.Overrides, v *bool) { o.DisableTUI = v }),
	"follow_transcript": boolOverrideField(
		"follow_transcript",
		func(o *runtimeconfig.Overrides, v *bool) { o.FollowTranscript = v },
	),
	"follow_stream":  boolOverrideField("follow_stream", func(o *runtimeconfig.Overrides, v *bool) { o.FollowStream = v }),
	"stop_sequences": stopSequencesOverrideField(),
}

func setOverrideField(overrides *runtimeconfig.Overrides, key, value string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	normalizedKey := normalizeOverrideKey(key)
	trimmedValue := strings.TrimSpace(value)
	if trimmedValue == "" {
		return fmt.Errorf("value for %s cannot be empty", normalizedKey)
	}
	handler, ok := overrideFieldHandlers[normalizedKey]
	if !ok {
		return fmt.Errorf("unsupported override field %q", normalizedKey)
	}
	return handler.set(overrides, trimmedValue)
}

func clearOverrideField(overrides *runtimeconfig.Overrides, key, _ string) error {
	if overrides == nil {
		return fmt.Errorf("overrides not initialized")
	}
	normalizedKey := normalizeOverrideKey(key)
	handler, ok := overrideFieldHandlers[normalizedKey]
	if !ok {
		return fmt.Errorf("unsupported override field %q", normalizedKey)
	}
	handler.clear(overrides)
	return nil
}

func stringOverrideField(assign func(*runtimeconfig.Overrides, *string)) overrideFieldHandler {
	return normalizedStringOverrideField(func(value string) string { return value }, assign)
}

func normalizedStringOverrideField(
	normalize func(string) string,
	assign func(*runtimeconfig.Overrides, *string),
) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			assign(overrides, stringPtr(normalize(value)))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func positiveIntOverrideField(name string, assign func(*runtimeconfig.Overrides, *int)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parsePositiveInt(value, name)
			if err != nil {
				return err
			}
			assign(overrides, intPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func floatOverrideField(name string, assign func(*runtimeconfig.Overrides, *float64)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parseFloat(value, name)
			if err != nil {
				return err
			}
			assign(overrides, floatPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func boolOverrideField(name string, assign func(*runtimeconfig.Overrides, *bool)) overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			parsed, err := parseBool(value, name)
			if err != nil {
				return err
			}
			assign(overrides, boolPtr(parsed))
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			assign(overrides, nil)
		},
	}
}

func stopSequencesOverrideField() overrideFieldHandler {
	return overrideFieldHandler{
		set: func(overrides *runtimeconfig.Overrides, value string) error {
			seqs := splitListValue(value)
			if len(seqs) == 0 {
				return fmt.Errorf("stop_sequences requires at least one entry")
			}
			overrides.StopSequences = &seqs
			return nil
		},
		clear: func(overrides *runtimeconfig.Overrides) {
			overrides.StopSequences = nil
		},
	}
}

func parsePositiveInt(value string, name string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", name, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("%s must be greater than zero", name)
	}
	return parsed, nil
}

func parseFloat(value string, name string) (float64, error) {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a float: %w", name, err)
	}
	return parsed, nil
}

func parseBool(value string, name string) (bool, error) {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %w", name, err)
	}
	return parsed, nil
}

func normalizeOverrideKey(key string) string {
	trimmed := strings.TrimSpace(strings.ToLower(key))
	trimmed = strings.ReplaceAll(trimmed, "-", "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
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

func validateRuntimeConfiguration(args []string, out io.Writer) error {
	cfg, _, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}

	profile := strings.TrimSpace(cfg.Profile)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile", "-p":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: alex config validate [--profile quickstart|standard|production]")
			}
			profile = strings.TrimSpace(args[i+1])
			i++
		default:
			return fmt.Errorf("usage: alex config validate [--profile quickstart|standard|production]")
		}
	}

	cfg.Profile = runtimeconfig.NormalizeRuntimeProfile(profile)
	report := runtimeconfig.ValidateRuntimeConfig(cfg)

	if _, err := fmt.Fprintf(out, "Validation Profile: %s\n", report.Profile); err != nil {
		return fmt.Errorf("write validation profile: %w", err)
	}
	if len(report.Errors) == 0 && len(report.Warnings) == 0 {
		if _, err := fmt.Fprintln(out, "STATUS: OK"); err != nil {
			return fmt.Errorf("write validation status: %w", err)
		}
	}
	for _, item := range report.Errors {
		if _, err := fmt.Fprintf(out, "ERROR %s: %s\n", item.ID, item.Message); err != nil {
			return fmt.Errorf("write validation error: %w", err)
		}
		if hint := strings.TrimSpace(item.Hint); hint != "" {
			if _, err := fmt.Fprintf(out, "  hint: %s\n", hint); err != nil {
				return fmt.Errorf("write validation error hint: %w", err)
			}
		}
	}
	for _, item := range report.Warnings {
		if _, err := fmt.Fprintf(out, "WARNING %s: %s\n", item.ID, item.Message); err != nil {
			return fmt.Errorf("write validation warning: %w", err)
		}
		if hint := strings.TrimSpace(item.Hint); hint != "" {
			if _, err := fmt.Fprintf(out, "  hint: %s\n", hint); err != nil {
				return fmt.Errorf("write validation warning hint: %w", err)
			}
		}
	}
	if len(report.DisabledTools) > 0 {
		if _, err := fmt.Fprintln(out, "Disabled tools:"); err != nil {
			return fmt.Errorf("write disabled tools heading: %w", err)
		}
		for _, item := range report.DisabledTools {
			if _, err := fmt.Fprintf(out, "  - %s: %s\n", item.Name, item.Reason); err != nil {
				return fmt.Errorf("write disabled tool: %w", err)
			}
		}
	}

	if report.HasErrors() {
		return fmt.Errorf("config validation failed with %d error(s)", len(report.Errors))
	}
	return nil
}
