package signals

import (
	"context"
	"sync"
)

// SignalHandler processes scored and routed signals.
type SignalHandler interface {
	HandleSignal(ctx context.Context, event SignalEvent)
}

// Graph orchestrates signal ingestion, scoring, and routing.
type Graph struct {
	sources  []Source
	scorer   *Scorer
	router   *Router
	buffer   *RingBuffer
	handlers []SignalHandler
	sink     chan SignalEvent
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewGraph creates a Graph with the given dependencies.
func NewGraph(
	sources []Source,
	scorer *Scorer,
	router *Router,
	bufferSize int,
	handlers []SignalHandler,
) *Graph {
	if bufferSize <= 0 {
		bufferSize = 500
	}
	return &Graph{
		sources:  sources,
		scorer:   scorer,
		router:   router,
		buffer:   NewRingBuffer(bufferSize),
		handlers: handlers,
		sink:     make(chan SignalEvent, bufferSize),
	}
}

// Start begins all sources and the processing loop.
func (g *Graph) Start(ctx context.Context) error {
	ctx, g.cancel = context.WithCancel(ctx)
	for _, src := range g.sources {
		if err := src.Start(ctx, g.sink); err != nil {
			g.Stop()
			return err
		}
	}
	g.wg.Add(1)
	go g.processLoop(ctx)
	return nil
}

// Stop gracefully shuts down all sources and the processing loop.
func (g *Graph) Stop() {
	for _, src := range g.sources {
		src.Stop()
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.wg.Wait()
}

// Ingest manually injects a signal for processing.
func (g *Graph) Ingest(event SignalEvent) {
	select {
	case g.sink <- event:
	default:
		g.buffer.Push(event)
	}
}

// RecentSignals returns a snapshot of recently buffered signals.
func (g *Graph) RecentSignals() []SignalEvent {
	return g.buffer.Snapshot()
}

func (g *Graph) processLoop(ctx context.Context) {
	defer g.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-g.sink:
			g.processEvent(ctx, event)
		}
	}
}

func (g *Graph) processEvent(ctx context.Context, event SignalEvent) {
	g.scorer.Score(ctx, &event)
	event.Route = g.router.Route(ctx, &event)
	g.buffer.Push(event)
	for _, h := range g.handlers {
		h.HandleSignal(ctx, event)
	}
}
