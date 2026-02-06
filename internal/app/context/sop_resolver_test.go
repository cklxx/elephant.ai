package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

func TestParseSOPRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantFile string
		wantAnc  string
	}{
		{"file.md#anchor", "file.md", "anchor"},
		{"path/to/file.md#some-section", "path/to/file.md", "some-section"},
		{"file.md", "file.md", ""},
		{"#anchor-only", "", "anchor-only"},
		{"file.md#a#b", "file.md#a", "b"},
		{"  file.md#anchor  ", "file.md", "anchor"},
	}
	for _, tt := range tests {
		f, a := ParseSOPRef(tt.ref)
		if f != tt.wantFile || a != tt.wantAnc {
			t.Errorf("ParseSOPRef(%q) = (%q, %q), want (%q, %q)", tt.ref, f, a, tt.wantFile, tt.wantAnc)
		}
	}
}

func TestSlugifyHeading(t *testing.T) {
	tests := []struct {
		heading string
		want    string
	}{
		{"Task Execution Framework", "task-execution-framework"},
		{"TODO Lifecycle", "todo-lifecycle"},
		{"Research & Investigation Strategy", "research--investigation-strategy"},
		{"Context Budget & Compression", "context-budget--compression"},
		{"Observability & Traceability", "observability--traceability"},
		{"  Spaces  Around  ", "spaces--around"},
		{"ReAct Execution Loop", "react-execution-loop"},
		{"Memory & Sessions", "memory--sessions"},
		{"Hello World!", "hello-world"},
		{"Simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		got := SlugifyHeading(tt.heading)
		if got != tt.want {
			t.Errorf("SlugifyHeading(%q) = %q, want %q", tt.heading, got, tt.want)
		}
	}
}

func TestExtractMarkdownSection(t *testing.T) {
	content := `# Top Level

Introduction text.

## Section A

Content A line 1.
Content A line 2.

### Sub Section A1

Sub content.

## Section B

Content B.

## Section C

Content C.
`

	t.Run("extracts section with sub-headings", func(t *testing.T) {
		section := ExtractMarkdownSection(content, "section-a")
		if !strings.Contains(section, "## Section A") {
			t.Fatalf("expected section heading, got %q", section)
		}
		if !strings.Contains(section, "Content A line 1") {
			t.Fatalf("expected section content, got %q", section)
		}
		if !strings.Contains(section, "### Sub Section A1") {
			t.Fatalf("expected sub-heading to be included, got %q", section)
		}
		if strings.Contains(section, "## Section B") {
			t.Fatalf("should not include next same-level section, got %q", section)
		}
	})

	t.Run("extracts leaf section", func(t *testing.T) {
		section := ExtractMarkdownSection(content, "section-b")
		if !strings.Contains(section, "Content B") {
			t.Fatalf("expected section content, got %q", section)
		}
		if strings.Contains(section, "Content C") {
			t.Fatalf("should not include next section, got %q", section)
		}
	})

	t.Run("extracts top level", func(t *testing.T) {
		section := ExtractMarkdownSection(content, "top-level")
		if !strings.Contains(section, "# Top Level") {
			t.Fatalf("expected top level heading, got %q", section)
		}
		if !strings.Contains(section, "Introduction text") {
			t.Fatalf("expected intro text, got %q", section)
		}
		// Top level captures until the next h1 (none), so it captures everything.
		if !strings.Contains(section, "Content C") {
			t.Fatalf("expected all content under top level, got %q", section)
		}
	})

	t.Run("returns empty for missing anchor", func(t *testing.T) {
		section := ExtractMarkdownSection(content, "nonexistent")
		if section != "" {
			t.Fatalf("expected empty for missing anchor, got %q", section)
		}
	})

	t.Run("returns full content for empty anchor", func(t *testing.T) {
		section := ExtractMarkdownSection(content, "")
		if section != content {
			t.Fatalf("expected full content for empty anchor")
		}
	})
}

func TestResolveRefFileNotFound(t *testing.T) {
	resolver := NewSOPResolver(t.TempDir(), nil)
	content, err := resolver.ResolveRef("nonexistent.md#section")
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if content != "" {
		t.Fatalf("expected empty content for missing file, got %q", content)
	}
}

func TestResolveRefAnchorNotFound(t *testing.T) {
	dir := t.TempDir()
	md := "# Heading\n\nSome content.\n"
	if err := os.WriteFile(filepath.Join(dir, "test.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewSOPResolver(dir, nil)
	content, err := resolver.ResolveRef("test.md#nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should fall back to full file content.
	if !strings.Contains(content, "Some content.") {
		t.Fatalf("expected full file content as fallback, got %q", content)
	}
}

func TestResolveRefCaching(t *testing.T) {
	dir := t.TempDir()
	md := "# Test\n\nOriginal content.\n"
	mdPath := filepath.Join(dir, "cached.md")
	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewSOPResolver(dir, nil)

	// First call — cache miss.
	content1, err := resolver.ResolveRef("cached.md#test")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content1, "Original content") {
		t.Fatalf("expected original content, got %q", content1)
	}

	// Modify file on disk — cache should still serve original.
	if err := os.WriteFile(mdPath, []byte("# Test\n\nModified content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	content2, err := resolver.ResolveRef("cached.md#test")
	if err != nil {
		t.Fatal(err)
	}
	if content2 != content1 {
		t.Fatalf("expected cached content %q, got %q", content1, content2)
	}
}

func TestResolveKnowledgeRefs(t *testing.T) {
	dir := t.TempDir()
	md := "# Framework\n\n## Section One\n\nContent one.\n\n## Section Two\n\nContent two.\n"
	subDir := filepath.Join(dir, "docs", "ref")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "sop.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewSOPResolver(dir, nil)
	refs := []agent.KnowledgeReference{{
		ID:      "test-knowledge",
		SOPRefs: []string{"docs/ref/sop.md#section-one", "docs/ref/sop.md#section-two"},
	}}

	enriched := resolver.ResolveKnowledgeRefs(refs)
	if len(enriched) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(enriched))
	}
	if enriched[0].ResolvedSOPContent == nil {
		t.Fatal("expected ResolvedSOPContent to be populated")
	}
	s1 := enriched[0].ResolvedSOPContent["docs/ref/sop.md#section-one"]
	if !strings.Contains(s1, "Content one") {
		t.Fatalf("expected section one content, got %q", s1)
	}
	s2 := enriched[0].ResolvedSOPContent["docs/ref/sop.md#section-two"]
	if !strings.Contains(s2, "Content two") {
		t.Fatalf("expected section two content, got %q", s2)
	}
}

func TestResolveRefPathTraversal(t *testing.T) {
	dir := t.TempDir()
	resolver := NewSOPResolver(dir, nil)

	_, err := resolver.ResolveRef("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "outside repo root") {
		t.Fatalf("expected 'outside repo root' error, got %v", err)
	}
}

func TestResolveRefTruncation(t *testing.T) {
	dir := t.TempDir()
	// Create file larger than maxSOPContentBytes.
	bigContent := "# Big\n\n" + strings.Repeat("x", maxSOPContentBytes+1000) + "\n"
	if err := os.WriteFile(filepath.Join(dir, "big.md"), []byte(bigContent), 0o644); err != nil {
		t.Fatal(err)
	}

	resolver := NewSOPResolver(dir, nil)
	content, err := resolver.ResolveRef("big.md#big")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(content, "... (truncated)") {
		t.Fatalf("expected truncation suffix, got tail %q", content[len(content)-30:])
	}
	// Content before truncation marker should be exactly maxSOPContentBytes.
	beforeSuffix := strings.TrimSuffix(content, "\n... (truncated)")
	if len(beforeSuffix) != maxSOPContentBytes {
		t.Fatalf("expected %d bytes before truncation, got %d", maxSOPContentBytes, len(beforeSuffix))
	}
}

func TestSOPRefLabel(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"docs/ref/TASK_EXECUTION_FRAMEWORK.md#todo-lifecycle", "TASK EXECUTION FRAMEWORK > todo lifecycle"},
		{"file.md", "file"},
		{"path/to/README.md#intro", "README > intro"},
	}
	for _, tt := range tests {
		got := SOPRefLabel(tt.ref)
		if got != tt.want {
			t.Errorf("SOPRefLabel(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestBuildKnowledgeSectionRendersResolvedContent(t *testing.T) {
	refs := []agent.KnowledgeReference{{
		ID:          "sop-pack",
		Description: "Operating procedures",
		SOPRefs:     []string{"docs/ref/sop.md#section-one"},
		ResolvedSOPContent: map[string]string{
			"docs/ref/sop.md#section-one": "## Section One\n\nResolved content here.",
		},
		MemoryKeys: []string{"key1"},
	}}

	section := buildKnowledgeSection(refs)
	if !strings.Contains(section, "SOP [") {
		t.Fatalf("expected SOP label, got %q", section)
	}
	if !strings.Contains(section, "Resolved content here") {
		t.Fatalf("expected resolved content in section, got %q", section)
	}
	// Should NOT contain raw ref string when resolved content is present.
	if strings.Contains(section, "SOP refs:") {
		t.Fatalf("should not contain raw SOP refs when resolved, got %q", section)
	}
	// Memory keys should still be present.
	if !strings.Contains(section, "Memory keys: key1") {
		t.Fatalf("expected memory keys, got %q", section)
	}
}

func TestBuildKnowledgeSectionFallsBackToRawRefs(t *testing.T) {
	refs := []agent.KnowledgeReference{{
		ID:      "no-resolve",
		SOPRefs: []string{"file.md#section"},
	}}

	section := buildKnowledgeSection(refs)
	if !strings.Contains(section, "SOP refs: file.md#section") {
		t.Fatalf("expected raw SOP refs fallback, got %q", section)
	}
}
