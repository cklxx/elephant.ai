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

// decodeJSONRequest decodes a single JSON value from the request body and
// writes a stable 400 response when decoding fails.
func decodeJSONRequest(w http.ResponseWriter, r *http.Request, v any, invalidMessage string) bool {
	if invalidMessage == "" {
		invalidMessage = "invalid JSON payload"
	}
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, invalidMessage, http.StatusBadRequest)
		return false
	}
	return true
}

// ParseTrustedProxies parses a list of CIDR strings into net.IPNet values.
// Invalid CIDRs are silently skipped.
func ParseTrustedProxies(cidrs []string) []net.IPNet {
	var nets []net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		nets = append(nets, *ipNet)
	}
	return nets
}

// peerIP extracts the direct peer IP from r.RemoteAddr.
func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return strings.Trim(r.RemoteAddr, "[]")
}

// isTrustedPeer returns true if the peer IP falls within any of the trusted CIDR ranges.
func isTrustedPeer(peer string, trusted []net.IPNet) bool {
	ip := net.ParseIP(peer)
	if ip == nil {
		return false
	}
	for i := range trusted {
		if trusted[i].Contains(ip) {
			return true
		}
	}
	return false
}

// clientIP extracts the client IP, only trusting X-Forwarded-For and X-Real-IP
// headers when the direct peer is within the trusted proxy CIDR ranges.
// When trustedProxies is nil or empty, proxy headers are never trusted.
func clientIP(r *http.Request, trustedProxies []net.IPNet) string {
	peer := peerIP(r)

	if len(trustedProxies) > 0 && isTrustedPeer(peer, trustedProxies) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
		if realIP := r.Header.Get("X-Real-IP"); realIP != "" {
			return strings.TrimSpace(realIP)
		}
	}

	return peer
}
