package lark

import (
	"fmt"
	"strings"

	jsonx "alex/internal/shared/json"
)

// buildAuthCard builds an interactive card JSON for in-message OAuth authorization.
// The card shows a "Go to authorize" button that opens the verification URL
// in Feishu's in-app webview for a seamless one-click auth experience.
func buildAuthCard(verificationURL string, userCode string, scopes []string, expiresIn int) string {
	scopeText := ""
	if len(scopes) > 0 {
		scopeText = fmt.Sprintf("\n\n**Requested permissions**: %s", strings.Join(scopes, ", "))
	}

	expiryText := ""
	if expiresIn > 0 {
		minutes := expiresIn / 60
		if minutes > 0 {
			expiryText = fmt.Sprintf("\n\n*Authorization link expires in %d minutes*", minutes)
		}
	}

	// Wrap the verification URL in a Feishu applink so it opens as an
	// in-app webview rather than launching an external browser.
	applinkURL := buildFeishuApplink(verificationURL)

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Feishu Authorization Required",
			},
			"template": "blue",
		},
		"elements": []any{
			map[string]any{
				"tag": "markdown",
				"content": fmt.Sprintf(
					"Please click the button below to authorize access to your Feishu account.%s%s",
					scopeText, expiryText,
				),
			},
			map[string]any{
				"tag": "action",
				"actions": []any{
					map[string]any{
						"tag":  "button",
						"text": map[string]any{"tag": "plain_text", "content": "Go to authorize"},
						"type": "primary",
						"url":  applinkURL,
					},
				},
			},
		},
	}

	data, _ := jsonx.Marshal(card)
	return string(data)
}

// buildAuthSuccessCard builds a card indicating authorization was successful.
func buildAuthSuccessCard() string {
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Authorization Successful",
			},
			"template": "green",
		},
		"elements": []any{
			map[string]any{
				"tag":     "markdown",
				"content": "Authorization completed successfully. Continuing with the previous operation...",
			},
		},
	}
	data, _ := jsonx.Marshal(card)
	return string(data)
}

// buildAuthFailedCard builds a card indicating authorization failed or expired.
func buildAuthFailedCard(reason string) string {
	if reason == "" {
		reason = "Authorization incomplete. Please try again."
	}
	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Authorization Incomplete",
			},
			"template": "yellow",
		},
		"elements": []any{
			map[string]any{
				"tag":     "markdown",
				"content": reason,
			},
		},
	}
	data, _ := jsonx.Marshal(card)
	return string(data)
}

// buildFeishuApplink wraps a URL in a Feishu applink so it opens in the
// in-app webview, providing a seamless authorization experience.
func buildFeishuApplink(rawURL string) string {
	return fmt.Sprintf(
		"https://applink.feishu.cn/client/web_url/open?url=%s&mode=sidebar-semi",
		urlEncode(rawURL),
	)
}

// urlEncode applies percent-encoding to a URL string.
func urlEncode(s string) string {
	// Use a simple replacer for the most common characters that need encoding.
	// For production, use net/url.QueryEscape but it encodes spaces as +.
	replacer := strings.NewReplacer(
		":", "%3A",
		"/", "%2F",
		"?", "%3F",
		"=", "%3D",
		"&", "%26",
		"#", "%23",
	)
	return replacer.Replace(s)
}
