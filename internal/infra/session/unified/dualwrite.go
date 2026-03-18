package unified

import (
	"context"
	"fmt"
	"log/slog"

	storage "alex/internal/domain/agent/ports/storage"
)

// ParityIssue describes a discrepancy between primary and secondary stores.
type ParityIssue struct {
	SessionID    string
	Field        string
	PrimaryVal   string
	SecondaryVal string
}

// DualWriteStore wraps two SessionStores for migration.
// Writes go to both. Reads come from primary (new).
// If primary read fails, falls back to secondary (old).
type DualWriteStore struct {
	primary   storage.SessionStore
	secondary storage.SessionStore
	logger    *slog.Logger
}

// NewDualWrite creates a dual-write migration store.
func NewDualWrite(primary, secondary storage.SessionStore, logger *slog.Logger) *DualWriteStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &DualWriteStore{primary: primary, secondary: secondary, logger: logger}
}

func (d *DualWriteStore) Create(ctx context.Context) (*storage.Session, error) {
	session, err := d.primary.Create(ctx)
	if err != nil {
		return nil, err
	}
	if saveErr := d.secondary.Save(ctx, session); saveErr != nil {
		d.logger.Warn("dual-write: secondary create failed", "id", session.ID, "err", saveErr)
	}
	return session, nil
}

func (d *DualWriteStore) Get(ctx context.Context, id string) (*storage.Session, error) {
	session, err := d.primary.Get(ctx, id)
	if err == nil {
		return session, nil
	}
	d.logger.Warn("dual-write: primary get failed, falling back", "id", id, "err", err)
	return d.secondary.Get(ctx, id)
}

func (d *DualWriteStore) Save(ctx context.Context, session *storage.Session) error {
	if err := d.primary.Save(ctx, session); err != nil {
		return err
	}
	if err := d.secondary.Save(ctx, session); err != nil {
		d.logger.Warn("dual-write: secondary save failed", "id", session.ID, "err", err)
	}
	return nil
}

func (d *DualWriteStore) List(ctx context.Context, limit int, offset int) ([]string, error) {
	return d.primary.List(ctx, limit, offset)
}

func (d *DualWriteStore) Delete(ctx context.Context, id string) error {
	primaryErr := d.primary.Delete(ctx, id)
	if secondaryErr := d.secondary.Delete(ctx, id); secondaryErr != nil {
		d.logger.Warn("dual-write: secondary delete failed", "id", id, "err", secondaryErr)
	}
	return primaryErr
}

// ValidateParity checks that both stores agree on session data.
func (d *DualWriteStore) ValidateParity(ctx context.Context) ([]ParityIssue, error) {
	ids, err := d.primary.List(ctx, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("list primary sessions: %w", err)
	}

	var issues []ParityIssue
	for _, id := range ids {
		found := d.compareSession(ctx, id)
		issues = append(issues, found...)
	}
	return issues, nil
}

func (d *DualWriteStore) compareSession(ctx context.Context, id string) []ParityIssue {
	pSess, pErr := d.primary.Get(ctx, id)
	sSess, sErr := d.secondary.Get(ctx, id)

	if pErr != nil || sErr != nil {
		return []ParityIssue{{
			SessionID:    id,
			Field:        "existence",
			PrimaryVal:   errToString(pErr),
			SecondaryVal: errToString(sErr),
		}}
	}
	return diffSessions(id, pSess, sSess)
}

func diffSessions(id string, p, s *storage.Session) []ParityIssue {
	var issues []ParityIssue
	if len(p.Messages) != len(s.Messages) {
		issues = append(issues, ParityIssue{
			SessionID:    id,
			Field:        "messages_count",
			PrimaryVal:   fmt.Sprintf("%d", len(p.Messages)),
			SecondaryVal: fmt.Sprintf("%d", len(s.Messages)),
		})
	}
	if len(p.Metadata) != len(s.Metadata) {
		issues = append(issues, ParityIssue{
			SessionID:    id,
			Field:        "metadata_count",
			PrimaryVal:   fmt.Sprintf("%d", len(p.Metadata)),
			SecondaryVal: fmt.Sprintf("%d", len(s.Metadata)),
		})
	}
	return issues
}

func errToString(err error) string {
	if err == nil {
		return "ok"
	}
	return err.Error()
}
