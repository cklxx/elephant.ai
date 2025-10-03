package http

import (
	"net/http"
	"os"

	"alex/internal/utils"
)

// CORSMiddleware handles CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// In development, allow all origins
		// In production, configure specific allowed origins
		allowedOrigins := []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"https://alex.yourdomain.com",
		}

		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// Check if in development mode (via env var)
		isDev := os.Getenv("ALEX_ENV") != "production"

		// Only set CORS headers for allowed origins (or all in dev mode)
		if origin != "" && (allowed || isDev) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			// Only set credentials header for explicitly allowed origins
			if allowed {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
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
