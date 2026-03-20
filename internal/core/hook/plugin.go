package hook

// Plugin is the base interface all plugins must implement.
type Plugin interface {
	Name() string
	Priority() int
}
