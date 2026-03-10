package artifacts

import (
	"encoding/base64"
	"testing"

	"alex/internal/domain/agent/ports"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePlaceholder(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "brackets_stripped", input: "[foo]", want: "foo"},
		{name: "brackets_trimmed", input: "[ bar ]", want: "bar"},
		{name: "no_brackets", input: "foo", want: ""},
		{name: "empty_brackets", input: "[]", want: ""},
		{name: "single_char", input: "[x]", want: "x"},
		{name: "empty_string", input: "", want: ""},
		{name: "too_short", input: "ab", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePlaceholder(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLooksLikeURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "https", input: "https://example.com", want: true},
		{name: "http_with_path", input: "http://example.com/path", want: true},
		{name: "ftp_rejected", input: "ftp://example.com", want: false},
		{name: "not_a_url", input: "not-a-url", want: false},
		{name: "empty", input: "", want: false},
		{name: "data_uri", input: "data:image/png;base64,abc", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeURL(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLooksLikeBase64(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{name: "valid_base64", input: "SGVsbG8gV29ybGQh", want: true},
		{name: "too_short", input: "abc", want: false},
		{name: "exactly_16", input: "AAAAAAAAAAAAAAAA", want: true},
		{name: "invalid_char", input: "AAAAAA!AAAAAAAAA", want: false},
		{name: "empty", input: "", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := looksLikeBase64(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLookupAttachmentCaseInsensitive(t *testing.T) {
	img := ports.Attachment{Name: "image.png", MediaType: "image/png"}

	tests := []struct {
		name        string
		attachments map[string]ports.Attachment
		key         string
		wantAtt     ports.Attachment
		wantOK      bool
	}{
		{
			name:        "nil_map",
			attachments: nil,
			key:         "image.png",
			wantOK:      false,
		},
		{
			name:        "exact_match",
			attachments: map[string]ports.Attachment{"image.png": img},
			key:         "image.png",
			wantAtt:     img,
			wantOK:      true,
		},
		{
			name:        "case_insensitive_match",
			attachments: map[string]ports.Attachment{"Image.PNG": img},
			key:         "image.png",
			wantAtt:     img,
			wantOK:      true,
		},
		{
			name:        "no_match",
			attachments: map[string]ports.Attachment{"other.png": img},
			key:         "image.png",
			wantOK:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lookupAttachmentCaseInsensitive(tt.attachments, tt.key)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAtt, got)
			}
		})
	}
}

func TestLookupAttachmentByURI(t *testing.T) {
	att := ports.Attachment{Name: "file.png", URI: "https://cdn.example.com/file.png"}

	tests := []struct {
		name        string
		attachments map[string]ports.Attachment
		uri         string
		wantAtt     ports.Attachment
		wantOK      bool
	}{
		{
			name:        "nil_map",
			attachments: nil,
			uri:         "https://cdn.example.com/file.png",
			wantOK:      false,
		},
		{
			name:        "empty_uri",
			attachments: map[string]ports.Attachment{"file.png": att},
			uri:         "",
			wantOK:      false,
		},
		{
			name:        "exact_uri_case_insensitive",
			attachments: map[string]ports.Attachment{"file.png": att},
			uri:         "HTTPS://CDN.EXAMPLE.COM/FILE.PNG",
			wantAtt:     att,
			wantOK:      true,
		},
		{
			name:        "no_match",
			attachments: map[string]ports.Attachment{"file.png": att},
			uri:         "https://other.com/nope.png",
			wantOK:      false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := lookupAttachmentByURI(tt.attachments, tt.uri)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantAtt, got)
			}
		})
	}
}

func TestResolveAttachmentURI(t *testing.T) {
	tests := []struct {
		name string
		att  ports.Attachment
		want string
	}{
		{
			name: "direct_uri",
			att:  ports.Attachment{URI: "https://cdn.example.com/file.png"},
			want: "https://cdn.example.com/file.png",
		},
		{
			name: "preview_assets_image_cdn",
			att: ports.Attachment{
				PreviewAssets: []ports.AttachmentPreviewAsset{
					{CDNURL: "https://cdn.example.com/preview.png", MimeType: "image/png"},
				},
			},
			want: "https://cdn.example.com/preview.png",
		},
		{
			name: "preview_assets_non_image_cdn",
			att: ports.Attachment{
				PreviewAssets: []ports.AttachmentPreviewAsset{
					{CDNURL: "https://cdn.example.com/doc.pdf", MimeType: "application/pdf"},
				},
			},
			want: "https://cdn.example.com/doc.pdf",
		},
		{
			name: "no_uri_no_preview_assets",
			att:  ports.Attachment{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveAttachmentURI(tt.att)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDecodeDataURI(t *testing.T) {
	validPayload := base64.StdEncoding.EncodeToString([]byte("hello"))

	tests := []struct {
		name      string
		input     string
		wantData  []byte
		wantMime  string
		wantError bool
	}{
		{
			name:     "valid_data_uri",
			input:    "data:image/png;base64," + validPayload,
			wantData: []byte("hello"),
			wantMime: "image/png",
		},
		{
			name:      "invalid_format",
			input:     "not-a-data-uri",
			wantError: true,
		},
		{
			name:      "empty_payload",
			input:     "data:image/png;base64,",
			wantError: true,
		},
		{
			name:      "non_base64_data_uri",
			input:     "data:text/plain,hello world",
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, mimeType, err := decodeDataURI(tt.input)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantData, data)
			assert.Equal(t, tt.wantMime, mimeType)
		})
	}
}
