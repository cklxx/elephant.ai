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
	// "总结如下" + list should merge into one chunk, then "最后一段" separate.
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks (intro+list merged, tail), got %d: %v", len(result), result)
	}
	// First chunk should contain both intro and all list items.
	if !strings.Contains(result[0], "总结如下") {
		t.Errorf("expected intro in chunk 0, got %q", result[0])
	}
	if !strings.Contains(result[0], "1.") || !strings.Contains(result[0], "2.") || !strings.Contains(result[0], "3.") {
		t.Errorf("numbered list was split from intro: %q", result[0])
	}
}

func TestSplitMessageMaxChunks(t *testing.T) {
	var parts []string
	for i := 0; i < 8; i++ {
		parts = append(parts, "段落内容")
	}
	text := strings.Join(parts, "\n\n")
	result := splitMessage(text)
	if len(result) > messageSplitMaxChunks {
		t.Fatalf("expected at most %d chunks, got %d", messageSplitMaxChunks, len(result))
	}
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

// --- New tests for structural splitting ---

func TestSplitMessageHeadingSections(t *testing.T) {
	text := "## 总结\n\n这是总结内容。\n\n补充说明。\n\n## 下一步\n\n接下来做什么。"
	result := splitMessage(text)
	// Should split into 2 sections by heading.
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	if !strings.HasPrefix(result[0], "## 总结") {
		t.Errorf("section 0 should start with heading, got %q", result[0])
	}
	if !strings.Contains(result[0], "这是总结内容") {
		t.Errorf("section 0 should contain body, got %q", result[0])
	}
	if !strings.Contains(result[0], "补充说明") {
		t.Errorf("section 0 should contain all body paragraphs, got %q", result[0])
	}
	if !strings.HasPrefix(result[1], "## 下一步") {
		t.Errorf("section 1 should start with heading, got %q", result[1])
	}
}

func TestSplitMessageHeadingWithCodeFence(t *testing.T) {
	text := "## 代码\n\n```go\nfunc main() {}\n```\n\n## 说明\n\n解释一下。"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	// Code fence should stay with its heading section.
	if !strings.Contains(result[0], "```go") {
		t.Errorf("code fence should be in section 0: %q", result[0])
	}
}

func TestSplitMessageHeadingCodeFenceWithFakeHeading(t *testing.T) {
	// A heading-like line inside a code fence should NOT trigger a split.
	text := "## 开始\n\n```\n## 这不是标题\nsome code\n```\n\n## 结束\n\n完了。"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "## 这不是标题") {
		t.Errorf("fake heading in code fence should stay in section 0: %q", result[0])
	}
}

func TestSplitMessageContentBeforeFirstHeading(t *testing.T) {
	text := "前言内容\n\n## 第一节\n\n正文内容"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	if result[0] != "前言内容" {
		t.Errorf("pre-heading content should be section 0, got %q", result[0])
	}
}

func TestSplitMessageIntroWithBulletList(t *testing.T) {
	text := "需要注意以下几点\n\n- 第一点\n- 第二点\n- 第三点\n\n最后总结"
	result := splitMessage(text)
	// Intro + bullet list should merge.
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks (intro+list, tail), got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "需要注意") && !strings.Contains(result[0], "- 第一点") {
		t.Errorf("intro and list should be merged in chunk 0: %q", result[0])
	}
}

func TestSplitMessageSingleHeading(t *testing.T) {
	text := "## 标题\n\n段落一\n\n段落二"
	result := splitMessage(text)
	// Single heading = single section, no split.
	if len(result) != 1 {
		t.Fatalf("expected 1 section for single heading, got %d: %v", len(result), result)
	}
}

func TestSplitMessageBulletListIntact(t *testing.T) {
	text := "要点\n\n- A\n- B\n- C"
	result := splitMessage(text)
	// Intro + list merged = single chunk.
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk (intro merged with list), got %d: %v", len(result), result)
	}
}

func TestIsHeadingLine(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"# Title", true},
		{"## Section", true},
		{"### Sub", true},
		{"###### Deep", true},
		{"####### TooDeep", false},
		{"#NoSpace", false},
		{"Not a heading", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isHeadingLine(tt.line); got != tt.want {
			t.Errorf("isHeadingLine(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}
