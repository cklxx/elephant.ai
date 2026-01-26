package http

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"unicode"
)

type contextKey string

const (
	authUserContextKey       contextKey = "authUser"
	canonicalRouteContextKey contextKey = "canonicalRoute"
)

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
