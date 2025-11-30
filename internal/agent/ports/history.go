package ports

import "context"

// HistoryManager persists and replays session turns without reordering.
type HistoryManager interface {
	// AppendTurn persists the new turn derived from the provided conversation
	// messages. Implementations should detect the existing prefix and only
	// store the newly added messages for this turn.
	AppendTurn(ctx context.Context, sessionID string, messages []Message) error

	// Replay returns the concatenated messages across previously recorded
	// turns. If uptoTurn is greater than zero, only turns with IDs less than
	// uptoTurn are included; otherwise all turns are returned.
	Replay(ctx context.Context, sessionID string, uptoTurn int) ([]Message, error)
}
