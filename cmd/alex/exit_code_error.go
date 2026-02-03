package main

// ExitCodeError wraps an error with a specific process exit code.
//
// This is intentionally small and local to the CLI: most commands still return
// plain errors and exit with code 1. We only use ExitCodeError where scripts
// need stable non-1 codes (e.g., scenario test failures vs. usage errors).
type ExitCodeError struct {
	Code int
	Err  error
}

func (e *ExitCodeError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitCodeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
