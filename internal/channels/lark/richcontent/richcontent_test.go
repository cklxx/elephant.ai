package richcontent

import (
	"encoding/json"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// mustParseJSON unmarshals a JSON string into a generic map.
func mustParseJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("invalid JSON: %v\nraw: %s", err, raw)
	}
	return m
}

// postContent extracts the content array from a post JSON at the given locale.
func postContent(t *testing.T, m map[string]any, locale string) []any {
	t.Helper()
	loc, ok := m[locale].(map[string]any)
	if !ok {
		t.Fatalf("missing locale %q in post JSON", locale)
	}
	content, ok := loc["content"].([]any)
	if !ok {
		t.Fatal("missing or invalid content array")
	}
	return content
}

// postTitle extracts the title string from a post JSON at the given locale.
func postTitle(t *testing.T, m map[string]any, locale string) string {
	t.Helper()
	loc, ok := m[locale].(map[string]any)
	if !ok {
		t.Fatalf("missing locale %q in post JSON", locale)
	}
	title, _ := loc["title"].(string)
	return title
}

// paragraph extracts a single paragraph (line) from the content array.
func paragraph(t *testing.T, content []any, idx int) []any {
	t.Helper()
	if idx >= len(content) {
		t.Fatalf("paragraph index %d out of range (len=%d)", idx, len(content))
	}
	para, ok := content[idx].([]any)
	if !ok {
		t.Fatalf("paragraph %d is not an array", idx)
	}
	return para
}

// elem extracts a single element from a paragraph.
func elem(t *testing.T, para []any, idx int) map[string]any {
	t.Helper()
	if idx >= len(para) {
		t.Fatalf("element index %d out of range (len=%d)", idx, len(para))
	}
	e, ok := para[idx].(map[string]any)
	if !ok {
		t.Fatalf("element %d is not an object", idx)
	}
	return e
}

// ---------------------------------------------------------------------------
// PostBuilder tests
// ---------------------------------------------------------------------------

func TestNewPostBuilder_BasicBuild(t *testing.T) {
	result := NewPostBuilder("Test Title").Build()
	m := mustParseJSON(t, result)

	title := postTitle(t, m, "zh_cn")
	if title != "Test Title" {
		t.Errorf("expected title 'Test Title', got %q", title)
	}

	content := postContent(t, m, "zh_cn")
	if len(content) != 1 {
		t.Errorf("expected 1 paragraph, got %d", len(content))
	}
}

func TestPostBuilder_SetLocale(t *testing.T) {
	result := NewPostBuilder("Hello").SetLocale("en_us").Build()
	m := mustParseJSON(t, result)

	if _, ok := m["en_us"]; !ok {
		t.Error("expected locale 'en_us' in output")
	}
	if _, ok := m["zh_cn"]; ok {
		t.Error("did not expect locale 'zh_cn' in output")
	}
}

func TestPostBuilder_SetLocaleEmpty(t *testing.T) {
	result := NewPostBuilder("Hello").SetLocale("").Build()
	m := mustParseJSON(t, result)
	if _, ok := m["zh_cn"]; !ok {
		t.Error("empty locale should keep default 'zh_cn'")
	}
}

func TestPostBuilder_AddText(t *testing.T) {
	result := NewPostBuilder("").AddText("hello world").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "text" {
		t.Errorf("expected tag 'text', got %v", e["tag"])
	}
	if e["text"] != "hello world" {
		t.Errorf("expected text 'hello world', got %v", e["text"])
	}
}

func TestPostBuilder_AddBold(t *testing.T) {
	result := NewPostBuilder("").AddBold("important").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "text" {
		t.Errorf("expected tag 'text', got %v", e["tag"])
	}
	if e["text"] != "important" {
		t.Errorf("expected text 'important', got %v", e["text"])
	}

	style, ok := e["style"].([]any)
	if !ok {
		t.Fatal("expected style array")
	}
	if len(style) != 1 || style[0] != "bold" {
		t.Errorf("expected style ['bold'], got %v", style)
	}
}

func TestPostBuilder_AddItalic(t *testing.T) {
	result := NewPostBuilder("").AddItalic("emphasis").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	style, ok := e["style"].([]any)
	if !ok {
		t.Fatal("expected style array")
	}
	if len(style) != 1 || style[0] != "italic" {
		t.Errorf("expected style ['italic'], got %v", style)
	}
}

func TestPostBuilder_AddBoldItalic(t *testing.T) {
	result := NewPostBuilder("").AddBoldItalic("both").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	style, ok := e["style"].([]any)
	if !ok {
		t.Fatal("expected style array")
	}
	if len(style) != 2 {
		t.Fatalf("expected 2 style entries, got %d", len(style))
	}
	if style[0] != "bold" || style[1] != "italic" {
		t.Errorf("expected style ['bold', 'italic'], got %v", style)
	}
}

func TestPostBuilder_AddLink(t *testing.T) {
	result := NewPostBuilder("").AddLink("Click here", "https://example.com").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "a" {
		t.Errorf("expected tag 'a', got %v", e["tag"])
	}
	if e["text"] != "Click here" {
		t.Errorf("expected text 'Click here', got %v", e["text"])
	}
	if e["href"] != "https://example.com" {
		t.Errorf("expected href 'https://example.com', got %v", e["href"])
	}
}

func TestPostBuilder_AddCodeBlock(t *testing.T) {
	code := "fmt.Println(\"hello\")"
	result := NewPostBuilder("").AddCodeBlock(code, "go").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "code_block" {
		t.Errorf("expected tag 'code_block', got %v", e["tag"])
	}
	if e["text"] != code {
		t.Errorf("expected code text, got %v", e["text"])
	}
	if e["language"] != "go" {
		t.Errorf("expected language 'go', got %v", e["language"])
	}
}

func TestPostBuilder_AddMention(t *testing.T) {
	result := NewPostBuilder("").AddMention("ou_xxx123", "Alice").Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "at" {
		t.Errorf("expected tag 'at', got %v", e["tag"])
	}
	if e["user_id"] != "ou_xxx123" {
		t.Errorf("expected user_id 'ou_xxx123', got %v", e["user_id"])
	}
	if e["user_name"] != "Alice" {
		t.Errorf("expected user_name 'Alice', got %v", e["user_name"])
	}
}

func TestPostBuilder_AddImage(t *testing.T) {
	result := NewPostBuilder("").AddImage("img_v2_xxx", 200, 100).Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "img" {
		t.Errorf("expected tag 'img', got %v", e["tag"])
	}
	if e["image_key"] != "img_v2_xxx" {
		t.Errorf("expected image_key 'img_v2_xxx', got %v", e["image_key"])
	}
	// JSON numbers are float64 after unmarshal.
	if w, _ := e["width"].(float64); w != 200 {
		t.Errorf("expected width 200, got %v", e["width"])
	}
	if h, _ := e["height"].(float64); h != 100 {
		t.Errorf("expected height 100, got %v", e["height"])
	}
}

func TestPostBuilder_AddImageZeroDimensions(t *testing.T) {
	result := NewPostBuilder("").AddImage("img_key", 0, 0).Build()
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if _, hasWidth := e["width"]; hasWidth {
		t.Error("zero width should not be included")
	}
	if _, hasHeight := e["height"]; hasHeight {
		t.Error("zero height should not be included")
	}
}

func TestPostBuilder_NewLine(t *testing.T) {
	result := NewPostBuilder("Lines").
		AddText("line one").
		NewLine().
		AddText("line two").
		NewLine().
		AddText("line three").
		Build()

	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")

	if len(content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(content))
	}

	texts := []string{"line one", "line two", "line three"}
	for i, expected := range texts {
		para := paragraph(t, content, i)
		e := elem(t, para, 0)
		if e["text"] != expected {
			t.Errorf("paragraph %d: expected %q, got %v", i, expected, e["text"])
		}
	}
}

func TestPostBuilder_MethodChaining(t *testing.T) {
	result := NewPostBuilder("Report").
		AddBold("Status: ").
		AddText("OK").
		NewLine().
		AddText("See ").
		AddLink("details", "https://example.com").
		NewLine().
		AddItalic("Note: ").
		AddMention("ou_123", "Bob").
		Build()

	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")

	if len(content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(content))
	}

	// First paragraph: bold + text.
	p0 := paragraph(t, content, 0)
	if len(p0) != 2 {
		t.Fatalf("paragraph 0: expected 2 elements, got %d", len(p0))
	}

	// Second paragraph: text + link.
	p1 := paragraph(t, content, 1)
	if len(p1) != 2 {
		t.Fatalf("paragraph 1: expected 2 elements, got %d", len(p1))
	}
	linkElem := elem(t, p1, 1)
	if linkElem["tag"] != "a" {
		t.Errorf("expected link tag 'a', got %v", linkElem["tag"])
	}

	// Third paragraph: italic + mention.
	p2 := paragraph(t, content, 2)
	if len(p2) != 2 {
		t.Fatalf("paragraph 2: expected 2 elements, got %d", len(p2))
	}
	mentionElem := elem(t, p2, 1)
	if mentionElem["tag"] != "at" {
		t.Errorf("expected mention tag 'at', got %v", mentionElem["tag"])
	}
}

func TestPostBuilder_EmptyTitle(t *testing.T) {
	result := NewPostBuilder("").AddText("content").Build()
	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "" {
		t.Errorf("expected empty title, got %q", title)
	}
}

func TestPostBuilder_ValidJSON(t *testing.T) {
	result := NewPostBuilder("JSON Test").
		AddText("normal ").
		AddBold("bold ").
		AddItalic("italic ").
		AddLink("link", "https://example.com").
		NewLine().
		AddMention("ou_1", "User").
		AddCodeBlock("x := 1", "go").
		Build()

	// Verify it round-trips.
	var intermediate any
	if err := json.Unmarshal([]byte(result), &intermediate); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	reJSON, err := json.Marshal(intermediate)
	if err != nil {
		t.Fatalf("re-marshal failed: %v", err)
	}
	if len(reJSON) == 0 {
		t.Error("re-marshaled JSON is empty")
	}
}

func TestPostBuilder_SpecialCharacters(t *testing.T) {
	result := NewPostBuilder("Special").
		AddText("quotes: \"hello\" & <world>").
		Build()

	// Must be valid JSON.
	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("invalid JSON with special chars: %v", err)
	}

	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)
	if e["text"] != "quotes: \"hello\" & <world>" {
		t.Errorf("special characters not preserved: %v", e["text"])
	}
}

func TestPostBuilder_MultipleNewLinesEmpty(t *testing.T) {
	// Multiple NewLine calls create empty paragraphs.
	result := NewPostBuilder("").
		AddText("first").
		NewLine().
		NewLine().
		AddText("after gap").
		Build()

	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	if len(content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(content))
	}

	// Middle paragraph should be empty.
	p1 := paragraph(t, content, 1)
	if len(p1) != 0 {
		t.Errorf("expected empty middle paragraph, got %d elements", len(p1))
	}
}

// ---------------------------------------------------------------------------
// TableBuilder tests
// ---------------------------------------------------------------------------

func TestTableBuilder_BasicTable(t *testing.T) {
	result := NewTableBuilder([]string{"Name", "Status"}).
		AddRow([]string{"Task 1", "Done"}).
		AddRow([]string{"Task 2", "Active"}).
		Build()

	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines, got %d:\n%s", len(lines), result)
	}

	// Header line.
	if !strings.Contains(lines[0], "Name") || !strings.Contains(lines[0], "Status") {
		t.Errorf("header missing expected values: %s", lines[0])
	}

	// Separator line.
	if !strings.Contains(lines[1], "---") {
		t.Errorf("expected separator with dashes: %s", lines[1])
	}

	// Data rows.
	if !strings.Contains(lines[2], "Task 1") || !strings.Contains(lines[2], "Done") {
		t.Errorf("row 1 missing expected values: %s", lines[2])
	}
	if !strings.Contains(lines[3], "Task 2") || !strings.Contains(lines[3], "Active") {
		t.Errorf("row 2 missing expected values: %s", lines[3])
	}
}

func TestTableBuilder_EmptyHeaders(t *testing.T) {
	result := NewTableBuilder(nil).Build()
	if result != "" {
		t.Errorf("expected empty string for nil headers, got %q", result)
	}

	result = NewTableBuilder([]string{}).Build()
	if result != "" {
		t.Errorf("expected empty string for empty headers, got %q", result)
	}
}

func TestTableBuilder_NoRows(t *testing.T) {
	result := NewTableBuilder([]string{"Col1", "Col2"}).Build()
	lines := strings.Split(result, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (header + separator), got %d:\n%s", len(lines), result)
	}
}

func TestTableBuilder_FewerCellsThanHeaders(t *testing.T) {
	result := NewTableBuilder([]string{"A", "B", "C"}).
		AddRow([]string{"only one"}).
		Build()

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// The data row should have pipes for all columns.
	pipeCount := strings.Count(lines[2], "|")
	if pipeCount != 4 { // leading + 3 separators
		t.Errorf("expected 4 pipes in data row, got %d: %s", pipeCount, lines[2])
	}
}

func TestTableBuilder_MoreCellsThanHeaders(t *testing.T) {
	result := NewTableBuilder([]string{"A"}).
		AddRow([]string{"one", "extra", "cells"}).
		Build()

	lines := strings.Split(result, "\n")
	dataRow := lines[2]
	// Only 1 column, so extra cells should be silently ignored.
	pipeCount := strings.Count(dataRow, "|")
	if pipeCount != 2 {
		t.Errorf("expected 2 pipes for single column, got %d: %s", pipeCount, dataRow)
	}
	if !strings.Contains(dataRow, "one") {
		t.Errorf("expected 'one' in data row: %s", dataRow)
	}
}

func TestTableBuilder_Alignment(t *testing.T) {
	result := NewTableBuilder([]string{"Name", "X"}).
		AddRow([]string{"Long Name Here", "Y"}).
		Build()

	lines := strings.Split(result, "\n")
	// Header and data row pipe positions should be consistent.
	headerPipes := pipePositions(lines[0])
	dataPipes := pipePositions(lines[2])

	if len(headerPipes) != len(dataPipes) {
		t.Fatalf("pipe count mismatch: header=%d data=%d", len(headerPipes), len(dataPipes))
	}
	for i := range headerPipes {
		if headerPipes[i] != dataPipes[i] {
			t.Errorf("pipe position mismatch at index %d: header=%d data=%d",
				i, headerPipes[i], dataPipes[i])
		}
	}
}

func TestTableBuilder_BuildPost(t *testing.T) {
	result := NewTableBuilder([]string{"H1", "H2"}).
		AddRow([]string{"a", "b"}).
		BuildPost("Table Title")

	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "Table Title" {
		t.Errorf("expected title 'Table Title', got %q", title)
	}

	// Content should contain the table text.
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)
	text, _ := e["text"].(string)
	if !strings.Contains(text, "H1") || !strings.Contains(text, "H2") {
		t.Errorf("post content missing table headers: %s", text)
	}
}

func TestTableBuilder_BuildPostEmptyHeaders(t *testing.T) {
	result := NewTableBuilder(nil).BuildPost("Empty")
	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "Empty" {
		t.Errorf("expected title 'Empty', got %q", title)
	}
}

func TestFormatTableJSON(t *testing.T) {
	result := FormatTableJSON("My Table",
		[]string{"Key", "Value"},
		[][]string{{"name", "alice"}, {"age", "30"}},
	)
	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "My Table" {
		t.Errorf("expected title 'My Table', got %q", title)
	}
}

func TestFormatTableText(t *testing.T) {
	result := FormatTableText(
		[]string{"A", "B"},
		[][]string{{"1", "2"}, {"3", "4"}},
	)
	if !strings.Contains(result, "| A") {
		t.Errorf("expected pipe-separated table, got:\n%s", result)
	}
	lines := strings.Split(result, "\n")
	if len(lines) != 4 {
		t.Errorf("expected 4 lines, got %d", len(lines))
	}
}

func TestTableBuilder_BuildPostElements(t *testing.T) {
	result := NewTableBuilder([]string{"X"}).
		AddRow([]string{"y"}).
		BuildPostElements()

	// Should be a valid JSON string.
	var s string
	if err := json.Unmarshal([]byte(result), &s); err != nil {
		t.Fatalf("BuildPostElements should produce a JSON string: %v", err)
	}
	if !strings.Contains(s, "X") || !strings.Contains(s, "y") {
		t.Errorf("table content missing in BuildPostElements output: %s", s)
	}
}

// ---------------------------------------------------------------------------
// FormatMarkdown tests
// ---------------------------------------------------------------------------

func TestFormatMarkdown_PlainText(t *testing.T) {
	result := FormatMarkdown("hello world")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "text" {
		t.Errorf("expected tag 'text', got %v", e["tag"])
	}
	if e["text"] != "hello world" {
		t.Errorf("expected 'hello world', got %v", e["text"])
	}
}

func TestFormatMarkdown_Bold(t *testing.T) {
	result := FormatMarkdown("this is **bold** text")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)

	if len(para) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(para))
	}

	// Before bold.
	e0 := elem(t, para, 0)
	if e0["text"] != "this is " {
		t.Errorf("expected 'this is ', got %v", e0["text"])
	}

	// Bold element.
	e1 := elem(t, para, 1)
	if e1["text"] != "bold" {
		t.Errorf("expected 'bold', got %v", e1["text"])
	}
	style, _ := e1["style"].([]any)
	if len(style) == 0 || style[0] != "bold" {
		t.Errorf("expected bold style, got %v", style)
	}

	// After bold.
	e2 := elem(t, para, 2)
	if e2["text"] != " text" {
		t.Errorf("expected ' text', got %v", e2["text"])
	}
}

func TestFormatMarkdown_BoldUnderscore(t *testing.T) {
	result := FormatMarkdown("__underscore bold__")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	style, _ := e["style"].([]any)
	if len(style) == 0 || style[0] != "bold" {
		t.Errorf("expected bold style for __ syntax, got %v", style)
	}
	if e["text"] != "underscore bold" {
		t.Errorf("expected 'underscore bold', got %v", e["text"])
	}
}

func TestFormatMarkdown_Italic(t *testing.T) {
	result := FormatMarkdown("this is *italic* text")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)

	if len(para) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(para))
	}

	e1 := elem(t, para, 1)
	style, _ := e1["style"].([]any)
	if len(style) == 0 || style[0] != "italic" {
		t.Errorf("expected italic style, got %v", style)
	}
}

func TestFormatMarkdown_ItalicUnderscore(t *testing.T) {
	result := FormatMarkdown("_underscore italic_")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	style, _ := e["style"].([]any)
	if len(style) == 0 || style[0] != "italic" {
		t.Errorf("expected italic style for _ syntax, got %v", style)
	}
}

func TestFormatMarkdown_Link(t *testing.T) {
	result := FormatMarkdown("visit [Google](https://google.com) now")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)

	if len(para) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(para))
	}

	linkElem := elem(t, para, 1)
	if linkElem["tag"] != "a" {
		t.Errorf("expected tag 'a', got %v", linkElem["tag"])
	}
	if linkElem["text"] != "Google" {
		t.Errorf("expected link text 'Google', got %v", linkElem["text"])
	}
	if linkElem["href"] != "https://google.com" {
		t.Errorf("expected href 'https://google.com', got %v", linkElem["href"])
	}
}

func TestFormatMarkdown_InlineCode(t *testing.T) {
	result := FormatMarkdown("run `go test` now")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)

	if len(para) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(para))
	}

	// Inline code is rendered as plain text.
	codeElem := elem(t, para, 1)
	if codeElem["tag"] != "text" {
		t.Errorf("expected tag 'text' for inline code, got %v", codeElem["tag"])
	}
	if codeElem["text"] != "go test" {
		t.Errorf("expected 'go test', got %v", codeElem["text"])
	}
}

func TestFormatMarkdown_MultiLine(t *testing.T) {
	md := "line one\nline two\nline three"
	result := FormatMarkdown(md)
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")

	if len(content) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d", len(content))
	}
}

func TestFormatMarkdown_MixedElements(t *testing.T) {
	md := "**bold** and *italic* with [link](https://x.com)"
	result := FormatMarkdown(md)
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)

	// Expecting: bold, " and ", italic, " with ", link
	if len(para) != 5 {
		t.Fatalf("expected 5 elements, got %d", len(para))
	}

	// Verify bold.
	boldElem := elem(t, para, 0)
	boldStyle, _ := boldElem["style"].([]any)
	if len(boldStyle) == 0 || boldStyle[0] != "bold" {
		t.Errorf("first element should be bold")
	}

	// Verify italic.
	italicElem := elem(t, para, 2)
	italicStyle, _ := italicElem["style"].([]any)
	if len(italicStyle) == 0 || italicStyle[0] != "italic" {
		t.Errorf("third element should be italic")
	}

	// Verify link.
	linkElem := elem(t, para, 4)
	if linkElem["tag"] != "a" {
		t.Errorf("fifth element should be a link")
	}
}

func TestFormatMarkdown_EmptyString(t *testing.T) {
	result := FormatMarkdown("")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	if len(content) != 1 {
		t.Errorf("expected 1 paragraph for empty string, got %d", len(content))
	}
}

func TestFormatMarkdownWithTitle(t *testing.T) {
	result := FormatMarkdownWithTitle("**hello**", "My Title")
	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "My Title" {
		t.Errorf("expected title 'My Title', got %q", title)
	}
}

// ---------------------------------------------------------------------------
// FormatCodeBlock tests
// ---------------------------------------------------------------------------

func TestFormatCodeBlock(t *testing.T) {
	code := "package main\n\nfunc main() {}"
	result := FormatCodeBlock(code, "go")
	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)

	if e["tag"] != "code_block" {
		t.Errorf("expected tag 'code_block', got %v", e["tag"])
	}
	if e["language"] != "go" {
		t.Errorf("expected language 'go', got %v", e["language"])
	}
	if e["text"] != code {
		t.Errorf("code text mismatch")
	}
}

func TestFormatCodeBlockWithTitle(t *testing.T) {
	result := FormatCodeBlockWithTitle("print('hi')", "python", "Code Example")
	m := mustParseJSON(t, result)
	title := postTitle(t, m, "zh_cn")
	if title != "Code Example" {
		t.Errorf("expected title 'Code Example', got %q", title)
	}
}

// ---------------------------------------------------------------------------
// FormatTable (format.go convenience) tests
// ---------------------------------------------------------------------------

func TestFormatTable(t *testing.T) {
	result := FormatTable(
		[]string{"Key", "Val"},
		[][]string{{"a", "1"}, {"b", "2"}},
	)
	if !strings.Contains(result, "| Key") {
		t.Errorf("expected table with headers: %s", result)
	}
	if !strings.Contains(result, "| a") {
		t.Errorf("expected table with data: %s", result)
	}
}

// ---------------------------------------------------------------------------
// Edge cases and integration tests
// ---------------------------------------------------------------------------

func TestPostBuilder_UnicodeContent(t *testing.T) {
	result := NewPostBuilder("Unicode Test").
		AddText("Hello, world!").
		Build()

	m := mustParseJSON(t, result)
	content := postContent(t, m, "zh_cn")
	para := paragraph(t, content, 0)
	e := elem(t, para, 0)
	if e["text"] != "Hello, world!" {
		t.Errorf("unicode text not preserved: %v", e["text"])
	}
}

func TestTableBuilder_UnicodeContent(t *testing.T) {
	result := NewTableBuilder([]string{"Name", "Desc"}).
		AddRow([]string{"test", "description"}).
		Build()

	if !strings.Contains(result, "test") {
		t.Errorf("unicode content not in table: %s", result)
	}
}

func TestDisplayWidth(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"hello", 5},
		{"", 0},
		{"abc", 3},
	}
	for _, tc := range tests {
		got := displayWidth(tc.input)
		if got != tc.expected {
			t.Errorf("displayWidth(%q) = %d, want %d", tc.input, got, tc.expected)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// pipePositions returns the byte indices of all '|' characters in a string.
func pipePositions(s string) []int {
	var positions []int
	for i, c := range s {
		if c == '|' {
			positions = append(positions, i)
		}
	}
	return positions
}
