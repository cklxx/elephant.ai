package signals

import "context"

// Source produces SignalEvents from an external system.
type Source interface {
	Name() SignalSource
	Start(ctx context.Context, sink chan<- SignalEvent) error
	Stop()
}

// LarkSource normalizes Lark WebSocket messages into SignalEvents.
type LarkSource struct {
	chatIDs []string
	cancel  context.CancelFunc
}

// NewLarkSource creates a LarkSource monitoring the given chat IDs.
func NewLarkSource(chatIDs []string) *LarkSource {
	return &LarkSource{chatIDs: chatIDs}
}

func (s *LarkSource) Name() SignalSource { return SourceLark }

// Start begins listening for Lark events and pushes them to sink.
func (s *LarkSource) Start(ctx context.Context, sink chan<- SignalEvent) error {
	ctx, s.cancel = context.WithCancel(ctx)
	go s.loop(ctx, sink)
	return nil
}

// Stop terminates the Lark source listener.
func (s *LarkSource) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *LarkSource) loop(ctx context.Context, _ chan<- SignalEvent) {
	// Placeholder: integrate with Lark WebSocket event stream.
	<-ctx.Done()
}
