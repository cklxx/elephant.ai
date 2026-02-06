package async

import "runtime/debug"

// PanicLogger captures panic reports from background goroutines.
type PanicLogger interface {
	Error(format string, args ...any)
}

// Go runs fn in a goroutine guarded by panic recovery.
func Go(logger PanicLogger, name string, fn func()) {
	go func() {
		defer Recover(logger, name)
		fn()
	}()
}

// Recover logs panic details without crashing the process.
func Recover(logger PanicLogger, name string) {
	if r := recover(); r != nil {
		if logger == nil {
			return
		}
		if name == "" {
			logger.Error("goroutine panic: %v, stack: %s", r, debug.Stack())
			return
		}
		logger.Error("goroutine panic [%s]: %v, stack: %s", name, r, debug.Stack())
	}
}
