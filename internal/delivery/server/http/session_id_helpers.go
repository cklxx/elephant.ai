package http

import (
	"net/http"
	"strings"
)

func extractRequiredSessionID(value string) (string, error) {
	sessionID := strings.TrimSpace(value)
	if err := validateSessionID(sessionID); err != nil {
		return "", err
	}
	return sessionID, nil
}

func extractRequiredSessionIDFromPath(r *http.Request) (string, error) {
	return extractRequiredSessionID(r.PathValue("session_id"))
}

func extractRequiredSessionIDFromQuery(r *http.Request) (string, error) {
	return extractRequiredSessionID(r.URL.Query().Get("session_id"))
}
