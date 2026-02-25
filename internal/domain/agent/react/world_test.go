package react

import (
	"fmt"
	"strings"
	"testing"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agentports "alex/internal/domain/agent/ports/agent"
)

// --- ensureWorldStateMap ---

func TestEnsureWorldStateMap_InitializesNilMap(t *testing.T) {
	cog := &agentports.CognitiveExtension{}
	ensureWorldStateMap(cog)
	if cog.WorldState == nil {
		t.Fatal("expected WorldState to be initialized")
	}
}

func TestEnsureWorldStateMap_PreservesExisting(t *testing.T) {
	cog := &agentports.CognitiveExtension{
		WorldState: map[string]any{"key": "value"},
	}
	ensureWorldStateMap(cog)
	if cog.WorldState["key"] != "value" {
		t.Fatal("expected existing values preserved")
	}
}

// --- summarizeToolResultForWorld ---

func TestSummarizeToolResultForWorld_Success(t *testing.T) {
	result := ToolResult{
		CallID:  "web_fetch",
		Content: "Found relevant data",
	}
	entry := summarizeToolResultForWorld(result)
	if entry["call_id"] != "web_fetch" {
		t.Fatalf("expected call_id, got %v", entry["call_id"])
	}
	if entry["status"] != "success" {
		t.Fatalf("expected success status, got %v", entry["status"])
	}
	if _, ok := entry["error"]; ok {
		t.Fatal("expected no error field")
	}
	if entry["output_preview"] != "Found relevant data" {
		t.Fatalf("expected content preview, got %v", entry["output_preview"])
	}
}

func TestSummarizeToolResultForWorld_Error(t *testing.T) {
	result := ToolResult{
		CallID: "shell_exec",
		Error:  fmt.Errorf("exit code 1"),
	}
	entry := summarizeToolResultForWorld(result)
	if entry["status"] != "error" {
		t.Fatalf("expected error status, got %v", entry["status"])
	}
	if entry["error"] != "exit code 1" {
		t.Fatalf("expected error message, got %v", entry["error"])
	}
}

func TestSummarizeToolResultForWorld_WithAttachments(t *testing.T) {
	result := ToolResult{
		CallID: "seedream",
		Attachments: map[string]ports.Attachment{
			"img.png": {Name: "img.png"},
		},
	}
	entry := summarizeToolResultForWorld(result)
	names, ok := entry["attachments"].([]string)
	if !ok || len(names) != 1 || names[0] != "img.png" {
		t.Fatalf("expected attachment names, got %v", entry["attachments"])
	}
}

func TestSummarizeToolResultForWorld_WithMetadata(t *testing.T) {
	result := ToolResult{
		CallID: "call-1",
		Metadata: map[string]any{
			"count": 42,
		},
	}
	entry := summarizeToolResultForWorld(result)
	metadata, ok := entry["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata map, got %v", entry["metadata"])
	}
	if metadata["count"] != 42 {
		t.Fatalf("expected count=42, got %v", metadata["count"])
	}
}

// --- summarizeForWorld ---

func TestSummarizeForWorld_ShortContent(t *testing.T) {
	got := summarizeForWorld("hello", 100)
	if got != "hello" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestSummarizeForWorld_LongContent(t *testing.T) {
	long := strings.Repeat("x", domain.ToolResultPreviewRunes+10)
	got := summarizeForWorld(long, domain.ToolResultPreviewRunes)
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix, got %q", got)
	}
	if len([]rune(got)) > domain.ToolResultPreviewRunes+1 { // +1 for ellipsis
		t.Fatalf("expected truncation, got len=%d", len([]rune(got)))
	}
}

func TestSummarizeForWorld_EmptyContent(t *testing.T) {
	if got := summarizeForWorld("  ", 100); got != "" {
		t.Fatalf("expected empty for whitespace, got %q", got)
	}
}

func TestSummarizeForWorld_ZeroLimit(t *testing.T) {
	if got := summarizeForWorld("hello", 0); got != "" {
		t.Fatalf("expected empty for zero limit, got %q", got)
	}
}

// --- summarizeWorldMetadata ---

func TestSummarizeWorldMetadata_Empty(t *testing.T) {
	if got := summarizeWorldMetadata(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSummarizeWorldMetadata_PreservesNumeric(t *testing.T) {
	meta := map[string]any{"count": 42, "ratio": 3.14}
	got := summarizeWorldMetadata(meta)
	if got["count"] != 42 || got["ratio"] != 3.14 {
		t.Fatalf("expected numeric passthrough, got %v", got)
	}
}

func TestSummarizeWorldMetadata_TruncatesLongStrings(t *testing.T) {
	long := strings.Repeat("x", domain.ToolResultPreviewRunes)
	meta := map[string]any{"description": long}
	got := summarizeWorldMetadata(meta)
	desc, ok := got["description"].(string)
	if !ok {
		t.Fatalf("expected string, got %T", got["description"])
	}
	if len([]rune(desc)) >= len([]rune(long)) {
		t.Fatalf("expected truncation, got len=%d", len([]rune(desc)))
	}
}

func TestSummarizeWorldMetadata_NestedMap(t *testing.T) {
	meta := map[string]any{
		"inner": map[string]any{"key": "val"},
	}
	got := summarizeWorldMetadata(meta)
	inner, ok := got["inner"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", got["inner"])
	}
	if inner["key"] != "val" {
		t.Fatalf("expected nested value, got %v", inner["key"])
	}
}

func TestSummarizeWorldMetadata_NilValues(t *testing.T) {
	meta := map[string]any{"null_field": nil}
	got := summarizeWorldMetadata(meta)
	if got != nil {
		t.Fatalf("expected nil after filtering nil values, got %v", got)
	}
}

// --- summarizeMetadataValue ---

func TestSummarizeMetadataValue_Bool(t *testing.T) {
	got := summarizeMetadataValue(true)
	if got != true {
		t.Fatalf("expected bool passthrough, got %v", got)
	}
}

func TestSummarizeMetadataValue_StringSlice(t *testing.T) {
	got := summarizeMetadataValue([]string{"a", "b"})
	result, ok := got.([]string)
	if !ok || len(result) != 2 {
		t.Fatalf("expected string slice, got %v", got)
	}
}

func TestSummarizeMetadataValue_Nil(t *testing.T) {
	got := summarizeMetadataValue(nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// --- summarizeAttachmentNames ---

func TestSummarizeAttachmentNames_Empty(t *testing.T) {
	if got := summarizeAttachmentNames(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSummarizeAttachmentNames_UsesAttName(t *testing.T) {
	atts := map[string]ports.Attachment{
		"key1": {Name: "real_name.png"},
	}
	got := summarizeAttachmentNames(atts)
	if len(got) != 1 || got[0] != "real_name.png" {
		t.Fatalf("expected [real_name.png], got %v", got)
	}
}

func TestSummarizeAttachmentNames_FallsBackToKey(t *testing.T) {
	atts := map[string]ports.Attachment{
		"fallback.txt": {},
	}
	got := summarizeAttachmentNames(atts)
	if len(got) != 1 || got[0] != "fallback.txt" {
		t.Fatalf("expected [fallback.txt], got %v", got)
	}
}

func TestSummarizeAttachmentNames_SkipsEmptyNames(t *testing.T) {
	atts := map[string]ports.Attachment{
		"": {Name: ""},
	}
	got := summarizeAttachmentNames(atts)
	if got != nil {
		t.Fatalf("expected nil for empty names, got %v", got)
	}
}

func TestSummarizeAttachmentNames_Sorted(t *testing.T) {
	atts := map[string]ports.Attachment{
		"z.png": {Name: "z.png"},
		"a.png": {Name: "a.png"},
		"m.png": {Name: "m.png"},
	}
	got := summarizeAttachmentNames(atts)
	if len(got) != 3 || got[0] != "a.png" || got[1] != "m.png" || got[2] != "z.png" {
		t.Fatalf("expected sorted names, got %v", got)
	}
}
