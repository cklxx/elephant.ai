package agent_eval

import "log"

type panicLogger struct{}

func (panicLogger) Error(format string, args ...any) {
	log.Printf(format, args...)
}
