package httpclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateOutboundURL(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		opts    URLValidationOptions
		wantErr string
	}{
		{
			name:    "empty URL returns error",
			raw:     "",
			opts:    DefaultURLValidationOptions(),
			wantErr: "url is required",
		},
		{
			name:    "invalid scheme ftp",
			raw:     "ftp://example.com",
			opts:    DefaultURLValidationOptions(),
			wantErr: "unsupported url scheme: ftp",
		},
		{
			name:    "missing host",
			raw:     "http://",
			opts:    DefaultURLValidationOptions(),
			wantErr: "url host is required",
		},
		{
			name:    "localhost blocked by default",
			raw:     "http://localhost/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "local urls are not allowed",
		},
		{
			name:    "sub.localhost blocked by default",
			raw:     "http://sub.localhost/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "local urls are not allowed",
		},
		{
			name: "localhost allowed with AllowLocalhost",
			raw:  "http://localhost/path",
			opts: URLValidationOptions{AllowLocalhost: true},
		},
		{
			name:    "127.0.0.1 blocked by default",
			raw:     "http://127.0.0.1/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "local urls are not allowed",
		},
		{
			name:    "::1 blocked by default",
			raw:     "http://[::1]/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "local urls are not allowed",
		},
		{
			name:    "private IP 10.0.0.1 blocked",
			raw:     "http://10.0.0.1/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "private network urls are not allowed",
		},
		{
			name:    "private IP 192.168.1.1 blocked",
			raw:     "http://192.168.1.1/path",
			opts:    DefaultURLValidationOptions(),
			wantErr: "private network urls are not allowed",
		},
		{
			name: "private IP allowed with AllowPrivateNetworks",
			raw:  "http://192.168.1.1/path",
			opts: URLValidationOptions{AllowPrivateNetworks: true},
		},
		{
			name: "valid public HTTP URL",
			raw:  "http://example.com/path",
			opts: DefaultURLValidationOptions(),
		},
		{
			name: "valid HTTPS URL",
			raw:  "https://example.com/secure",
			opts: DefaultURLValidationOptions(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ValidateOutboundURL(tt.raw, tt.opts)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, parsed)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, parsed)
			}
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
