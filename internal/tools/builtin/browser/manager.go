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

const sessionTTL = 30 * time.Minute

type Manager struct {
	cfg      Config
	mu       sync.Mutex
	sessions map[string]*session
}

type session struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	cancel      context.CancelFunc
	lastUsed    time.Time
	lastX       float64
	lastY       float64
	hasLastPos  bool
	mu          sync.Mutex
}

func NewManager(cfg Config) *Manager {
	return &Manager{
		cfg:      cfg,
		sessions: make(map[string]*session),
	}
}

func (m *Manager) Config() Config {
	if m == nil {
		return Config{}
	}
	return m.cfg
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

	sess, err := m.newSession(sessionID)
	if err != nil {
		return nil, err
	}
	m.sessions[sessionID] = sess
	return sess, nil
}

func (m *Manager) newSession(sessionID string) (*session, error) {
	baseCtx := context.Background()
	var allocCtx context.Context
	var allocCancel context.CancelFunc

	if strings.TrimSpace(m.cfg.CDPURL) != "" {
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(baseCtx, m.cfg.CDPURL)
	} else {
		opts := append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.Flag("headless", m.cfg.Headless),
			chromedp.Flag("disable-gpu", m.cfg.Headless),
		)
		if path := strings.TrimSpace(m.cfg.ChromePath); path != "" {
			opts = append(opts, chromedp.ExecPath(path))
		}
		if dir := strings.TrimSpace(m.cfg.UserDataDir); dir != "" {
			userDataDir := filepath.Join(dir, sessionID)
			if err := os.MkdirAll(userDataDir, 0o755); err == nil {
				opts = append(opts, chromedp.UserDataDir(userDataDir))
			}
		}
		allocCtx, allocCancel = chromedp.NewExecAllocator(baseCtx, opts...)
	}

	ctx, cancel := chromedp.NewContext(allocCtx)
	if err := chromedp.Run(ctx, chromedp.Navigate("about:blank")); err != nil {
		cancel()
		allocCancel()
		return nil, err
	}

	return &session{
		allocCtx:    allocCtx,
		allocCancel: allocCancel,
		ctx:         ctx,
		cancel:      cancel,
		lastUsed:    time.Now(),
	}, nil
}

func (s *session) close() {
	if s == nil {
		return
	}
	if s.cancel != nil {
		s.cancel()
	}
	if s.allocCancel != nil {
		s.allocCancel()
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
