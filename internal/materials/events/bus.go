package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/broker"
)

const defaultBuffer = 16

type Bus struct {
	mu       sync.RWMutex
	watchers map[string]map[uint64]*watchRegistration
	nextID   uint64
}

type watchRegistration struct {
	ch chan *materialapi.MaterialEvent
}

func NewBus() *Bus {
	return &Bus{watchers: make(map[string]map[uint64]*watchRegistration)}
}

func (b *Bus) Watch(ctx context.Context, requestID string) (<-chan *materialapi.MaterialEvent, error) {
	if requestID == "" {
		return nil, errors.New("material events: request id is required")
	}

	ch := make(chan *materialapi.MaterialEvent, defaultBuffer)
	id := atomic.AddUint64(&b.nextID, 1)

	b.mu.Lock()
	if _, ok := b.watchers[requestID]; !ok {
		b.watchers[requestID] = make(map[uint64]*watchRegistration)
	}
	b.watchers[requestID][id] = &watchRegistration{ch: ch}
	b.mu.Unlock()

	go func() {
		<-ctx.Done()
		b.removeWatcher(requestID, id)
	}()

	return ch, nil
}

func (b *Bus) PublishMaterial(ctx context.Context, material *materialapi.Material) error {
	if material == nil {
		return errors.New("material events: material is required")
	}
	var requestID string
	if material.Context != nil {
		requestID = material.Context.RequestID
	}
	if requestID == "" {
		return errors.New("material events: material missing request context")
	}
	event := &materialapi.MaterialEvent{RequestID: requestID, Material: material}
	b.dispatch(requestID, event)
	return nil
}

func (b *Bus) PublishTombstone(ctx context.Context, requestID, materialID string) error {
	if requestID == "" || materialID == "" {
		return errors.New("material events: request id and material id are required for tombstones")
	}
	event := &materialapi.MaterialEvent{RequestID: requestID, TombstoneID: materialID}
	b.dispatch(requestID, event)
	return nil
}

func (b *Bus) dispatch(requestID string, event *materialapi.MaterialEvent) {
	b.mu.RLock()
	watchers := b.watchers[requestID]
	copies := make([]*watchRegistration, 0, len(watchers))
	for _, reg := range watchers {
		copies = append(copies, reg)
	}
	b.mu.RUnlock()

	for _, reg := range copies {
		b.safeSend(reg, event)
	}
}

func (b *Bus) safeSend(reg *watchRegistration, event *materialapi.MaterialEvent) {
	defer func() {
		if recover() != nil {
			// The watcher channel was closed after we copied the registration. Ignore the event
			// and keep publishing to other watchers.
		}
	}()

	select {
	case reg.ch <- event:
	default:
	}
}

func (b *Bus) removeWatcher(requestID string, id uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	watchers := b.watchers[requestID]
	if watchers == nil {
		return
	}
	if reg, ok := watchers[id]; ok {
		delete(watchers, id)
		close(reg.ch)
	}
	if len(watchers) == 0 {
		delete(b.watchers, requestID)
	}
}

var _ broker.EventPublisher = (*Bus)(nil)
