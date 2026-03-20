package tool

// ToolSet is a frozen, immutable collection of tools.
type ToolSet struct {
	tools map[string]Tool
	order []string // preserves insertion order
}

// NewToolSet creates a frozen ToolSet from the given tools.
// If duplicate names are provided, the last one wins and the
// insertion order reflects the first occurrence.
func NewToolSet(tools ...Tool) *ToolSet {
	s := &ToolSet{
		tools: make(map[string]Tool, len(tools)),
	}
	seen := make(map[string]bool, len(tools))
	for _, t := range tools {
		if !seen[t.Name] {
			s.order = append(s.order, t.Name)
			seen[t.Name] = true
		}
		s.tools[t.Name] = t
	}
	return s
}

// Schemas returns all tool definitions in insertion order for LLM prompt building.
func (s *ToolSet) Schemas() []Tool {
	out := make([]Tool, 0, len(s.order))
	for _, name := range s.order {
		out = append(out, s.tools[name])
	}
	return out
}

// Get retrieves a tool by name. Returns the tool and whether it was found.
func (s *ToolSet) Get(name string) (Tool, bool) {
	t, ok := s.tools[name]
	return t, ok
}

// Names returns the tool names in insertion order.
func (s *ToolSet) Names() []string {
	out := make([]string, len(s.order))
	copy(out, s.order)
	return out
}

// Merge creates a new ToolSet by combining this set with another.
// Tools in other override same-named tools in this set.
func (s *ToolSet) Merge(other *ToolSet) *ToolSet {
	merged := make([]Tool, 0, len(s.order)+len(other.order))
	for _, name := range s.order {
		if t, ok := other.tools[name]; ok {
			merged = append(merged, t)
		} else {
			merged = append(merged, s.tools[name])
		}
	}
	// Append tools from other that are not in this set.
	for _, name := range other.order {
		if _, ok := s.tools[name]; !ok {
			merged = append(merged, other.tools[name])
		}
	}
	return NewToolSet(merged...)
}

// Len returns the number of tools.
func (s *ToolSet) Len() int {
	return len(s.order)
}
