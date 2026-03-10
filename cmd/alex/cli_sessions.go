package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/shared/utils"
)

func (c *CLI) handleSessions(args []string) error {
	ctx := cliBaseContext()

	if len(args) == 0 || args[0] == "list" {
		listArgs := args
		if len(args) > 0 {
			listArgs = args[1:]
		}
		return c.listSessionsCommand(ctx, listArgs)
	}

	switch args[0] {
	case "cleanup", "clean", "prune":
		return c.cleanupSessions(ctx, args[1:])
	case "inspect", "show", "detail":
		return c.inspectSessionCommand(ctx, args[1:])
	case "pull":
		return c.pullSessionSnapshotsWithWriter(ctx, args[1:], os.Stdout)
	default:
		return fmt.Errorf("unknown sessions subcommand: %s\nUsage: alex sessions [list|inspect|clean|pull]", args[0])
	}
}

const (
	defaultSnapshotListLimit = 20
	llmTurnSearchPageSize   = 50
	flagUsageLine           = "usage: alex sessions pull <session-id> [--turn N|--llm-turn N]"
)

func (c *CLI) pullSessionSnapshotsWithWriter(ctx context.Context, args []string, out io.Writer) error {
	if c == nil || c.container == nil {
		return fmt.Errorf("container not initialized")
	}
	store := c.container.Container.StateStore
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

// sessionListRow is the structured data for one session in list output.
type sessionListRow struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Messages  int    `json:"messages"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Age       string `json:"age"`
}

func (c *CLI) listSessionsCommand(ctx context.Context, args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex sessions list")
	jsonOut := fs.Bool("json", false, "Output as JSON array")
	if err := fs.Parse(args); err != nil {
		return formatBufferedFlagParseError(err, flagBuf)
	}
	return c.listSessionsWithWriter(ctx, os.Stdout, *jsonOut)
}

func (c *CLI) listSessionsWithWriter(ctx context.Context, out io.Writer, jsonOut bool) error {
	sessionIDs, err := c.listAllSessions(ctx)
	if err != nil {
		return err
	}
	if len(sessionIDs) == 0 {
		if jsonOut {
			_, _ = fmt.Fprintln(out, "[]")
			return nil
		}
		_, _ = fmt.Fprintln(out, "No sessions found")
		return nil
	}

	now := time.Now()
	rows := make([]sessionListRow, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		session, err := c.container.Container.SessionStore.Get(ctx, sid)
		if err != nil {
			rows = append(rows, sessionListRow{ID: sid, Title: "(error)"})
			continue
		}
		title := session.Metadata["title"]
		if title == "" {
			title = "-"
		}
		rows = append(rows, sessionListRow{
			ID:        sid,
			Title:     truncateStr(title, 40),
			Messages:  len(session.Messages),
			CreatedAt: session.CreatedAt.Format("2006-01-02 15:04"),
			UpdatedAt: session.UpdatedAt.Format("2006-01-02 15:04"),
			Age:       formatAge(now.Sub(session.UpdatedAt)),
		})
	}

	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(rows)
	}

	_, _ = fmt.Fprintf(out, "Sessions: %d\n\n", len(rows))
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(tw, "ID\tTITLE\tMSGS\tCREATED\tLAST ACTIVE\tAGE")
	for _, r := range rows {
		_, _ = fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\t%s\n",
			r.ID, r.Title, r.Messages, r.CreatedAt, r.UpdatedAt, r.Age)
	}
	return tw.Flush()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func formatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// sessionInspectResult is the structured output for inspect.
type sessionInspectResult struct {
	ID           string            `json:"id"`
	Title        string            `json:"title"`
	CreatedAt    string            `json:"created_at"`
	UpdatedAt    string            `json:"updated_at"`
	MessageCount int               `json:"message_count"`
	TodoCount    int               `json:"todo_count"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Roles        map[string]int    `json:"roles"`
	Cost         *inspectCostInfo  `json:"cost,omitempty"`
}

type inspectCostInfo struct {
	TotalTokens  int     `json:"total_tokens"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost"`
	RequestCount int     `json:"request_count"`
	Model        string  `json:"model,omitempty"`
}

func (c *CLI) inspectSessionCommand(ctx context.Context, args []string) error {
	fs, flagBuf := newBufferedFlagSet("alex sessions inspect")
	jsonOut := fs.Bool("json", false, "Output as JSON")

	var positionalID string
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		positionalID = strings.TrimSpace(args[0])
		args = args[1:]
	}
	if err := fs.Parse(args); err != nil {
		return formatBufferedFlagParseError(err, flagBuf)
	}
	sessionID := positionalID
	if sessionID == "" {
		if rest := fs.Args(); len(rest) > 0 {
			sessionID = strings.TrimSpace(rest[0])
		}
	}
	if sessionID == "" {
		return fmt.Errorf("usage: alex sessions inspect <session-id> [--json]")
	}
	return c.inspectSessionWithWriter(ctx, sessionID, os.Stdout, *jsonOut)
}

func (c *CLI) inspectSessionWithWriter(ctx context.Context, sessionID string, out io.Writer, jsonOut bool) error {
	if c == nil || c.container == nil || c.container.Container == nil {
		return fmt.Errorf("container not initialized")
	}
	store := c.container.Container.SessionStore
	if store == nil {
		return fmt.Errorf("session store not configured")
	}

	session, err := store.Get(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get session %s: %w", sessionID, err)
	}

	title := session.Metadata["title"]
	if title == "" {
		title = "(untitled)"
	}

	// Count messages by role
	roles := make(map[string]int)
	for _, msg := range session.Messages {
		roles[msg.Role]++
	}

	result := sessionInspectResult{
		ID:           session.ID,
		Title:        title,
		CreatedAt:    session.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    session.UpdatedAt.Format(time.RFC3339),
		MessageCount: len(session.Messages),
		TodoCount:    len(session.Todos),
		Metadata:     session.Metadata,
		Roles:        roles,
	}

	// Try to fetch cost data if CostTracker is available
	if ct := c.container.Container.CostTracker; ct != nil {
		if summary, err := ct.GetSessionCost(ctx, sessionID); err == nil && summary != nil {
			model := topModel(summary.ByModel)
			result.Cost = &inspectCostInfo{
				TotalTokens:  summary.TotalTokens,
				InputTokens:  summary.InputTokens,
				OutputTokens: summary.OutputTokens,
				TotalCost:    summary.TotalCost,
				RequestCount: summary.RequestCount,
				Model:        model,
			}
		}
	}

	if jsonOut {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return printSessionInspect(out, result)
}

func printSessionInspect(out io.Writer, r sessionInspectResult) error {
	_, _ = fmt.Fprintf(out, "Session: %s\n", r.ID)
	_, _ = fmt.Fprintf(out, "Title:   %s\n", r.Title)
	_, _ = fmt.Fprintf(out, "Created: %s\n", r.CreatedAt)
	_, _ = fmt.Fprintf(out, "Updated: %s\n", r.UpdatedAt)
	_, _ = fmt.Fprintf(out, "\nMessages: %d\n", r.MessageCount)

	if len(r.Roles) > 0 {
		parts := make([]string, 0, len(r.Roles))
		for role, count := range r.Roles {
			parts = append(parts, fmt.Sprintf("%s=%d", role, count))
		}
		sort.Strings(parts)
		_, _ = fmt.Fprintf(out, "  Breakdown: %s\n", strings.Join(parts, ", "))
	}

	_, _ = fmt.Fprintf(out, "Todos:    %d\n", r.TodoCount)

	if r.Cost != nil {
		_, _ = fmt.Fprintf(out, "\nToken Usage:\n")
		_, _ = fmt.Fprintf(out, "  Total:    %d (in: %d, out: %d)\n",
			r.Cost.TotalTokens, r.Cost.InputTokens, r.Cost.OutputTokens)
		_, _ = fmt.Fprintf(out, "  Cost:     $%.4f\n", r.Cost.TotalCost)
		_, _ = fmt.Fprintf(out, "  Requests: %d\n", r.Cost.RequestCount)
		if r.Cost.Model != "" {
			_, _ = fmt.Fprintf(out, "  Model:    %s\n", r.Cost.Model)
		}
	}

	if len(r.Metadata) > 0 {
		_, _ = fmt.Fprintf(out, "\nMetadata:\n")
		keys := make([]string, 0, len(r.Metadata))
		for k := range r.Metadata {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			_, _ = fmt.Fprintf(out, "  %s: %s\n", k, r.Metadata[k])
		}
	}

	return nil
}

// topModel returns the model with the highest cost from a ByModel map.
func topModel(byModel map[string]float64) string {
	if len(byModel) == 0 {
		return ""
	}
	best := ""
	bestCost := 0.0
	for model, cost := range byModel {
		if cost > bestCost || best == "" {
			best = model
			bestCost = cost
		}
	}
	return best
}

func (c *CLI) listAllSessions(ctx context.Context) ([]string, error) {
	const pageSize = 200
	var sessionIDs []string
	offset := 0
	for {
		ids, err := c.container.Container.AgentCoordinator.ListSessions(ctx, pageSize, offset)
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
