package tape

import (
	"context"

	coretape "alex/internal/core/tape"
	"alex/internal/domain/agent/ports"
)

// MessageReader reconstructs ports.Message slices from tape entries.
type MessageReader struct {
	store coretape.TapeStore
}

// NewMessageReader returns a MessageReader backed by the given TapeStore.
func NewMessageReader(store coretape.TapeStore) *MessageReader {
	return &MessageReader{store: store}
}

// ReadMessagesAfterLabel returns messages recorded after the last anchor or
// compression entry with the given label. If no matching label is found, all
// messages are returned.
func (r *MessageReader) ReadMessagesAfterLabel(ctx context.Context, sessionID, label string) ([]ports.Message, error) {
	entries, err := r.store.Query(ctx, sessionID,
		coretape.Query().AfterLabel(label).Kinds(coretape.KindMessage))
	if err != nil {
		return nil, err
	}
	return entriesToMessages(entries)
}

// ReadAllMessages returns every message entry in the session tape.
func (r *MessageReader) ReadAllMessages(ctx context.Context, sessionID string) ([]ports.Message, error) {
	entries, err := r.store.Query(ctx, sessionID,
		coretape.Query().Kinds(coretape.KindMessage))
	if err != nil {
		return nil, err
	}
	return entriesToMessages(entries)
}

func entriesToMessages(entries []coretape.TapeEntry) ([]ports.Message, error) {
	msgs := make([]ports.Message, 0, len(entries))
	for _, e := range entries {
		msg, err := entryToMessage(e)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, nil
}
