package http

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	authapp "alex/internal/auth/app"
	authdomain "alex/internal/auth/domain"
)

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
