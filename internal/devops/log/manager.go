package log

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager handles log files for services.
type Manager struct {
	dir   string
	files map[string]*os.File
	mu    sync.RWMutex
}

// NewManager creates a new log manager.
func NewManager(dir string) *Manager {
	return &Manager{
		dir:   dir,
		files: make(map[string]*os.File),
	}
}

// EnsureDir creates the log directory.
func (m *Manager) EnsureDir() error {
	return os.MkdirAll(m.dir, 0o755)
}

// LogFile returns the log file path for a service.
func (m *Manager) LogFile(service string) string {
	return filepath.Join(m.dir, service+".log")
}

// Tail follows log files for the given services, writing output to w.
func (m *Manager) Tail(ctx context.Context, services []string, follow bool, w io.Writer) error {
	if len(services) == 0 {
		entries, err := os.ReadDir(m.dir)
		if err != nil {
			return fmt.Errorf("read log dir: %w", err)
		}
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".log" {
				name := e.Name()[:len(e.Name())-4]
				services = append(services, name)
			}
		}
	}

	if len(services) == 0 {
		return fmt.Errorf("no log files found")
	}

	multi := len(services) > 1
	var wg sync.WaitGroup

	for _, svc := range services {
		path := m.LogFile(svc)
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("open log %s: %w", path, err)
		}

		wg.Add(1)
		go func(name string, file *os.File) {
			defer wg.Done()
			defer file.Close()
			m.tailFile(ctx, name, file, follow, multi, w)
		}(svc, f)
	}

	wg.Wait()
	return nil
}

func (m *Manager) tailFile(ctx context.Context, name string, f *os.File, follow, prefix bool, w io.Writer) {
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if prefix {
			fmt.Fprintf(w, "[%s] %s\n", name, line)
		} else {
			fmt.Fprintln(w, line)
		}
	}

	if !follow {
		return
	}

	buf := make([]byte, 4096)
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		n, _ := f.Read(buf)
		if n > 0 {
			lines := string(buf[:n])
			if prefix {
				fmt.Fprintf(w, "[%s] %s", name, lines)
			} else {
				fmt.Fprint(w, lines)
			}
		}
	}
}

// Rotate renames the current log file and creates a fresh one.
func (m *Manager) Rotate(service string) error {
	path := m.LogFile(service)

	m.mu.Lock()
	if f, ok := m.files[service]; ok {
		f.Close()
		delete(m.files, service)
	}
	m.mu.Unlock()

	rotated := path + ".1"
	if err := os.Rename(path, rotated); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("rotate %s: %w", path, err)
	}
	return nil
}
