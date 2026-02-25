package ports

import "testing"

func TestNormalizeAttachmentMapFillsOnlyExactEmptyName(t *testing.T) {
	got := NormalizeAttachmentMap(map[string]Attachment{
		" report.md ": {URI: "https://example.com/report.md"},
		"seed.png":    {Name: "   ", URI: "https://example.com/seed.png"},
	})

	if len(got) != 2 {
		t.Fatalf("expected two normalized entries, got %#v", got)
	}

	report, ok := got["report.md"]
	if !ok {
		t.Fatalf("expected trimmed key to exist, got %#v", got)
	}
	if report.Name != "report.md" {
		t.Fatalf("expected exact-empty Name to be filled, got %q", report.Name)
	}

	seed, ok := got["seed.png"]
	if !ok {
		t.Fatalf("expected seed attachment to exist, got %#v", got)
	}
	if seed.Name != "   " {
		t.Fatalf("expected whitespace-only Name to remain unchanged, got %q", seed.Name)
	}
}

func TestNormalizeAttachmentMapFillBlankNameFillsWhitespaceName(t *testing.T) {
	got := NormalizeAttachmentMapFillBlankName(map[string]Attachment{
		"seed.png": {Name: "   ", URI: "https://example.com/seed.png"},
	})

	seed, ok := got["seed.png"]
	if !ok {
		t.Fatalf("expected seed attachment to exist, got %#v", got)
	}
	if seed.Name != "seed.png" {
		t.Fatalf("expected whitespace-only Name to be replaced, got %q", seed.Name)
	}
}

func TestNormalizeAttachmentMapFallsBackToAttachmentNameAndSkipsEmpty(t *testing.T) {
	got := NormalizeAttachmentMap(map[string]Attachment{
		"":    {Name: "  diagram.svg  "},
		"   ": {Name: "   "},
	})

	if len(got) != 1 {
		t.Fatalf("expected only one valid normalized entry, got %#v", got)
	}

	diagram, ok := got["diagram.svg"]
	if !ok {
		t.Fatalf("expected fallback to trimmed attachment name, got %#v", got)
	}
	if diagram.Name != "  diagram.svg  " {
		t.Fatalf("expected original non-empty Name to be preserved, got %q", diagram.Name)
	}
}
