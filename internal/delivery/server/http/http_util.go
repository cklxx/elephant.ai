package http

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

// writeJSON serialises payload as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// clientIP extracts the client IP from common proxy headers or the remote address.
func clientIP(r *http.Request) string {
	if realIP := r.Header.Get("X-Forwarded-For"); realIP != "" {
		parts := strings.Split(realIP, ",")
		return strings.TrimSpace(parts[0])
	}
	if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.Trim(r.RemoteAddr, "[]")
}
