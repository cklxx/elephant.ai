package tape

import "time"

// TapeQuery is an immutable builder for querying tape entries.
// Each setter method returns a copy with the field set.
type TapeQuery struct {
	afterAnchor string
	kinds       []EntryKind
	fromDate    time.Time
	toDate      time.Time
	limit       int
	sessionID   string
	runID       string
	beforeSeq   int64
	afterSeq    int64
}

// Query returns a new empty TapeQuery builder.
func Query() TapeQuery { return TapeQuery{} }

// AfterAnchor filters entries after the given anchor ID.
func (q TapeQuery) AfterAnchor(id string) TapeQuery {
	q.afterAnchor = id
	return q
}

// Kinds filters entries to the specified kinds.
func (q TapeQuery) Kinds(kinds ...EntryKind) TapeQuery {
	cp := make([]EntryKind, len(kinds))
	copy(cp, kinds)
	q.kinds = cp
	return q
}

// BetweenDates filters entries within the given time range.
func (q TapeQuery) BetweenDates(from, to time.Time) TapeQuery {
	q.fromDate = from
	q.toDate = to
	return q
}

// Limit caps the number of returned entries.
func (q TapeQuery) Limit(n int) TapeQuery {
	q.limit = n
	return q
}

// SessionID filters entries by session ID.
func (q TapeQuery) SessionID(id string) TapeQuery {
	q.sessionID = id
	return q
}

// RunID filters entries by run ID.
func (q TapeQuery) RunID(id string) TapeQuery {
	q.runID = id
	return q
}

// BeforeSeq filters entries with sequence number before the given value.
func (q TapeQuery) BeforeSeq(seq int64) TapeQuery {
	q.beforeSeq = seq
	return q
}

// AfterSeq filters entries with sequence number after the given value.
func (q TapeQuery) AfterSeq(seq int64) TapeQuery {
	q.afterSeq = seq
	return q
}

// GetAfterAnchor returns the after-anchor filter value.
func (q TapeQuery) GetAfterAnchor() string { return q.afterAnchor }

// GetKinds returns the kinds filter.
func (q TapeQuery) GetKinds() []EntryKind { return q.kinds }

// GetFromDate returns the from-date filter.
func (q TapeQuery) GetFromDate() time.Time { return q.fromDate }

// GetToDate returns the to-date filter.
func (q TapeQuery) GetToDate() time.Time { return q.toDate }

// GetLimit returns the limit.
func (q TapeQuery) GetLimit() int { return q.limit }

// GetSessionID returns the session ID filter.
func (q TapeQuery) GetSessionID() string { return q.sessionID }

// GetRunID returns the run ID filter.
func (q TapeQuery) GetRunID() string { return q.runID }

// GetBeforeSeq returns the before-seq filter.
func (q TapeQuery) GetBeforeSeq() int64 { return q.beforeSeq }

// GetAfterSeq returns the after-seq filter.
func (q TapeQuery) GetAfterSeq() int64 { return q.afterSeq }
