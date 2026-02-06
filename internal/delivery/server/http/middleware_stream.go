package http

import (
	"context"
	"net/http"
	"strings"
	"time"
)

type StreamGuardConfig struct {
	MaxDuration   time.Duration
	MaxBytes      int64
	MaxConcurrent int
}

type streamLimitWriter struct {
	http.ResponseWriter
	cancel  context.CancelFunc
	limit   int64
	written int64
}

func (w *streamLimitWriter) Unwrap() http.ResponseWriter {
	if w == nil {
		return nil
	}
	return w.ResponseWriter
}

func (w *streamLimitWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	if n > 0 {
		w.written += int64(n)
	}
	if w.limit > 0 && w.written >= w.limit && w.cancel != nil {
		w.cancel()
	}
	return n, err
}

func StreamGuardMiddleware(cfg StreamGuardConfig) func(http.Handler) http.Handler {
	if cfg.MaxDuration <= 0 && cfg.MaxBytes <= 0 && cfg.MaxConcurrent <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	var sem chan struct{}
	if cfg.MaxConcurrent > 0 {
		sem = make(chan struct{}, cfg.MaxConcurrent)
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isStreamRequest(r) {
				next.ServeHTTP(w, r)
				return
			}

			if sem != nil {
				select {
				case sem <- struct{}{}:
					defer func() { <-sem }()
				default:
					http.Error(w, "stream limit exceeded", http.StatusTooManyRequests)
					return
				}
			}

			ctx := r.Context()
			cancel := func() {}
			if cfg.MaxDuration > 0 {
				var cancelTimeout context.CancelFunc
				ctx, cancelTimeout = context.WithTimeout(ctx, cfg.MaxDuration)
				cancel = cancelTimeout
			}
			if cfg.MaxBytes > 0 {
				var cancelBytes context.CancelFunc
				ctx, cancelBytes = context.WithCancel(ctx)
				prevCancel := cancel
				cancel = func() {
					if cancelBytes != nil {
						cancelBytes()
					}
					if prevCancel != nil {
						prevCancel()
					}
				}
				w = &streamLimitWriter{ResponseWriter: w, cancel: cancelBytes, limit: cfg.MaxBytes}
			}
			defer cancel()

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isStreamRequest(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return false
	}
	path := strings.TrimSpace(r.URL.Path)
	if strings.HasPrefix(path, "/api/sse") || strings.Contains(path, "/stream") {
		return true
	}
	accept := strings.ToLower(strings.TrimSpace(r.Header.Get("Accept")))
	return strings.Contains(accept, "text/event-stream")
}
