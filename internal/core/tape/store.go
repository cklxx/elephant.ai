package tape

import "context"

// TapeStore is the persistence interface for tape entries.
type TapeStore interface {
	// Append adds an entry to the named tape.
	Append(ctx context.Context, tapeName string, entry TapeEntry) error
	// Query returns entries from the named tape matching the query.
	Query(ctx context.Context, tapeName string, q TapeQuery) ([]TapeEntry, error)
	// List returns all known tape names.
	List(ctx context.Context) ([]string, error)
	// Delete removes a tape and all its entries.
	Delete(ctx context.Context, tapeName string) error
}
