package domain

import (
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestCollectGeneratedAttachmentsIncludesAllGeneratedUpToIteration(t *testing.T) {
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"v1.png": {
				Name:      "v1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"v2.png": {
				Name:      "v2.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"user.png": {
				Name:      "user.png",
				MediaType: "image/png",
				Source:    "user_upload",
			},
		},
		AttachmentIterations: map[string]int{
			"v1.png": 1,
			"v2.png": 2,
		},
	}

	got := collectGeneratedAttachments(state, 2)
	if len(got) != 2 {
		t.Fatalf("expected two generated attachments, got %d", len(got))
	}
	if _, ok := got["v1.png"]; !ok {
		t.Fatalf("expected earlier iteration attachment to be present: %+v", got)
	}
	if _, ok := got["v2.png"]; !ok {
		t.Fatalf("expected latest iteration attachment to be present: %+v", got)
	}
	if _, ok := got["user.png"]; ok {
		t.Fatalf("did not expect user-uploaded attachment in result: %+v", got)
	}
}

func TestExpandPlaceholdersPrefersAttachmentData(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
			URI:       "https://example.com/seed.png",
		},
	}
	args := map[string]any{"init_image": "[seed.png]"}
	state := &TaskState{Attachments: attachments}
	expanded := engine.expandPlaceholders(args, state)

	value, ok := expanded["init_image"].(string)
	if !ok {
		t.Fatalf("expected expanded init_image to be a string, got %T", expanded["init_image"])
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI, got %q", value)
	}
	if strings.Contains(value, "example.com") {
		t.Fatalf("expected base64 payload, got remote URL %q", value)
	}
	if !strings.HasSuffix(value, attachments["seed.png"].Data) {
		t.Fatalf("expected payload to include base64 data, got %q", value)
	}
}

func TestExpandPlaceholdersResolvesGenericAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"seedream_0.png": {
				Name:      "seedream_0.png",
				MediaType: "image/png",
				Data:      "YmFzZTY0",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"seedream_0.png": 3,
		},
	}
	args := map[string]any{"init_image": "[image.png]"}
	expanded := engine.expandPlaceholders(args, state)

	value, ok := expanded["init_image"].(string)
	if !ok {
		t.Fatalf("expected expanded init_image to be a string, got %T", expanded["init_image"])
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI fallback, got %q", value)
	}
}

func TestEnsureAttachmentPlaceholdersUsesURIWhenAvailable(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			Data:      "YmFzZTY0",
			URI:       "https://example.com/seed.png",
		},
	}

	result := ensureAttachmentPlaceholders("", attachments)
	if !strings.Contains(result, "https://example.com/seed.png") {
		t.Fatalf("expected markdown to reference attachment URI, got %q", result)
	}
	if strings.Contains(result, "data:image/png") {
		t.Fatalf("expected markdown to avoid embedding base64 data, got %q", result)
	}
}

func TestEnsureAttachmentPlaceholdersReplacesInlinePlaceholders(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"seed.png": {
			Name:      "seed.png",
			MediaType: "image/png",
			URI:       "https://example.com/seed.png",
		},
	}

	answer := "Latest render: [seed.png]"
	result := ensureAttachmentPlaceholders(answer, attachments)

	if strings.Contains(result, " [seed.png]") {
		t.Fatalf("expected placeholder brackets to be removed from plain text, got %q", result)
	}
	if !strings.Contains(result, "![seed.png](https://example.com/seed.png)") {
		t.Fatalf("expected inline image markdown, got %q", result)
	}
}

func TestEnsureAttachmentPlaceholdersRemovesUnknownPlaceholders(t *testing.T) {
	answer := "Attachments: [missing.png]"
	result := ensureAttachmentPlaceholders(answer, nil)
	if strings.Contains(result, "[missing.png]") {
		t.Fatalf("expected missing placeholder to be removed, got %q", result)
	}
	if strings.TrimSpace(result) != "Attachments:" {
		t.Fatalf("expected surrounding text to remain, got %q", result)
	}
}

func TestLookupAttachmentByNameResolvesSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_0.png": {
				Name:      "doubao-seedream-3-0_nonce_0.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_nonce_0.png": 2,
		},
	}

	alias := "doubao-seedream-3-0_0.png"
	att, canonical, ok := engine.lookupAttachmentByName(alias, state)
	if !ok {
		t.Fatalf("expected alias %q to resolve", alias)
	}
	if canonical != "doubao-seedream-3-0_nonce_0.png" {
		t.Fatalf("expected canonical placeholder to include nonce, got %q", canonical)
	}
	if att.MediaType != "image/png" {
		t.Fatalf("expected attachment metadata to be preserved")
	}
}

func TestLookupAttachmentByNamePrefersLatestSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_old_1.png": {
				Name:      "doubao-seedream-3-0_old_1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"doubao-seedream-3-0_new_1.png": {
				Name:      "doubao-seedream-3-0_new_1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_old_1.png": 1,
			"doubao-seedream-3-0_new_1.png": 5,
		},
	}

	alias := "doubao-seedream-3-0_1.png"
	_, canonical, ok := engine.lookupAttachmentByName(alias, state)
	if !ok {
		t.Fatalf("expected alias %q to resolve", alias)
	}
	if canonical != "doubao-seedream-3-0_new_1.png" {
		t.Fatalf("expected latest iteration to win, got %q", canonical)
	}
}

func TestResolveContentAttachmentsSupportsSeedreamAlias(t *testing.T) {
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_temp_2.png": {
				Name:      "doubao-seedream-3-0_temp_2.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
		AttachmentIterations: map[string]int{
			"doubao-seedream-3-0_temp_2.png": 3,
		},
	}

	content := "Analyze [doubao-seedream-3-0_2.png] for details."
	resolved := resolveContentAttachments(content, state)
	if len(resolved) != 1 {
		t.Fatalf("expected one attachment, got %d", len(resolved))
	}
	if _, ok := resolved["doubao-seedream-3-0_2.png"]; !ok {
		t.Fatalf("expected resolved map to use alias key, got %+v", resolved)
	}
}

func TestExpandPlaceholdersResolvesPlainSeedreamAlias(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_0.png": {
				Name:      "doubao-seedream-3-0_nonce_0.png",
				MediaType: "image/png",
				Data:      "YmFzZTY0",
				Source:    "seedream",
			},
		},
	}
	args := map[string]any{"images": []any{"doubao-seedream-3-0_0.png"}}
	expanded := engine.expandPlaceholders(args, state)

	images, ok := expanded["images"].([]any)
	if !ok || len(images) != 1 {
		t.Fatalf("expected expanded images slice, got %#v", expanded["images"])
	}
	value, ok := images[0].(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", images[0])
	}
	if !strings.HasPrefix(value, "data:image/png;base64,") {
		t.Fatalf("expected data URI payload, got %q", value)
	}
	if !strings.HasSuffix(value, "YmFzZTY0") {
		t.Fatalf("expected inline base64 data, got %q", value)
	}
}

func TestExpandPlaceholdersResolvesCanonicalSeedreamNameWithoutBrackets(t *testing.T) {
	engine := NewReactEngine(ReactEngineConfig{})
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"doubao-seedream-3-0_nonce_1.png": {
				Name:      "doubao-seedream-3-0_nonce_1.png",
				MediaType: "image/png",
				Data:      "ZGF0YTEyMw==",
				Source:    "seedream",
			},
		},
	}
	args := map[string]any{"images": []string{"doubao-seedream-3-0_nonce_1.png"}}
	expanded := engine.expandPlaceholders(args, state)

	images, ok := expanded["images"].([]string)
	if !ok || len(images) != 1 {
		t.Fatalf("expected expanded images slice, got %#v", expanded["images"])
	}
	if !strings.HasPrefix(images[0], "data:image/png;base64,") {
		t.Fatalf("expected canonical placeholder to convert to data URI, got %q", images[0])
	}
	if !strings.HasSuffix(images[0], "ZGF0YTEyMw==") {
		t.Fatalf("expected appended base64 payload, got %q", images[0])
	}
}
