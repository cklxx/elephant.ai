package react

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
)

const (
	contextPlaceholderPrefix   = ports.ArtifactPlaceholderPrefix
	compactionCooldownTurns    = 2
	compactionArtifactKind     = "context_compaction_artifact"
	compactionArtifactFilename = "compaction-%04d.md"
)

type compactionReason string

const (
	compactionReasonThreshold compactionReason = "threshold"
	compactionReasonOverflow  compactionReason = "error_overflow"
)

type contextCompactionPlan struct {
	compressibleOriginalIndexes map[int]struct{}
	summarySource               []Message
}

type contextCompactionArtifact struct {
	Kind          string                   `json:"kind"`
	SessionID     string                   `json:"session_id"`
	RunID         string                   `json:"run_id"`
	Iteration     int                      `json:"iteration"`
	Sequence      int                      `json:"sequence"`
	Reason        compactionReason         `json:"reason"`
	CreatedAt     string                   `json:"created_at"`
	MessageCount  int                      `json:"message_count"`
	TokensBefore  int                      `json:"tokens_before"`
	TokensRemoved int                      `json:"tokens_removed"`
	Messages      []contextCompactionEntry `json:"messages"`
}

type contextCompactionEntry struct {
	Role       string              `json:"role,omitempty"`
	Source     ports.MessageSource `json:"source,omitempty"`
	Content    string              `json:"content,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	ToolCalls  []ports.ToolCall    `json:"tool_calls,omitempty"`
}

func (e *ReactEngine) tryArtifactCompaction(
	ctx context.Context,
	state *TaskState,
	services Services,
	messages []Message,
	reason compactionReason,
	force bool,
) ([]Message, bool) {
	if e == nil || state == nil || services.Context == nil || len(messages) == 0 {
		return messages, false
	}
	sessionID := strings.TrimSpace(state.SessionID)
	if sessionID == "" {
		return messages, false
	}
	if !force && isCompactionInCooldown(state) {
		return messages, false
	}

	plan := buildContextCompactionPlan(messages)
	if len(plan.compressibleOriginalIndexes) == 0 || len(plan.summarySource) == 0 {
		return messages, false
	}

	sequence := state.ContextCompactionSeq + 1
	path, hash, removedTokens, err := e.writeContextCompactionArtifact(ctx, state, services, plan.summarySource, reason, sequence)
	if err != nil {
		e.logger.Warn("Context artifact compaction write failed: %v", err)
		return messages, false
	}

	placeholder := ports.Message{
		Role: "assistant",
		Content: fmt.Sprintf(
			`[CTX_PLACEHOLDER path=%q sha256=%q msgs=%d tokens=%d reason=%q seq=%d]`,
			path,
			hash,
			len(plan.summarySource),
			removedTokens,
			string(reason),
			sequence,
		),
		Source: ports.MessageSourceCheckpoint,
		Metadata: map[string]any{
			"context_placeholder": true,
			"path":                path,
			"sha256":              hash,
			"messages":            len(plan.summarySource),
			"tokens_removed":      removedTokens,
			"reason":              string(reason),
			"sequence":            sequence,
			"created_at":          e.clock.Now().Format(time.RFC3339),
		},
	}

	compacted := make([]Message, 0, len(messages)-len(plan.compressibleOriginalIndexes)+1)
	inserted := false
	for idx, msg := range messages {
		if _, ok := plan.compressibleOriginalIndexes[idx]; ok {
			if !inserted {
				compacted = append(compacted, placeholder)
				inserted = true
			}
			continue
		}
		compacted = append(compacted, msg)
	}
	if !inserted {
		compacted = append(compacted, placeholder)
	}

	state.ContextCompactionSeq = sequence
	state.LastCompactionArtifact = path
	state.NextCompactionAllowed = state.Iterations + compactionCooldownTurns

	e.logger.Info(
		"Context artifact compaction applied: session=%s iter=%d reason=%s seq=%d path=%s removed_msgs=%d removed_tokens=%d",
		sessionID,
		state.Iterations,
		string(reason),
		sequence,
		path,
		len(plan.summarySource),
		removedTokens,
	)
	return compacted, true
}

func buildContextCompactionPlan(messages []Message) contextCompactionPlan {
	shared := ports.BuildCompressionPlan(messages, ports.CompressionPlanOptions{
		KeepRecentTurns: 1,
		PreserveSource:  isPreservedSource,
		IsSynthetic: func(msg ports.Message) bool {
			return isSyntheticCompactionMessage(msg)
		},
	})
	return contextCompactionPlan{
		compressibleOriginalIndexes: shared.CompressibleIndexes,
		summarySource:               shared.SummarySource,
	}
}

func isSyntheticCompactionMessage(msg Message) bool {
	return ports.IsSyntheticSummary(msg.Content)
}

func isCompactionInCooldown(state *TaskState) bool {
	if state == nil {
		return false
	}
	return state.NextCompactionAllowed > 0 && state.Iterations < state.NextCompactionAllowed
}

func (e *ReactEngine) writeContextCompactionArtifact(
	_ context.Context,
	state *TaskState,
	services Services,
	source []Message,
	reason compactionReason,
	sequence int,
) (string, string, int, error) {
	if len(source) == 0 {
		return "", "", 0, fmt.Errorf("empty compaction source")
	}

	entries := make([]contextCompactionEntry, 0, len(source))
	for _, msg := range source {
		entry := contextCompactionEntry{
			Role:       strings.TrimSpace(msg.Role),
			Source:     msg.Source,
			Content:    msg.Content,
			ToolCallID: strings.TrimSpace(msg.ToolCallID),
		}
		if len(msg.ToolCalls) > 0 {
			entry.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
		}
		entries = append(entries, entry)
	}

	tokensRemoved := services.Context.EstimateTokens(source)
	tokensBefore := services.Context.EstimateTokens(state.Messages)
	artifact := contextCompactionArtifact{
		Kind:          compactionArtifactKind,
		SessionID:     strings.TrimSpace(state.SessionID),
		RunID:         strings.TrimSpace(state.RunID),
		Iteration:     state.Iterations,
		Sequence:      sequence,
		Reason:        reason,
		CreatedAt:     e.clock.Now().Format(time.RFC3339),
		MessageCount:  len(entries),
		TokensBefore:  tokensBefore,
		TokensRemoved: tokensRemoved,
		Messages:      entries,
	}

	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		return "", "", 0, fmt.Errorf("marshal compaction artifact: %w", err)
	}

	sum := sha256.Sum256(payload)
	hash := hex.EncodeToString(sum[:])

	path := filepath.Join(resolveContextCompactionRoot(e), artifact.SessionID, "context", fmt.Sprintf(compactionArtifactFilename, sequence))
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}

	var doc strings.Builder
	doc.WriteString("---\n")
	doc.WriteString("kind: context_compaction_artifact\n")
	doc.WriteString(fmt.Sprintf("session_id: %q\n", artifact.SessionID))
	doc.WriteString(fmt.Sprintf("run_id: %q\n", artifact.RunID))
	doc.WriteString(fmt.Sprintf("iteration: %d\n", artifact.Iteration))
	doc.WriteString(fmt.Sprintf("sequence: %d\n", artifact.Sequence))
	doc.WriteString(fmt.Sprintf("reason: %q\n", string(artifact.Reason)))
	doc.WriteString(fmt.Sprintf("created_at: %q\n", artifact.CreatedAt))
	doc.WriteString(fmt.Sprintf("message_count: %d\n", artifact.MessageCount))
	doc.WriteString(fmt.Sprintf("tokens_before: %d\n", artifact.TokensBefore))
	doc.WriteString(fmt.Sprintf("tokens_removed: %d\n", artifact.TokensRemoved))
	doc.WriteString(fmt.Sprintf("sha256: %q\n", hash))
	doc.WriteString("---\n\n")
	doc.WriteString("```json\n")
	doc.Write(payload)
	doc.WriteString("\n```\n")

	var writeErr error
	if e != nil && e.atomicWriter != nil {
		writeErr = e.atomicWriter.WriteFileAtomically(path, []byte(doc.String()), 0o644)
	} else {
		writeErr = writeFileAtomically(path, []byte(doc.String()), 0o644)
	}
	if writeErr != nil {
		return "", "", 0, fmt.Errorf("write compaction artifact: %w", writeErr)
	}
	return path, hash, tokensRemoved, nil
}

func writeFileAtomically(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmp, err := os.CreateTemp(dir, ".tmp-compaction-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}
	defer cleanup()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

func resolveContextCompactionRoot(engine *ReactEngine) string {
	if engine != nil {
		if dp, ok := engine.checkpointStore.(CheckpointDirProvider); ok {
			dir := strings.TrimSpace(dp.BaseDir())
			if dir != "" {
				return filepath.Dir(dir)
			}
		}
	}
	return filepath.Join("runtime", "sessions")
}
