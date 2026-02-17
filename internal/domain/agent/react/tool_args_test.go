package react

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// --- formatToolArgumentsForLog ---

func TestFormatToolArgumentsForLog_Empty(t *testing.T) {
	if got := formatToolArgumentsForLog(nil); got != "{}" {
		t.Fatalf("expected {}, got %q", got)
	}
	if got := formatToolArgumentsForLog(map[string]any{}); got != "{}" {
		t.Fatalf("expected {}, got %q", got)
	}
}

func TestFormatToolArgumentsForLog_SimpleArgs(t *testing.T) {
	args := map[string]any{"path": "/tmp/foo.txt", "mode": "rw"}
	got := formatToolArgumentsForLog(args)
	if !strings.Contains(got, `"path"`) || !strings.Contains(got, `"/tmp/foo.txt"`) {
		t.Fatalf("expected JSON-encoded args, got %q", got)
	}
}

func TestFormatToolArgumentsForLog_LargeStringTruncated(t *testing.T) {
	large := strings.Repeat("x", toolArgInlineLengthLimit+100)
	args := map[string]any{"content": large}
	got := formatToolArgumentsForLog(args)
	if strings.Contains(got, large) {
		t.Fatalf("expected large string to be summarized, got %q", got)
	}
	if !strings.Contains(got, "len=") {
		t.Fatalf("expected len= in summary, got %q", got)
	}
}

// --- sanitizeToolArgumentsForLog ---

func TestSanitizeToolArgumentsForLog_Nil(t *testing.T) {
	if got := sanitizeToolArgumentsForLog(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSanitizeToolArgumentsForLog_PreservesShortValues(t *testing.T) {
	args := map[string]any{"key": "short"}
	got := sanitizeToolArgumentsForLog(args)
	if v, ok := got["key"].(string); !ok || v != "short" {
		t.Fatalf("expected short value preserved, got %v", got["key"])
	}
}

// --- summarizeToolArgumentValue ---

func TestSummarizeToolArgumentValue_ShortString(t *testing.T) {
	got := summarizeToolArgumentValue("key", "hello")
	if got != "hello" {
		t.Fatalf("expected unchanged short string, got %v", got)
	}
}

func TestSummarizeToolArgumentValue_NestedMap(t *testing.T) {
	nested := map[string]any{"inner": "value"}
	got := summarizeToolArgumentValue("key", nested)
	result, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if result["inner"] != "value" {
		t.Fatalf("expected inner value preserved, got %v", result["inner"])
	}
}

func TestSummarizeToolArgumentValue_SliceAny(t *testing.T) {
	slice := []any{"short", "also short"}
	got := summarizeToolArgumentValue("key", slice)
	result, ok := got.([]any)
	if !ok || len(result) != 2 {
		t.Fatalf("expected slice of 2, got %v", got)
	}
}

func TestSummarizeToolArgumentValue_SliceString(t *testing.T) {
	slice := []string{"a", "b"}
	got := summarizeToolArgumentValue("key", slice)
	result, ok := got.([]string)
	if !ok || len(result) != 2 {
		t.Fatalf("expected string slice of 2, got %v", got)
	}
}

func TestSummarizeToolArgumentValue_NonStringPassthrough(t *testing.T) {
	got := summarizeToolArgumentValue("key", 42)
	if got != 42 {
		t.Fatalf("expected int passthrough, got %v", got)
	}
}

// --- summarizeToolArgumentString ---

func TestSummarizeToolArgumentString_Empty(t *testing.T) {
	if got := summarizeToolArgumentString("key", "  "); got != "" {
		t.Fatalf("expected empty after trim, got %q", got)
	}
}

func TestSummarizeToolArgumentString_DataURI(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgoAAAA"
	got := summarizeToolArgumentString("key", dataURI)
	if !strings.HasPrefix(got, "data_uri(") {
		t.Fatalf("expected data_uri summary, got %q", got)
	}
}

func TestSummarizeToolArgumentString_ImageKeyWithURL(t *testing.T) {
	url := "https://example.com/photo.png"
	got := summarizeToolArgumentString("image_url", url)
	if got != url {
		t.Fatalf("expected URL to pass through for image key, got %q", got)
	}
}

func TestSummarizeToolArgumentString_ImageKeyWithLargeBinaryLike(t *testing.T) {
	// Binary-like: has non-printable chars and exceeds limit
	binary := strings.Repeat("a", toolArgInlineLengthLimit) + "\x00"
	got := summarizeToolArgumentString("image_data", binary)
	if !strings.HasPrefix(got, "base64(") {
		t.Fatalf("expected base64 summary for binary-like image data, got %q", got)
	}
}

func TestSummarizeToolArgumentString_LongPlainText(t *testing.T) {
	long := strings.Repeat("x", toolArgInlineLengthLimit+50)
	got := summarizeToolArgumentString("content", long)
	if !strings.Contains(got, "len=") {
		t.Fatalf("expected len= in long text summary, got %q", got)
	}
	if !strings.HasSuffix(got, ")") {
		t.Fatalf("expected trailing paren, got %q", got)
	}
}

// --- summarizeDataURIForLog ---

func TestSummarizeDataURIForLog_WithComma(t *testing.T) {
	data := "data:text/plain;base64,SGVsbG8gV29ybGQ="
	got := summarizeDataURIForLog(data)
	if !strings.Contains(got, "data_uri(") {
		t.Fatalf("expected data_uri prefix, got %q", got)
	}
	if !strings.Contains(got, "header=") {
		t.Fatalf("expected header field, got %q", got)
	}
	if !strings.Contains(got, "payload_prefix=") {
		t.Fatalf("expected payload_prefix field, got %q", got)
	}
}

func TestSummarizeDataURIForLog_NoComma(t *testing.T) {
	data := "data:text/plain;base64"
	got := summarizeDataURIForLog(data)
	if !strings.HasPrefix(got, "data_uri(len=") {
		t.Fatalf("expected simple data_uri(len=...), got %q", got)
	}
}

// --- summarizeBinaryLikeString ---

func TestSummarizeBinaryLikeString(t *testing.T) {
	value := strings.Repeat("A", 200)
	got := summarizeBinaryLikeString(value)
	if !strings.HasPrefix(got, "base64(len=") {
		t.Fatalf("expected base64(len=..., got %q", got)
	}
	if !strings.Contains(got, fmt.Sprintf("len=%d", len(value))) {
		t.Fatalf("expected correct length, got %q", got)
	}
}

// --- summarizeLongPlainString ---

func TestSummarizeLongPlainString(t *testing.T) {
	value := strings.Repeat("abc", 100) // 300 chars
	got := summarizeLongPlainString(value)
	if !strings.Contains(got, "len=300") {
		t.Fatalf("expected len=300 in summary, got %q", got)
	}
	if !strings.Contains(got, "...") {
		t.Fatalf("expected ellipsis, got %q", got)
	}
}

// --- looksLikeBinaryString ---

func TestLooksLikeBinaryString_ShortString(t *testing.T) {
	if looksLikeBinaryString("short") {
		t.Fatal("short strings should not be binary-like")
	}
}

func TestLooksLikeBinaryString_LongPrintable(t *testing.T) {
	long := strings.Repeat("a", toolArgInlineLengthLimit+10)
	if looksLikeBinaryString(long) {
		t.Fatal("long printable string should not be binary-like")
	}
}

func TestLooksLikeBinaryString_LongWithNonPrintableInSample(t *testing.T) {
	// Non-printable must be within the 128-byte sample window
	long := strings.Repeat("a", 64) + "\x01" + strings.Repeat("a", toolArgInlineLengthLimit)
	if !looksLikeBinaryString(long) {
		t.Fatal("long string with non-printable in sample should be binary-like")
	}
}

func TestLooksLikeBinaryString_NonPrintableOutsideSample(t *testing.T) {
	// Non-printable beyond the 128-byte sample window → not detected
	long := strings.Repeat("a", toolArgInlineLengthLimit) + "\x01"
	if looksLikeBinaryString(long) {
		t.Fatal("non-printable outside sample window should not trigger binary detection")
	}
}

func TestLooksLikeBinaryString_ControlCharInSample(t *testing.T) {
	// Control char at index 0 with enough length
	long := "\x00" + strings.Repeat("a", toolArgInlineLengthLimit)
	if !looksLikeBinaryString(long) {
		t.Fatal("string starting with control char should be binary-like")
	}
}

// --- truncateStringForLog ---

func TestTruncateStringForLog_UnderLimit(t *testing.T) {
	got := truncateStringForLog("hello", 10)
	if got != "hello" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestTruncateStringForLog_AtLimit(t *testing.T) {
	got := truncateStringForLog("hello", 5)
	if got != "hello" {
		t.Fatalf("expected unchanged at exact limit, got %q", got)
	}
}

func TestTruncateStringForLog_OverLimit(t *testing.T) {
	got := truncateStringForLog("hello world", 5)
	if got != "hello" {
		t.Fatalf("expected truncated to 5 runes, got %q", got)
	}
}

func TestTruncateStringForLog_ZeroLimit(t *testing.T) {
	got := truncateStringForLog("hello", 0)
	if got != "" {
		t.Fatalf("expected empty for zero limit, got %q", got)
	}
}

func TestTruncateStringForLog_MultiByte(t *testing.T) {
	// 4 CJK characters = 12 bytes but only 4 runes
	got := truncateStringForLog("你好世界", 2)
	if got != "你好" {
		t.Fatalf("expected 2-rune truncation, got %q", got)
	}
}

// --- compactToolCallArguments ---

func TestCompactToolCallArguments_EmptyArgs(t *testing.T) {
	call := ToolCall{Arguments: nil}
	result := ToolResult{}
	compacted, changed := compactToolCallArguments(call, result)
	if compacted != nil || changed {
		t.Fatalf("expected nil/false for empty args, got %v/%v", compacted, changed)
	}
}

func TestCompactToolCallArguments_ShortArgs(t *testing.T) {
	call := ToolCall{
		Name:      "file_write",
		ID:        "c1",
		Arguments: map[string]any{"path": "foo.txt", "content": "short"},
	}
	result := ToolResult{CallID: "c1", Metadata: map[string]any{"path": "/workspace/foo.txt"}}
	compacted, changed := compactToolCallArguments(call, result)
	if compacted != nil || changed {
		t.Fatalf("expected no compaction for short args, got %v/%v", compacted, changed)
	}
}

func TestCompactToolCallArguments_LargeContent(t *testing.T) {
	large := strings.Repeat("x", toolArgHistoryInlineLimit+100)
	call := ToolCall{
		Name:      "file_write",
		ID:        "c1",
		Arguments: map[string]any{"path": "big.txt", "content": large},
	}
	result := ToolResult{
		CallID:   "c1",
		Metadata: map[string]any{"path": "/workspace/big.txt"},
	}

	compacted, changed := compactToolCallArguments(call, result)
	if !changed {
		t.Fatal("expected compaction for large content")
	}
	ref, ok := compacted["content"].(map[string]any)
	if !ok {
		t.Fatalf("expected map reference, got %T", compacted["content"])
	}
	if ref["content_len"] != len(large) {
		t.Fatalf("expected correct length, got %v", ref["content_len"])
	}
	if ref["content_ref"] != "/workspace/big.txt" {
		t.Fatalf("expected content_ref from result metadata, got %v", ref["content_ref"])
	}
	sum := sha256.Sum256([]byte(large))
	expectedHash := fmt.Sprintf("%x", sum)
	if ref["content_sha256"] != expectedHash {
		t.Fatalf("expected sha256 hash, got %v", ref["content_sha256"])
	}
	// path should remain unchanged
	if compacted["path"] != "big.txt" {
		t.Fatalf("expected path preserved, got %v", compacted["path"])
	}
}

func TestCompactToolCallArguments_DataURI(t *testing.T) {
	dataURI := "data:image/png;base64," + strings.Repeat("A", 100)
	call := ToolCall{
		Name:      "vision_analyze",
		ID:        "c1",
		Arguments: map[string]any{"image": dataURI},
	}
	result := ToolResult{CallID: "c1"}

	compacted, changed := compactToolCallArguments(call, result)
	if !changed {
		t.Fatal("expected compaction for data URI")
	}
	ref, ok := compacted["image"].(map[string]any)
	if !ok {
		t.Fatalf("expected map reference for data URI, got %T", compacted["image"])
	}
	if ref["content_len"] != len(dataURI) {
		t.Fatalf("expected correct length, got %v", ref["content_len"])
	}
}

// --- compactToolArgumentValue ---

func TestCompactToolArgumentValue_NestedMap(t *testing.T) {
	large := strings.Repeat("x", toolArgHistoryInlineLimit+10)
	nested := map[string]any{"deep": large}
	got, changed := compactToolArgumentValue("", nested)
	if !changed {
		t.Fatal("expected nested map compaction")
	}
	gotMap := got.(map[string]any)
	if _, ok := gotMap["deep"].(map[string]any); !ok {
		t.Fatalf("expected inner value compacted to map, got %T", gotMap["deep"])
	}
}

func TestCompactToolArgumentValue_SkipsContentReferenceMap(t *testing.T) {
	ref := map[string]any{"content_len": 100, "content_sha256": "abc123"}
	got, changed := compactToolArgumentValue("", ref)
	if changed {
		t.Fatal("expected no change for content reference map")
	}
	gotMap, ok := got.(map[string]any)
	if !ok {
		t.Fatal("expected map returned")
	}
	if gotMap["content_len"] != 100 || gotMap["content_sha256"] != "abc123" {
		t.Fatalf("expected original values preserved, got %v", gotMap)
	}
}

func TestCompactToolArgumentValue_SliceAny(t *testing.T) {
	large := strings.Repeat("x", toolArgHistoryInlineLimit+10)
	slice := []any{"short", large}
	got, changed := compactToolArgumentValue("", slice)
	if !changed {
		t.Fatal("expected slice compaction")
	}
	gotSlice := got.([]any)
	if gotSlice[0] != "short" {
		t.Fatalf("expected short string preserved, got %v", gotSlice[0])
	}
	if _, ok := gotSlice[1].(map[string]any); !ok {
		t.Fatalf("expected large string compacted to map, got %T", gotSlice[1])
	}
}

func TestCompactToolArgumentValue_NonStringPassthrough(t *testing.T) {
	got, changed := compactToolArgumentValue("", 42)
	if changed || got != 42 {
		t.Fatalf("expected passthrough for non-string, got %v/%v", got, changed)
	}
}

// --- shouldCompactToolArgString ---

func TestShouldCompactToolArgString_Empty(t *testing.T) {
	if shouldCompactToolArgString("  ") {
		t.Fatal("empty/whitespace should not compact")
	}
}

func TestShouldCompactToolArgString_DataPrefix(t *testing.T) {
	if !shouldCompactToolArgString("data:image/png;base64,abc") {
		t.Fatal("data: prefix should compact")
	}
}

func TestShouldCompactToolArgString_BinaryLike(t *testing.T) {
	binary := strings.Repeat("a", toolArgInlineLengthLimit) + "\x00"
	if !shouldCompactToolArgString(binary) {
		t.Fatal("binary-like should compact")
	}
}

func TestShouldCompactToolArgString_LongText(t *testing.T) {
	long := strings.Repeat("a", toolArgHistoryInlineLimit+10)
	if !shouldCompactToolArgString(long) {
		t.Fatal("long text exceeding inline limit should compact")
	}
}

func TestShouldCompactToolArgString_ShortText(t *testing.T) {
	if shouldCompactToolArgString("hello world") {
		t.Fatal("short text should not compact")
	}
}

// --- isContentReferenceMap ---

func TestIsContentReferenceMap_Valid(t *testing.T) {
	ref := map[string]any{"content_len": 100, "content_sha256": "abc"}
	if !isContentReferenceMap(ref) {
		t.Fatal("expected valid reference map")
	}
}

func TestIsContentReferenceMap_MissingLen(t *testing.T) {
	ref := map[string]any{"content_sha256": "abc"}
	if isContentReferenceMap(ref) {
		t.Fatal("expected invalid without content_len")
	}
}

func TestIsContentReferenceMap_MissingHash(t *testing.T) {
	ref := map[string]any{"content_len": 100}
	if isContentReferenceMap(ref) {
		t.Fatal("expected invalid without content_sha256")
	}
}

func TestIsContentReferenceMap_Nil(t *testing.T) {
	if isContentReferenceMap(nil) {
		t.Fatal("expected false for nil")
	}
}

// --- toolArgumentContentRef ---

func TestToolArgumentContentRef_FileWrite(t *testing.T) {
	tests := []struct {
		name     string
		call     ToolCall
		result   ToolResult
		expected string
	}{
		{
			name:     "from result metadata path",
			call:     ToolCall{Name: "file_write", Arguments: map[string]any{"path": "fallback.txt"}},
			result:   ToolResult{Metadata: map[string]any{"path": "/resolved/path.txt"}},
			expected: "/resolved/path.txt",
		},
		{
			name:     "from result metadata resolved_path",
			call:     ToolCall{Name: "file_write", Arguments: map[string]any{"path": "fallback.txt"}},
			result:   ToolResult{Metadata: map[string]any{"resolved_path": "/resolved/path.txt"}},
			expected: "/resolved/path.txt",
		},
		{
			name:     "fallback to call args",
			call:     ToolCall{Name: "file_write", Arguments: map[string]any{"path": "fallback.txt"}},
			result:   ToolResult{},
			expected: "fallback.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toolArgumentContentRef(tt.call, tt.result)
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestToolArgumentContentRef_FileEdit(t *testing.T) {
	call := ToolCall{Name: "file_edit", Arguments: map[string]any{"file_path": "/src/main.go"}}
	result := ToolResult{Metadata: map[string]any{"resolved_path": "/workspace/src/main.go"}}
	got := toolArgumentContentRef(call, result)
	if got != "/workspace/src/main.go" {
		t.Fatalf("expected resolved_path, got %q", got)
	}
}

func TestToolArgumentContentRef_ArtifactsWrite(t *testing.T) {
	call := ToolCall{Name: "artifacts_write", Arguments: map[string]any{"name": "report.md"}}
	result := ToolResult{}
	got := toolArgumentContentRef(call, result)
	if got != "report.md" {
		t.Fatalf("expected name from args, got %q", got)
	}
}

func TestToolArgumentContentRef_UnknownTool(t *testing.T) {
	call := ToolCall{Name: "web_search", Arguments: map[string]any{"query": "test"}}
	result := ToolResult{}
	got := toolArgumentContentRef(call, result)
	if got != "" {
		t.Fatalf("expected empty for unknown tool, got %q", got)
	}
}

func TestToolArgumentContentRef_CaseInsensitive(t *testing.T) {
	call := ToolCall{Name: "File_Write", Arguments: map[string]any{"path": "test.txt"}}
	result := ToolResult{}
	got := toolArgumentContentRef(call, result)
	if got != "test.txt" {
		t.Fatalf("expected case-insensitive match, got %q", got)
	}
}

// --- stringFromMap ---

func TestStringFromMap_Empty(t *testing.T) {
	if got := stringFromMap(nil, "key"); got != "" {
		t.Fatalf("expected empty for nil map, got %q", got)
	}
	if got := stringFromMap(map[string]any{}, "key"); got != "" {
		t.Fatalf("expected empty for empty map, got %q", got)
	}
}

func TestStringFromMap_FirstKeyMatch(t *testing.T) {
	m := map[string]any{"path": "/a.txt", "resolved_path": "/b.txt"}
	got := stringFromMap(m, "path", "resolved_path")
	if got != "/a.txt" {
		t.Fatalf("expected first key match, got %q", got)
	}
}

func TestStringFromMap_FallbackToSecondKey(t *testing.T) {
	m := map[string]any{"resolved_path": "/b.txt"}
	got := stringFromMap(m, "path", "resolved_path")
	if got != "/b.txt" {
		t.Fatalf("expected fallback to second key, got %q", got)
	}
}

func TestStringFromMap_NonStringValue(t *testing.T) {
	m := map[string]any{"key": 42}
	got := stringFromMap(m, "key")
	if got != "" {
		t.Fatalf("expected empty for non-string value, got %q", got)
	}
}

func TestStringFromMap_WhitespaceOnlyValue(t *testing.T) {
	m := map[string]any{"key": "   "}
	got := stringFromMap(m, "key")
	if got != "" {
		t.Fatalf("expected empty for whitespace-only value, got %q", got)
	}
}
