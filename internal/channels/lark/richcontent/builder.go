// Package richcontent provides fluent builders for Lark rich text (post)
// messages. The post message type supports structured elements such as plain
// text, bold/italic text, hyperlinks, mentions, and code blocks arranged
// into paragraphs (lines).
//
// Usage:
//
//	content := richcontent.NewPostBuilder("Report").
//		AddBold("Status: ").AddText("OK").NewLine().
//		AddLink("Details", "https://example.com").
//		Build()
//
// The resulting JSON string can be sent via the Lark messenger with
// msgType "post".
package richcontent

import "encoding/json"

// element is a single inline element within a post paragraph.
type element = map[string]any

// PostBuilder constructs a Lark rich text (post) message using the fluent
// builder pattern. Elements are grouped into paragraphs (lines); calling
// NewLine starts a new paragraph.
type PostBuilder struct {
	title      string
	locale     string
	paragraphs [][]element
}

// NewPostBuilder creates a PostBuilder with the given title.
// The default locale is "zh_cn".
func NewPostBuilder(title string) *PostBuilder {
	return &PostBuilder{
		title:      title,
		locale:     "zh_cn",
		paragraphs: [][]element{{}},
	}
}

// SetLocale overrides the default locale key used in the post envelope.
// Common values: "zh_cn", "en_us", "ja_jp".
func (b *PostBuilder) SetLocale(locale string) *PostBuilder {
	if locale != "" {
		b.locale = locale
	}
	return b
}

// AddText appends a plain text element to the current paragraph.
func (b *PostBuilder) AddText(text string) *PostBuilder {
	b.appendElement(element{
		"tag":  "text",
		"text": text,
	})
	return b
}

// AddBold appends a bold text element to the current paragraph.
// Lark post represents bold via the "text" tag with a style object.
func (b *PostBuilder) AddBold(text string) *PostBuilder {
	b.appendElement(element{
		"tag":   "text",
		"text":  text,
		"style": []string{"bold"},
	})
	return b
}

// AddItalic appends an italic text element to the current paragraph.
func (b *PostBuilder) AddItalic(text string) *PostBuilder {
	b.appendElement(element{
		"tag":   "text",
		"text":  text,
		"style": []string{"italic"},
	})
	return b
}

// AddBoldItalic appends a bold-italic text element to the current paragraph.
func (b *PostBuilder) AddBoldItalic(text string) *PostBuilder {
	b.appendElement(element{
		"tag":   "text",
		"text":  text,
		"style": []string{"bold", "italic"},
	})
	return b
}

// AddLink appends a hyperlink element to the current paragraph.
func (b *PostBuilder) AddLink(text, href string) *PostBuilder {
	b.appendElement(element{
		"tag":  "a",
		"text": text,
		"href": href,
	})
	return b
}

// AddCodeBlock appends a code block rendered as a monospace text element.
// Since Lark post does not have a native code block tag, we render the code
// as a "code_block" tag element (supported in newer Lark post versions) with
// language metadata. For older API versions, the content will degrade to a
// plain text code representation.
func (b *PostBuilder) AddCodeBlock(code, language string) *PostBuilder {
	elem := element{
		"tag":      "code_block",
		"language": language,
		"text":     code,
	}
	b.appendElement(elem)
	return b
}

// AddMention appends an @mention element to the current paragraph.
// The userID should be the Lark open_id or user_id of the target user.
func (b *PostBuilder) AddMention(userID, name string) *PostBuilder {
	b.appendElement(element{
		"tag":       "at",
		"user_id":   userID,
		"user_name": name,
	})
	return b
}

// AddImage appends an inline image element to the current paragraph.
func (b *PostBuilder) AddImage(imageKey string, width, height int) *PostBuilder {
	elem := element{
		"tag":       "img",
		"image_key": imageKey,
	}
	if width > 0 {
		elem["width"] = width
	}
	if height > 0 {
		elem["height"] = height
	}
	b.appendElement(elem)
	return b
}

// NewLine starts a new paragraph/line in the post content.
func (b *PostBuilder) NewLine() *PostBuilder {
	b.paragraphs = append(b.paragraphs, []element{})
	return b
}

// Build serializes the post content to Lark post message JSON.
func (b *PostBuilder) Build() string {
	post := map[string]any{
		b.locale: map[string]any{
			"title":   b.title,
			"content": b.paragraphs,
		},
	}
	data, err := json.Marshal(post)
	if err != nil {
		// JSON marshal of basic map types should never fail; return empty
		// object as a safe fallback.
		return "{}"
	}
	return string(data)
}

// appendElement adds an element to the last (current) paragraph.
func (b *PostBuilder) appendElement(elem element) {
	idx := len(b.paragraphs) - 1
	b.paragraphs[idx] = append(b.paragraphs[idx], elem)
}
