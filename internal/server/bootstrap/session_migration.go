package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/domain"
	agentports "alex/internal/agent/ports"
	"alex/internal/logging"
	serverapp "alex/internal/server/app"
	"alex/internal/session/filestore"
	sessionstate "alex/internal/session/state_store"
)

const (
	replayTaskPrefix = "replay"
)

// MigrateSessionsToDatabase migrates file-backed sessions into the provided stores.
func MigrateSessionsToDatabase(
	ctx context.Context,
	sessionDir string,
	destSessions agentports.SessionStore,
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

	sourceSessions := filestore.New(sessionDir)
	sourceSnapshots := sessionstate.NewFileStore(filepath.Join(sessionDir, "snapshots"))
	sourceHistory := sessionstate.NewFileStore(filepath.Join(sessionDir, "turns"))

	ids, err := sourceSessions.List(ctx)
	if err != nil {
		return fmt.Errorf("list sessions for migration: %w", err)
	}
	if len(ids) == 0 {
		return nil
	}

	logger.Info("Migrating %d sessions to database", len(ids))
	for _, sessionID := range ids {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		session, err := sourceSessions.Get(ctx, sessionID)
		if err != nil {
			logger.Warn("Skipping session %s: %v", sessionID, err)
			continue
		}
		if err := destSessions.Save(ctx, session); err != nil {
			logger.Warn("Failed to migrate session %s: %v", sessionID, err)
		}

		if err := migrateSnapshots(ctx, sourceSnapshots, destSnapshots, sessionID); err != nil {
			logger.Warn("Failed to migrate snapshots for session %s: %v", sessionID, err)
		}
		if err := migrateSnapshots(ctx, sourceHistory, destHistory, sessionID); err != nil {
			logger.Warn("Failed to migrate history turns for session %s: %v", sessionID, err)
		}

		hasEvents, err := historyStore.HasSessionEvents(ctx, sessionID)
		if err != nil {
			logger.Warn("Failed to check event history for session %s: %v", sessionID, err)
			continue
		}
		if hasEvents {
			continue
		}

		turns, err := loadSnapshots(ctx, sourceHistory, sessionID)
		if err != nil {
			logger.Warn("Failed to load turn history for session %s: %v", sessionID, err)
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
				break
			}
		}
	}

	logger.Info("Session migration complete")
	return nil
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

func buildReplayEvents(sessionID string, turns []sessionstate.Snapshot) []agentports.AgentEvent {
	if len(turns) == 0 {
		return nil
	}

	var events []agentports.AgentEvent
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
		var finalAttachments map[string]agentports.Attachment

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
					agentports.LevelCore,
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

func resolveToolName(callID string, result agentports.ToolResult, callNames map[string]string) string {
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

func replayEnvelope(sessionID, taskID string, ts time.Time, eventType, nodeKind, nodeID string, payload map[string]any) agentports.AgentEvent {
	return &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, sessionID, taskID, "", ts),
		Version:   1,
		Event:     eventType,
		NodeKind:  nodeKind,
		NodeID:    nodeID,
		Payload:   payload,
	}
}
