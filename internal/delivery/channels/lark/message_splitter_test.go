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
	// Intro + list should merge into one chunk, then "最后一段" separate.
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks (intro+list merged, tail), got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "总结如下") {
		t.Errorf("expected intro in chunk 0, got %q", result[0])
	}
	if !strings.Contains(result[0], "1.") || !strings.Contains(result[0], "2.") || !strings.Contains(result[0], "3.") {
		t.Errorf("numbered list was split from intro: %q", result[0])
	}
}

func TestSplitMessageMaxChunks(t *testing.T) {
	var parts []string
	for i := 0; i < 12; i++ {
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

// --- Heading-based structural tests ---

func TestSplitMessageHeadingSections(t *testing.T) {
	text := "## 总结\n\n这是总结内容。\n\n补充说明。\n\n## 下一步\n\n接下来做什么。"
	result := splitMessage(text)
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
	if !strings.Contains(result[0], "```go") {
		t.Errorf("code fence should be in section 0: %q", result[0])
	}
}

func TestSplitMessageHeadingCodeFenceWithFakeHeading(t *testing.T) {
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
	if len(result) != 2 {
		t.Fatalf("expected 2 chunks (intro+list, tail), got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "需要注意") {
		t.Errorf("intro should be in chunk 0: %q", result[0])
	}
	if !strings.Contains(result[0], "- 第一点") {
		t.Errorf("list should be merged with intro in chunk 0: %q", result[0])
	}
}

func TestSplitMessageSingleHeading(t *testing.T) {
	text := "## 标题\n\n段落一\n\n段落二"
	result := splitMessage(text)
	if len(result) != 1 {
		t.Fatalf("expected 1 section for single heading, got %d: %v", len(result), result)
	}
}

func TestSplitMessageBulletListIntact(t *testing.T) {
	text := "要点\n\n- A\n- B\n- C"
	result := splitMessage(text)
	if len(result) != 1 {
		t.Fatalf("expected 1 chunk (intro merged with list), got %d: %v", len(result), result)
	}
}

// --- AST-specific structural tests ---

func TestSplitMessageBlockquoteIntact(t *testing.T) {
	text := "引用说明\n\n> 这是一段引用\n> 引用第二行\n\n后续内容"
	result := splitMessage(text)
	// Blockquote should be a single AST node, not split by internal newlines.
	found := false
	for _, chunk := range result {
		if strings.Contains(chunk, "> 这是一段引用") && strings.Contains(chunk, "> 引用第二行") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("blockquote was split across chunks: %v", result)
	}
}

func TestSplitMessageTableIntact(t *testing.T) {
	text := "数据表\n\n| 名称 | 值 |\n|------|----|\n| A | 1 |\n| B | 2 |\n\n结论"
	result := splitMessage(text)
	// Table should remain in one chunk.
	found := false
	for _, chunk := range result {
		if strings.Contains(chunk, "| 名称") && strings.Contains(chunk, "| B") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("table was split across chunks: %v", result)
	}
}

func TestSplitMessageNestedListIntact(t *testing.T) {
	text := "概述\n\n- 项目一\n  - 子项 A\n  - 子项 B\n- 项目二\n\n总结"
	result := splitMessage(text)
	found := false
	for _, chunk := range result {
		if strings.Contains(chunk, "项目一") && strings.Contains(chunk, "子项 B") && strings.Contains(chunk, "项目二") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("nested list was split: %v", result)
	}
}

func TestSplitMessageHeadingWithTable(t *testing.T) {
	text := "## 分析\n\n| 指标 | 结果 |\n|------|------|\n| CPU | 90% |\n\n## 建议\n\n降低负载。"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "| CPU") {
		t.Errorf("table should stay in heading section: %q", result[0])
	}
}

func TestSplitMessageHeadingWithBlockquote(t *testing.T) {
	text := "## 引用\n\n> 重要提示\n> 请注意\n\n## 操作\n\n执行步骤。"
	result := splitMessage(text)
	if len(result) != 2 {
		t.Fatalf("expected 2 sections, got %d: %v", len(result), result)
	}
	if !strings.Contains(result[0], "> 重要提示") {
		t.Errorf("blockquote should stay in heading section: %q", result[0])
	}
}

func TestSplitMessageMixedStructure(t *testing.T) {
	text := "## 背景\n\n情况说明。\n\n> 关键引用\n\n## 方案\n\n1. 第一步\n2. 第二步\n\n```go\nfmt.Println(\"ok\")\n```\n\n## 总结\n\n完成。"
	result := splitMessage(text)
	if len(result) != 3 {
		t.Fatalf("expected 3 sections, got %d: %v", len(result), result)
	}
	// Section 0: 背景 + body + blockquote
	if !strings.Contains(result[0], "关键引用") {
		t.Errorf("blockquote should be in section 0: %q", result[0])
	}
	// Section 1: 方案 + list + code
	if !strings.Contains(result[1], "1. 第一步") {
		t.Errorf("list should be in section 1: %q", result[1])
	}
	if !strings.Contains(result[1], "fmt.Println") {
		t.Errorf("code block should be in section 1: %q", result[1])
	}
}
