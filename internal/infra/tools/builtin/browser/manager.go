package browser

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	sessionTTL     = 30 * time.Minute
	reaperInterval = 1 * time.Minute
)

// Manager manages a single shared Chrome process and multiplexes sessions as
// browser tabs. Call Close to terminate the Chrome process on shutdown.
type Manager struct {
	cfg         Config
	mu          sync.Mutex
	sessions    map[string]*session
	allocCtx    context.Context
	allocCancel context.CancelFunc
	stopReaper  context.CancelFunc
}

type session struct {
	ctx        context.Context
	cancel     context.CancelFunc
	lastUsed   time.Time
	lastX      float64
	lastY      float64
	hasLastPos bool
	mu         sync.Mutex
}

func NewManager(cfg Config) *Manager {
	m := &Manager{
		cfg:      cfg,
		sessions: make(map[string]*session),
	}
	reaperCtx, reaperCancel := context.WithCancel(context.Background())
	m.stopReaper = reaperCancel
	go m.reapLoop(reaperCtx)
	return m
}

func (m *Manager) Config() Config {
	if m == nil {
		return Config{}
	}
	return m.cfg
}

// ensureAllocator lazily starts the shared Chrome process. Must be called with m.mu held.
func (m *Manager) ensureAllocator() error {
	if m.allocCtx != nil && m.allocCtx.Err() == nil {
		return nil
	}
	// Previous allocator dead (Chrome crashed or first call) — recreate.
	if m.allocCancel != nil {
		m.allocCancel()
	}

	baseCtx := context.Background()

	if rawCDPURL := strings.TrimSpace(m.cfg.CDPURL); rawCDPURL != "" {
		cdpURL, err := resolveCDPURL(baseCtx, rawCDPURL)
		if err != nil {
			return fmt.Errorf("resolve cdp_url %q: %w", rawCDPURL, err)
		}
		m.allocCtx, m.allocCancel = chromedp.NewRemoteAllocator(baseCtx, cdpURL)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", m.cfg.Headless),
			chromedp.Flag("disable-gpu", m.cfg.Headless),
		)
		if path := strings.TrimSpace(m.cfg.ChromePath); path != "" {
			opts = append(opts, chromedp.ExecPath(path))
		}
		if dir := strings.TrimSpace(m.cfg.UserDataDir); dir != "" {
			userDataDir := filepath.Join(dir, "shared")
			if err := os.MkdirAll(userDataDir, 0o755); err == nil {
				opts = append(opts, chromedp.UserDataDir(userDataDir))
			}
		}
		m.allocCtx, m.allocCancel = chromedp.NewExecAllocator(baseCtx, opts...)
	}
	return nil
}

// resetAllocator tears down the current Chrome process so the next
// ensureAllocator call starts a fresh one. Must be called with m.mu held.
func (m *Manager) resetAllocator() {
	// Close all existing sessions — their tabs are dead.
	for id, s := range m.sessions {
		s.close()
		delete(m.sessions, id)
	}
	if m.allocCancel != nil {
		m.allocCancel()
		m.allocCancel = nil
		m.allocCtx = nil
	}
}

func (m *Manager) Session(sessionID string) (*session, error) {
	if m == nil {
		return nil, fmt.Errorf("browser manager is nil")
	}
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "default"
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.sessions[sessionID]; ok {
		if existing.ctx.Err() == nil && time.Since(existing.lastUsed) < sessionTTL {
			existing.lastUsed = time.Now()
			return existing, nil
		}
		existing.close()
		delete(m.sessions, sessionID)
	}

	sess, err := m.newTab()
	if err != nil {
		// Chrome may have crashed — reset and retry once.
		m.resetAllocator()
		sess, err = m.newTab()
		if err != nil {
			return nil, err
		}
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

// newTab creates a new browser tab in the shared Chrome. Must be called with m.mu held.
func (m *Manager) newTab() (*session, error) {
	if err := m.ensureAllocator(); err != nil {
		return nil, err
	}

	ctx, cancel := chromedp.NewContext(m.allocCtx)
	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		cancel()
		return nil, err
	}

	return &session{
		ctx:      ctx,
		cancel:   cancel,
		lastUsed: time.Now(),
	}, nil
}

// NewTemporaryContext creates a temporary browser tab from the shared Chrome
// process. The caller must call the returned cleanup function when done.
// This is intended for short-lived operations like diagram rendering.
func (m *Manager) NewTemporaryContext(parent context.Context) (context.Context, func(), error) {
	if m == nil {
		return nil, nil, fmt.Errorf("browser manager is nil")
	}

	m.mu.Lock()
	if err := m.ensureAllocator(); err != nil {
		m.mu.Unlock()
		return nil, nil, err
	}
	allocCtx := m.allocCtx
	m.mu.Unlock()

	chromeCtx, cancel := chromedp.NewContext(allocCtx)
	if parent != nil {
		go func() {
			select {
			case <-parent.Done():
				cancel()
			case <-chromeCtx.Done():
			}
		}()
	}
	return chromeCtx, cancel, nil
}

// Close terminates all sessions and the shared Chrome process.
func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopReaper != nil {
		m.stopReaper()
		m.stopReaper = nil
	}
	for id, s := range m.sessions {
		s.close()
		delete(m.sessions, id)
	}
	if m.allocCancel != nil {
		m.allocCancel()
		m.allocCancel = nil
		m.allocCtx = nil
	}
}

func (m *Manager) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(reaperInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.reapExpired()
		}
	}
}

func (m *Manager) reapExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.ctx.Err() != nil || time.Since(s.lastUsed) >= sessionTTL {
			s.close()
			delete(m.sessions, id)
		}
	}
}

func (s *session) close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
}

func (s *session) withRunContext(callCtx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if s == nil {
		return fmt.Errorf("browser session is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastUsed = time.Now()
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	runCtx, cancel := context.WithTimeout(s.ctx, timeout)
	defer cancel()
	if callCtx != nil {
		done := callCtx.Done()
		if done != nil {
			go func() {
				select {
				case <-done:
					cancel()
				case <-runCtx.Done():
				}
			}()
		}
	}
	return fn(runCtx)
}
