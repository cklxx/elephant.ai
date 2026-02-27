package process

import (
	"syscall"
	"time"
)

// ShutdownPolicy controls graceful shutdown behavior.
type ShutdownPolicy struct {
	// Grace is the SIGTERM→SIGKILL grace period. Default 5s.
	Grace time.Duration
	// PollInterval is how often to check liveness during grace. Default 250ms.
	PollInterval time.Duration
	// Signal is the initial signal. Default SIGTERM.
	Signal syscall.Signal
	// UseProcessGroup sends signals to -pid (the process group) when true.
	UseProcessGroup bool
}

func (p ShutdownPolicy) grace() time.Duration {
	if p.Grace <= 0 {
		return 5 * time.Second
	}
	return p.Grace
}

func (p ShutdownPolicy) pollInterval() time.Duration {
	if p.PollInterval <= 0 {
		return 250 * time.Millisecond
	}
	return p.PollInterval
}

func (p ShutdownPolicy) signal() syscall.Signal {
	if p.Signal == 0 {
		return syscall.SIGTERM
	}
	return p.Signal
}

// GracefulStop sends the configured signal (default SIGTERM), polls for exit
// over the grace period, and escalates to SIGKILL if the process survives.
//
// target is the PID (or PGID when UseProcessGroup is true — caller passes the
// positive value; this function negates it).
//
// done, if non-nil, is selected alongside the poll loop: if the channel closes
// before the grace period expires the function returns immediately without
// sending SIGKILL.
func GracefulStop(target int, done <-chan struct{}, policy ShutdownPolicy) {
	if target <= 0 {
		return
	}

	sig := policy.signal()
	killTarget := target
	if policy.UseProcessGroup {
		killTarget = -target
	}

	_ = syscall.Kill(killTarget, sig)

	grace := policy.grace()
	poll := policy.pollInterval()
	deadline := time.Now().Add(grace)

	for time.Now().Before(deadline) {
		if done != nil {
			select {
			case <-done:
				return
			default:
			}
		}
		if !IsAlive(target) {
			return
		}
		time.Sleep(poll)
	}

	// Escalate.
	_ = syscall.Kill(killTarget, syscall.SIGKILL)
}
