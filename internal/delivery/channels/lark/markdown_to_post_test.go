package lark

import (
	"encoding/json"
	"testing"
)

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
			name: "single pattern not enough",
			text: "这是 **加粗** 的文字",
			want: false,
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
