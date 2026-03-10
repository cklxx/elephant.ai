//go:build integration

package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

func TestStore_PersistRecoverAndAppendEvents(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	running := session.New("running-1", session.MemberClaudeCode, "continue coding", "/tmp/running")
	running.SetPane(51, 5)
	running.SetParentSession("leader-1")
	running.SetPoolPane(true)
	if err := running.Transition(session.StateStarting); err != nil {
		t.Fatalf("running Transition(starting): %v", err)
	}
	if err := running.Transition(session.StateRunning); err != nil {
		t.Fatalf("running Transition(running): %v", err)
	}
	running.RecordHeartbeat()

	completed := session.New("completed-1", session.MemberCodex, "ship patch", "/tmp/completed")
	if err := completed.Transition(session.StateStarting); err != nil {
		t.Fatalf("completed Transition(starting): %v", err)
	}
	if err := completed.Transition(session.StateRunning); err != nil {
		t.Fatalf("completed Transition(running): %v", err)
	}
	completed.SetResult("patched")
	if err := completed.Transition(session.StateCompleted); err != nil {
		t.Fatalf("completed Transition(completed): %v", err)
	}

	for _, sess := range []*session.Session{running, completed} {
		if err := st.Save(sess); err != nil {
			t.Fatalf("Save(%s): %v", sess.ID, err)
		}
	}

	st.AppendEvent("running-1", "heartbeat", map[string]any{"step": "compile"})
	st.AppendEvent("completed-1", "completed", map[string]any{"answer": "patched"})

	recovered, err := store.New(dir)
	if err != nil {
		t.Fatalf("store.New(recovered): %v", err)
	}

	all, err := recovered.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("LoadAll returned %d sessions, want 2", len(all))
	}

	slices.SortFunc(all, func(a, b *session.Session) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})

	if all[0].ID != "completed-1" || all[0].State != session.StateCompleted || all[0].Answer != "patched" {
		t.Fatalf("completed snapshot = %+v", all[0].Snapshot())
	}
	if all[1].ID != "running-1" || all[1].State != session.StateRunning || !all[1].PoolPane {
		t.Fatalf("running snapshot = %+v", all[1].Snapshot())
	}
	if all[1].ParentSessionID != "leader-1" || all[1].PaneID != 51 || all[1].LastHeartbeat == nil {
		t.Fatalf("running snapshot missing metadata = %+v", all[1].Snapshot())
	}

	eventsData, err := os.ReadFile(filepath.Join(dir, "completed-1.events.jsonl"))
	if err != nil {
		t.Fatalf("ReadFile completed events: %v", err)
	}
	eventsText := string(eventsData)
	if !strings.Contains(eventsText, "\"session_id\":\"completed-1\"") {
		t.Fatalf("event log missing session id: %s", eventsText)
	}
	if !strings.Contains(eventsText, "\"type\":\"completed\"") {
		t.Fatalf("event log missing type: %s", eventsText)
	}
	if !strings.Contains(eventsText, "\"answer\":\"patched\"") {
		t.Fatalf("event log missing answer: %s", eventsText)
	}
}

func TestStore_LoadAllSkipsCorruptSessionFiles(t *testing.T) {
	dir := t.TempDir()
	st, err := store.New(dir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	good := session.New("good-1", session.MemberKimi, "summarize", "/tmp/good")
	if err := st.Save(good); err != nil {
		t.Fatalf("Save(good): %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile broken.json: %v", err)
	}

	all, err := st.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("LoadAll returned %d sessions, want 1", len(all))
	}
	if all[0].ID != "good-1" {
		t.Fatalf("LoadAll returned session %q, want good-1", all[0].ID)
	}
}
