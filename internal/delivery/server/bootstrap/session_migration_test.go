package bootstrap

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	serverapp "alex/internal/delivery/server/app"
	"alex/internal/domain/agent/ports"
	"alex/internal/infra/session/filestore"
	"alex/internal/infra/session/postgresstore"
	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/shared/logging"
	"alex/internal/shared/testutil"
)

func TestMigrateSessionsToDatabase(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	sessionDir := t.TempDir()

	sourceSessions := filestore.New(sessionDir)
	session, err := sourceSessions.Create(ctx)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	session.Messages = []ports.Message{
		{Role: "user", Content: "hello"},
	}
	if err := sourceSessions.Save(ctx, session); err != nil {
		t.Fatalf("save session: %v", err)
	}

	sourceSnapshots := sessionstate.NewFileStore(filepath.Join(sessionDir, "snapshots"))
	sourceTurns := sessionstate.NewFileStore(filepath.Join(sessionDir, "turns"))
	snapshot := sessionstate.Snapshot{
		SessionID: session.ID,
		TurnID:    1,
		Summary:   "first",
		Messages:  session.Messages,
		CreatedAt: time.Now(),
	}
	if err := sourceSnapshots.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	if err := sourceTurns.SaveSnapshot(ctx, snapshot); err != nil {
		t.Fatalf("save history snapshot: %v", err)
	}

	destSessions := postgresstore.New(pool)
	destSnapshots := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindState)
	destHistory := sessionstate.NewPostgresStore(pool, sessionstate.SnapshotKindTurn)
	historyStore := serverapp.NewPostgresEventHistoryStore(pool)

	if err := destSessions.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure session schema: %v", err)
	}
	if err := destSnapshots.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure snapshot schema: %v", err)
	}
	if err := destHistory.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure history schema: %v", err)
	}
	if err := historyStore.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure history store schema: %v", err)
	}

	if err := MigrateSessionsToDatabase(
		ctx,
		sessionDir,
		destSessions,
		destSnapshots,
		destHistory,
		historyStore,
		logging.NewComponentLogger("SessionMigrationTest"),
	); err != nil {
		t.Fatalf("migrate sessions: %v", err)
	}

	migrated, err := destSessions.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("get migrated session: %v", err)
	}
	if migrated.ID != session.ID {
		t.Fatalf("expected session %q, got %q", session.ID, migrated.ID)
	}

	gotSnapshot, err := destSnapshots.GetSnapshot(ctx, session.ID, 1)
	if err != nil {
		t.Fatalf("get migrated snapshot: %v", err)
	}
	if gotSnapshot.Summary != "first" {
		t.Fatalf("expected snapshot summary, got %q", gotSnapshot.Summary)
	}

	gotHistory, err := destHistory.GetSnapshot(ctx, session.ID, 1)
	if err != nil {
		t.Fatalf("get migrated history snapshot: %v", err)
	}
	if gotHistory.TurnID != 1 {
		t.Fatalf("expected history turn 1, got %d", gotHistory.TurnID)
	}

	hasEvents, err := historyStore.HasSessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("check migrated event history: %v", err)
	}
	if !hasEvents {
		t.Fatalf("expected migrated event history for session %s", session.ID)
	}
}
