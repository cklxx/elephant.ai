package ports

// CloneImportantNotes deep copies a map of important notes so callers can
// mutate slices safely.
func CloneImportantNotes(notes map[string]ImportantNote) map[string]ImportantNote {
	if len(notes) == 0 {
		return nil
	}
	cloned := make(map[string]ImportantNote, len(notes))
	for key, note := range notes {
		cloned[key] = note
	}
	return cloned
}
