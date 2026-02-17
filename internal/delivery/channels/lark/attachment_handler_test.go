package lark

import (
	"testing"

	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
)

// --- allowExtension ---

func TestAllowExtension_EmptyAllowlist(t *testing.T) {
	if !allowExtension(".pdf", nil) {
		t.Fatal("expected true for empty allowlist (allow all)")
	}
}

func TestAllowExtension_Allowed(t *testing.T) {
	if !allowExtension(".pdf", []string{".pdf", ".png"}) {
		t.Fatal("expected true for allowed extension")
	}
}

func TestAllowExtension_NotAllowed(t *testing.T) {
	if allowExtension(".exe", []string{".pdf", ".png"}) {
		t.Fatal("expected false for disallowed extension")
	}
}

func TestAllowExtension_CaseInsensitive(t *testing.T) {
	if !allowExtension(".PDF", []string{".pdf"}) {
		t.Fatal("expected case-insensitive match")
	}
}

func TestAllowExtension_EmptyExt(t *testing.T) {
	if allowExtension("", []string{".pdf"}) {
		t.Fatal("expected false for empty extension")
	}
}

// --- larkFileType ---

func TestLarkFileType_Supported(t *testing.T) {
	for _, ext := range []string{"opus", "mp4", "pdf", "doc", "xls", "ppt"} {
		if got := larkFileType(ext); got != ext {
			t.Fatalf("expected %q, got %q", ext, got)
		}
	}
}

func TestLarkFileType_Unsupported(t *testing.T) {
	for _, ext := range []string{"xlsx", "docx", "txt", "bin", "zip"} {
		if got := larkFileType(ext); got != "stream" {
			t.Fatalf("expected stream for %q, got %q", ext, got)
		}
	}
}

func TestLarkFileType_CaseInsensitive(t *testing.T) {
	if got := larkFileType("PDF"); got != "pdf" {
		t.Fatalf("expected pdf, got %q", got)
	}
}

func TestLarkFileType_Empty(t *testing.T) {
	if got := larkFileType(""); got != "stream" {
		t.Fatalf("expected stream for empty, got %q", got)
	}
}

// --- sortedAttachmentNames ---

func TestSortedAttachmentNames_Sorted(t *testing.T) {
	atts := map[string]ports.Attachment{
		"z.png": {}, "a.pdf": {}, "m.txt": {},
	}
	got := sortedAttachmentNames(atts)
	if len(got) != 3 || got[0] != "a.pdf" || got[1] != "m.txt" || got[2] != "z.png" {
		t.Fatalf("expected sorted, got %v", got)
	}
}

func TestSortedAttachmentNames_Empty(t *testing.T) {
	got := sortedAttachmentNames(nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

// --- mergeAttachments ---

func TestMergeAttachments_MergesNew(t *testing.T) {
	out := map[string]ports.Attachment{}
	incoming := map[string]ports.Attachment{
		"file.pdf": {Name: "file.pdf", MediaType: "application/pdf"},
	}
	mergeAttachments(out, incoming)
	if len(out) != 1 || out["file.pdf"].MediaType != "application/pdf" {
		t.Fatalf("expected merged, got %v", out)
	}
}

func TestMergeAttachments_SkipsDuplicate(t *testing.T) {
	out := map[string]ports.Attachment{
		"file.pdf": {Name: "file.pdf", MediaType: "old"},
	}
	incoming := map[string]ports.Attachment{
		"file.pdf": {Name: "file.pdf", MediaType: "new"},
	}
	mergeAttachments(out, incoming)
	if out["file.pdf"].MediaType != "old" {
		t.Fatalf("expected original preserved, got %q", out["file.pdf"].MediaType)
	}
}

func TestMergeAttachments_SkipsEmptyKey(t *testing.T) {
	out := map[string]ports.Attachment{}
	incoming := map[string]ports.Attachment{
		"": {Name: ""},
	}
	mergeAttachments(out, incoming)
	if len(out) != 0 {
		t.Fatalf("expected empty (key skipped), got %v", out)
	}
}

func TestMergeAttachments_FallsBackToAttName(t *testing.T) {
	out := map[string]ports.Attachment{}
	incoming := map[string]ports.Attachment{
		"  ": {Name: "actual.png"},
	}
	mergeAttachments(out, incoming)
	if _, ok := out["actual.png"]; !ok {
		t.Fatalf("expected fallback to att.Name, got %v", out)
	}
}

func TestMergeAttachments_SetsName(t *testing.T) {
	out := map[string]ports.Attachment{}
	incoming := map[string]ports.Attachment{
		"key_name": {},
	}
	mergeAttachments(out, incoming)
	if out["key_name"].Name != "key_name" {
		t.Fatalf("expected Name set from key, got %q", out["key_name"].Name)
	}
}

func TestMergeAttachments_EmptyIncoming(t *testing.T) {
	out := map[string]ports.Attachment{"old": {}}
	mergeAttachments(out, nil)
	if len(out) != 1 {
		t.Fatalf("expected unchanged, got %v", out)
	}
}

// --- isA2UIAttachment ---

func TestIsA2UIAttachment_MediaType(t *testing.T) {
	att := ports.Attachment{MediaType: "application/a2ui+json"}
	if !isA2UIAttachment(att) {
		t.Fatal("expected true for a2ui media type")
	}
}

func TestIsA2UIAttachment_Format(t *testing.T) {
	att := ports.Attachment{Format: "a2ui"}
	if !isA2UIAttachment(att) {
		t.Fatal("expected true for a2ui format")
	}
}

func TestIsA2UIAttachment_PreviewProfile(t *testing.T) {
	att := ports.Attachment{PreviewProfile: "a2ui-renderer"}
	if !isA2UIAttachment(att) {
		t.Fatal("expected true for a2ui preview profile")
	}
}

func TestIsA2UIAttachment_NotA2UI(t *testing.T) {
	att := ports.Attachment{MediaType: "image/png"}
	if isA2UIAttachment(att) {
		t.Fatal("expected false for non-a2ui")
	}
}

// --- isImageAttachment ---

func TestIsImageAttachment_ByMediaType(t *testing.T) {
	att := ports.Attachment{}
	if !isImageAttachment(att, "image/png", "file.bin") {
		t.Fatal("expected true for image media type")
	}
}

func TestIsImageAttachment_ByAttMediaType(t *testing.T) {
	att := ports.Attachment{MediaType: "image/jpeg"}
	if !isImageAttachment(att, "", "file.bin") {
		t.Fatal("expected true for att image media type")
	}
}

func TestIsImageAttachment_ByExtension(t *testing.T) {
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp"} {
		att := ports.Attachment{}
		if !isImageAttachment(att, "", "file"+ext) {
			t.Fatalf("expected true for extension %s", ext)
		}
	}
}

func TestIsImageAttachment_NonImage(t *testing.T) {
	att := ports.Attachment{}
	if isImageAttachment(att, "application/pdf", "file.pdf") {
		t.Fatal("expected false for non-image")
	}
}

// --- fileNameForAttachment ---

func TestFileNameForAttachment_UsesAttName(t *testing.T) {
	att := ports.Attachment{Name: "doc.pdf"}
	if got := fileNameForAttachment(att, "fallback.txt"); got != "doc.pdf" {
		t.Fatalf("expected doc.pdf, got %q", got)
	}
}

func TestFileNameForAttachment_UsesFallback(t *testing.T) {
	att := ports.Attachment{}
	if got := fileNameForAttachment(att, "fallback.txt"); got != "fallback.txt" {
		t.Fatalf("expected fallback.txt, got %q", got)
	}
}

func TestFileNameForAttachment_DefaultName(t *testing.T) {
	att := ports.Attachment{}
	got := fileNameForAttachment(att, "")
	if got != "attachment" {
		t.Fatalf("expected 'attachment', got %q", got)
	}
}

func TestFileNameForAttachment_AppendsExtFromMediaType(t *testing.T) {
	att := ports.Attachment{Name: "doc", MediaType: "application/pdf"}
	got := fileNameForAttachment(att, "")
	// mime.ExtensionsByType returns .pdf for application/pdf
	if got != "doc.pdf" {
		t.Fatalf("expected doc.pdf, got %q", got)
	}
}

// --- fileTypeForAttachment ---

func TestFileTypeForAttachment_ByExtension(t *testing.T) {
	if got := fileTypeForAttachment("report.pdf", ""); got != "pdf" {
		t.Fatalf("expected pdf, got %q", got)
	}
}

func TestFileTypeForAttachment_ByMediaType(t *testing.T) {
	got := fileTypeForAttachment("noext", "application/pdf")
	if got != "pdf" {
		t.Fatalf("expected pdf from media type, got %q", got)
	}
}

func TestFileTypeForAttachment_Default(t *testing.T) {
	if got := fileTypeForAttachment("noext", ""); got != "bin" {
		t.Fatalf("expected bin, got %q", got)
	}
}

// --- extensionForMediaType ---

func TestExtensionForMediaType_Known(t *testing.T) {
	got := extensionForMediaType("application/pdf")
	if got != ".pdf" {
		t.Fatalf("expected .pdf, got %q", got)
	}
}

func TestExtensionForMediaType_Empty(t *testing.T) {
	if got := extensionForMediaType(""); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtensionForMediaType_Unknown(t *testing.T) {
	got := extensionForMediaType("application/x-totally-unknown-type")
	if got != "" {
		t.Fatalf("expected empty for unknown, got %q", got)
	}
}

// --- normalizeExtensions ---

func TestNormalizeExtensions_Empty(t *testing.T) {
	if got := normalizeExtensions(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestNormalizeExtensions_AddsDotPrefix(t *testing.T) {
	got := normalizeExtensions([]string{"pdf", "png"})
	if len(got) != 2 || got[0] != ".pdf" || got[1] != ".png" {
		t.Fatalf("expected [.pdf .png], got %v", got)
	}
}

func TestNormalizeExtensions_PreservesDot(t *testing.T) {
	got := normalizeExtensions([]string{".pdf"})
	if len(got) != 1 || got[0] != ".pdf" {
		t.Fatalf("expected [.pdf], got %v", got)
	}
}

func TestNormalizeExtensions_Deduplicates(t *testing.T) {
	got := normalizeExtensions([]string{"pdf", ".PDF", "Pdf"})
	if len(got) != 1 || got[0] != ".pdf" {
		t.Fatalf("expected single [.pdf], got %v", got)
	}
}

func TestNormalizeExtensions_SkipsEmpty(t *testing.T) {
	got := normalizeExtensions([]string{"", "  ", "pdf"})
	if len(got) != 1 || got[0] != ".pdf" {
		t.Fatalf("expected [.pdf], got %v", got)
	}
}

// --- filterNonA2UIAttachments ---

func TestFilterNonA2UIAttachments_FiltersA2UI(t *testing.T) {
	atts := map[string]ports.Attachment{
		"normal.pdf": {MediaType: "application/pdf"},
		"ui.json":    {MediaType: "application/a2ui+json"},
	}
	got := filterNonA2UIAttachments(atts)
	if len(got) != 1 || got["normal.pdf"].MediaType != "application/pdf" {
		t.Fatalf("expected only normal.pdf, got %v", got)
	}
}

func TestFilterNonA2UIAttachments_AllA2UI(t *testing.T) {
	atts := map[string]ports.Attachment{
		"ui.json": {Format: "a2ui"},
	}
	if got := filterNonA2UIAttachments(atts); got != nil {
		t.Fatalf("expected nil when all filtered, got %v", got)
	}
}

func TestFilterNonA2UIAttachments_Empty(t *testing.T) {
	if got := filterNonA2UIAttachments(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// --- collectAttachmentsFromResult ---

func TestCollectAttachmentsFromResult_Nil(t *testing.T) {
	if got := collectAttachmentsFromResult(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestCollectAttachmentsFromResult_FromMessages(t *testing.T) {
	result := &agent.TaskResult{
		Messages: []ports.Message{
			{Attachments: map[string]ports.Attachment{"a.pdf": {Name: "a.pdf"}}},
		},
	}
	got := collectAttachmentsFromResult(result)
	if len(got) != 1 || got["a.pdf"].Name != "a.pdf" {
		t.Fatalf("expected a.pdf, got %v", got)
	}
}

func TestCollectAttachmentsFromResult_FromToolResults(t *testing.T) {
	result := &agent.TaskResult{
		Messages: []ports.Message{
			{
				ToolResults: []ports.ToolResult{
					{Attachments: map[string]ports.Attachment{"b.png": {Name: "b.png"}}},
				},
			},
		},
	}
	got := collectAttachmentsFromResult(result)
	if len(got) != 1 || got["b.png"].Name != "b.png" {
		t.Fatalf("expected b.png, got %v", got)
	}
}

// --- buildAttachmentSummary ---

func TestBuildAttachmentSummary_Nil(t *testing.T) {
	if got := buildAttachmentSummary(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestBuildAttachmentSummary_WithURI(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"report.pdf": {URI: "https://cdn/report.pdf"},
		},
	}
	got := buildAttachmentSummary(result)
	if got == "" {
		t.Fatal("expected non-empty summary")
	}
	if got != "---\n[Attachments]\n- report.pdf: https://cdn/report.pdf" {
		t.Fatalf("unexpected summary: %q", got)
	}
}

func TestBuildAttachmentSummary_DataURINoLink(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"img.png": {URI: "data:image/png;base64,abc"},
		},
	}
	got := buildAttachmentSummary(result)
	if got != "---\n[Attachments]\n- img.png" {
		t.Fatalf("unexpected summary for data: URI: %q", got)
	}
}

func TestBuildAttachmentSummary_FiltersA2UI(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"ui.json": {Format: "a2ui", URI: "https://cdn/ui.json"},
		},
	}
	if got := buildAttachmentSummary(result); got != "" {
		t.Fatalf("expected empty for all-A2UI, got %q", got)
	}
}

// --- deref ---

func TestDeref_Nil(t *testing.T) {
	if got := deref(nil); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestDeref_Value(t *testing.T) {
	s := "hello"
	if got := deref(&s); got != "hello" {
		t.Fatalf("expected hello, got %q", got)
	}
}

// --- isResultAwaitingInput ---

func TestIsResultAwaitingInput_True(t *testing.T) {
	result := &agent.TaskResult{StopReason: "await_user_input"}
	if !isResultAwaitingInput(result) {
		t.Fatal("expected true")
	}
}

func TestIsResultAwaitingInput_CaseInsensitive(t *testing.T) {
	result := &agent.TaskResult{StopReason: "  AWAIT_USER_INPUT  "}
	if !isResultAwaitingInput(result) {
		t.Fatal("expected case-insensitive match")
	}
}

func TestIsResultAwaitingInput_Nil(t *testing.T) {
	if isResultAwaitingInput(nil) {
		t.Fatal("expected false for nil")
	}
}

func TestIsResultAwaitingInput_Other(t *testing.T) {
	result := &agent.TaskResult{StopReason: "completed"}
	if isResultAwaitingInput(result) {
		t.Fatal("expected false for non-await")
	}
}

// --- sessionHasAwaitFlag ---

func TestSessionHasAwaitFlag_True(t *testing.T) {
	session := &storage.Session{
		Metadata: map[string]string{"await_user_input": "true"},
	}
	if !sessionHasAwaitFlag(session) {
		t.Fatal("expected true")
	}
}

func TestSessionHasAwaitFlag_False(t *testing.T) {
	session := &storage.Session{
		Metadata: map[string]string{},
	}
	if sessionHasAwaitFlag(session) {
		t.Fatal("expected false")
	}
}

func TestSessionHasAwaitFlag_Nil(t *testing.T) {
	if sessionHasAwaitFlag(nil) {
		t.Fatal("expected false for nil session")
	}
}
