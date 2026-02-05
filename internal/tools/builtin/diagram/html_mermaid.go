package diagram

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

//go:embed assets/mermaid.min.js
var mermaidMinJS string

type mermaidHTMLParams struct {
	Source           string
	Theme            string
	Padding          int
	HasInitDirective bool
}

func buildMermaidHTML(params mermaidHTMLParams) (string, error) {
	theme := normalizeTheme(params.Theme)
	padding := clampInt(params.Padding, 0, 128)
	if padding == 0 {
		padding = 32
	}

	card := cardTheme(theme)
	bg := backgroundTheme(theme)

	config, err := mermaidInitConfig(theme, params.HasInitDirective)
	if err != nil {
		return "", err
	}

	escapedSource := html.EscapeString(params.Source)

	var b strings.Builder
	b.Grow(len(escapedSource) + 4096)

	b.WriteString(`<!doctype html><html data-diagram-status="loading"><head>`)
	b.WriteString(`<meta charset="utf-8">`)
	b.WriteString(`<meta name="viewport" content="width=device-width, initial-scale=1">`)
	b.WriteString(`<style>`)
	fmt.Fprintf(&b, `
html,body{margin:0;padding:0}
body{background:transparent}
*{box-sizing:border-box}
#capture{
  display:inline-block;
  padding:%dpx;
  background:%s;
  font-family:Inter,system-ui,-apple-system,Segoe UI,Roboto,Arial,sans-serif;
}
#card{
  display:inline-block;
  background:%s;
  border:2px solid %s;
  border-radius:16px;
  box-shadow:%s;
  padding:24px;
}
#diagram svg{max-width:100%%;height:auto}
`, padding, bg, card.Background, card.Border, card.Shadow)
	b.WriteString(`</style></head><body>`)
	b.WriteString(`<div id="capture"><div id="card"><div id="diagram"></div></div></div>`)
	b.WriteString(`<pre id="source" style="display:none">`)
	b.WriteString(escapedSource)
	b.WriteString(`</pre>`)

	b.WriteString(`<script>`)
	b.WriteString(mermaidMinJS)
	b.WriteString(`</script>`)

	b.WriteString(`<script>(async()=>{`)
	b.WriteString(`try{`)
	b.WriteString(`const config=`)
	b.WriteString(string(config))
	b.WriteString(`;`)
	b.WriteString(`mermaid.initialize(config);`)
	b.WriteString(`const code=document.getElementById('source').textContent||'';`)
	b.WriteString(`const res=await mermaid.render('m1',code);`)
	b.WriteString(`document.getElementById('diagram').innerHTML=res.svg||'';`)
	b.WriteString(`document.documentElement.dataset.diagramStatus='ready';`)
	b.WriteString(`}catch(e){`)
	b.WriteString(`document.documentElement.dataset.diagramStatus='error';`)
	b.WriteString(`document.documentElement.dataset.diagramError=String(e&&e.message?e.message:e);`)
	b.WriteString(`}`)
	b.WriteString(`})();</script>`)

	b.WriteString(`</body></html>`)
	return b.String(), nil
}

func mermaidInitConfig(theme string, hasInitDirective bool) (json.RawMessage, error) {
	config := map[string]any{
		"startOnLoad":   false,
		"securityLevel": "strict",
		"flowchart":     map[string]any{"useMaxWidth": true},
	}

	if !hasInitDirective {
		config["theme"] = "base"
		config["themeVariables"] = defaultThemeVariables(theme)
	}

	encoded, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func defaultThemeVariables(theme string) map[string]any {
	if theme == "dark" {
		return map[string]any{
			"fontFamily":         "Inter, system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif",
			"fontSize":           "16px",
			"primaryColor":       "#0B1220",
			"primaryTextColor":   "#E2E8F0",
			"primaryBorderColor": "#334155",
			"lineColor":          "#94A3B8",
			"secondaryColor":     "#111827",
			"tertiaryColor":      "#0F172A",
		}
	}
	return map[string]any{
		"fontFamily":         "Inter, system-ui, -apple-system, Segoe UI, Roboto, Arial, sans-serif",
		"fontSize":           "16px",
		"primaryColor":       "#EEF2FF",
		"primaryTextColor":   "#0F172A",
		"primaryBorderColor": "#A5B4FC",
		"lineColor":          "#334155",
		"secondaryColor":     "#ECFEFF",
		"tertiaryColor":      "#FDF2F8",
	}
}
