package lark

import "testing"

func TestApiToWebDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://open.feishu.cn", "https://feishu.cn"},
		{"https://open.larksuite.com", "https://larksuite.com"},
		{"https://open.larkoffice.com", "https://larkoffice.com"},
		{"https://open.custom.example.com", "https://custom.example.com"},
		{"http://open.feishu.cn", "http://feishu.cn"},
		{"https://nodot.example.com", "https://nodot.example.com"},
	}
	for _, tt := range tests {
		got := apiToWebDomain(tt.input)
		if got != tt.want {
			t.Errorf("apiToWebDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBuildDocumentURL(t *testing.T) {
	tests := []struct {
		baseDomain string
		docID      string
		want       string
	}{
		{"https://open.feishu.cn", "doc123", "https://feishu.cn/docx/doc123"},
		{"https://open.larksuite.com", "abc", "https://larksuite.com/docx/abc"},
		{"", "doc123", ""},
		{"https://open.feishu.cn", "", ""},
	}
	for _, tt := range tests {
		got := BuildDocumentURL(tt.baseDomain, tt.docID)
		if got != tt.want {
			t.Errorf("BuildDocumentURL(%q, %q) = %q, want %q", tt.baseDomain, tt.docID, got, tt.want)
		}
	}
}

func TestBuildWikiNodeURL(t *testing.T) {
	tests := []struct {
		baseDomain string
		nodeToken  string
		want       string
	}{
		{"https://open.feishu.cn", "node_abc", "https://feishu.cn/wiki/node_abc"},
		{"https://open.larkoffice.com", "xyz", "https://larkoffice.com/wiki/xyz"},
		{"", "node_abc", ""},
		{"https://open.feishu.cn", "", ""},
	}
	for _, tt := range tests {
		got := BuildWikiNodeURL(tt.baseDomain, tt.nodeToken)
		if got != tt.want {
			t.Errorf("BuildWikiNodeURL(%q, %q) = %q, want %q", tt.baseDomain, tt.nodeToken, got, tt.want)
		}
	}
}

func TestBuildSpreadsheetURL(t *testing.T) {
	tests := []struct {
		baseDomain string
		token      string
		want       string
	}{
		{"https://open.feishu.cn", "ss_abc", "https://feishu.cn/sheets/ss_abc"},
		{"https://open.larksuite.com", "123", "https://larksuite.com/sheets/123"},
		{"", "ss_abc", ""},
		{"https://open.feishu.cn", "", ""},
	}
	for _, tt := range tests {
		got := BuildSpreadsheetURL(tt.baseDomain, tt.token)
		if got != tt.want {
			t.Errorf("BuildSpreadsheetURL(%q, %q) = %q, want %q", tt.baseDomain, tt.token, got, tt.want)
		}
	}
}

// TestBuildURL_WithTestServerURL simulates the test server scenario where
// baseDomain is a local HTTP URL like http://127.0.0.1:PORT.
func TestBuildURL_WithTestServerURL(t *testing.T) {
	base := "http://127.0.0.1:12345"
	got := BuildDocumentURL(base, "doc_001")
	want := "http://127.0.0.1:12345/docx/doc_001"
	if got != want {
		t.Errorf("BuildDocumentURL with test server URL = %q, want %q", got, want)
	}
}
