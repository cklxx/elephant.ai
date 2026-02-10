package react

import "strings"

// stewardDeltaFilter strips <NEW_STATE>...</NEW_STATE> blocks from a stream
// of text chunks. Content inside the tags is silently dropped; content outside
// is returned immediately (modulo a small holdback buffer for partial-tag
// detection at chunk boundaries).
//
// Usage:
//
//	f := &stewardDeltaFilter{}
//	for each chunk { emit(f.Write(chunk)) }
//	emit(f.Flush()) // at stream end
type stewardDeltaFilter struct {
	holdback   strings.Builder
	suppressed bool
}

const (
	deltaFilterOpenTag  = stewardStateOpenTag  // "<NEW_STATE>"
	deltaFilterCloseTag = stewardStateCloseTag // "</NEW_STATE>"
)

// Write processes a chunk and returns the portion safe to emit.
// Content inside <NEW_STATE>...</NEW_STATE> is silently dropped.
func (f *stewardDeltaFilter) Write(chunk string) string {
	// Prepend any previous holdback.
	if f.holdback.Len() > 0 {
		prev := f.holdback.String()
		f.holdback.Reset()
		chunk = prev + chunk
	}

	var out strings.Builder
	f.process(chunk, &out)
	return out.String()
}

// Flush returns any remaining holdback content (call at stream end).
// If currently inside a suppressed block, holdback is discarded.
func (f *stewardDeltaFilter) Flush() string {
	if f.suppressed {
		f.holdback.Reset()
		return ""
	}
	s := f.holdback.String()
	f.holdback.Reset()
	return s
}

// process is the core state-machine loop. It appends safe-to-emit content to
// out and stores any trailing ambiguous bytes in f.holdback.
func (f *stewardDeltaFilter) process(data string, out *strings.Builder) {
	for len(data) > 0 {
		if f.suppressed {
			data = f.processSuppressed(data, out)
		} else {
			data = f.processNormal(data, out)
		}
	}
}

// processNormal handles data while outside a <NEW_STATE> block.
// It returns the unprocessed remainder (empty when done).
func (f *stewardDeltaFilter) processNormal(data string, out *strings.Builder) string {
	idx := strings.Index(data, deltaFilterOpenTag)
	if idx >= 0 {
		// Emit everything before the open tag.
		out.WriteString(data[:idx])
		f.suppressed = true
		return data[idx+len(deltaFilterOpenTag):]
	}

	// No full open tag found. Check whether the tail could be a partial prefix
	// of "<NEW_STATE>". We need to hold back up to len(openTag)-1 chars.
	holdbackLen := len(deltaFilterOpenTag) - 1
	if holdbackLen > len(data) {
		holdbackLen = len(data)
	}

	// Find the longest suffix of data that matches a prefix of the open tag.
	safe := len(data)
	for n := holdbackLen; n >= 1; n-- {
		suffix := data[len(data)-n:]
		if strings.HasPrefix(deltaFilterOpenTag, suffix) {
			safe = len(data) - n
			break
		}
	}

	out.WriteString(data[:safe])
	if safe < len(data) {
		f.holdback.WriteString(data[safe:])
	}
	return ""
}

// processSuppressed handles data while inside a <NEW_STATE>...</NEW_STATE> block.
// All content is dropped until the close tag is found.
func (f *stewardDeltaFilter) processSuppressed(data string, out *strings.Builder) string {
	idx := strings.Index(data, deltaFilterCloseTag)
	if idx >= 0 {
		f.suppressed = false
		return data[idx+len(deltaFilterCloseTag):]
	}

	// No full close tag found. Hold back a potential partial close-tag suffix.
	holdbackLen := len(deltaFilterCloseTag) - 1
	if holdbackLen > len(data) {
		holdbackLen = len(data)
	}

	for n := holdbackLen; n >= 1; n-- {
		suffix := data[len(data)-n:]
		if strings.HasPrefix(deltaFilterCloseTag, suffix) {
			// Hold back the partial match in case the next chunk completes it.
			f.holdback.WriteString(suffix)
			return ""
		}
	}
	// No partial match — everything can be dropped.
	return ""
}
