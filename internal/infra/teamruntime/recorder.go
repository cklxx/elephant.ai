package teamruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// EventRecorder appends structured JSONL runtime events.
type EventRecorder struct {
	path string
	mu   sync.Mutex
}

func NewEventRecorder(path string) *EventRecorder {
	return &EventRecorder{path: path}
}

func (r *EventRecorder) Path() string {
	if r == nil {
		return ""
	}
	return r.path
}

func (r *EventRecorder) Record(eventType string, fields map[string]any) error {
	if r == nil || r.path == "" {
		return nil
	}

	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      eventType,
	}
	for k, v := range fields {
		payload[k] = v
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}
