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
