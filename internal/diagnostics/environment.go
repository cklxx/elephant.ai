package diagnostics

import (
	"sync"
	"time"
)

type EnvironmentPayload struct {
	Host     map[string]string
	Sandbox  map[string]string
	Captured time.Time
}

type EnvironmentListener func(EnvironmentPayload)

var (
	envMu       sync.RWMutex
	latestEnv   EnvironmentPayload
	envPresent  bool
	listeners   = map[int]EnvironmentListener{}
	listenerSeq int
)

// PublishEnvironments stores the latest environment payload and notifies subscribers.
func PublishEnvironments(payload EnvironmentPayload) {
	sanitized := sanitizeEnvironmentPayload(payload)
	clone := clonePayload(sanitized)

	envMu.Lock()
	latestEnv = clone
	envPresent = true
	callbacks := make([]EnvironmentListener, 0, len(listeners))
	for _, listener := range listeners {
		callbacks = append(callbacks, listener)
	}
	envMu.Unlock()

	for _, listener := range callbacks {
		if listener != nil {
			listener(clone)
		}
	}
}

// SubscribeEnvironments registers a listener for environment updates.
// It returns a function that must be called to unsubscribe the listener.
func SubscribeEnvironments(listener EnvironmentListener) func() {
	envMu.Lock()
	defer envMu.Unlock()

	listenerSeq++
	id := listenerSeq
	listeners[id] = listener

	return func() {
		envMu.Lock()
		defer envMu.Unlock()
		delete(listeners, id)
	}
}

// LatestEnvironments returns the most recently published payload if one exists.
func LatestEnvironments() (EnvironmentPayload, bool) {
	envMu.RLock()
	defer envMu.RUnlock()

	if !envPresent {
		return EnvironmentPayload{}, false
	}
	return clonePayload(latestEnv), true
}

func clonePayload(payload EnvironmentPayload) EnvironmentPayload {
	return EnvironmentPayload{
		Host:     cloneMap(payload.Host),
		Sandbox:  cloneMap(payload.Sandbox),
		Captured: payload.Captured,
	}
}

func sanitizeEnvironmentPayload(payload EnvironmentPayload) EnvironmentPayload {
	return EnvironmentPayload{
		Host:     cloneMap(payload.Host),
		Sandbox:  cloneMap(payload.Sandbox),
		Captured: payload.Captured,
	}
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	clone := make(map[string]string, len(values))
	for k, v := range values {
		clone[k] = v
	}
	return clone
}
