package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	id "alex/internal/utils/id"
)

// userIDKey is the context key used internally by the middleware to store the parsed user identifier.
type userIDKey struct{}

// Middleware validates the Authorization header and injects the extracted user ID into the request context.
//
// The current implementation treats any non-empty Bearer token as the authenticated user identifier. It keeps the
// validation logic pluggable so that future iterations can integrate real JWT/OIDC verification without touching
// the handlers.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		userID, err := extractUserID(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		ctx := id.WithUserID(r.Context(), userID)
		ctx = context.WithValue(ctx, userIDKey{}, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// extractUserID parses the Authorization header and returns the encoded user identifier.
func extractUserID(r *http.Request) (string, error) {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header != "" {
		const prefix = "Bearer "
		if !strings.HasPrefix(header, prefix) {
			return "", errors.New("invalid authorization scheme")
		}
		token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
		if token == "" {
			return "", errors.New("empty bearer token")
		}
		return token, nil
	}

	if token := strings.TrimSpace(r.URL.Query().Get("auth_token")); token != "" {
		return token, nil
	}

	if cookie, err := r.Cookie("alex_auth_token"); err == nil {
		trimmed := strings.TrimSpace(cookie.Value)
		if trimmed != "" {
			return trimmed, nil
		}
	}

	return "", errors.New("missing Authorization header")
}

// FromContext retrieves the user identifier that was parsed by the middleware.
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if userID, ok := ctx.Value(userIDKey{}).(string); ok {
		return userID
	}
	return ""
}

func isPublicPath(path string) bool {
	switch {
	case path == "/health", path == "/health/":
		return true
	}
	return false
}
