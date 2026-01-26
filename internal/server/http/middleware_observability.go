package http

import (
	"net/http"
	"time"

	"alex/internal/logging"
	"alex/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

// ObservabilityMiddleware instruments HTTP requests with tracing, metrics, and optional latency logging.
func ObservabilityMiddleware(obs *observability.Observability, latencyLogger logging.Logger) func(http.Handler) http.Handler {
	hasLatencyLogger := !logging.IsNil(latencyLogger)
	return func(next http.Handler) http.Handler {
		// When neither observability nor latency logging is enabled, skip the wrapper entirely.
		if obs == nil && !hasLatencyLogger {
			return next
		}
		latencyLogger = logging.OrNop(latencyLogger)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			rec, wrapped := wrapResponseWriter(w)
			start := time.Now()
			initialRoute := canonicalPath(r.URL.Path)
			var spanEnd func(error)
			if obs != nil && obs.Tracer != nil {
				ctx, span := obs.Tracer.StartSpan(ctx, observability.SpanHTTPServer,
					attribute.String("http.route", initialRoute),
					attribute.String("http.method", r.Method),
				)
				r = r.WithContext(ctx)
				spanEnd = func(err error) {
					if err != nil {
						span.RecordError(err)
						span.SetStatus(codes.Error, err.Error())
					}
					span.SetAttributes(attribute.Int("http.status_code", rec.status))
					resolvedRoute := routeFromContext(r.Context())
					if resolvedRoute == "" {
						resolvedRoute = initialRoute
					}
					span.SetAttributes(attribute.String("http.route", resolvedRoute))
					span.End()
				}
				defer func() {
					if spanEnd != nil {
						spanEnd(nil)
					}
				}()
			}
			next.ServeHTTP(wrapped, r)
			resolvedRoute := routeFromContext(r.Context())
			if resolvedRoute == "" {
				resolvedRoute = initialRoute
			}
			latency := time.Since(start)
			if obs != nil {
				obs.Metrics.RecordHTTPServerRequest(ctx, r.Method, resolvedRoute, rec.status, latency, rec.bytes)
			}
			if hasLatencyLogger {
				latencyLogger.Info(
					"route=%s method=%s status=%d latency_ms=%.2f bytes=%d",
					resolvedRoute,
					r.Method,
					rec.status,
					float64(latency.Microseconds())/1000.0,
					rec.bytes,
				)
			}
		})
	}
}
