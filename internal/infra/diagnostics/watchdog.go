package diagnostics

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync/atomic"
	"time"

	"alex/internal/shared/logging"
)

// WatchdogConfig controls memory/goroutine monitoring thresholds.
type WatchdogConfig struct {
	// Interval between checks (default 30s).
	Interval time.Duration

	// HeapThresholdBytes triggers a heap dump when runtime heap alloc exceeds
	// this value. Default 2GB.
	HeapThresholdBytes uint64

	// GoroutineThreshold triggers a goroutine dump when count exceeds this.
	// Default 10000.
	GoroutineThreshold int

	// DumpDir is the directory for automatic profile dumps.
	// Default: logs/ relative to working directory.
	DumpDir string

	// MaxDumps limits how many auto-dumps are kept to avoid filling disk.
	// Default 5.
	MaxDumps int
}

func (c *WatchdogConfig) withDefaults() WatchdogConfig {
	out := *c
	if out.Interval == 0 {
		out.Interval = 30 * time.Second
	}
	if out.HeapThresholdBytes == 0 {
		out.HeapThresholdBytes = 2 * 1024 * 1024 * 1024 // 2GB
	}
	if out.GoroutineThreshold == 0 {
		out.GoroutineThreshold = 10000
	}
	if out.DumpDir == "" {
		out.DumpDir = "logs"
	}
	if out.MaxDumps == 0 {
		out.MaxDumps = 5
	}
	return out
}

// Watchdog monitors runtime memory and goroutine stats, logging warnings
// and auto-dumping profiles when thresholds are exceeded.
type Watchdog struct {
	cfg    WatchdogConfig
	logger logging.Logger

	dumpCount atomic.Int32
}

// NewWatchdog creates a new runtime watchdog with the given config.
func NewWatchdog(cfg WatchdogConfig, logger logging.Logger) *Watchdog {
	return &Watchdog{
		cfg:    cfg.withDefaults(),
		logger: logger,
	}
}

// Run starts the watchdog loop. It blocks until ctx is cancelled.
func (w *Watchdog) Run(ctx context.Context) {
	w.logger.Info("Watchdog started: interval=%s heap_threshold=%s goroutine_threshold=%d dump_dir=%s",
		w.cfg.Interval, formatBytes(w.cfg.HeapThresholdBytes), w.cfg.GoroutineThreshold, w.cfg.DumpDir)

	ticker := time.NewTicker(w.cfg.Interval)
	defer ticker.Stop()

	var prevHeapAlloc uint64
	var prevGoroutines int

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Watchdog stopped")
			return
		case <-ticker.C:
			w.check(&prevHeapAlloc, &prevGoroutines)
		}
	}
}

func (w *Watchdog) check(prevHeap *uint64, prevGoroutines *int) {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	goroutines := runtime.NumGoroutine()
	heapAlloc := ms.HeapAlloc
	heapSys := ms.HeapSys
	stackSys := ms.StackSys
	numGC := ms.NumGC

	// Always log a summary at debug level.
	w.logger.Debug("Watchdog: heap_alloc=%s heap_sys=%s stack_sys=%s goroutines=%d gc_cycles=%d",
		formatBytes(heapAlloc), formatBytes(heapSys), formatBytes(stackSys), goroutines, numGC)

	// Detect heap spike.
	if heapAlloc > w.cfg.HeapThresholdBytes {
		w.logger.Warn("WATCHDOG ALERT: heap_alloc=%s exceeds threshold %s (heap_sys=%s goroutines=%d)",
			formatBytes(heapAlloc), formatBytes(w.cfg.HeapThresholdBytes), formatBytes(heapSys), goroutines)
		w.autoDumpHeap(heapAlloc)
	}

	// Detect goroutine explosion.
	if goroutines > w.cfg.GoroutineThreshold {
		w.logger.Warn("WATCHDOG ALERT: goroutines=%d exceeds threshold %d (heap_alloc=%s)",
			goroutines, w.cfg.GoroutineThreshold, formatBytes(heapAlloc))
		w.autoDumpGoroutine(goroutines)
	}

	// Detect rapid growth (>50% increase between checks).
	if *prevHeap > 0 && heapAlloc > *prevHeap && float64(heapAlloc-*prevHeap)/float64(*prevHeap) > 0.5 {
		w.logger.Warn("WATCHDOG: rapid heap growth: %s -> %s (+%.0f%%) in %s",
			formatBytes(*prevHeap), formatBytes(heapAlloc),
			float64(heapAlloc-*prevHeap)/float64(*prevHeap)*100,
			w.cfg.Interval)
	}

	if *prevGoroutines > 100 && goroutines > *prevGoroutines*2 {
		w.logger.Warn("WATCHDOG: goroutine spike: %d -> %d in %s",
			*prevGoroutines, goroutines, w.cfg.Interval)
	}

	*prevHeap = heapAlloc
	*prevGoroutines = goroutines
}

func (w *Watchdog) autoDumpHeap(heapAlloc uint64) {
	if int(w.dumpCount.Load()) >= w.cfg.MaxDumps {
		w.logger.Warn("Watchdog: max dump count (%d) reached, skipping heap dump", w.cfg.MaxDumps)
		return
	}

	path, err := w.dumpProfile("heap", func(f *os.File) error {
		return pprof.WriteHeapProfile(f)
	})
	if err != nil {
		w.logger.Error("Watchdog: failed to dump heap profile: %v", err)
		return
	}
	w.dumpCount.Add(1)
	w.logger.Warn("Watchdog: heap profile dumped to %s (heap_alloc=%s)", path, formatBytes(heapAlloc))
}

func (w *Watchdog) autoDumpGoroutine(count int) {
	if int(w.dumpCount.Load()) >= w.cfg.MaxDumps {
		w.logger.Warn("Watchdog: max dump count (%d) reached, skipping goroutine dump", w.cfg.MaxDumps)
		return
	}

	path, err := w.dumpProfile("goroutine", func(f *os.File) error {
		profile := pprof.Lookup("goroutine")
		if profile == nil {
			return fmt.Errorf("goroutine profile not found")
		}
		return profile.WriteTo(f, 1)
	})
	if err != nil {
		w.logger.Error("Watchdog: failed to dump goroutine profile: %v", err)
		return
	}
	w.dumpCount.Add(1)
	w.logger.Warn("Watchdog: goroutine profile dumped to %s (count=%d)", path, count)
}

func (w *Watchdog) dumpProfile(kind string, writeFn func(*os.File) error) (string, error) {
	if err := os.MkdirAll(w.cfg.DumpDir, 0o755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", w.cfg.DumpDir, err)
	}

	ts := time.Now().UTC().Format("20060102T150405Z")
	filename := fmt.Sprintf("watchdog-%s-%s.prof", kind, ts)
	path := filepath.Join(w.cfg.DumpDir, filename)

	f, err := os.Create(path)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()

	if err := writeFn(f); err != nil {
		return "", err
	}
	return path, nil
}

func formatBytes(b uint64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1fGB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1fMB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1fKB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%dB", b)
	}
}
