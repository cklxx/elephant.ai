package tape

// TapeContext holds the current tape execution context.
type TapeContext struct {
	TapeName string
	RunID    string
	Meta     EntryMeta
}
