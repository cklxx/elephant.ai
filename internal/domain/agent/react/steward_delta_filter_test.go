package react

import (
	"strings"
	"testing"
)

func runFilter(chunks []string) string {
	f := &stewardDeltaFilter{}
	var out strings.Builder
	for _, c := range chunks {
		out.WriteString(f.Write(c))
	}
	out.WriteString(f.Flush())
	return out.String()
}

func runFilterChunks(chunks []string) []string {
	f := &stewardDeltaFilter{}
	results := make([]string, 0, len(chunks)+1)
	for _, c := range chunks {
		results = append(results, f.Write(c))
	}
	results = append(results, f.Flush())
	return results
}

func TestStewardDeltaFilter_NoTag(t *testing.T) {
	got := runFilter([]string{"Hello world"})
	if got != "Hello world" {
		t.Errorf("got %q, want %q", got, "Hello world")
	}
}

func TestStewardDeltaFilter_CompleteTagInOneChunk(t *testing.T) {
	got := runFilter([]string{`Hello <NEW_STATE>{"version":1}</NEW_STATE>`})
	if got != "Hello " {
		t.Errorf("got %q, want %q", got, "Hello ")
	}
}

func TestStewardDeltaFilter_TagSplitAcrossTwoChunks(t *testing.T) {
	got := runFilter([]string{"Hello <NEW_S", `TATE>{"v":1}</NEW_STATE> done`})
	if got != "Hello  done" {
		t.Errorf("got %q, want %q", got, "Hello  done")
	}
}

func TestStewardDeltaFilter_TagAtEndNoClose(t *testing.T) {
	got := runFilter([]string{`Hello <NEW_STATE>{"partial":true`})
	if got != "Hello " {
		t.Errorf("got %q, want %q", got, "Hello ")
	}
}

func TestStewardDeltaFilter_NormalAngleBracket(t *testing.T) {
	got := runFilter([]string{"a < b and <div>"})
	if got != "a < b and <div>" {
		t.Errorf("got %q, want %q", got, "a < b and <div>")
	}
}

func TestStewardDeltaFilter_MultipleTags(t *testing.T) {
	got := runFilter([]string{"a<NEW_STATE>x</NEW_STATE>b<NEW_STATE>y</NEW_STATE>c"})
	if got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestStewardDeltaFilter_CloseTagSplit(t *testing.T) {
	got := runFilter([]string{"<NEW_STATE>x</NEW_", "STATE>done"})
	if got != "done" {
		t.Errorf("got %q, want %q", got, "done")
	}
}

func TestStewardDeltaFilter_OpenTagSplitSingleChar(t *testing.T) {
	// Split open tag one char at a time.
	tag := "<NEW_STATE>"
	chunks := make([]string, 0, len(tag)+2)
	chunks = append(chunks, "before")
	for _, c := range tag {
		chunks = append(chunks, string(c))
	}
	chunks = append(chunks, `{"v":1}</NEW_STATE>after`)
	got := runFilter(chunks)
	if got != "beforeafter" {
		t.Errorf("got %q, want %q", got, "beforeafter")
	}
}

func TestStewardDeltaFilter_EmptyChunks(t *testing.T) {
	got := runFilter([]string{"", "", "hello", "", ""})
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestStewardDeltaFilter_OnlyTag(t *testing.T) {
	got := runFilter([]string{"<NEW_STATE>data</NEW_STATE>"})
	if got != "" {
		t.Errorf("got %q, want %q", got, "")
	}
}

func TestStewardDeltaFilter_ChunkPerCharOutput(t *testing.T) {
	// Verify per-chunk outputs for the split case.
	chunks := runFilterChunks([]string{"Hi<NEW_S", `TATE>x</NEW_STATE>ok`})
	// chunks[0] = Write("Hi<NEW_S") → "Hi" (holdback "<NEW_S")
	// chunks[1] = Write("TATE>x</NEW_STATE>ok") → "ok"
	// chunks[2] = Flush() → ""
	if chunks[0] != "Hi" {
		t.Errorf("chunk 0: got %q, want %q", chunks[0], "Hi")
	}
	if chunks[1] != "ok" {
		t.Errorf("chunk 1: got %q, want %q", chunks[1], "ok")
	}
	if chunks[2] != "" {
		t.Errorf("flush: got %q, want %q", chunks[2], "")
	}
}

func TestStewardDeltaFilter_FlushReleasesHoldback(t *testing.T) {
	// A trailing "<" that is not part of a tag should be released on Flush.
	f := &stewardDeltaFilter{}
	out := f.Write("hello<")
	out += f.Flush()
	if out != "hello<" {
		t.Errorf("got %q, want %q", out, "hello<")
	}
}

func TestStewardDeltaFilter_FlushDuringSuppression(t *testing.T) {
	// If stream ends mid-tag, Flush should return empty (content is dropped).
	f := &stewardDeltaFilter{}
	out := f.Write("start<NEW_STATE>partial")
	flush := f.Flush()
	combined := out + flush
	if combined != "start" {
		t.Errorf("got %q, want %q", combined, "start")
	}
}

func TestStewardDeltaFilter_LargeContentInsideTag(t *testing.T) {
	inner := strings.Repeat("x", 10000)
	input := "before<NEW_STATE>" + inner + "</NEW_STATE>after"
	got := runFilter([]string{input})
	if got != "beforeafter" {
		t.Errorf("got length %d, want %d", len(got), len("beforeafter"))
	}
}

func TestStewardDeltaFilter_PartialOpenTagThenNonMatch(t *testing.T) {
	// "<NEW_" followed by something that doesn't complete the tag.
	got := runFilter([]string{"<NEW_", "STUFF>rest"})
	if got != "<NEW_STUFF>rest" {
		t.Errorf("got %q, want %q", got, "<NEW_STUFF>rest")
	}
}

func TestStewardDeltaFilter_ContentAfterMultipleTags(t *testing.T) {
	got := runFilter([]string{
		"a",
		"<NEW_STATE>1</NEW_STATE>",
		"b",
		"<NEW_STATE>2</NEW_STATE>",
		"c",
	})
	if got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}
