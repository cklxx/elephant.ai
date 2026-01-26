package artifacts

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"

	_ "embed"

	"alex/internal/tools/builtin/shared"
	"github.com/jung-kurt/gofpdf"
)

const (
	pptxFromImagesMediaType = "application/vnd.openxmlformats-officedocument.presentationml.presentation"
	pptxSlideLayoutTarget   = "../slideLayouts/slideLayout7.xml"

	pptxDefaultSlideCX = 12192000
	pptxDefaultSlideCY = 6858000

	pdfMediaType = "application/pdf"
)

//go:embed assets/pptx_blank_template.pptx
var pptxBlankTemplate []byte

type pptxFromImages struct {
	httpClient *http.Client
}

func NewPPTXFromImages() tools.ToolExecutor {
	return &pptxFromImages{
		httpClient: NewAttachmentHTTPClient(2*time.Minute, "PPTXFromImages"),
	}
}

func (t *pptxFromImages) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "pptx_from_images",
		Version:  "1.0.0",
		Category: "design",
		Tags:     []string{"pptx", "slides", "deck", "powerpoint", "image"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg"},
			Produces: []string{pptxFromImagesMediaType, pdfMediaType},
		},
	}
}

func (t *pptxFromImages) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "pptx_from_images",
		Description: "Assemble a PPTX deck from a list of slide images (pure-image slides).",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"images": {
					Type:        "array",
					Description: "Ordered list of images (data URI, HTTPS URL, base64 string, or prior attachment placeholder such as `[slide.png]`).",
					Items:       &ports.Property{Type: "string"},
				},
				"output_name": {
					Type:        "string",
					Description: "Output filename (default: deck.pptx).",
				},
				"description": {
					Type:        "string",
					Description: "Optional description for the resulting deck attachment.",
				},
			},
			Required: []string{"images"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"image/png", "image/jpeg"},
			Produces: []string{pptxFromImagesMediaType, pdfMediaType},
		},
	}
}

func (t *pptxFromImages) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	images := shared.StringSliceArg(call.Arguments, "images")
	if len(images) == 0 {
		err := errors.New("images is required")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	outputName := strings.TrimSpace(shared.StringArg(call.Arguments, "output_name"))
	if outputName == "" {
		outputName = "deck.pptx"
	}
	outputName = filepath.Base(outputName)
	if outputName == "." || outputName == "/" || outputName == "" {
		outputName = "deck.pptx"
	}
	if !strings.HasSuffix(strings.ToLower(outputName), ".pptx") {
		outputName += ".pptx"
	}

	description := strings.TrimSpace(shared.StringArg(call.Arguments, "description"))
	if description == "" {
		description = fmt.Sprintf("Generated deck with %d image slide(s)", len(images))
	}

	resolved := make([]resolvedPPTXImage, 0, len(images))
	for idx, ref := range images {
		payload, mimeType, err := t.resolveImageBytes(ctx, ref)
		if err != nil {
			err = fmt.Errorf("resolve images[%d]: %w", idx, err)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		ext, ok := extensionForImageMIME(mimeType)
		if !ok {
			err := fmt.Errorf("unsupported image type %q (expected image/png or image/jpeg)", mimeType)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}

		resolved = append(resolved, resolvedPPTXImage{
			bytes:    payload,
			mimeType: mimeType,
			ext:      ext,
		})
	}

	pptxBytes, err := buildPPTXDeckFromImages(pptxBlankTemplate, resolved)
	if err != nil {
		wrapped := fmt.Errorf("build pptx: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	pdfBytes, err := buildPDFDeckFromImages(resolved)
	if err != nil {
		wrapped := fmt.Errorf("build pdf: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	baseName := strings.TrimSuffix(outputName, filepath.Ext(outputName))
	pdfName := baseName + ".pdf"

	encoded := base64.StdEncoding.EncodeToString(pptxBytes)
	pdfEncoded := base64.StdEncoding.EncodeToString(pdfBytes)
	pdfDataURI := fmt.Sprintf("data:%s;base64,%s", pdfMediaType, pdfEncoded)

	attachment := ports.Attachment{
		Name:           outputName,
		MediaType:      pptxFromImagesMediaType,
		Data:           encoded,
		URI:            fmt.Sprintf("data:%s;base64,%s", pptxFromImagesMediaType, encoded),
		Source:         call.Name,
		Description:    description,
		Kind:           "artifact",
		Format:         "pptx",
		PreviewProfile: "document.ppt",
		PreviewAssets: []ports.AttachmentPreviewAsset{
			{
				AssetID:     "pdf_download",
				Label:       "PDF export",
				MimeType:    pdfMediaType,
				CDNURL:      pdfDataURI,
				PreviewType: "document.pdf",
			},
		},
	}

	pdfAttachment := ports.Attachment{
		Name:           pdfName,
		MediaType:      pdfMediaType,
		Data:           pdfEncoded,
		URI:            pdfDataURI,
		Source:         call.Name,
		Description:    description,
		Kind:           "artifact",
		Format:         "pdf",
		PreviewProfile: "document.pdf",
	}

	resultAttachments := map[string]ports.Attachment{
		outputName: attachment,
		pdfName:    pdfAttachment,
	}
	mutations := map[string]any{
		"attachment_mutations": map[string]any{
			"add": resultAttachments,
		},
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("Created %s and %s with %d slide(s).", outputName, pdfName, len(images)),
		Metadata:    mutations,
		Attachments: resultAttachments,
	}, nil
}

type resolvedPPTXImage struct {
	bytes    []byte
	mimeType string
	ext      string
}

func extensionForImageMIME(mimeType string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/png":
		return "png", true
	case "image/jpeg":
		return "jpeg", true
	default:
		return "", false
	}
}

func (t *pptxFromImages) resolveImageBytes(ctx context.Context, ref string) ([]byte, string, error) {
	bytes, mimeType, err := ResolveAttachmentBytes(ctx, ref, t.httpClient)
	if err != nil {
		return nil, "", err
	}
	return bytes, mimeType, nil
}

func buildPDFDeckFromImages(images []resolvedPPTXImage) ([]byte, error) {
	if len(images) == 0 {
		return nil, errors.New("no images provided")
	}

	pdf := gofpdf.NewCustom(&gofpdf.InitType{
		UnitStr: "pt",
		Size: gofpdf.SizeType{
			Wd: float64(pptxDefaultSlideCX) / 12700.0, // convert from EMU (914400 per inch) to points (72 per inch)
			Ht: float64(pptxDefaultSlideCY) / 12700.0,
		},
	})

	for idx, img := range images {
		cfg, _, err := image.DecodeConfig(bytes.NewReader(img.bytes))
		if err != nil {
			return nil, fmt.Errorf("decode image %d: %w", idx, err)
		}

		pageSize := gofpdf.SizeType{
			Wd: float64(cfg.Width),
			Ht: float64(cfg.Height),
		}
		orientation := "P"
		if pageSize.Wd >= pageSize.Ht {
			orientation = "L"
		}

		pdf.AddPageFormat(orientation, pageSize)

		imageType := strings.ToUpper(img.ext)
		if imageType == "JPEG" {
			imageType = "JPG"
		}

		if info := pdf.RegisterImageOptionsReader(
			fmt.Sprintf("slide-%d", idx+1),
			gofpdf.ImageOptions{ImageType: imageType},
			bytes.NewReader(img.bytes),
		); info == nil {
			return nil, fmt.Errorf("register image %d: %v", idx, pdf.Error())
		}

		pdf.ImageOptions(
			fmt.Sprintf("slide-%d", idx+1),
			0,
			0,
			pageSize.Wd,
			pageSize.Ht,
			false,
			gofpdf.ImageOptions{ImageType: imageType},
			0,
			"",
		)
	}

	if err := pdf.Error(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func buildPPTXDeckFromImages(template []byte, images []resolvedPPTXImage) ([]byte, error) {
	if len(images) == 0 {
		return nil, errors.New("no images provided")
	}
	reader, err := zip.NewReader(bytes.NewReader(template), int64(len(template)))
	if err != nil {
		return nil, fmt.Errorf("open template: %w", err)
	}

	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)

	for _, file := range reader.File {
		name := file.Name
		if shouldSkipTemplateEntry(name) {
			continue
		}

		rc, err := file.Open()
		if err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("read template entry %s: %w", name, err)
		}

		w, err := writer.Create(name)
		if err != nil {
			_ = rc.Close()
			_ = writer.Close()
			return nil, fmt.Errorf("write template entry %s: %w", name, err)
		}
		if _, err := io.Copy(w, rc); err != nil {
			_ = rc.Close()
			_ = writer.Close()
			return nil, fmt.Errorf("copy template entry %s: %w", name, err)
		}
		if err := rc.Close(); err != nil {
			_ = writer.Close()
			return nil, fmt.Errorf("close template entry %s: %w", name, err)
		}
	}

	if err := writeZipTextFile(writer, "[Content_Types].xml", contentTypesXML(len(images))); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeZipTextFile(writer, "_rels/.rels", rootRelsXML()); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeZipTextFile(writer, "docProps/core.xml", corePropsXML()); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeZipTextFile(writer, "docProps/app.xml", appPropsXML(len(images))); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeZipTextFile(writer, "ppt/presentation.xml", presentationXML(len(images))); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writeZipTextFile(writer, "ppt/_rels/presentation.xml.rels", presentationRelsXML(len(images))); err != nil {
		_ = writer.Close()
		return nil, err
	}

	for idx, img := range images {
		slideNumber := idx + 1
		mediaName := fmt.Sprintf("ppt/media/image%d.%s", slideNumber, img.ext)
		if err := writeZipBytes(writer, mediaName, img.bytes); err != nil {
			_ = writer.Close()
			return nil, err
		}

		slideName := fmt.Sprintf("ppt/slides/slide%d.xml", slideNumber)
		slideRelsName := fmt.Sprintf("ppt/slides/_rels/slide%d.xml.rels", slideNumber)

		if err := writeZipTextFile(writer, slideName, slideXML(slideNumber)); err != nil {
			_ = writer.Close()
			return nil, err
		}

		if err := writeZipTextFile(writer, slideRelsName, slideRelsXML(mediaName)); err != nil {
			_ = writer.Close()
			return nil, err
		}
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func shouldSkipTemplateEntry(name string) bool {
	switch name {
	case "[Content_Types].xml",
		"_rels/.rels",
		"docProps/core.xml",
		"docProps/app.xml",
		"docProps/thumbnail.jpeg",
		"ppt/presentation.xml",
		"ppt/_rels/presentation.xml.rels":
		return true
	}

	if strings.HasPrefix(name, "ppt/slides/") {
		return true
	}
	if strings.HasPrefix(name, "ppt/media/") {
		return true
	}

	return false
}

func writeZipTextFile(writer *zip.Writer, name string, content string) error {
	w, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", name, err)
	}
	if _, err := io.Copy(w, strings.NewReader(content)); err != nil {
		return fmt.Errorf("write zip entry %s: %w", name, err)
	}
	return nil
}

func writeZipBytes(writer *zip.Writer, name string, payload []byte) error {
	w, err := writer.Create(name)
	if err != nil {
		return fmt.Errorf("create zip entry %s: %w", name, err)
	}
	if _, err := w.Write(payload); err != nil {
		return fmt.Errorf("write zip entry %s: %w", name, err)
	}
	return nil
}

func contentTypesXML(slideCount int) string {
	if slideCount < 1 {
		slideCount = 1
	}

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">`)
	builder.WriteString(`<Default Extension="bin" ContentType="application/vnd.openxmlformats-officedocument.presentationml.printerSettings"/>`)
	builder.WriteString(`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>`)
	builder.WriteString(`<Default Extension="xml" ContentType="application/xml"/>`)
	builder.WriteString(`<Default Extension="png" ContentType="image/png"/>`)
	builder.WriteString(`<Default Extension="jpeg" ContentType="image/jpeg"/>`)

	builder.WriteString(`<Override PartName="/docProps/app.xml" ContentType="application/vnd.openxmlformats-officedocument.extended-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/presProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presProps+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>`)

	for _, layout := range []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11} {
		fmt.Fprintf(&builder, `<Override PartName="/ppt/slideLayouts/slideLayout%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideLayout+xml"/>`, layout)
	}
	builder.WriteString(`<Override PartName="/ppt/slideMasters/slideMaster1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slideMaster+xml"/>`)

	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&builder, `<Override PartName="/ppt/slides/slide%d.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>`, i)
	}

	builder.WriteString(`<Override PartName="/ppt/tableStyles.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.tableStyles+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>`)
	builder.WriteString(`<Override PartName="/ppt/viewProps.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.viewProps+xml"/>`)
	builder.WriteString(`</Types>`)
	return builder.String()
}

func rootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
		`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/extended-properties" Target="docProps/app.xml"/>` +
		`</Relationships>`
}

func corePropsXML() string {
	now := time.Now().UTC().Format(time.RFC3339)
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:dcmitype="http://purl.org/dc/dcmitype/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
		`<dc:title/>` +
		`<dc:creator>alex</dc:creator>` +
		`<cp:lastModifiedBy>alex</cp:lastModifiedBy>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">` + now + `</dcterms:created>` +
		`<dcterms:modified xsi:type="dcterms:W3CDTF">` + now + `</dcterms:modified>` +
		`</cp:coreProperties>`
}

func appPropsXML(slideCount int) string {
	if slideCount < 0 {
		slideCount = 0
	}
	titlesSize := slideCount
	if titlesSize == 0 {
		titlesSize = 1
	}

	var titlesBuilder strings.Builder
	fmt.Fprintf(&titlesBuilder, `<vt:vector size="%d" baseType="lpstr">`, titlesSize)
	for i := 1; i <= slideCount; i++ {
		fmt.Fprintf(&titlesBuilder, `<vt:lpstr>Slide %d</vt:lpstr>`, i)
	}
	if slideCount == 0 {
		titlesBuilder.WriteString(`<vt:lpstr>Slides</vt:lpstr>`)
	}
	titlesBuilder.WriteString(`</vt:vector>`)

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`)
	builder.WriteString(`<Application>alex</Application>`)
	builder.WriteString(`<PresentationFormat>On-screen Show (16:9)</PresentationFormat>`)
	fmt.Fprintf(&builder, `<Slides>%d</Slides>`, slideCount)
	builder.WriteString(`<Notes>0</Notes><HiddenSlides>0</HiddenSlides><MMClips>0</MMClips><ScaleCrop>false</ScaleCrop>`)
	builder.WriteString(`<HeadingPairs><vt:vector size="2" baseType="variant"><vt:variant><vt:lpstr>Slides</vt:lpstr></vt:variant><vt:variant><vt:i4>`)
	fmt.Fprintf(&builder, `%d`, slideCount)
	builder.WriteString(`</vt:i4></vt:variant></vt:vector></HeadingPairs>`)
	builder.WriteString(`<TitlesOfParts>`)
	builder.WriteString(titlesBuilder.String())
	builder.WriteString(`</TitlesOfParts>`)
	builder.WriteString(`<AppVersion>16.0000</AppVersion>`)
	builder.WriteString(`</Properties>`)
	return builder.String()
}

func presentationXML(slideCount int) string {
	if slideCount < 1 {
		slideCount = 1
	}

	var slidesBuilder strings.Builder
	slidesBuilder.WriteString(`<p:sldIdLst>`)
	for i := 0; i < slideCount; i++ {
		id := 256 + i
		rid := 7 + i
		fmt.Fprintf(&slidesBuilder, `<p:sldId id="%d" r:id="rId%d"/>`, id, rid)
	}
	slidesBuilder.WriteString(`</p:sldIdLst>`)

	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:presentation xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" saveSubsetFonts="1" autoCompressPictures="0">` +
		`<p:sldMasterIdLst><p:sldMasterId id="2147483648" r:id="rId1"/></p:sldMasterIdLst>` +
		slidesBuilder.String() +
		fmt.Sprintf(`<p:sldSz cx="%d" cy="%d" type="screen16x9"/>`, pptxDefaultSlideCX, pptxDefaultSlideCY) +
		`<p:notesSz cx="6858000" cy="9144000"/>` +
		`<p:defaultTextStyle/>` +
		`</p:presentation>`
}

func presentationRelsXML(slideCount int) string {
	if slideCount < 1 {
		slideCount = 1
	}

	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	builder.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	builder.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster" Target="slideMasters/slideMaster1.xml"/>`)
	builder.WriteString(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/printerSettings" Target="printerSettings/printerSettings1.bin"/>`)
	builder.WriteString(`<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/presProps" Target="presProps.xml"/>`)
	builder.WriteString(`<Relationship Id="rId4" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/viewProps" Target="viewProps.xml"/>`)
	builder.WriteString(`<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>`)
	builder.WriteString(`<Relationship Id="rId6" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/tableStyles" Target="tableStyles.xml"/>`)

	for i := 0; i < slideCount; i++ {
		rid := 7 + i
		slideName := fmt.Sprintf("slides/slide%d.xml", i+1)
		fmt.Fprintf(&builder, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide" Target="%s"/>`, rid, slideName)
	}

	builder.WriteString(`</Relationships>`)
	return builder.String()
}

func slideXML(slideNumber int) string {
	if slideNumber < 1 {
		slideNumber = 1
	}

	picID := 2
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<p:cSld><p:spTree>` +
		`<p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>` +
		`<p:grpSpPr/>` +
		`<p:pic>` +
		`<p:nvPicPr>` +
		fmt.Sprintf(`<p:cNvPr id="%d" name="Slide %d"/>`, picID, slideNumber) +
		`<p:cNvPicPr/><p:nvPr/>` +
		`</p:nvPicPr>` +
		`<p:blipFill><a:blip r:embed="rId2"/><a:stretch><a:fillRect/></a:stretch></p:blipFill>` +
		fmt.Sprintf(`<p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr>`, pptxDefaultSlideCX, pptxDefaultSlideCY) +
		`</p:pic>` +
		`</p:spTree></p:cSld>` +
		`<p:clrMapOvr><a:masterClrMapping/></p:clrMapOvr>` +
		`</p:sld>`
}

func slideRelsXML(mediaName string) string {
	target := strings.TrimPrefix(mediaName, "ppt/")
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout" Target="` + pptxSlideLayoutTarget + `"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../` + target + `"/>` +
		`</Relationships>`
}
