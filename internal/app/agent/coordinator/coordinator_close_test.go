package coordinator

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
)

type stubAttachmentPersister struct {
	closed bool
}

func (s *stubAttachmentPersister) Persist(ctx context.Context, att ports.Attachment) (ports.Attachment, error) {
	return att, nil
}

func (s *stubAttachmentPersister) Close() error {
	s.closed = true
	return nil
}

func TestAgentCoordinatorCloseClosesPersister(t *testing.T) {
	coordinator := &AgentCoordinator{}
	persister := &stubAttachmentPersister{}
	coordinator.SetAttachmentPersister(persister)

	if err := coordinator.Close(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !persister.closed {
		t.Fatalf("expected persister to be closed")
	}
}
