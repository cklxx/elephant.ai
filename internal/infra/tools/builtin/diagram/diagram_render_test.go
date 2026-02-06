package diagram

import (
	"strings"
	"testing"
)

func TestNormalizeMermaidSource_StripsCodeFence(t *testing.T) {
	source := "```mermaid\ngraph LR\n  A-->B\n```"
	got, hasInit, err := normalizeMermaidSource(source)
	if err != nil {
		t.Fatalf("normalizeMermaidSource error: %v", err)
	}
	if hasInit {
		t.Fatalf("expected hasInit=false, got true")
	}
	if strings.Contains(got, "```") {
		t.Fatalf("expected code fence stripped, got %q", got)
	}
	if !strings.Contains(got, "graph LR") {
		t.Fatalf("expected graph content preserved, got %q", got)
	}
}

func TestNormalizeMermaidSource_DetectsInitDirective(t *testing.T) {
	source := "%%{init: {'theme':'dark'}}%%\nflowchart LR\nA-->B"
	got, hasInit, err := normalizeMermaidSource(source)
	if err != nil {
		t.Fatalf("normalizeMermaidSource error: %v", err)
	}
	if !hasInit {
		t.Fatalf("expected hasInit=true, got false")
	}
	if got != source {
		t.Fatalf("expected source unchanged, got %q", got)
	}
}

func TestBuildMermaidHTML_EscapesSource(t *testing.T) {
	raw := "graph LR\nA-->B\nA-->C</script><script>alert(1)</script>"
	normalized, _, err := normalizeMermaidSource(raw)
	if err != nil {
		t.Fatalf("normalizeMermaidSource error: %v", err)
	}

	htmlDoc, err := buildMermaidHTML(mermaidHTMLParams{
		Source:           normalized,
		Theme:            "light",
		Padding:          32,
		HasInitDirective: false,
	})
	if err != nil {
		t.Fatalf("buildMermaidHTML error: %v", err)
	}
	if !strings.Contains(htmlDoc, `data-diagram-status="loading"`) {
		t.Fatalf("expected diagram status marker")
	}
	if !strings.Contains(htmlDoc, `<pre id="source"`) {
		t.Fatalf("expected source pre block")
	}

	escaped := `C&lt;/script&gt;&lt;script&gt;alert(1)&lt;/script&gt;`
	if !strings.Contains(htmlDoc, escaped) {
		t.Fatalf("expected escaped source to be present")
	}
}

func TestBuildMermaidHTML_ThemeVariablesOmittedWhenInitDirectivePresent(t *testing.T) {
	source := "%%{init: {'theme':'dark'}}%%\nflowchart LR\nA-->B"
	normalized, hasInit, err := normalizeMermaidSource(source)
	if err != nil {
		t.Fatalf("normalizeMermaidSource error: %v", err)
	}
	if !hasInit {
		t.Fatalf("expected init directive detected")
	}

	htmlDoc, err := buildMermaidHTML(mermaidHTMLParams{
		Source:           normalized,
		Theme:            "light",
		Padding:          32,
		HasInitDirective: true,
	})
	if err != nil {
		t.Fatalf("buildMermaidHTML error: %v", err)
	}

	idx := strings.Index(htmlDoc, "const config=")
	if idx == -1 {
		t.Fatalf("expected config script")
	}
	window := htmlDoc[idx:]
	if len(window) > 600 {
		window = window[:600]
	}
	if strings.Contains(window, "themeVariables") {
		t.Fatalf("expected themeVariables to be omitted when init directive is present")
	}
}

func TestBuildIconBlocksHTML_RendersItems(t *testing.T) {
	htmlDoc, err := buildIconBlocksHTML(iconBlocksHTMLParams{
		Title: "Release Highlights",
		Items: []IconBlockItem{
			{Icon: "🚀", Title: "Ship", Description: "Deploy to prod"},
			{Icon: "🔍", Title: "Observe", Description: "Monitor SLO"},
		},
		Theme:   "dark",
		Padding: 24,
	})
	if err != nil {
		t.Fatalf("buildIconBlocksHTML error: %v", err)
	}
	if !strings.Contains(htmlDoc, "Release Highlights") {
		t.Fatalf("expected title to be present")
	}
	if !strings.Contains(htmlDoc, "Ship") || !strings.Contains(htmlDoc, "Observe") {
		t.Fatalf("expected item titles to be present")
	}
	if !strings.Contains(htmlDoc, "dataset.diagramStatus='ready'") {
		t.Fatalf("expected ready marker")
	}
}

func TestSanitizeOutputBase_DefaultsAndStripsExt(t *testing.T) {
	if got := sanitizeOutputBase("", "mermaid"); got != "diagram" {
		t.Fatalf("expected default diagram, got %q", got)
	}
	if got := sanitizeOutputBase("", "icon_blocks"); got != "icons" {
		t.Fatalf("expected default icons, got %q", got)
	}
	if got := sanitizeOutputBase("foo.png", "mermaid"); got != "foo" {
		t.Fatalf("expected ext stripped, got %q", got)
	}
	if got := sanitizeOutputBase("foo.svg", "mermaid"); got != "foo" {
		t.Fatalf("expected ext stripped, got %q", got)
	}
	if got := sanitizeOutputBase("../weird name!!.png", "mermaid"); got == "" || strings.Contains(got, " ") || strings.Contains(got, "/") {
		t.Fatalf("expected sanitized base, got %q", got)
	}
}

func TestParseIconBlockItems_ValidatesRequiredFields(t *testing.T) {
	_, err := parseIconBlockItems([]any{map[string]any{"title": "T"}})
	if err == nil {
		t.Fatalf("expected error when icon is missing")
	}
	_, err = parseIconBlockItems([]any{map[string]any{"icon": "DB"}})
	if err == nil {
		t.Fatalf("expected error when title is missing")
	}

	items, err := parseIconBlockItems([]any{map[string]any{"icon": "DB", "title": "Storage"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 || items[0].Icon != "DB" || items[0].Title != "Storage" {
		t.Fatalf("unexpected parsed items: %#v", items)
	}
}

func TestParseDiagramRenderArgs_Defaults(t *testing.T) {
	parsed, err := parseDiagramRenderArgs(map[string]any{
		"source": "graph LR; A-->B;",
	})
	if err != nil {
		t.Fatalf("parseDiagramRenderArgs error: %v", err)
	}
	if parsed.Format != "mermaid" {
		t.Fatalf("expected default format=mermaid, got %q", parsed.Format)
	}
	if parsed.Output != "png" {
		t.Fatalf("expected default output=png, got %q", parsed.Output)
	}
	if parsed.Width != defaultWidth || parsed.Height != defaultHeight {
		t.Fatalf("expected default viewport %dx%d, got %dx%d", defaultWidth, defaultHeight, parsed.Width, parsed.Height)
	}
	if parsed.Padding != defaultPadding {
		t.Fatalf("expected default padding %d, got %d", defaultPadding, parsed.Padding)
	}
}

