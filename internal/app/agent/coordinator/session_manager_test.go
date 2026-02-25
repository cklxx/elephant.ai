package coordinator

import (
	"context"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// --- cloneSessionForSave ---

func TestCloneSessionForSave_Nil(t *testing.T) {
	if got := cloneSessionForSave(nil); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestCloneSessionForSave_DeepCopiesMessages(t *testing.T) {
	session := &storage.Session{
		ID: "s1",
		Messages: []ports.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}
	cloned := cloneSessionForSave(session)
	if cloned.ID != "s1" {
		t.Fatalf("expected ID copied, got %q", cloned.ID)
	}
	if len(cloned.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(cloned.Messages))
	}
	// Mutate original — clone should not be affected
	session.Messages[0].Content = "mutated"
	if cloned.Messages[0].Content == "mutated" {
		t.Fatal("expected deep copy of messages")
	}
}

func TestCloneSessionForSave_DeepCopiesMetadata(t *testing.T) {
	session := &storage.Session{
		ID:       "s1",
		Metadata: map[string]string{"key": "value"},
	}
	cloned := cloneSessionForSave(session)
	session.Metadata["key"] = "mutated"
	if cloned.Metadata["key"] == "mutated" {
		t.Fatal("expected deep copy of metadata")
	}
}

func TestCloneSessionForSave_DeepCopiesTodos(t *testing.T) {
	session := &storage.Session{
		ID: "s1",
		Todos: []storage.Todo{
			{Description: "task1", Status: "pending"},
		},
	}
	cloned := cloneSessionForSave(session)
	if len(cloned.Todos) != 1 {
		t.Fatalf("expected 1 todo, got %d", len(cloned.Todos))
	}
	session.Todos[0].Description = "mutated"
	if cloned.Todos[0].Description == "mutated" {
		t.Fatal("expected deep copy of todos")
	}
}

func TestCloneSessionForSave_EmptyFieldsNil(t *testing.T) {
	session := &storage.Session{ID: "s1"}
	cloned := cloneSessionForSave(session)
	if cloned.Todos != nil {
		t.Fatal("expected nil todos for empty")
	}
	if cloned.Metadata != nil {
		t.Fatal("expected nil metadata for empty")
	}
}

// --- sanitizeAttachmentForPersistence ---

func TestSanitizeAttachmentForPersistence_ClearsDataWhenURI(t *testing.T) {
	att := ports.Attachment{
		Name: "file.pdf",
		Data: "base64data",
		URI:  "https://cdn.example.com/file.pdf",
	}
	got := sanitizeAttachmentForPersistence(att)
	if got.Data != "" {
		t.Fatalf("expected Data cleared, got %q", got.Data)
	}
	if got.URI != "https://cdn.example.com/file.pdf" {
		t.Fatalf("expected URI preserved, got %q", got.URI)
	}
}

func TestSanitizeAttachmentForPersistence_PreservesDataURI(t *testing.T) {
	att := ports.Attachment{
		Data: "inline_data",
		URI:  "data:image/png;base64,inline_data",
	}
	got := sanitizeAttachmentForPersistence(att)
	if got.Data != "inline_data" {
		t.Fatalf("expected Data preserved for data: URI, got %q", got.Data)
	}
}

func TestSanitizeAttachmentForPersistence_PreservesDataWhenNoURI(t *testing.T) {
	att := ports.Attachment{Data: "some_data"}
	got := sanitizeAttachmentForPersistence(att)
	if got.Data != "some_data" {
		t.Fatalf("expected Data preserved, got %q", got.Data)
	}
}

// --- sanitizeMessagesForPersistence ---

func TestSanitizeMessagesForPersistence_Empty(t *testing.T) {
	msgs, atts := sanitizeMessagesForPersistence(nil)
	if msgs != nil || atts != nil {
		t.Fatalf("expected nil/nil, got %v/%v", msgs, atts)
	}
}

func TestSanitizeMessagesForPersistence_FiltersUserHistory(t *testing.T) {
	messages := []ports.Message{
		{Role: "system", Content: "boot", Source: ports.MessageSourceSystemPrompt},
		{Role: "user", Content: "old", Source: ports.MessageSourceUserHistory},
		{Role: "user", Content: "current", Source: ports.MessageSourceUserInput},
	}
	sanitized, _ := sanitizeMessagesForPersistence(messages)
	if len(sanitized) != 2 {
		t.Fatalf("expected 2 messages (history filtered), got %d", len(sanitized))
	}
	for _, msg := range sanitized {
		if msg.Source == ports.MessageSourceUserHistory {
			t.Fatal("expected user history messages filtered out")
		}
	}
}

func TestSanitizeMessagesForPersistence_ExtractsAttachments(t *testing.T) {
	messages := []ports.Message{{
		Role: "tool",
		Attachments: map[string]ports.Attachment{
			"img.png": {Name: "img.png", URI: "https://cdn/img.png", Data: "base64"},
		},
	}}
	sanitized, atts := sanitizeMessagesForPersistence(messages)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 message, got %d", len(sanitized))
	}
	if sanitized[0].Attachments != nil {
		t.Fatal("expected message attachments stripped")
	}
	if len(atts) != 1 {
		t.Fatalf("expected 1 extracted attachment, got %d", len(atts))
	}
	if atts["img.png"].Data != "" {
		t.Fatal("expected attachment data sanitized (URI present)")
	}
}

func TestSanitizeMessagesForPersistence_SetsAttachmentName(t *testing.T) {
	messages := []ports.Message{{
		Role: "tool",
		Attachments: map[string]ports.Attachment{
			"key_name": {MediaType: "text/plain"},
		},
	}}
	_, atts := sanitizeMessagesForPersistence(messages)
	if atts["key_name"].Name != "key_name" {
		t.Fatalf("expected Name set from key, got %q", atts["key_name"].Name)
	}
}

func TestSanitizeMessagesForPersistence_SkipsEmptyKeys(t *testing.T) {
	messages := []ports.Message{{
		Role: "tool",
		Attachments: map[string]ports.Attachment{
			"": {Name: ""},
		},
	}}
	_, atts := sanitizeMessagesForPersistence(messages)
	if len(atts) != 0 {
		t.Fatalf("expected empty key skipped, got %d attachments", len(atts))
	}
}

func TestSanitizeMessagesForPersistence_AllHistoryReturnsNil(t *testing.T) {
	messages := []ports.Message{
		{Source: ports.MessageSourceUserHistory, Content: "old"},
	}
	msgs, atts := sanitizeMessagesForPersistence(messages)
	if msgs != nil || atts != nil {
		t.Fatalf("expected nil/nil when all filtered, got %v/%v", msgs, atts)
	}
}

// --- stripUserHistoryMessages ---

func TestStripUserHistoryMessages_Empty(t *testing.T) {
	if got := stripUserHistoryMessages(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestStripUserHistoryMessages_FiltersHistory(t *testing.T) {
	messages := []ports.Message{
		{Content: "keep", Source: ports.MessageSourceUserInput},
		{Content: "remove", Source: ports.MessageSourceUserHistory},
		{Content: "also keep", Source: ports.MessageSourceAssistantReply},
	}
	got := stripUserHistoryMessages(messages)
	if len(got) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(got))
	}
	for _, msg := range got {
		if msg.Source == ports.MessageSourceUserHistory {
			t.Fatal("expected history messages filtered")
		}
	}
}

func TestStripUserHistoryMessages_AllHistory(t *testing.T) {
	messages := []ports.Message{
		{Source: ports.MessageSourceUserHistory},
	}
	if got := stripUserHistoryMessages(messages); got != nil {
		t.Fatalf("expected nil when all filtered, got %v", got)
	}
}

// --- updateAwaitUserInputMetadata ---

func TestUpdateAwaitUserInputMetadata_NilSession(t *testing.T) {
	updateAwaitUserInputMetadata(nil, nil) // should not panic
}

func TestUpdateAwaitUserInputMetadata_ClearsOnNonAwait(t *testing.T) {
	session := &storage.Session{
		Metadata: map[string]string{
			"await_user_input":          "true",
			"await_user_input_question": "old question?",
		},
	}
	result := &agent.TaskResult{StopReason: "max_iterations"}
	updateAwaitUserInputMetadata(session, result)
	if _, ok := session.Metadata["await_user_input"]; ok {
		t.Fatal("expected await_user_input cleared")
	}
	if _, ok := session.Metadata["await_user_input_question"]; ok {
		t.Fatal("expected await_user_input_question cleared")
	}
}

func TestUpdateAwaitUserInputMetadata_ClearsOnNilResult(t *testing.T) {
	session := &storage.Session{
		Metadata: map[string]string{
			"await_user_input": "true",
		},
	}
	updateAwaitUserInputMetadata(session, nil)
	if _, ok := session.Metadata["await_user_input"]; ok {
		t.Fatal("expected await_user_input cleared for nil result")
	}
}

func TestUpdateAwaitUserInputMetadata_InitializesMetadata(t *testing.T) {
	session := &storage.Session{}
	result := &agent.TaskResult{StopReason: "completed"}
	updateAwaitUserInputMetadata(session, result)
	if session.Metadata == nil {
		t.Fatal("expected metadata initialized")
	}
}

// --- ResetSession ---

func TestResetSession_EmptyID(t *testing.T) {
	store := &ensureSessionStore{}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})
	err := coordinator.ResetSession(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty session ID")
	}
}

func TestResetSession_NotFound(t *testing.T) {
	store := &ensureSessionStore{}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})
	err := coordinator.ResetSession(context.Background(), "missing")
	if err != nil {
		t.Fatalf("expected no error for missing session, got %v", err)
	}
}

func TestResetSession_ClearsSession(t *testing.T) {
	fixedTime := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	store := &ensureSessionStore{
		sessions: map[string]*storage.Session{
			"s1": {
				ID:          "s1",
				Messages:    []ports.Message{{Content: "old"}},
				Metadata:    map[string]string{"key": "val"},
				Attachments: map[string]ports.Attachment{"a": {}},
				Todos:       []storage.Todo{{Description: "todo"}},
			},
		},
	}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{},
		WithClock(agent.ClockFunc(func() time.Time { return fixedTime })))

	err := coordinator.ResetSession(context.Background(), "s1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	session := store.sessions["s1"]
	if session.Messages != nil || session.Metadata != nil || session.Attachments != nil || session.Todos != nil {
		t.Fatalf("expected all fields cleared, got %+v", session)
	}
	if !session.UpdatedAt.Equal(fixedTime) {
		t.Fatalf("expected UpdatedAt set to clock time, got %v", session.UpdatedAt)
	}
}

// --- SaveSessionAfterExecution ---

func TestSaveSessionAfterExecution_BasicSave(t *testing.T) {
	fixedTime := time.Date(2026, 2, 17, 12, 0, 0, 0, time.UTC)
	store := &ensureSessionStore{
		sessions: map[string]*storage.Session{},
	}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{},
		WithClock(agent.ClockFunc(func() time.Time { return fixedTime })))

	session := &storage.Session{ID: "s1"}
	result := &agent.TaskResult{
		Messages:  []ports.Message{{Role: "user", Content: "hello"}},
		SessionID: "s1",
		RunID:     "run-1",
	}

	err := coordinator.SaveSessionAfterExecution(context.Background(), session, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if store.saveCalls != 1 {
		t.Fatalf("expected 1 save call, got %d", store.saveCalls)
	}
	if !session.UpdatedAt.Equal(fixedTime) {
		t.Fatalf("expected UpdatedAt set, got %v", session.UpdatedAt)
	}
	if session.Metadata["session_id"] != "s1" {
		t.Fatalf("expected session_id in metadata, got %v", session.Metadata)
	}
	if session.Metadata["last_task_id"] != "run-1" {
		t.Fatalf("expected last_task_id in metadata, got %v", session.Metadata)
	}
}

func TestSaveSessionAfterExecution_ClearsParentRunID(t *testing.T) {
	store := &ensureSessionStore{sessions: map[string]*storage.Session{}}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	session := &storage.Session{
		ID:       "s1",
		Metadata: map[string]string{"last_parent_task_id": "old-parent"},
	}
	result := &agent.TaskResult{} // no ParentRunID

	err := coordinator.SaveSessionAfterExecution(context.Background(), session, result)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := session.Metadata["last_parent_task_id"]; ok {
		t.Fatal("expected last_parent_task_id deleted when empty")
	}
}

// --- GetSession ---

func TestGetSession_EmptyIDCreates(t *testing.T) {
	store := &ensureSessionStore{createID: "new-session"}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	session, err := coordinator.GetSession(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "new-session" {
		t.Fatalf("expected created session, got %q", session.ID)
	}
}

func TestGetSession_ExistingReturns(t *testing.T) {
	store := &ensureSessionStore{
		sessions: map[string]*storage.Session{
			"existing": {ID: "existing"},
		},
	}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	session, err := coordinator.GetSession(context.Background(), "existing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ID != "existing" {
		t.Fatalf("expected existing session, got %q", session.ID)
	}
}

// --- ListSessions ---

func TestListSessions_DelegatesToStore(t *testing.T) {
	store := &ensureSessionStore{}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	ids, err := coordinator.ListSessions(context.Background(), 10, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ids != nil {
		t.Fatalf("expected nil from stub, got %v", ids)
	}
}
