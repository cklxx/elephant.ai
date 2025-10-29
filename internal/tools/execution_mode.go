package tools

import "fmt"

// ExecutionMode determines how builtin tools perform file and shell operations.
type ExecutionMode int

const (
	// ExecutionModeUnknown guards against misconfiguration.
	ExecutionModeUnknown ExecutionMode = iota
	// ExecutionModeLocal performs operations directly on the host machine.
	ExecutionModeLocal
	// ExecutionModeSandbox routes operations through the remote sandbox APIs.
	ExecutionModeSandbox
)

// String implements fmt.Stringer for logging/debugging.
func (m ExecutionMode) String() string {
	switch m {
	case ExecutionModeLocal:
		return "local"
	case ExecutionModeSandbox:
		return "sandbox"
	default:
		return "unknown"
	}
}

// Validate ensures the mode is explicitly configured to either local or sandbox.
func (m ExecutionMode) Validate() error {
	if m != ExecutionModeLocal && m != ExecutionModeSandbox {
		return fmt.Errorf("invalid execution mode: %d", m)
	}
	return nil
}
