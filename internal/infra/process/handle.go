package process

import "io"

// ProcessHandle is the lifecycle interface for any managed process.
type ProcessHandle interface {
	Name() string
	PID() int
	Done() <-chan struct{}
	Wait() error
	Stop() error
	StderrTail() string
	Alive() bool
}

// PipedHandle extends ProcessHandle with raw stdio pipes.
// Only available from ExecBackend (not TmuxBackend).
type PipedHandle interface {
	ProcessHandle
	Stdin() io.WriteCloser
	Stdout() io.ReadCloser // nil when Detached+OutputFile
}
