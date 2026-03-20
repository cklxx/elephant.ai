package envelope

import (
	"fmt"
	"time"
)

// Envelope is a typed wrapper around map[string]any for inbound requests.
// It provides safe typed access to common fields.
type Envelope struct {
	fields map[string]any
}

// New creates an Envelope from a map.
func New(fields map[string]any) Envelope {
	if fields == nil {
		fields = make(map[string]any)
	}
	cp := make(map[string]any, len(fields))
	for k, v := range fields {
		cp[k] = v
	}
	return Envelope{fields: cp}
}

// FieldOf returns a typed field value. Returns zero value if missing or wrong type.
func FieldOf[T any](e Envelope, key string) T {
	v, ok := e.fields[key]
	if !ok {
		var zero T
		return zero
	}
	t, ok := v.(T)
	if !ok {
		var zero T
		return zero
	}
	return t
}

// ContentOf returns the "content" field as a string.
func (e Envelope) ContentOf() string {
	return FieldOf[string](e, "content")
}

// Normalize fills in default fields (e.g., timestamp if missing).
func (e Envelope) Normalize() Envelope {
	out := e.clone()
	if _, ok := out.fields["timestamp"]; !ok {
		out.fields["timestamp"] = time.Now().UTC().Format(time.RFC3339)
	}
	return out
}

// Get returns a field value and whether it exists.
func (e Envelope) Get(key string) (any, bool) {
	v, ok := e.fields[key]
	return v, ok
}

// Set returns a new Envelope with the given field set (immutable).
func (e Envelope) Set(key string, value any) Envelope {
	out := e.clone()
	out.fields[key] = value
	return out
}

// Fields returns a copy of the underlying map.
func (e Envelope) Fields() map[string]any {
	cp := make(map[string]any, len(e.fields))
	for k, v := range e.fields {
		cp[k] = v
	}
	return cp
}

// String returns the content or a debug representation.
func (e Envelope) String() string {
	if c := e.ContentOf(); c != "" {
		return c
	}
	return fmt.Sprintf("Envelope{%d fields}", len(e.fields))
}

func (e Envelope) clone() Envelope {
	cp := make(map[string]any, len(e.fields))
	for k, v := range e.fields {
		cp[k] = v
	}
	return Envelope{fields: cp}
}
