package main

import (
	"encoding/base64"
	"fmt"
	"mime"
	"path"
	"strings"

	"alex/internal/agent/ports"
)

type acpPromptInput struct {
	Text        string
	Attachments []ports.Attachment
}

func parseACPPrompt(prompt any) (acpPromptInput, error) {
	blocks, ok := prompt.([]any)
	if !ok {
		return acpPromptInput{}, fmt.Errorf("prompt must be an array")
	}

	parts := make([]string, 0, len(blocks))
	attachments := make([]ports.Attachment, 0)
	usedNames := make(map[string]bool)

	appendAttachment := func(att ports.Attachment) string {
		name := strings.TrimSpace(att.Name)
		if name == "" {
			name = fmt.Sprintf("attachment-%d", len(attachments)+1)
		}
		name = ensureUniqueName(name, usedNames)
		att.Name = name
		attachments = append(attachments, att)
		usedNames[name] = true
		return name
	}

	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		blockType := strings.ToLower(stringParam(blockMap, "type"))
		switch blockType {
		case "text":
			text := strings.TrimSpace(stringParam(blockMap, "text"))
			if text != "" {
				parts = append(parts, text)
			}
		case "resource_link":
			name := strings.TrimSpace(stringParam(blockMap, "name"))
			uri := strings.TrimSpace(stringParam(blockMap, "uri"))
			if name == "" && uri != "" {
				name = path.Base(uri)
			}
			if name == "" {
				name = fmt.Sprintf("resource-%d", len(attachments)+1)
			}
			att := ports.Attachment{
				Name:        name,
				URI:         uri,
				MediaType:   strings.TrimSpace(stringParam(blockMap, "mimeType")),
				Description: strings.TrimSpace(stringParam(blockMap, "description")),
			}
			finalName := appendAttachment(att)
			parts = append(parts, fmt.Sprintf("[%s]", finalName))
		case "resource":
			if att, ok := attachmentFromEmbeddedResource(blockMap); ok {
				finalName := appendAttachment(att)
				parts = append(parts, fmt.Sprintf("[%s]", finalName))
			}
		case "image":
			data := strings.TrimSpace(stringParam(blockMap, "data"))
			mimeType := strings.TrimSpace(stringParam(blockMap, "mimeType"))
			name := attachmentNameForMedia("image", mimeType, len(attachments)+1)
			att := ports.Attachment{
				Name:      name,
				MediaType: mimeType,
				Data:      data,
			}
			finalName := appendAttachment(att)
			parts = append(parts, fmt.Sprintf("[%s]", finalName))
		case "audio":
			data := strings.TrimSpace(stringParam(blockMap, "data"))
			mimeType := strings.TrimSpace(stringParam(blockMap, "mimeType"))
			name := attachmentNameForMedia("audio", mimeType, len(attachments)+1)
			att := ports.Attachment{
				Name:      name,
				MediaType: mimeType,
				Data:      data,
			}
			finalName := appendAttachment(att)
			parts = append(parts, fmt.Sprintf("[%s]", finalName))
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	if len(attachments) > 0 && text != "" {
		missing := missingAttachmentRefs(text, attachments)
		if len(missing) > 0 {
			text = text + "\n\nAttachments: " + strings.Join(missing, " ")
		}
	} else if text == "" && len(attachments) > 0 {
		placeholders := missingAttachmentRefs("", attachments)
		text = strings.Join(placeholders, " ")
	}

	return acpPromptInput{Text: text, Attachments: attachments}, nil
}

func attachmentFromEmbeddedResource(block map[string]any) (ports.Attachment, bool) {
	resource, ok := block["resource"].(map[string]any)
	if !ok {
		return ports.Attachment{}, false
	}

	uri := strings.TrimSpace(stringParam(resource, "uri"))
	name := ""
	if uri != "" {
		name = path.Base(uri)
	}

	if text := stringParam(resource, "text"); text != "" {
		if name == "" {
			name = fmt.Sprintf("resource-%s", randSuffix(6))
		}
		return ports.Attachment{
			Name:      name,
			URI:       uri,
			MediaType: strings.TrimSpace(stringParam(resource, "mimeType")),
			Data:      base64.StdEncoding.EncodeToString([]byte(text)),
		}, true
	}

	if blob := stringParam(resource, "blob"); blob != "" {
		if name == "" {
			name = fmt.Sprintf("resource-%s", randSuffix(6))
		}
		return ports.Attachment{
			Name:      name,
			URI:       uri,
			MediaType: strings.TrimSpace(stringParam(resource, "mimeType")),
			Data:      blob,
		}, true
	}

	return ports.Attachment{}, false
}

func attachmentNameForMedia(prefix, mimeType string, index int) string {
	ext := ""
	if mimeType != "" {
		if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		if prefix == "image" {
			ext = ".png"
		} else if prefix == "audio" {
			ext = ".wav"
		}
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("%s-%d%s", prefix, index, ext)
}

func ensureUniqueName(name string, used map[string]bool) string {
	if !used[name] {
		return name
	}
	base := strings.TrimSuffix(name, path.Ext(name))
	ext := path.Ext(name)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !used[candidate] {
			return candidate
		}
	}
}

func missingAttachmentRefs(text string, attachments []ports.Attachment) []string {
	missing := make([]string, 0, len(attachments))
	for _, att := range attachments {
		if att.Name == "" {
			continue
		}
		placeholder := fmt.Sprintf("[%s]", att.Name)
		if !strings.Contains(text, placeholder) {
			missing = append(missing, placeholder)
		}
	}
	return missing
}

