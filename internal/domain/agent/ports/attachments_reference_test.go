package ports

import "testing"

func TestAttachmentReferenceValuePrefersURI(t *testing.T) {
	att := Attachment{
		Name:      "diagram.png",
		MediaType: "image/png",
		Data:      "ZGF0YQo=",
		URI:       "https://example.com/diagram.png",
	}

	if got := AttachmentReferenceValue(att); got != att.URI {
		t.Fatalf("expected URI to be preferred, got %q", got)
	}
}

func TestAttachmentReferenceValueFallsBackToDataURI(t *testing.T) {
	att := Attachment{
		Name:      "diagram.png",
		MediaType: "image/png",
		Data:      "ZGF0YQo=",
	}

	got := AttachmentReferenceValue(att)
	if wantPrefix := "data:image/png;base64,"; len(got) < len(wantPrefix) || got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("expected data URI prefix %q, got %q", wantPrefix, got)
	}
}

func TestAttachmentInlineBase64PrefersDataField(t *testing.T) {
	att := Attachment{Data: "ZGF0YQo="}
	if got := AttachmentInlineBase64(att); got != att.Data {
		t.Fatalf("expected base64 payload, got %q", got)
	}
}

func TestAttachmentInlineBase64ExtractsFromDataURI(t *testing.T) {
	att := Attachment{URI: "data:image/png;base64,ZGF0YQo="}
	if got := AttachmentInlineBase64(att); got != "ZGF0YQo=" {
		t.Fatalf("expected extracted payload, got %q", got)
	}
}
