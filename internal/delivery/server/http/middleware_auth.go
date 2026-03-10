package http

import (
	"net/http"
	"strings"
)

// BearerAuthMiddleware returns middleware that validates bearer tokens.
// If token is empty, all requests pass through (auth disabled).
func BearerAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if token == "" {
			return next // auth disabled
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				// Also check X-API-Key header as fallback.
				if key := r.Header.Get("X-API-Key"); key != "" {
					auth = "Bearer " + key
				}
			}

			if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != token {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
