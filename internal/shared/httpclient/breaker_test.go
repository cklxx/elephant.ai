package httpclient

import (
	"net/http"
	"testing"

	alexerrors "alex/internal/shared/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrapTransportWithCircuitBreaker(t *testing.T) {
	cfg := alexerrors.DefaultCircuitBreakerConfig()

	tests := []struct {
		name     string
		base     http.RoundTripper
		cbName   string
		wantType bool
	}{
		{
			name:     "nil base uses default transport",
			base:     nil,
			cbName:   "test",
			wantType: true,
		},
		{
			name:     "empty name defaults to http-client",
			base:     http.DefaultTransport,
			cbName:   "",
			wantType: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := WrapTransportWithCircuitBreaker(tt.base, tt.cbName, cfg)
			require.NotNil(t, rt)

			cbrt, ok := rt.(*circuitBreakerRoundTripper)
			require.True(t, ok, "expected *circuitBreakerRoundTripper")
			assert.NotNil(t, cbrt.base)
			assert.NotNil(t, cbrt.breaker)
		})
	}
}

func TestIsBreakerFailureStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		want   bool
	}{
		{name: "500 is failure", status: 500, want: true},
		{name: "502 is failure", status: 502, want: true},
		{name: "429 is failure", status: 429, want: true},
		{name: "200 is not failure", status: 200, want: false},
		{name: "404 is not failure", status: 404, want: false},
		{name: "399 is not failure", status: 399, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBreakerFailureStatus(tt.status)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCircuitBreakerRoundTripNilRequest(t *testing.T) {
	cfg := alexerrors.DefaultCircuitBreakerConfig()
	rt := WrapTransportWithCircuitBreaker(http.DefaultTransport, "test", cfg)

	resp, err := rt.RoundTrip(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nil request")
	assert.Nil(t, resp)
}
