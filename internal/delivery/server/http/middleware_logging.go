package http

import (
	"net/http"
	"strings"

	"alex/internal/shared/logging"
	id "alex/internal/shared/utils/id"
)

func resolveLogID(r *http.Request) string {
	if r == nil {
		return ""
	}
	for _, header := range []string{"X-Log-Id", "X-Request-Id", "X-Correlation-Id"} {
		if value := strings.TrimSpace(r.Header.Get(header)); value != "" {
			return value
		}
	}
	return ""
}

// LoggingMiddleware logs incoming requests
func LoggingMiddleware(logger logging.Logger) func(http.Handler) http.Handler {
	logger = logging.OrNop(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip log_id generation and request logging for dev endpoints to avoid
			// self-referential noise (log analyzer polling creates log entries).
			if strings.HasPrefix(r.URL.Path, "/api/dev/") {
				next.ServeHTTP(w, r)
				return
			}
			ctx := r.Context()
			logID := id.LogIDFromContext(ctx)
			if logID == "" {
				logID = resolveLogID(r)
				if logID == "" {
					logID = id.NewLogID()
				}
				ctx = id.WithLogID(ctx, logID)
			}
			if logID != "" {
				w.Header().Set("X-Log-Id", logID)
			}
			reqLogger := logging.WithLogID(logger, logID)
			reqLogger.Info("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
