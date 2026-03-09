package lark

import (
	"strings"
	"testing"
	"time"
)

func TestSplitMessageShortText(t *testing.T) {
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 400}
	result := splitMessage("一段短文本", cfg)
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(result))
	}
	if result[0] != "一段短文本" {
		t.Fatalf("unexpected content: %q", result[0])
	}
}

func TestSplitMessageEmpty(t *testing.T) {
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 400}
	result := splitMessage("", cfg)
	if len(result) != 1 || result[0] != "" {
		t.Fatalf("expected 1 empty chunk, got %d chunks: %v", len(result), result)
	}
}

func TestSplitMessageWhitespace(t *testing.T) {
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 400}
	result := splitMessage("   \n\n  ", cfg)
	if len(result) != 1 || result[0] != "" {
		t.Fatalf("expected 1 empty chunk for whitespace, got %d: %v", len(result), result)
	}
}

func TestSplitMessageMultipleParagraphs(t *testing.T) {
	// Create 3 paragraphs, each ~150 chars (Chinese chars).
	para := strings.Repeat("测试文本内容。", 25) // ~150 chars
	text := para + "\n\n" + para + "\n\n" + para
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 400}
	result := splitMessage(text, cfg)
	if len(result) < 2 {
		t.Fatalf("expected at least 2 chunks for 3 long paragraphs, got %d", len(result))
	}
	// Verify no chunk exceeds ~400 chars (except when a single segment is longer).
	for i, chunk := range result {
		if i < len(result)-1 && len([]rune(chunk)) > 500 { // generous buffer
			t.Errorf("chunk %d exceeds limit: %d chars", i, len([]rune(chunk)))
		}
	}
}

func TestSplitMessageCodeFenceIntact(t *testing.T) {
	text := "看下这段代码\n\n```\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n后续说明文字后续说明文字后续说明文字"
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 50}
	result := splitMessage(text, cfg)
	// The code fence should be in a single chunk.
	foundFence := false
	for _, chunk := range result {
		if strings.Contains(chunk, "```") {
			if strings.Count(chunk, "```") < 2 {
				// Unclosed fence in plan — should have opening and closing.
				// Only if the original had both.
				if strings.Count(text, "```") >= 2 {
					t.Errorf("code fence split across chunks: %q", chunk)
				}
			}
			foundFence = true
		}
	}
	if !foundFence && strings.Contains(text, "```") {
		t.Error("code fence disappeared from output")
	}
}

func TestSplitMessageNumberedListIntact(t *testing.T) {
	text := "总结如下\n\n1. 第一点内容描述\n2. 第二点内容描述\n3. 第三点内容描述\n\n最后一段"
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 50}
	result := splitMessage(text, cfg)
	// Find the chunk with numbered list items.
	for _, chunk := range result {
		if strings.Contains(chunk, "1.") {
			// All three items should be together.
			if !strings.Contains(chunk, "2.") || !strings.Contains(chunk, "3.") {
				t.Errorf("numbered list was split: %q", chunk)
			}
		}
	}
}

func TestSplitMessageMaxChunks(t *testing.T) {
	// Create 10 separate paragraphs.
	var parts []string
	for i := 0; i < 10; i++ {
		parts = append(parts, strings.Repeat("段落内容。", 20))
	}
	text := strings.Join(parts, "\n\n")
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 100, MaxChunks: 5}
	result := splitMessage(text, cfg)
	if len(result) > 5 {
		t.Fatalf("expected at most 5 chunks (MaxChunks=5), got %d", len(result))
	}
}

func TestSplitMessageConfigDefaults(t *testing.T) {
	cfg := MessageSplitConfig{}
	if cfg.maxChunkChars() != 400 {
		t.Errorf("expected default maxChunkChars=400, got %d", cfg.maxChunkChars())
	}
	if cfg.maxChunks() != 5 {
		t.Errorf("expected default maxChunks=5, got %d", cfg.maxChunks())
	}
	if cfg.delayBetween() != 500*time.Millisecond {
		t.Errorf("expected default delayBetween=500ms, got %v", cfg.delayBetween())
	}
}

func TestSplitMessageSingleLongParagraph(t *testing.T) {
	// A single paragraph with no natural break points — should still return it.
	text := strings.Repeat("长段落文字", 200)
	cfg := MessageSplitConfig{Enabled: true, MaxChunkChars: 400}
	result := splitMessage(text, cfg)
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk for single unbreakable paragraph, got %d", len(result))
	}
	if result[0] != text {
		t.Error("content was modified")
	}
}
