package react

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

// --- offloadAttachmentMap ---

func TestOffloadAttachmentMap_ClearsDataWhenURIExists(t *testing.T) {
	atts := map[string]ports.Attachment{
		"file.pdf": {
			Name: "file.pdf",
			Data: "base64data",
			URI:  "https://cdn.example.com/file.pdf",
		},
	}
	changed := offloadAttachmentMap(atts)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if atts["file.pdf"].Data != "" {
		t.Fatalf("expected Data cleared, got %q", atts["file.pdf"].Data)
	}
	if atts["file.pdf"].URI != "https://cdn.example.com/file.pdf" {
		t.Fatalf("expected URI preserved, got %q", atts["file.pdf"].URI)
	}
}

func TestOffloadAttachmentMap_PreservesDataWhenURIIsDataScheme(t *testing.T) {
	atts := map[string]ports.Attachment{
		"icon.png": {
			Name: "icon.png",
			Data: "inline_data",
			URI:  "data:image/png;base64,inline_data",
		},
	}
	changed := offloadAttachmentMap(atts)
	if changed {
		t.Fatal("expected no change when URI is data: scheme")
	}
	if atts["icon.png"].Data != "inline_data" {
		t.Fatalf("expected Data preserved, got %q", atts["icon.png"].Data)
	}
}

func TestOffloadAttachmentMap_PreservesDataWhenNoURI(t *testing.T) {
	atts := map[string]ports.Attachment{
		"img.png": {
			Name: "img.png",
			Data: "some_data",
		},
	}
	changed := offloadAttachmentMap(atts)
	if changed {
		t.Fatal("expected no change when no URI")
	}
	if atts["img.png"].Data != "some_data" {
		t.Fatalf("expected Data preserved, got %q", atts["img.png"].Data)
	}
}

func TestOffloadAttachmentMap_SkipsEmptyData(t *testing.T) {
	atts := map[string]ports.Attachment{
		"doc.txt": {
			Name: "doc.txt",
			URI:  "https://cdn.example.com/doc.txt",
		},
	}
	changed := offloadAttachmentMap(atts)
	if changed {
		t.Fatal("expected no change when Data is already empty")
	}
}

func TestOffloadAttachmentMap_NilMap(t *testing.T) {
	changed := offloadAttachmentMap(nil)
	if changed {
		t.Fatal("expected no change for nil map")
	}
}

func TestOffloadAttachmentMap_MultipleAttachments(t *testing.T) {
	atts := map[string]ports.Attachment{
		"a.pdf": {Name: "a.pdf", Data: "data_a", URI: "https://cdn/a.pdf"},
		"b.png": {Name: "b.png", Data: "data_b"},                              // no URI
		"c.jpg": {Name: "c.jpg", Data: "data_c", URI: "data:image/jpg;base64"}, // data: URI
	}
	changed := offloadAttachmentMap(atts)
	if !changed {
		t.Fatal("expected changed=true for at least one attachment")
	}
	if atts["a.pdf"].Data != "" {
		t.Fatal("expected a.pdf data cleared")
	}
	if atts["b.png"].Data != "data_b" {
		t.Fatal("expected b.png data preserved")
	}
	if atts["c.jpg"].Data != "data_c" {
		t.Fatal("expected c.jpg data preserved")
	}
}

// --- offloadToolResultAttachmentData ---

func TestOffloadToolResultAttachmentData_Empty(t *testing.T) {
	// Should not panic
	offloadToolResultAttachmentData(nil)
	offloadToolResultAttachmentData([]ToolResult{})
}

func TestOffloadToolResultAttachmentData_ClearsData(t *testing.T) {
	results := []ToolResult{{
		CallID: "c1",
		Attachments: map[string]ports.Attachment{
			"report.pdf": {
				Name: "report.pdf",
				Data: "pdf_data",
				URI:  "https://cdn/report.pdf",
			},
		},
	}}
	offloadToolResultAttachmentData(results)
	if results[0].Attachments["report.pdf"].Data != "" {
		t.Fatal("expected data cleared from tool result attachment")
	}
}

// --- offloadMessageAttachmentData ---

func TestOffloadMessageAttachmentData_NilState(t *testing.T) {
	// Should not panic
	offloadMessageAttachmentData(nil)
}

func TestOffloadMessageAttachmentData_ClearsMessageAndToolResult(t *testing.T) {
	state := &TaskState{
		Messages: []Message{
			{
				Role: "tool",
				Attachments: map[string]ports.Attachment{
					"file.txt": {Name: "file.txt", Data: "abc", URI: "https://cdn/file.txt"},
				},
				ToolResults: []ToolResult{{
					CallID: "c1",
					Attachments: map[string]ports.Attachment{
						"img.png": {Name: "img.png", Data: "png_data", URI: "https://cdn/img.png"},
					},
				}},
			},
		},
	}
	offloadMessageAttachmentData(state)

	if state.Messages[0].Attachments["file.txt"].Data != "" {
		t.Fatal("expected message attachment data cleared")
	}
	if state.Messages[0].ToolResults[0].Attachments["img.png"].Data != "" {
		t.Fatal("expected tool result attachment data cleared")
	}
}

func TestOffloadMessageAttachmentData_EmptyMessages(t *testing.T) {
	state := &TaskState{Messages: nil}
	offloadMessageAttachmentData(state) // should not panic
}
