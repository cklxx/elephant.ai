package digest

import "context"

// DigestSpec defines what data to gather and how to format it.
type DigestSpec interface {
	Name() string
	Generate(ctx context.Context) (*Content, error)
	Format(content *Content) string
}

// Content holds the generated digest data.
type Content struct {
	Title    string
	Sections []Section
	Metadata map[string]string
}

// Section is a named block of content within a digest.
type Section struct {
	Heading string
	Body    string
	Items   []Item
}

// Item is a single line item within a section.
type Item struct {
	Label  string
	Value  string
	Status string // "ok", "warning", "action_needed"
}
