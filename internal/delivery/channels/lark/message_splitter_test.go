package lark

import (
	"strings"
	"testing"
)

func TestSplitMessageShortText(t *testing.T) {
	result := splitMessage("一段短文本")
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(result))
	}
	if result[0] != "一段短文本" {
		t.Fatalf("unexpected content: %q", result[0])
	}
}

func TestSplitMessageEmpty(t *testing.T) {
	result := splitMessage("")
	if len(result) != 1 || result[0] != "" {
		t.Fatalf("expected 1 empty chunk, got %d chunks: %v", len(result), result)
	}
}

func TestSplitMessageWhitespace(t *testing.T) {
	result := splitMessage("   \n\n  ")
	if len(result) != 1 || result[0] != "" {
		t.Fatalf("expected 1 empty chunk for whitespace, got %d: %v", len(result), result)
	}
}

func TestSplitMessageNoParagraphBreak(t *testing.T) {
	// Single paragraph, no \n\n — should not split regardless of length.
	text := strings.Repeat("很长的一段文字没有换行。", 100)
	result := splitMessage(text)
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk for single paragraph, got %d", len(result))
	}
}

func TestSplitMessageTwoParagraphs(t *testing.T) {
	text := "第一段内容\n\n第二段内容"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks, got %d: %v", len(result), result)
	}
	if result[0] != "第一段内容" {
		t.Errorf("chunk 0: %q", result[0])
	}
	if result[1] != "第二段内容" {
		t.Errorf("chunk 1: %q", result[1])
	}
}

func TestSplitMessageThreeParagraphs(t *testing.T) {
	text := "段落一\n\n段落二\n\n段落三"
	result := splitMessage(text)
	if len(result) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(result))
	}
}

func TestSplitMessageCodeFenceIntact(t *testing.T) {
	text := "看下这段代码\n\n```\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```\n\n后续说明"
	result := splitMessage(text)
	if len(result) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(result), result)
	}
	// Code fence should be entirely in chunk 1.
	if !strings.HasPrefix(result[1], "```") {
		t.Errorf("expected code fence in chunk 1, got %q", result[1])
	}
	if strings.Count(result[1], "```") < 2 {
		t.Errorf("code fence split: %q", result[1])
	}
}

func TestSplitMessageNumberedListIntact(t *testing.T) {
	text := "总结如下\n\n1. 第一点\n2. 第二点\n3. 第三点\n\n最后一段"
	result := splitMessage(text)
	if len(result) != 3 {
		t.Fatalf("expected 3 chunks, got %d: %v", len(result), result)
	}
	// All list items should be in one chunk.
	listChunk := result[1]
	if !strings.Contains(listChunk, "1.") || !strings.Contains(listChunk, "2.") || !strings.Contains(listChunk, "3.") {
		t.Errorf("numbered list was split: %q", listChunk)
	}
}

func TestSplitMessageMaxChunks(t *testing.T) {
	// Create 8 paragraphs — should be capped to 5 chunks.
	var parts []string
	for i := 0; i < 8; i++ {
		parts = append(parts, "段落内容")
	}
	text := strings.Join(parts, "\n\n")
	result := splitMessage(text)
	if len(result) > messageSplitMaxChunks {
		t.Fatalf("expected at most %d chunks, got %d", messageSplitMaxChunks, len(result))
	}
	// Last chunk should contain the merged trailing paragraphs.
	if !strings.Contains(result[len(result)-1], "\n\n") {
		t.Error("expected trailing paragraphs merged into last chunk")
	}
}

func TestSplitMessagePreservesContent(t *testing.T) {
	text := "段落一\n\n段落二\n\n段落三"
	result := splitMessage(text)
	rejoined := strings.Join(result, "\n\n")
	if rejoined != text {
		t.Errorf("content changed after split+rejoin:\noriginal: %q\nrejoined: %q", text, rejoined)
	}
}
