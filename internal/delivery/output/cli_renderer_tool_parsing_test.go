package output

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSummarizeFileOperation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
		ok      bool
	}{
		{"empty", "", "", false},
		{"wrote with bytes", "Wrote 1024 bytes to main.go", "wrote main.go (1.0 KB)", true},
		{"wrote without valid bytes", "Wrote many bytes to main.go", "wrote main.go", true},
		{"created file", "Created main.go (42 lines)", "created main.go (42 lines)", true},
		{"created file no parens", "Created main.go", "created main.go", true},
		{"updated file", "Updated main.go (10 lines)", "updated main.go (10 lines)", true},
		{"replaced in file", "Replaced 3 occurrences in main.go", "replaced 3 occurrences in main.go", true},
		{"unrecognized", "Something else happened", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := summarizeFileOperation(tc.content)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestParseFileLineSummary(t *testing.T) {
	tests := []struct {
		name      string
		action    string
		remainder string
		want      string
		ok        bool
	}{
		{"empty remainder", "created", "", "", false},
		{"with line count", "created", "main.go (42 lines)", "created main.go (42 lines)", true},
		{"no parenthesized suffix", "updated", "main.go", "updated main.go", true},
		{"with paren but no closing", "created", "main.go (42 lines", "created main.go (42 lines", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseFileLineSummary(tc.action, tc.remainder)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestParseWebSearchContent(t *testing.T) {
	t.Run("full search content", func(t *testing.T) {
		content := `Search: golang testing
Summary: Go has great testing support
3 Results:
1. Go Testing Basics
URL: https://example.com/1
2. Advanced Testing
URL: https://example.com/2
3. Table-Driven Tests
URL: https://example.com/3`

		summary := parseWebSearchContent(content)
		assert.Equal(t, "golang testing", summary.Query)
		assert.Equal(t, "Go has great testing support", summary.Summary)
		assert.Equal(t, 3, summary.ResultCount)
		assert.Len(t, summary.Results, 3)
		assert.Equal(t, "Go Testing Basics", summary.Results[0].Title)
		assert.Equal(t, "https://example.com/1", summary.Results[0].URL)
	})

	t.Run("empty content", func(t *testing.T) {
		summary := parseWebSearchContent("")
		assert.Empty(t, summary.Query)
		assert.Equal(t, 0, summary.ResultCount)
	})

	t.Run("results without explicit count", func(t *testing.T) {
		content := `Search: test query
1. First Result
2. Second Result`

		summary := parseWebSearchContent(content)
		assert.Equal(t, "test query", summary.Query)
		assert.Equal(t, 2, summary.ResultCount)
		assert.Len(t, summary.Results, 2)
	})
}

func TestParseNumberedTitle(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		want  string
		ok    bool
	}{
		{"valid numbered", "1. Hello", "Hello", true},
		{"two digits", "12. World", "World", true},
		{"no dot", "1 Hello", "", false},
		{"dot at start", ".Hello", "", false},
		{"non-numeric prefix", "a. Hello", "", false},
		{"empty after dot", "1.", "", false},
		{"empty string", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parseNumberedTitle(tc.line)
			assert.Equal(t, tc.ok, ok)
			if ok {
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestHostFromURL(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		expect string
	}{
		{"valid URL", "https://example.com/path", "example.com"},
		{"URL with port", "https://example.com:8080/path", "example.com:8080"},
		{"empty", "", ""},
		{"not a URL", "not-a-url", "not-a-url"},
		{"whitespace trimmed", "  https://example.com  ", "example.com"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expect, hostFromURL(tc.raw))
		})
	}
}
