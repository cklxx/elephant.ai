package attachments

import (
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

const (
	defaultPersistQueueSize = 128
	defaultPersistWorkers   = 2
	defaultPersistTimeout   = 2 * time.Second
)

type persistRequest struct {
	name      string
	mediaType string
	payload   []byte
}

// AsyncStorePersister persists attachments in the background to avoid blocking
// the ReAct loop while still returning stable URIs immediately.
type AsyncStorePersister struct {
	store   *Store
	queue   chan persistRequest
	closeCh chan struct{}
	logger  agent.Logger
	workers int
	wg      sync.WaitGroup
}

// AsyncStorePersisterOption customizes AsyncStorePersister behavior.
type AsyncStorePersisterOption func(*AsyncStorePersister)

// WithAsyncPersistQueueSize sets the in-memory queue size for background persistence.
func WithAsyncPersistQueueSize(size int) AsyncStorePersisterOption {
	return func(p *AsyncStorePersister) {
		if size > 0 {
			p.queue = make(chan persistRequest, size)
		}
	}
}

// WithAsyncPersistWorkers sets the worker count for background persistence.
func WithAsyncPersistWorkers(workers int) AsyncStorePersisterOption {
	return func(p *AsyncStorePersister) {
		if workers > 0 {
			p.workers = workers
		}
	}
}

// WithAsyncPersistLogger attaches a logger for background persistence failures.
func WithAsyncPersistLogger(logger agent.Logger) AsyncStorePersisterOption {
	return func(p *AsyncStorePersister) {
		p.logger = logger
	}
}

// NewAsyncStorePersister creates a background persister backed by the given Store.
func NewAsyncStorePersister(store *Store, opts ...AsyncStorePersisterOption) *AsyncStorePersister {
	p := &AsyncStorePersister{
		store:   store,
		queue:   make(chan persistRequest, defaultPersistQueueSize),
		closeCh: make(chan struct{}),
		workers: defaultPersistWorkers,
	}
	for _, opt := range opts {
		opt(p)
	}

	if p.queue == nil {
		p.queue = make(chan persistRequest, defaultPersistQueueSize)
	}

	if p.workers <= 0 {
		p.workers = defaultPersistWorkers
	}
	p.wg.Add(p.workers)
	for i := 0; i < p.workers; i++ {
		go p.run()
	}

	return p
}

// Close stops background workers and waits for them to exit.
func (p *AsyncStorePersister) Close() {
	if p == nil {
		return
	}
	close(p.closeCh)
	p.wg.Wait()
}

// Persist queues the attachment for background persistence and immediately
// returns a stable URI. If enqueueing fails, the original attachment is
// returned with an error so callers can degrade gracefully.
func (p *AsyncStorePersister) Persist(ctx context.Context, att ports.Attachment) (ports.Attachment, error) {
	if p == nil || p.store == nil {
		return att, nil
	}
	if ctx != nil && ctx.Err() != nil {
		return att, ctx.Err()
	}

	// Already has an external URI and no inline data → nothing to do.
	if att.Data == "" && !isDataURI(att.URI) && strings.TrimSpace(att.URI) != "" {
		if att.Fingerprint == "" {
			att.Fingerprint = fingerprintFromURI(att.URI)
		}
		return att, nil
	}

	original := att
	payload, mediaType := decodeAttachmentInline(att)
	if len(payload) == 0 {
		return att, nil
	}

	fingerprint := attachmentFingerprint(payload)
	if att.Fingerprint == "" {
		att.Fingerprint = fingerprint
	}

	uri := buildAttachmentURI(p.store, att.Name, mediaType, fingerprint)
	if uri != "" {
		att.URI = uri
	}
	if att.MediaType == "" {
		att.MediaType = mediaType
	}

	if shouldRetainInline(att.MediaType, len(payload)) {
		att.Data = base64.StdEncoding.EncodeToString(payload)
	} else {
		att.Data = ""
	}

	if err := p.enqueue(ctx, persistRequest{name: att.Name, mediaType: mediaType, payload: payload}); err != nil {
		if original.Fingerprint == "" {
			original.Fingerprint = att.Fingerprint
		}
		return original, err
	}

	return att, nil
}

func (p *AsyncStorePersister) enqueue(ctx context.Context, req persistRequest) error {
	if p.queue == nil {
		return errors.New("attachment persist queue unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultPersistTimeout)
		defer cancel()
	}
	select {
	case p.queue <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *AsyncStorePersister) run() {
	defer p.wg.Done()
	for {
		select {
		case <-p.closeCh:
			return
		case req := <-p.queue:
			if err := p.persist(req); err != nil && p.logger != nil {
				p.logger.Warn("attachment async persist failed: %v", err)
			}
		}
	}
}

func (p *AsyncStorePersister) persist(req persistRequest) error {
	if p == nil || p.store == nil {
		return errors.New("attachment store unavailable")
	}
	if len(req.payload) == 0 {
		return errors.New("attachment payload empty")
	}
	_, err := p.store.StoreBytes(req.name, req.mediaType, req.payload)
	return err
}

func buildAttachmentURI(store *Store, name, mediaType, fingerprint string) string {
	if store == nil || fingerprint == "" {
		return ""
	}
	filename := buildFilenameWithFingerprint(name, mediaType, fingerprint)
	switch store.provider {
	case ProviderCloudflare:
		key := objectKey(store.cloudKeyPrefix, filename)
		if uri := store.buildCloudURI(key); strings.TrimSpace(uri) != "" {
			return uri
		}
		return store.buildURI(filename)
	case ProviderLocal:
		return store.buildURI(filename)
	default:
		return store.buildURI(filename)
	}
}

func buildFilenameWithFingerprint(name, mediaType, fingerprint string) string {
	ext := sanitizeAttachmentExt(filepath.Ext(strings.TrimSpace(name)))
	if ext == "" {
		ext = extFromMediaType(mediaType)
	}
	return fingerprint + ext
}
