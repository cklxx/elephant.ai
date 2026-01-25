package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/domain"
	core "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	agentstorage "alex/internal/agent/ports/storage"
	runtimeconfig "alex/internal/config"
	"alex/internal/logging"
	serverapp "alex/internal/server/app"
	serverports "alex/internal/server/ports"
	"alex/internal/session/filestore"
	sessionstate "alex/internal/session/state_store"
)

const (
	replayTaskPrefix = "replay"
	migrationMarker  = ".migrated_to_postgres_v1"

	envSkipSessionMigration  = "ALEX_SKIP_SESSION_MIGRATION"
	envForceSessionMigration = "ALEX_FORCE_SESSION_MIGRATION"
)

type sessionMigrationMarker struct {
	Version        int       `json:"version"`
	CompletedAt    time.Time `json:"completed_at"`
	SourceSessions int       `json:"source_sessions"`
	DestSessions   int       `json:"dest_sessions"`
}

// MigrateSessionsToDatabase migrates file-backed sessions into the provided stores.
func MigrateSessionsToDatabase(
	ctx context.Context,
	sessionDir string,
	destSessions agentstorage.SessionStore,
	destSnapshots sessionstate.Store,
	destHistory sessionstate.Store,
	historyStore serverapp.EventHistoryStore,
	logger logging.Logger,
) error {
	logger = logging.OrNop(logger)
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(sessionDir) == "" {
		return nil
	}
	if destSessions == nil || destSnapshots == nil || destHistory == nil || historyStore == nil {
		return nil
	}

	if envBool(envSkipSessionMigration) {
		logger.Info("Session migration skipped (%s enabled)", envSkipSessionMigration)
		return nil
	}

	markerPath := filepath.Join(sessionDir, migrationMarker)
	force := envBool(envForceSessionMigration)
	if !force {
		if _, err := os.Stat(markerPath); err == nil {
			logger.Info("Session migration skipped (marker present: %s)", markerPath)
			return nil
		}
	}

	ids, err := listFileSessionIDs(sessionDir)
	if err != nil {
		return fmt.Errorf("list sessions for migration: %w", err)
	}
	ids = filterSessionIDsForMigration(ids)
	if len(ids) == 0 {
		return nil
	}

	sourceSessions := filestore.New(sessionDir)
	sourceSnapshots := sessionstate.NewFileStore(filepath.Join(sessionDir, "snapshots"))
	sourceHistory := sessionstate.NewFileStore(filepath.Join(sessionDir, "turns"))

	if !force {
		destCount, err := countSessions(ctx, destSessions)
		if err == nil && destCount >= len(ids) {
			logger.Info(
				"Session migration skipped (destination already has %d sessions; source=%d)",
				destCount,
				len(ids),
			)
			_ = writeSessionMigrationMarker(markerPath, sessionMigrationMarker{
				Version:        1,
				CompletedAt:    time.Now().UTC(),
				SourceSessions: len(ids),
				DestSessions:   destCount,
			})
			return nil
		}
	}

	startedAt := time.Now()
	logger.Info("Migrating %d sessions to database", len(ids))
	var migratedCount int
	var failures int
	for idx, sessionID := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sessionStartedAt := time.Now()
		session, err := sourceSessions.Get(ctx, sessionID)
		if err != nil {
			logger.Warn("Skipping session %s: %v", sessionID, err)
			failures++
			continue
		}
		if err := destSessions.Save(ctx, session); err != nil {
			logger.Warn("Failed to migrate session %s: %v", sessionID, err)
			failures++
		} else {
			migratedCount++
		}

		if err := migrateSnapshots(ctx, sourceSnapshots, destSnapshots, sessionID); err != nil {
			logger.Warn("Failed to migrate snapshots for session %s: %v", sessionID, err)
			failures++
		}
		if err := migrateSnapshots(ctx, sourceHistory, destHistory, sessionID); err != nil {
			logger.Warn("Failed to migrate history turns for session %s: %v", sessionID, err)
			failures++
		}

		hasEvents, err := historyStore.HasSessionEvents(ctx, sessionID)
		if err != nil {
			logger.Warn("Failed to check event history for session %s: %v", sessionID, err)
			failures++
			continue
		}
		if hasEvents {
			continue
		}

		turns, err := loadSnapshots(ctx, sourceHistory, sessionID)
		if err != nil {
			logger.Warn("Failed to load turn history for session %s: %v", sessionID, err)
			failures++
			continue
		}
		if len(turns) == 0 && session != nil && len(session.Messages) > 0 {
			turns = []sessionstate.Snapshot{{
				SessionID: sessionID,
				TurnID:    1,
				CreatedAt: session.UpdatedAt,
				Messages:  session.Messages,
			}}
		}

		for _, event := range buildReplayEvents(sessionID, turns) {
			if err := historyStore.Append(ctx, event); err != nil {
				logger.Warn("Failed to persist replay event for session %s: %v", sessionID, err)
				failures++
				break
			}
		}

		sessionElapsed := time.Since(sessionStartedAt)
		if sessionElapsed > 3*time.Second {
			logger.Info(
				"Migrated session %s (%d/%d) elapsed=%s",
				sessionID,
				idx+1,
				len(ids),
				sessionElapsed.Truncate(time.Millisecond),
			)
		} else if (idx+1)%10 == 0 {
			logger.Info(
				"Migrating sessions... %d/%d (elapsed=%s)",
				idx+1,
				len(ids),
				time.Since(startedAt).Truncate(time.Second),
			)
		}
	}

	elapsed := time.Since(startedAt)
	logger.Info(
		"Session migration complete (migrated=%d/%d failures=%d elapsed=%s)",
		migratedCount,
		len(ids),
		failures,
		elapsed.Truncate(time.Millisecond),
	)
	_ = writeSessionMigrationMarker(markerPath, sessionMigrationMarker{
		Version:        1,
		CompletedAt:    time.Now().UTC(),
		SourceSessions: len(ids),
		DestSessions:   migratedCount,
	})
	return nil
}

func listFileSessionIDs(sessionDir string) ([]string, error) {
	entries, err := os.ReadDir(sessionDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSpace(entry.Name())
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	return ids, nil
}

func countSessions(ctx context.Context, store serverports.ServerSessionManager) (int, error) {
	if store == nil {
		return 0, nil
	}
	const pageSize = 200
	total := 0
	offset := 0
	for {
		ids, err := store.List(ctx, pageSize, offset)
		if err != nil {
			return total, err
		}
		if len(ids) == 0 {
			break
		}
		total += len(ids)
		if len(ids) < pageSize {
			break
		}
		offset += len(ids)
	}
	return total, nil
}

func envBool(name string) bool {
	lookup := runtimeconfig.DefaultEnvLookup
	value, ok := lookup(name)
	if !ok {
		return false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func writeSessionMigrationMarker(path string, marker sessionMigrationMarker) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	data, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func filterSessionIDsForMigration(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(ids))
	for _, sessionID := range ids {
		trimmed := strings.TrimSpace(sessionID)
		if trimmed == "" {
			continue
		}
		if strings.HasSuffix(trimmed, "_attachments") {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	return filtered
}

func migrateSnapshots(ctx context.Context, source, dest sessionstate.Store, sessionID string) error {
	snapshots, err := loadSnapshots(ctx, source, sessionID)
	if err != nil {
		return err
	}
	for _, snap := range snapshots {
		if err := dest.SaveSnapshot(ctx, snap); err != nil {
			return err
		}
	}
	return nil
}

func loadSnapshots(ctx context.Context, store sessionstate.Store, sessionID string) ([]sessionstate.Snapshot, error) {
	if store == nil {
		return nil, nil
	}
	cursor := ""
	var snaps []sessionstate.Snapshot
	for {
		metas, next, err := store.ListSnapshots(ctx, sessionID, cursor, 200)
		if err != nil {
			return nil, err
		}
		if len(metas) == 0 {
			break
		}
		for _, meta := range metas {
			snap, err := store.GetSnapshot(ctx, sessionID, meta.TurnID)
			if err != nil {
				if err == sessionstate.ErrSnapshotNotFound {
					continue
				}
				return nil, err
			}
			snaps = append(snaps, snap)
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(snaps) == 0 {
		return nil, nil
	}

	sort.Slice(snaps, func(i, j int) bool {
		return snaps[i].TurnID < snaps[j].TurnID
	})
	return snaps, nil
}

func buildReplayEvents(sessionID string, turns []sessionstate.Snapshot) []agent.AgentEvent {
	if len(turns) == 0 {
		return nil
	}

	var events []agent.AgentEvent
	for _, turn := range turns {
		if len(turn.Messages) == 0 {
			continue
		}

		taskID := fmt.Sprintf("%s-%s-%d", replayTaskPrefix, sessionID, turn.TurnID)
		baseTime := turn.CreatedAt
		if baseTime.IsZero() {
			baseTime = time.Now()
		}

		callNames := make(map[string]string)
		offset := 0
		finalAnswer := ""
		var finalAttachments map[string]core.Attachment

		for _, msg := range turn.Messages {
			if len(msg.ToolCalls) > 0 {
				for _, call := range msg.ToolCalls {
					if call.ID != "" && call.Name != "" {
						callNames[call.ID] = call.Name
					}
				}
			}

			role := strings.ToLower(strings.TrimSpace(msg.Role))
			switch role {
			case "user":
				event := domain.NewWorkflowInputReceivedEvent(
					agent.LevelCore,
					sessionID,
					taskID,
					"",
					msg.Content,
					msg.Attachments,
					baseTime.Add(time.Duration(offset)*time.Millisecond),
				)
				events = append(events, event)
				offset++
			case "assistant":
				if len(msg.ToolCalls) > 0 {
					events = append(events, replayEnvelope(
						sessionID,
						taskID,
						baseTime.Add(time.Duration(offset)*time.Millisecond),
						"workflow.node.output.summary",
						"generation",
						"",
						map[string]any{
							"iteration":       turn.TurnID,
							"content":         msg.Content,
							"tool_call_count": len(msg.ToolCalls),
						},
					))
					offset++
				} else if strings.TrimSpace(msg.Content) != "" {
					finalAnswer = msg.Content
					if len(msg.Attachments) > 0 {
						finalAttachments = msg.Attachments
					}
				}
			case "tool":
				for _, result := range msg.ToolResults {
					callID := result.CallID
					toolName := resolveToolName(callID, result, callNames)
					payload := map[string]any{
						"tool_name":   toolName,
						"result":      result.Content,
						"duration":    0,
						"metadata":    result.Metadata,
						"attachments": result.Attachments,
					}
					if result.Error != nil {
						payload["error"] = result.Error.Error()
					}
					events = append(events, replayEnvelope(
						sessionID,
						taskID,
						baseTime.Add(time.Duration(offset)*time.Millisecond),
						"workflow.tool.completed",
						"tool",
						callID,
						payload,
					))
					offset++
				}
			default:
				if len(msg.ToolResults) > 0 {
					for _, result := range msg.ToolResults {
						callID := result.CallID
						toolName := resolveToolName(callID, result, callNames)
						payload := map[string]any{
							"tool_name":   toolName,
							"result":      result.Content,
							"duration":    0,
							"metadata":    result.Metadata,
							"attachments": result.Attachments,
						}
						if result.Error != nil {
							payload["error"] = result.Error.Error()
						}
						events = append(events, replayEnvelope(
							sessionID,
							taskID,
							baseTime.Add(time.Duration(offset)*time.Millisecond),
							"workflow.tool.completed",
							"tool",
							callID,
							payload,
						))
						offset++
					}
				}
			}
		}

		if strings.TrimSpace(finalAnswer) != "" || len(finalAttachments) > 0 {
			payload := map[string]any{
				"final_answer":     finalAnswer,
				"total_iterations": turn.TurnID,
				"total_tokens":     0,
				"stop_reason":      "completed",
				"duration":         0,
				"is_streaming":     false,
				"stream_finished":  true,
				"attachments":      finalAttachments,
			}
			events = append(events, replayEnvelope(
				sessionID,
				taskID,
				baseTime.Add(time.Duration(offset)*time.Millisecond),
				"workflow.result.final",
				"result",
				"summarize",
				payload,
			))
		}
	}

	return events
}

func resolveToolName(callID string, result core.ToolResult, callNames map[string]string) string {
	if callID != "" {
		if name := callNames[callID]; name != "" {
			return name
		}
	}
	if result.Metadata != nil {
		for _, key := range []string{"tool_name", "tool", "name"} {
			if val, ok := result.Metadata[key].(string); ok && strings.TrimSpace(val) != "" {
				return strings.TrimSpace(val)
			}
		}
	}
	if callID != "" {
		return callID
	}
	return "tool"
}

func replayEnvelope(sessionID, taskID string, ts time.Time, eventType, nodeKind, nodeID string, payload map[string]any) agent.AgentEvent {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, taskID, "", ts),
		Version:   1,
		Event:     eventType,
		NodeKind:  nodeKind,
		NodeID:    nodeID,
		Payload:   payload,
	}
}
