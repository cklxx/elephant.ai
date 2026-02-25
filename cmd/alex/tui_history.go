package main

type inputHistory struct {
	entries []string
	index   int
	draft   string
}

func newInputHistory() *inputHistory {
	return &inputHistory{index: 0}
}

func (h *inputHistory) Add(entry string) {
	if entry == "" {
		return
	}
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == entry {
		h.index = len(h.entries)
		h.draft = ""
		return
	}
	h.entries = append(h.entries, entry)
	h.index = len(h.entries)
	h.draft = ""
}

func (h *inputHistory) Prev(current string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.index >= len(h.entries) {
		h.draft = current
		h.index = len(h.entries) - 1
		return h.entries[h.index], true
	}
	if h.index == 0 {
		return h.entries[0], true
	}
	h.index--
	return h.entries[h.index], true
}

func (h *inputHistory) Next(current string) (string, bool) {
	if len(h.entries) == 0 {
		return "", false
	}
	if h.index >= len(h.entries) {
		return current, false
	}
	if h.index < len(h.entries)-1 {
		h.index++
		return h.entries[h.index], true
	}
	h.index = len(h.entries)
	return h.draft, true
}
