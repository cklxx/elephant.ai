// Package cards provides a fluent builder for Lark interactive message cards.
//
// Cards are serialized to the Lark MessageCard JSON format and can be sent
// via the messenger layer using msgType "interactive". The builder is pure
// JSON — no Lark SDK import required.
package cards

import "encoding/json"

// --- Card config & header ---

// CardConfig holds top-level card configuration.
type CardConfig struct {
	Title         string // Header title text.
	TitleColor    string // Header template color: blue, green, red, orange, etc.
	EnableForward bool   // Whether the card can be forwarded.
}

// --- Button ---

// Button represents an action button inside an action element.
type Button struct {
	Text      string            // Display text.
	Type      string            // default, primary, or danger.
	Value     map[string]string // Arbitrary key-value payload returned on click.
	ActionTag string            // Developer-defined action identifier.
}

// NewButton creates a button with the default type.
func NewButton(text, actionTag string) Button {
	return Button{
		Text:      text,
		Type:      "default",
		ActionTag: actionTag,
	}
}

// NewPrimaryButton creates a button styled as the primary action.
func NewPrimaryButton(text, actionTag string) Button {
	return Button{
		Text:      text,
		Type:      "primary",
		ActionTag: actionTag,
	}
}

// NewDangerButton creates a button styled as a destructive action.
func NewDangerButton(text, actionTag string) Button {
	return Button{
		Text:      text,
		Type:      "danger",
		ActionTag: actionTag,
	}
}

// WithValue attaches a key-value pair to the button's callback payload.
func (b Button) WithValue(key, val string) Button {
	if b.Value == nil {
		b.Value = make(map[string]string)
	}
	b.Value[key] = val
	return b
}

// InputConfig configures a card input element.
type InputConfig struct {
	Name        string
	Label       string
	Placeholder string
	Value       string
	Required    bool
}

// --- Element types (internal JSON shapes) ---

type element = map[string]any

// --- Card builder ---

// Card is a fluent builder for Lark interactive message cards.
type Card struct {
	config   CardConfig
	elements []element
}

// NewCard creates a card builder with the given configuration.
func NewCard(config CardConfig) *Card {
	return &Card{config: config}
}

// AddMarkdownSection appends a markdown text section.
func (c *Card) AddMarkdownSection(content string) *Card {
	c.elements = append(c.elements, element{
		"tag": "div",
		"text": map[string]string{
			"tag":     "lark_md",
			"content": content,
		},
	})
	return c
}

// AddPlainTextSection appends a plain text section.
func (c *Card) AddPlainTextSection(content string) *Card {
	c.elements = append(c.elements, element{
		"tag": "div",
		"text": map[string]string{
			"tag":     "plain_text",
			"content": content,
		},
	})
	return c
}

// AddImage appends an image element using a Lark image key.
func (c *Card) AddImage(imgKey, alt string) *Card {
	if alt == "" {
		alt = "image"
	}
	c.elements = append(c.elements, element{
		"tag":     "img",
		"img_key": imgKey,
		"alt": map[string]string{
			"tag":     "plain_text",
			"content": alt,
		},
	})
	return c
}

// AddInput appends a form input element (returned in callback form_value).
func (c *Card) AddInput(cfg InputConfig) *Card {
	input := element{
		"tag":  "input",
		"name": cfg.Name,
	}
	if cfg.Label != "" {
		input["label"] = map[string]string{
			"tag":     "plain_text",
			"content": cfg.Label,
		}
	}
	if cfg.Placeholder != "" {
		input["placeholder"] = map[string]string{
			"tag":     "plain_text",
			"content": cfg.Placeholder,
		}
	}
	if cfg.Value != "" {
		input["value"] = cfg.Value
	}
	if cfg.Required {
		input["required"] = true
	}
	c.elements = append(c.elements, input)
	return c
}

// AddDivider appends a horizontal rule.
func (c *Card) AddDivider() *Card {
	c.elements = append(c.elements, element{"tag": "hr"})
	return c
}

// AddActionButtons appends a row of action buttons.
func (c *Card) AddActionButtons(buttons ...Button) *Card {
	actions := make([]map[string]any, 0, len(buttons))
	for _, b := range buttons {
		action := map[string]any{
			"tag": "button",
			"text": map[string]string{
				"tag":     "plain_text",
				"content": b.Text,
			},
			"type": b.Type,
		}
		if len(b.Value) > 0 {
			action["value"] = b.Value
		}
		if b.ActionTag != "" {
			action["action_tag"] = b.ActionTag
		}
		actions = append(actions, action)
	}
	c.elements = append(c.elements, element{
		"tag":     "action",
		"actions": actions,
	})
	return c
}

// AddNote appends a note/footer section.
func (c *Card) AddNote(content string) *Card {
	c.elements = append(c.elements, element{
		"tag": "note",
		"elements": []map[string]string{
			{
				"tag":     "plain_text",
				"content": content,
			},
		},
	})
	return c
}

// Build serializes the card to the Lark interactive card JSON string.
func (c *Card) Build() (string, error) {
	titleColor := c.config.TitleColor
	if titleColor == "" {
		titleColor = "blue"
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
			"enable_forward":   c.config.EnableForward,
		},
		"header": map[string]any{
			"title": map[string]string{
				"tag":     "plain_text",
				"content": c.config.Title,
			},
			"template": titleColor,
		},
		"elements": c.elements,
	}

	data, err := json.Marshal(card)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
