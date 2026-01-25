package react

import "alex/internal/agent/ports"

type attachmentMutations struct {
	replace map[string]ports.Attachment
	add     map[string]ports.Attachment
	update  map[string]ports.Attachment
	remove  []string
}

type attachmentCandidate struct {
	key        string
	attachment ports.Attachment
	iteration  int
	generated  bool
}
