package channels

import "context"

// PluginFactory builds a channel gateway from bootstrap dependencies.
// The Build closure captures its channel-specific config and dependencies.
type PluginFactory struct {
	Name     string
	Required bool // true = failure to start is fatal
	Build    func(ctx context.Context) (cleanup func(), err error)
}
