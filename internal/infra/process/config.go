package process

import "time"

// ProcessConfig describes how to spawn a managed process.
type ProcessConfig struct {
	// Name is a human-readable identifier used for listing and lookup.
	Name string

	Command    string
	Args       []string
	Env        map[string]string // merged via MergeEnv
	WorkingDir string
	Timeout    time.Duration // 0 = no timeout

	// Detached makes the process survive parent death (Setsid session leader).
	// Stdout is redirected to OutputFile; Stdout() on the handle returns nil.
	Detached   bool
	OutputFile string // required when Detached is true
	StatusFile string // optional; JSON {"pid":N,"started_at":"..."}
}
