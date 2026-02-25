package ports

import "testing"

func TestMergeAttachmentMapsOverrideFalseKeepsExisting(t *testing.T) {
	dst := map[string]Attachment{
		"report.md": {Name: "report.md", MediaType: "text/plain"},
	}
	src := map[string]Attachment{
		"report.md": {Name: "report.md", MediaType: "application/pdf"},
	}

	merged := MergeAttachmentMaps(dst, src, false)
	if merged["report.md"].MediaType != "text/plain" {
		t.Fatalf("expected existing value to win, got %q", merged["report.md"].MediaType)
	}
}

func TestMergeAttachmentMapsOverrideTrueReplacesExisting(t *testing.T) {
	dst := map[string]Attachment{
		"report.md": {Name: "report.md", MediaType: "text/plain"},
	}
	src := map[string]Attachment{
		"report.md": {Name: "report.md", MediaType: "application/pdf"},
	}

	merged := MergeAttachmentMaps(dst, src, true)
	if merged["report.md"].MediaType != "application/pdf" {
		t.Fatalf("expected incoming value to win, got %q", merged["report.md"].MediaType)
	}
}

func TestMergeAttachmentMapsNormalizesName(t *testing.T) {
	merged := MergeAttachmentMaps(nil, map[string]Attachment{
		"  photo.png  ": {},
	}, true)

	att, ok := merged["photo.png"]
	if !ok {
		t.Fatalf("expected normalized key, got %#v", merged)
	}
	if att.Name != "photo.png" {
		t.Fatalf("expected Name to be filled from normalized key, got %q", att.Name)
	}
}

func TestMergeAttachmentMapsFallsBackToAttachmentName(t *testing.T) {
	merged := MergeAttachmentMaps(nil, map[string]Attachment{
		"": {Name: "diagram.svg"},
	}, true)

	if _, ok := merged["diagram.svg"]; !ok {
		t.Fatalf("expected fallback to attachment name, got %#v", merged)
	}
}

func TestMergeAttachmentMapsSkipsEmptyNames(t *testing.T) {
	merged := MergeAttachmentMaps(nil, map[string]Attachment{
		"": {Name: " "},
	}, true)
	if len(merged) != 0 {
		t.Fatalf("expected no merged entries, got %#v", merged)
	}
}

func TestIsImageAttachmentByMediaType(t *testing.T) {
	att := Attachment{MediaType: "image/heic"}
	if !IsImageAttachment(att, "") {
		t.Fatal("expected image media type to be recognized")
	}
}

func TestIsImageAttachmentByNameExtension(t *testing.T) {
	att := Attachment{Name: "photo.tiff"}
	if !IsImageAttachment(att, "") {
		t.Fatal("expected tiff extension to be recognized")
	}
}

func TestIsImageAttachmentByFallbackName(t *testing.T) {
	if !IsImageAttachment(Attachment{}, "snapshot.webp") {
		t.Fatal("expected fallback filename extension to be recognized")
	}
}

func TestIsImageAttachmentReturnsFalseForNonImage(t *testing.T) {
	if IsImageAttachment(Attachment{Name: "report.pdf"}, "") {
		t.Fatal("expected non-image attachment")
	}
}
