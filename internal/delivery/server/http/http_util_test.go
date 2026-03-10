package http

import (
	"net"
	"net/http"
	"testing"
)

func TestClientIP_TrustedProxyPassesXFF(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8"})

	r := &http.Request{
		RemoteAddr: "10.0.0.1:12345",
		Header:     http.Header{"X-Forwarded-For": {"203.0.113.50, 10.0.0.1"}},
	}
	got := clientIP(r, trusted)
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", got)
	}
}

func TestClientIP_TrustedProxyPassesXRealIP(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8"})

	r := &http.Request{
		RemoteAddr: "10.0.0.1:12345",
		Header:     make(http.Header),
	}
	r.Header.Set("X-Real-IP", "203.0.113.50")
	got := clientIP(r, trusted)
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", got)
	}
}

func TestClientIP_UntrustedProxyIgnoresXFF(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8"})

	r := &http.Request{
		RemoteAddr: "192.168.1.100:12345",
		Header:     http.Header{"X-Forwarded-For": {"1.2.3.4"}},
	}
	got := clientIP(r, trusted)
	if got != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100 (peer IP, ignoring spoofed XFF), got %s", got)
	}
}

func TestClientIP_UntrustedProxyIgnoresXRealIP(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8"})

	r := &http.Request{
		RemoteAddr: "192.168.1.100:12345",
		Header:     make(http.Header),
	}
	r.Header.Set("X-Real-IP", "1.2.3.4")
	got := clientIP(r, trusted)
	if got != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", got)
	}
}

func TestClientIP_NoProxiesUsesRemoteAddr(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "203.0.113.50:54321",
		Header:     http.Header{"X-Forwarded-For": {"spoofed"}},
	}
	got := clientIP(r, nil)
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", got)
	}
}

func TestClientIP_EmptyTrustedProxiesUsesRemoteAddr(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "203.0.113.50:54321",
		Header:     http.Header{"X-Forwarded-For": {"spoofed"}},
	}
	got := clientIP(r, []net.IPNet{})
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", got)
	}
}

func TestClientIP_RemoteAddrWithoutPort(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "203.0.113.50",
		Header:     http.Header{},
	}
	got := clientIP(r, nil)
	if got != "203.0.113.50" {
		t.Errorf("expected 203.0.113.50, got %s", got)
	}
}

func TestClientIP_IPv6TrustedProxy(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"fd00::/8"})

	r := &http.Request{
		RemoteAddr: "[fd00::1]:12345",
		Header:     http.Header{"X-Forwarded-For": {"2001:db8::1"}},
	}
	got := clientIP(r, trusted)
	if got != "2001:db8::1" {
		t.Errorf("expected 2001:db8::1, got %s", got)
	}
}

func TestClientIP_MultipleTrustedCIDRs(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8", "172.16.0.0/12"})

	r := &http.Request{
		RemoteAddr: "172.16.0.5:9999",
		Header:     http.Header{"X-Forwarded-For": {"8.8.8.8"}},
	}
	got := clientIP(r, trusted)
	if got != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8, got %s", got)
	}
}

func TestParseTrustedProxies_InvalidCIDRsSkipped(t *testing.T) {
	nets := ParseTrustedProxies([]string{"10.0.0.0/8", "not-a-cidr", "", "192.168.0.0/16"})
	if len(nets) != 2 {
		t.Fatalf("expected 2 valid nets, got %d", len(nets))
	}
}

func TestParseTrustedProxies_Nil(t *testing.T) {
	nets := ParseTrustedProxies(nil)
	if nets != nil {
		t.Errorf("expected nil, got %v", nets)
	}
}

func TestRateLimitKey_WithTrustedProxy(t *testing.T) {
	trusted := ParseTrustedProxies([]string{"10.0.0.0/8"})

	r := &http.Request{
		RemoteAddr: "10.0.0.1:12345",
		Header:     http.Header{"X-Forwarded-For": {"203.0.113.50"}},
	}
	key := rateLimitKey(r, trusted)
	if key != "ip:203.0.113.50" {
		t.Errorf("expected ip:203.0.113.50, got %s", key)
	}
}

func TestRateLimitKey_WithoutTrustedProxy(t *testing.T) {
	r := &http.Request{
		RemoteAddr: "192.168.1.1:12345",
		Header:     http.Header{"X-Forwarded-For": {"spoofed"}},
	}
	key := rateLimitKey(r, nil)
	if key != "ip:192.168.1.1" {
		t.Errorf("expected ip:192.168.1.1, got %s", key)
	}
}
