package diagram

import (
	"fmt"
	"html"
	"strings"
)

type IconBlockItem struct {
	Icon        string `json:"icon"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

type iconBlocksHTMLParams struct {
	Title   string
	Items   []IconBlockItem
	Theme   string
	Padding int
}

func buildIconBlocksHTML(params iconBlocksHTMLParams) (string, error) {
	theme := normalizeTheme(params.Theme)
	padding := clampInt(params.Padding, 0, 128)
	if padding == 0 {
		padding = 32
	}

	card := cardTheme(theme)
	bg := backgroundTheme(theme)
	text := textTheme(theme)

	title := strings.TrimSpace(params.Title)
	if title == "" {
		title = "Highlights"
	}

	var b strings.Builder
	b.Grow(4096)

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
  color:%s;
}
.stack{display:flex;flex-direction:column;gap:16px}
.title{
  font-size:28px;
  font-weight:800;
  letter-spacing:-0.02em;
  color:%s;
}
.grid{
  display:grid;
  grid-template-columns:repeat(auto-fit,minmax(260px,1fr));
  gap:16px;
}
.card{
  background:%s;
  border:2px solid %s;
  border-radius:16px;
  box-shadow:%s;
  padding:18px 18px 16px 18px;
  display:flex;
  flex-direction:column;
  gap:10px;
  min-height:120px;
}
.iconRow{display:flex;align-items:center;gap:12px}
.icon{
  width:40px;height:40px;border-radius:12px;
  display:flex;align-items:center;justify-content:center;
  background:%s;
  border:1px solid %s;
  font-size:22px;
}
.cardTitle{font-size:18px;font-weight:800;letter-spacing:-0.01em;color:%s}
.cardDesc{font-size:14px;line-height:1.4;color:%s}
`, padding, bg, text.Body, text.Title, card.Background, card.Border, card.Shadow, card.IconBackground, card.IconBorder, text.Title, text.Muted)
	b.WriteString(`</style></head><body>`)

	b.WriteString(`<div id="capture"><div class="stack">`)
	b.WriteString(`<div class="title">`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</div>`)

	b.WriteString(`<div class="grid">`)
	for _, item := range params.Items {
		b.WriteString(`<div class="card">`)
		b.WriteString(`<div class="iconRow">`)
		b.WriteString(`<div class="icon">`)
		b.WriteString(html.EscapeString(strings.TrimSpace(item.Icon)))
		b.WriteString(`</div>`)
		b.WriteString(`<div class="cardTitle">`)
		b.WriteString(html.EscapeString(strings.TrimSpace(item.Title)))
		b.WriteString(`</div>`)
		b.WriteString(`</div>`)
		if desc := strings.TrimSpace(item.Description); desc != "" {
			b.WriteString(`<div class="cardDesc">`)
			b.WriteString(html.EscapeString(desc))
			b.WriteString(`</div>`)
		}
		b.WriteString(`</div>`)
	}
	b.WriteString(`</div>`)
	b.WriteString(`</div></div>`)

	b.WriteString(`<script>document.documentElement.dataset.diagramStatus='ready';</script>`)
	b.WriteString(`</body></html>`)

	return b.String(), nil
}

