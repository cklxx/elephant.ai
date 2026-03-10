package async

import "runtime/debug"

// PanicLogger captures panic reports from background goroutines.
type PanicLogger interface {
	Error(format string, args ...any)
}

// Run executes fn guarded by panic recovery.
func Run(logger PanicLogger, name string, fn func()) {
	defer recoverAndLog(logger, name)
	fn()
}

// Go runs fn in a goroutine guarded by panic recovery.
func Go(logger PanicLogger, name string, fn func()) {
	go Run(logger, name, fn)
}

func recoverAndLog(logger PanicLogger, name string) {
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
