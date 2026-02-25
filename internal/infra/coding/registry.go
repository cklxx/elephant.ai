package coding

import "fmt"

// AdapterRegistry stores available coding adapters.
type AdapterRegistry struct {
	adapters map[string]Adapter
}

// NewAdapterRegistry constructs a registry.
func NewAdapterRegistry() *AdapterRegistry {
	return &AdapterRegistry{adapters: make(map[string]Adapter)}
}

// Register adds an adapter.
func (r *AdapterRegistry) Register(adapter Adapter) error {
	if r == nil {
		return fmt.Errorf("adapter registry is nil")
	}
	if adapter == nil {
		return fmt.Errorf("adapter is nil")
	}
	name := adapter.Name()
	if name == "" {
		return fmt.Errorf("adapter name is required")
	}
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("adapter already exists: %s", name)
	}
	r.adapters[name] = adapter
	return nil
}

// Get returns an adapter by name.
func (r *AdapterRegistry) Get(name string) (Adapter, error) {
	if r == nil {
		return nil, fmt.Errorf("adapter registry is nil")
	}
	if adapter, ok := r.adapters[name]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("adapter not found: %s", name)
}

// List returns all adapters.
func (r *AdapterRegistry) List() []Adapter {
	if r == nil {
		return nil
	}
	items := make([]Adapter, 0, len(r.adapters))
	for _, adapter := range r.adapters {
		items = append(items, adapter)
	}
	return items
}

// Count returns number of registered adapters.
func (r *AdapterRegistry) Count() int {
	if r == nil {
		return 0
	}
	return len(r.adapters)
}
