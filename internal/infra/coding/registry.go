package coding

import "fmt"

// adapterRegistry stores available coding adapters.
type adapterRegistry struct {
	adapters map[string]Adapter
}

// NewAdapterRegistry constructs a registry.
func NewAdapterRegistry() *adapterRegistry {
	return &adapterRegistry{adapters: make(map[string]Adapter)}
}

// Register adds an adapter.
func (r *adapterRegistry) Register(adapter Adapter) error {
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
func (r *adapterRegistry) Get(name string) (Adapter, error) {
	if r == nil {
		return nil, fmt.Errorf("adapter registry is nil")
	}
	if adapter, ok := r.adapters[name]; ok {
		return adapter, nil
	}
	return nil, fmt.Errorf("adapter not found: %s", name)
}

// List returns all adapters.
func (r *adapterRegistry) List() []Adapter {
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
func (r *adapterRegistry) Count() int {
	if r == nil {
		return 0
	}
	return len(r.adapters)
}
