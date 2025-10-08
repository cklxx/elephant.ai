package http

import (
	"net/http"
	"strings"

	"alex/internal/utils"
)

// CORSMiddleware handles CORS headers
func CORSMiddleware(environment string) func(http.Handler) http.Handler {
	allowedOrigins := []string{
		"http://localhost:3000",
		"http://localhost:3001",
		"https://alex.yourdomain.com",
	}

	env := strings.ToLower(strings.TrimSpace(environment))
	isDev := env != "production"

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, allowedOrigin := range allowedOrigins {
				if origin == allowedOrigin {
					allowed = true
					break
				}
			}

			if origin != "" && (allowed || isDev) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				if allowed {
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

// LoggingMiddleware logs incoming requests
func LoggingMiddleware(logger *utils.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("%s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
			next.ServeHTTP(w, r)
		})
	}
}
