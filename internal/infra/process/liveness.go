package process

import "syscall"

// IsAlive checks whether a process with the given PID exists and is
// signalable by the current user. Uses kill(pid, 0) which is a
// no-op existence check.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}
