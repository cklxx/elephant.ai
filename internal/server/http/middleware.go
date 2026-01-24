package http

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	authapp "alex/internal/auth/app"
	authdomain "alex/internal/auth/domain"
	"alex/internal/logging"
	"alex/internal/observability"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/time/rate"
)

type contextKey string

const (
	authUserContextKey       contextKey = "authUser"
	canonicalRouteContextKey contextKey = "canonicalRoute"
)

type StreamGuardConfig struct {
	MaxDuration   time.Duration
	MaxBytes      int64
	MaxConcurrent int
}

type RateLimitConfig struct {
	RequestsPerMinute int
	Burst             int
	EntryTTL          time.Duration
	CleanupInterval   time.Duration
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

// Unwrap exposes the underlying ResponseWriter so downstream handlers
// can recover original capabilities like http.Flusher.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderFlusher struct {
	http.ResponseWriter
	http.Flusher
}

func (r *responseRecorderFlusher) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderHijacker struct {
	http.ResponseWriter
	http.Hijacker
}

func (r *responseRecorderHijacker) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

type responseRecorderPusher struct {
	http.ResponseWriter
	http.Pusher
}

func (r *responseRecorderPusher) Unwrap() http.ResponseWriter {
	if r == nil {
		return nil
	}
	return r.ResponseWriter
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	// ResponseRecorder passes through bytes unchanged; handlers are responsible for output encoding.
	n, err := r.ResponseWriter.Write(b)
	if n > 0 {
		r.bytes += int64(n)
	}
	return n, err
}

func wrapResponseWriter(w http.ResponseWriter) (*responseRecorder, http.ResponseWriter) {
	rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
	var wrapped http.ResponseWriter = rec

	if flusher, ok := w.(http.Flusher); ok {
		wrapped = &responseRecorderFlusher{ResponseWriter: wrapped, Flusher: flusher}
	}
	if hijacker, ok := w.(http.Hijacker); ok {
		wrapped = &responseRecorderHijacker{ResponseWriter: wrapped, Hijacker: hijacker}
	}
	if pusher, ok := w.(http.Pusher); ok {
		wrapped = &responseRecorderPusher{ResponseWriter: wrapped, Pusher: pusher}
	}
	return rec, wrapped
}

func annotateRequestRoute(r *http.Request, route string) {
	if r == nil || route == "" {
		return
	}
	ctx := context.WithValue(r.Context(), canonicalRouteContextKey, route)
	*r = *r.WithContext(ctx)
}

func routeFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if route, ok := ctx.Value(canonicalRouteContextKey).(string); ok {
		return route
	}
	return ""
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(environment string, allowedOrigins []string) func(http.Handler) http.Handler {
	allowedSet := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		allowedSet[origin] = struct{}{}
	}

	env := strings.ToLower(strings.TrimSpace(environment))
	allowAny := env != "production"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			originAllowed := false
			if origin != "" {
				if _, ok := allowedSet[origin]; ok {
					originAllowed = true
				} else if matchesForwardedOrigin(origin, r) {
					originAllowed = true
				}
			}

			if origin != "" && originAllowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				appendVary(w, "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			} else if origin != "" && allowAny {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func matchesForwardedOrigin(origin string, r *http.Request) bool {
	if origin == "" {
		return false
	}
	if forwarded := forwardedOrigin(r); forwarded != "" && origin == forwarded {
		return true
	}
	if hostOrigin := hostOriginFromRequest(r); hostOrigin != "" && origin == hostOrigin {
		return true
	}
	return false
}

func forwardedOrigin(r *http.Request) string {
	if val := parseForwardedHeader(r.Header.Get("Forwarded")); val != "" {
		return val
	}
	proto := headerFirst(r.Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = headerFirst(r.Header.Get("X-Forwarded-Scheme"))
	}
	host := headerFirst(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		return ""
	}
	return buildOrigin(proto, host, headerFirst(r.Header.Get("X-Forwarded-Port")))
}

func parseForwardedHeader(header string) string {
	if header == "" {
		return ""
	}
	entries := strings.Split(header, ",")
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		var proto, host string
		parts := strings.Split(entry, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			kv := strings.SplitN(part, "=", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(kv[0]))
			val := strings.Trim(kv[1], "\"")
			switch key {
			case "proto":
				proto = val
			case "host":
				host = val
			}
		}
		if host != "" {
			return buildOrigin(proto, host, "")
		}
	}
	return ""
}

func hostOriginFromRequest(r *http.Request) string {
	proto := "http"
	if r.TLS != nil {
		proto = "https"
	}
	host := strings.TrimSpace(r.Host)
	if host == "" {
		return ""
	}
	return buildOrigin(proto, host, "")
}

func buildOrigin(proto, host, port string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	proto = strings.TrimSpace(proto)
	if proto == "" {
		proto = "http"
	}
	if port != "" && !strings.Contains(host, ":") {
		host = host + ":" + strings.TrimSpace(port)
	}
	return proto + "://" + host
}

func headerFirst(val string) string {
	if val == "" {
		return ""
	}
	parts := strings.Split(val, ",")
	return strings.TrimSpace(parts[0])
}

func appendVary(w http.ResponseWriter, value string) {
	existing := w.Header().Values("Vary")
	for _, v := range existing {
		if strings.EqualFold(strings.TrimSpace(v), value) {
			return
		}
	}
	w.Header().Add("Vary", value)
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

type rateLimitEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type rateLimiter struct {
	mu              sync.Mutex
	limit           rate.Limit
	burst           int
	entries         map[string]*rateLimitEntry
	entryTTL        time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time
}

func newRateLimiter(cfg RateLimitConfig) *rateLimiter {
	ttl := cfg.EntryTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	cleanup := cfg.CleanupInterval
	if cleanup <= 0 {
		cleanup = 5 * time.Minute
	}
	return &rateLimiter{
		limit:           rate.Every(time.Minute / time.Duration(cfg.RequestsPerMinute)),
		burst:           cfg.Burst,
		entries:         make(map[string]*rateLimitEntry),
		entryTTL:        ttl,
		cleanupInterval: cleanup,
		lastCleanup:     time.Now(),
	}
}

func (r *rateLimiter) allow(key string) bool {
	if r == nil || key == "" {
		return true
	}

	now := time.Now()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.cleanupInterval > 0 && now.Sub(r.lastCleanup) >= r.cleanupInterval {
		for k, entry := range r.entries {
			if entry == nil || now.Sub(entry.lastSeen) > r.entryTTL {
				delete(r.entries, k)
			}
		}
		r.lastCleanup = now
	}

	entry, ok := r.entries[key]
	if !ok {
		entry = &rateLimitEntry{
			limiter:  rate.NewLimiter(r.limit, r.burst),
			lastSeen: now,
		}
		r.entries[key] = entry
	} else {
		entry.lastSeen = now
	}

	return entry.limiter.Allow()
}

func RateLimitMiddleware(cfg RateLimitConfig) func(http.Handler) http.Handler {
	if cfg.RequestsPerMinute <= 0 || cfg.Burst <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	limiter := newRateLimiter(cfg)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := rateLimitKey(r)
			if !limiter.allow(key) {
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.writer.Write(b)
}

func (w *gzipResponseWriter) Flush() {
	if w.writer != nil {
		_ = w.writer.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func acceptsGzip(r *http.Request) bool {
	if r == nil {
		return false
	}
	encoding := r.Header.Get("Accept-Encoding")
	return strings.Contains(strings.ToLower(encoding), "gzip")
}

func shouldSkipCompression(r *http.Request) bool {
	if r == nil || r.URL == nil {
		return true
	}
	path := r.URL.Path
	if isStreamRequest(r) {
		return true
	}
	if strings.HasPrefix(path, "/api/attachments/") || strings.HasPrefix(path, "/api/data/") {
		return true
	}
	return false
}

func CompressionMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if shouldSkipCompression(r) || !acceptsGzip(r) {
				next.ServeHTTP(w, r)
				return
			}

			appendVary(w, "Accept-Encoding")
			w.Header().Set("Content-Encoding", "gzip")

			gz := gzip.NewWriter(w)
			defer gz.Close()

			gzWriter := &gzipResponseWriter{ResponseWriter: w, writer: gz}
			if flusher, ok := w.(http.Flusher); ok {
				gzWriter.ResponseWriter = &responseRecorderFlusher{ResponseWriter: w, Flusher: flusher}
			}
			next.ServeHTTP(gzWriter, r)
		})
	}
}

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

func rateLimitKey(r *http.Request) string {
	if r == nil {
		return ""
	}
	if user, ok := CurrentUser(r.Context()); ok {
		if id := strings.TrimSpace(user.ID); id != "" {
			return "user:" + id
		}
	}
	if ip := clientIP(r); ip != "" {
		return "ip:" + ip
	}
	return "anonymous"
}


// LoggingMiddleware logs incoming requests
func LoggingMiddleware(logger logging.Logger) func(http.Handler) http.Handler {
	logger = logging.OrNop(logger)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}

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

func canonicalPath(path string) string {
	if path == "" {
		return "/"
	}
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	segments := strings.Split(trimmed, "/")
	filtered := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if looksLikeIdentifier(segment) {
			filtered = append(filtered, ":id")
			continue
		}
		filtered = append(filtered, segment)
	}
	if len(filtered) == 0 {
		return "/"
	}
	return "/" + strings.Join(filtered, "/")
}

func looksLikeIdentifier(segment string) bool {
	if len(segment) >= 8 {
		var alphanumeric bool
		for _, r := range segment {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				alphanumeric = true
				continue
			}
			if r == '-' || r == '_' {
				continue
			}
			return false
		}
		if alphanumeric {
			return true
		}
	}
	if _, err := strconv.Atoi(segment); err == nil {
		return true
	}
	return false
}

// AuthMiddleware enforces bearer token authentication on protected routes.
func AuthMiddleware(service *authapp.Service) func(http.Handler) http.Handler {
	if service == nil {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}
			token := extractBearerToken(r.Header.Get("Authorization"))
			if token == "" {
				token = readAccessTokenCookie(r)
			}
			if token == "" {
				token = strings.TrimSpace(r.URL.Query().Get("access_token"))
			}
			if token == "" {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}
			claims, err := service.ParseAccessToken(r.Context(), token)
			if err != nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			user, err := service.GetUser(r.Context(), claims.Subject)
			if err != nil {
				http.Error(w, "user not found", http.StatusUnauthorized)
				return
			}
			if user.Status != authdomain.UserStatusActive {
				http.Error(w, "user disabled", http.StatusForbidden)
				return
			}
			ctx := context.WithValue(r.Context(), authUserContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func readAccessTokenCookie(r *http.Request) string {
	if r == nil {
		return ""
	}
	cookie, err := r.Cookie(accessCookieName)
	if err != nil {
		return ""
	}
	value := strings.TrimSpace(cookie.Value)
	if value == "" {
		return ""
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		if token := strings.TrimSpace(string(decoded)); token != "" {
			return token
		}
	}
	return value
}

// CurrentUser extracts the authenticated user from request context.
func CurrentUser(ctx context.Context) (authdomain.User, bool) {
	user, ok := ctx.Value(authUserContextKey).(authdomain.User)
	return user, ok
}
