package broker

import (
	"context"
	"fmt"
	"html"
	"strings"

	materialapi "alex/internal/materials/api"
	"alex/internal/materials/storage"
	blackfriday "github.com/russross/blackfriday/v2"
)

type previewGenerationInput struct {
	Name     string
	Source   string
	Format   string
	Kind     materialapi.MaterialKind
	MimeType string
	Payload  []byte
	Upload   storage.UploadResult
}

var markdownExtensions = blackfriday.CommonExtensions |
	blackfriday.AutoHeadingIDs |
	blackfriday.FencedCode |
	blackfriday.Strikethrough |
	blackfriday.Tables

func (b *AttachmentBroker) generatePreviewAssets(ctx context.Context, input previewGenerationInput) ([]*materialapi.PreviewAsset, error) {
	if input.Kind != materialapi.MaterialKindArtifact {
		return nil, nil
	}

	format := strings.ToLower(input.Format)
	switch format {
	case "html":
		return []*materialapi.PreviewAsset{
			{
				AssetID:     input.Upload.StorageKey,
				Label:       "HTML Preview",
				MimeType:    coalesce(input.MimeType, "text/html"),
				CDNURL:      input.Upload.CDNURL,
				PreviewType: "document.html",
			},
		}, nil
	case "markdown":
		rendered := renderMarkdownPreview(input.Name, input.Payload)
		uploadName := fmt.Sprintf("%s-rendered.html", sanitizePreviewName(input.Name))
		uploaded, err := b.storage.Upload(ctx, storage.UploadRequest{
			Name:     uploadName,
			MimeType: "text/html",
			Data:     rendered,
			Source:   input.Source,
		})
		if err != nil {
			return nil, err
		}
		if err := b.storage.Prewarm(ctx, uploaded.StorageKey); err != nil {
			return nil, err
		}
		return []*materialapi.PreviewAsset{
			{
				AssetID:     uploaded.StorageKey,
				Label:       "Rendered Markdown",
				MimeType:    "text/html",
				CDNURL:      uploaded.CDNURL,
				PreviewType: "document.html",
			},
		}, nil
	default:
		return nil, nil
	}
}

func renderMarkdownPreview(name string, payload []byte) []byte {
	body := blackfriday.Run(payload, blackfriday.WithExtensions(markdownExtensions))
	title := html.EscapeString(coalesce(name, "Artifact"))
	template := `<!doctype html><html lang="en"><head><meta charset="utf-8"/><title>%s</title>` +
		`<style>body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;background:#f8fafc;color:#0f172a;padding:2rem;} ` +
		`main{max-width:960px;margin:0 auto;background:white;border-radius:1rem;box-shadow:0 10px 35px rgba(15,23,42,0.08);padding:2rem;} ` +
		`code{background:#0f172a0d;padding:0.2em 0.4em;border-radius:0.375rem;font-size:0.9em;} pre{background:#0f172a0d;padding:1rem;border-radius:0.75rem;overflow:auto;} ` +
		`table{width:100%%;border-collapse:collapse;margin:1rem 0;} th,td{border:1px solid #e2e8f0;padding:0.5rem;text-align:left;} ` +
		`img{max-width:100%%;height:auto;border-radius:0.5rem;}</style></head><body><main>%s</main></body></html>`
	return []byte(fmt.Sprintf(template, title, body))
}

func sanitizePreviewName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "artifact"
	}
	trimmed = strings.ReplaceAll(trimmed, " ", "-")
	trimmed = strings.ReplaceAll(trimmed, "..", ".")
	return trimmed
}
