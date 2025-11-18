package http

import (
	"context"
	"net/http"
	"strings"

	authapp "alex/internal/auth/app"
	authdomain "alex/internal/auth/domain"
	"alex/internal/utils"
)

type contextKey string

const authUserContextKey contextKey = "authUser"

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

			if origin != "" && (originAllowed || allowAny) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				appendVary(w, "Origin")
				if originAllowed || allowAny {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
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

// LoggingMiddleware logs incoming requests
func LoggingMiddleware(logger *utils.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
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

// CurrentUser extracts the authenticated user from request context.
func CurrentUser(ctx context.Context) (authdomain.User, bool) {
	user, ok := ctx.Value(authUserContextKey).(authdomain.User)
	return user, ok
}
