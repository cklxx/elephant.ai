package http

import (
	"net/http"
	"time"
)

func RequestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	if timeout <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isStreamRequest(r) {
				next.ServeHTTP(w, r)
				return
			}
			http.TimeoutHandler(next, timeout, "request timeout").ServeHTTP(w, r)
		})
	}
}
