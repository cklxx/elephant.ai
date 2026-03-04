package agent

import "os"

// EventAppender appends a line to an event log file.
type EventAppender interface {
	AppendLine(path string, line string)
}

// AtomicFileWriter writes files atomically using temp-file + rename.
type AtomicFileWriter interface {
	WriteFileAtomically(path string, data []byte, perm os.FileMode) error
}
