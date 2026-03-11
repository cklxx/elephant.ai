package lark

import (
	"encoding/json"
	"strings"
	"testing"
)

func extractCardPlainText(cardJSON string) []string {
	var card map[string]any
	if err := json.Unmarshal([]byte(cardJSON), &card); err != nil {
		return nil
	}
	elements, ok := card["elements"].([]any)
	if !ok || len(elements) == 0 {
		return nil
	}
	texts := make([]string, 0, len(elements))
	for _, el := range elements {
		elem, ok := el.(map[string]any)
		if !ok {
			continue
		}
		if tag, _ := elem["tag"].(string); tag != "div" {
			continue
		}
		textNode, ok := elem["text"].(map[string]any)
		if !ok {
			continue
		}
		if nodeTag, _ := textNode["tag"].(string); nodeTag != "plain_text" {
			continue
		}
		content, _ := textNode["content"].(string)
		texts = append(texts, content)
	}
	return texts
}

func TestHasMarkdownPatterns(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "plain text",
			text: "这是一段普通的中文回复，没有任何格式标记。",
			want: false,
		},
		{
			name: "single bold pattern triggers post",
			text: "这是 **加粗** 的文字",
			want: true,
		},
		{
			name: "bold and heading",
			text: "## 标题\n这是 **加粗** 的文字",
			want: true,
		},
		{
			name: "bold and link",
			text: "**重要** 请查看 [文档](https://example.com)",
			want: true,
		},
		{
			name: "code fence and heading",
			text: "## 代码示例\n```go\nfmt.Println(\"hello\")\n```",
			want: true,
		},
		{
			name: "inline code and bold",
			text: "使用 `fmt.Println` 来输出，这是 **关键** 函数",
			want: true,
		},
		{
			name: "numbered list only - not markdown",
			text: "1. 第一步\n2. 第二步\n3. 第三步",
			want: false,
		},
		{
			name: "chinese punctuation not mistaken",
			text: "「重要」这是用中文标点强调的内容",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasMarkdownPatterns(tt.text); got != tt.want {
				t.Errorf("hasMarkdownPatterns() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSmartContent_PlainText(t *testing.T) {
	text := "这是一段普通回复，没有 Markdown。"
	msgType, content := smartContent(text)
	if msgType != "text" {
		t.Errorf("expected msgType=text, got %s", msgType)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse text content: %v", err)
	}
	if payload["text"] != text {
		t.Errorf("text mismatch: %q vs %q", payload["text"], text)
	}
}

func TestSmartContent_Markdown(t *testing.T) {
	text := "## 标题\n\n这是 **加粗** 的内容\n\n- 列表项 1\n- 列表项 2"
	msgType, content := smartContent(text)
	if msgType != "post" {
		t.Errorf("expected msgType=post, got %s", msgType)
	}
	// Verify it's valid JSON.
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse post content: %v", err)
	}
	// Should have zh_cn key.
	if _, ok := payload["zh_cn"]; !ok {
		t.Error("post payload missing zh_cn key")
	}
}

func TestBuildPostContent_HeadingAndBold(t *testing.T) {
	text := "## 分析结果\n\n这是 **重要** 的发现。"
	content := buildPostContent(text)

	var payload postPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	lines := payload.ZhCN.Content
	if len(lines) == 0 {
		t.Fatal("expected non-empty content")
	}

	// First line should be bold heading.
	if lines[0][0].Tag != "text" || len(lines[0][0].Style) == 0 || lines[0][0].Style[0] != "bold" {
		t.Errorf("expected bold heading, got %+v", lines[0][0])
	}
	if lines[0][0].Text != "分析结果" {
		t.Errorf("heading text mismatch: %q", lines[0][0].Text)
	}
}

func TestBuildPostContent_Link(t *testing.T) {
	text := "## 参考\n请看 [飞书文档](https://feishu.cn/docs)"
	content := buildPostContent(text)

	var payload postPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Find the link element.
	found := false
	for _, line := range payload.ZhCN.Content {
		for _, el := range line {
			if el.Tag == "a" && el.Href == "https://feishu.cn/docs" && el.Text == "飞书文档" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected link element not found in post content")
	}
}

func TestBuildPostContent_ListItems(t *testing.T) {
	text := "## 任务列表\n- 完成设计\n- 编写代码\n- 运行测试"
	content := buildPostContent(text)

	var payload postPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	bulletCount := 0
	for _, line := range payload.ZhCN.Content {
		for _, el := range line {
			if el.Tag == "text" && len(el.Text) > 2 && el.Text[:len("  •")] == "  •" {
				bulletCount++
			}
		}
	}
	if bulletCount != 3 {
		t.Errorf("expected 3 bullet items, got %d", bulletCount)
	}
}

func TestBuildPostContent_CodeBlock(t *testing.T) {
	text := "## 代码\n```\nfmt.Println(\"hello\")\n```"
	content := buildPostContent(text)

	var payload postPayload
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Should contain the code line as text element.
	found := false
	for _, line := range payload.ZhCN.Content {
		for _, el := range line {
			if el.Tag == "text" && el.Text == "fmt.Println(\"hello\")" {
				found = true
			}
		}
	}
	if !found {
		t.Error("code block content not preserved")
	}
}

func TestBuildPostContent_BlankLinePreservesTextField(t *testing.T) {
	t.Parallel()

	content := buildPostContent("## 标题\n\n正文")
	var payload map[string]any
	if err := json.Unmarshal([]byte(content), &payload); err != nil {
		t.Fatalf("failed to parse post content: %v", err)
	}
	zhCN, ok := payload["zh_cn"].(map[string]any)
	if !ok {
		t.Fatalf("expected zh_cn object, got %T", payload["zh_cn"])
	}
	rows, ok := zhCN["content"].([]any)
	if !ok {
		t.Fatalf("expected zh_cn.content array, got %T", zhCN["content"])
	}

	foundBlankText := false
	for _, rowRaw := range rows {
		row, ok := rowRaw.([]any)
		if !ok {
			t.Fatalf("expected row array, got %T", rowRaw)
		}
		for _, elemRaw := range row {
			elem, ok := elemRaw.(map[string]any)
			if !ok {
				t.Fatalf("expected element object, got %T", elemRaw)
			}
			tag, _ := elem["tag"].(string)
			if tag != "text" {
				continue
			}
			textVal, hasText := elem["text"]
			if !hasText {
				t.Fatalf("text tag missing text field: %+v", elem)
			}
			if text, _ := textVal.(string); text == "" {
				foundBlankText = true
			}
		}
	}
	if !foundBlankText {
		t.Fatal("expected at least one blank text line with explicit text field")
	}
}

func TestFlattenPostContentToText(t *testing.T) {
	t.Parallel()

	post := buildPostContent("## 标题\n\n正文 [链接](https://example.com)")
	plain := flattenPostContentToText(post)
	if !strings.Contains(plain, "标题") {
		t.Fatalf("flattened text missing heading: %q", plain)
	}
	if !strings.Contains(plain, "链接") {
		t.Fatalf("flattened text missing link text: %q", plain)
	}
	if !strings.Contains(plain, "https://example.com") {
		t.Fatalf("flattened text missing link url: %q", plain)
	}
}

func TestHasTableSyntax(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{
			name: "simple table",
			text: "| Name | Age |\n|------|-----|\n| Alice | 30 |",
			want: true,
		},
		{
			name: "table with alignment",
			text: "| Left | Center | Right |\n|:-----|:------:|------:|\n| a | b | c |",
			want: true,
		},
		{
			name: "table inside code block is ignored",
			text: "```\n| Name | Age |\n|------|-----|\n| Alice | 30 |\n```",
			want: false,
		},
		{
			name: "pipe in plain text is not a table",
			text: "this | is | not | a table",
			want: false,
		},
		{
			name: "single pipe line is not a table",
			text: "| only one column",
			want: false,
		},
		{
			name: "table with surrounding text",
			text: "## Results\n\n| Metric | Value |\n|--------|-------|\n| CPU | 80% |\n\nDone.",
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasTableSyntax(tt.text); got != tt.want {
				t.Errorf("hasTableSyntax() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSmartContent_Table(t *testing.T) {
	text := "## Report\n\n| Name | Score |\n|------|-------|\n| Alice | 95 |\n| Bob | 88 |"
	msgType, content := smartContent(text)
	if msgType != "interactive" {
		t.Fatalf("expected msgType=interactive, got %s", msgType)
	}
	var card map[string]any
	if err := json.Unmarshal([]byte(content), &card); err != nil {
		t.Fatalf("failed to parse card JSON: %v", err)
	}
	elements, ok := card["elements"].([]any)
	if !ok || len(elements) == 0 {
		t.Fatal("card missing elements")
	}
	elem := elements[0].(map[string]any)
	if elem["tag"] != "div" {
		t.Errorf("expected div tag, got %v", elem["tag"])
	}
	textBlocks := extractCardPlainText(content)
	if len(textBlocks) != 2 {
		t.Fatalf("expected 2 card text blocks, got %d: %+v", len(textBlocks), textBlocks)
	}
	if textBlocks[0] != "Report" {
		t.Fatalf("expected heading block to be normalized, got %q", textBlocks[0])
	}
	if got, want := textBlocks[1], "Name | Score\nAlice | 95\nBob | 88"; got != want {
		t.Fatalf("unexpected table block:\n got: %q\nwant: %q", got, want)
	}
	// Card should have no header when title is empty.
	if _, hasHeader := card["header"]; hasHeader {
		t.Error("content card should not have header when title is empty")
	}
}

func TestBuildTableSafeCard_NormalizesMentionsAndTableSyntax(t *testing.T) {
	text := "通知 @Alice(ou_123)\n\n| Owner | Status |\n|-------|--------|\n| @Alice(ou_123) | done |"
	cardJSON := buildTableSafeCard(text)
	blocks := extractCardPlainText(cardJSON)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 text blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0] != "通知 @Alice" {
		t.Fatalf("unexpected prose block: %q", blocks[0])
	}
	if got, want := blocks[1], "Owner | Status\n@Alice | done"; got != want {
		t.Fatalf("unexpected table block:\n got: %q\nwant: %q", got, want)
	}
}

func TestBuildTableSafeCard_PreservesMarkdownAroundTable(t *testing.T) {
	text := "## Summary\n\n| Name | Score |\n|------|-------|\n| Alice | 95 |\n\n- done"
	cardJSON := buildTableSafeCard(text)
	blocks := extractCardPlainText(cardJSON)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 text blocks, got %d: %+v", len(blocks), blocks)
	}
	if blocks[0] != "Summary" {
		t.Fatalf("unexpected first block: %q", blocks[0])
	}
	if blocks[1] != "Name | Score\nAlice | 95" {
		t.Fatalf("unexpected table block: %q", blocks[1])
	}
	if blocks[2] != "• done" {
		t.Fatalf("unexpected trailing block: %q", blocks[2])
	}
}

func TestExtractCardText_TableSafeCard(t *testing.T) {
	cardJSON := buildTableSafeCard("## Report\n\n| Name | Score |\n|------|-------|\n| Alice | 95 |")
	if got, want := extractCardText(cardJSON), "Report\n\nName | Score\nAlice | 95"; got != want {
		t.Fatalf("unexpected extracted card text:\n got: %q\nwant: %q", got, want)
	}
}

func TestSplitMarkdownTableCells_EscapedPipe(t *testing.T) {
	cells := splitMarkdownTableCells(`name \| alias|value`)
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d: %+v", len(cells), cells)
	}
	if cells[0] != `name | alias` {
		t.Fatalf("unexpected first cell: %q", cells[0])
	}
	if cells[1] != "value" {
		t.Fatalf("unexpected second cell: %q", cells[1])
	}
}

func TestConvertInlineMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTags []string
	}{
		{
			name:     "plain text",
			input:    "普通文字",
			wantTags: []string{"text"},
		},
		{
			name:     "bold",
			input:    "这是 **加粗** 内容",
			wantTags: []string{"text", "text", "text"},
		},
		{
			name:     "link",
			input:    "看 [这里](https://example.com) 了解更多",
			wantTags: []string{"text", "a", "text"},
		},
		{
			name:     "inline code",
			input:    "运行 `go test` 命令",
			wantTags: []string{"text", "text", "text"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			elems := convertInlineMarkdown(tt.input)
			if len(elems) != len(tt.wantTags) {
				t.Errorf("expected %d elements, got %d: %+v", len(tt.wantTags), len(elems), elems)
				return
			}
			for i, want := range tt.wantTags {
				if elems[i].Tag != want {
					t.Errorf("element[%d] tag = %q, want %q", i, elems[i].Tag, want)
				}
			}
		})
	}
}
