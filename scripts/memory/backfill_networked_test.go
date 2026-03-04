package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunes(t *testing.T) {
	t.Parallel()

	const input = "这是一个用于验证按 rune 截断是否安全的测试字符串。"
	got := truncateRunes(input, 8)
	if !utf8.ValidString(got) {
		t.Fatalf("expected valid utf-8, got %q", got)
	}
	if strings.Count(got, "...") != 1 {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
}

func TestExtractSummaryAndTags_UsesRuneSafeTruncation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "entry.md")
	line := strings.Repeat("这是中文摘要。", 50)
	content := "# Title\n\n" + line + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	summary, _, err := extractSummaryAndTags(path, "error_summary", "sample")
	if err != nil {
		t.Fatalf("extract summary: %v", err)
	}
	if !utf8.ValidString(summary) {
		t.Fatalf("expected valid utf-8 summary, got %q", summary)
	}
	if !strings.HasSuffix(summary, "...") {
		t.Fatalf("expected truncated summary with ellipsis, got %q", summary)
	}
	if len([]rune(summary)) != 183 {
		t.Fatalf("expected 183 runes (180 + ellipsis), got %d", len([]rune(summary)))
	}
}
