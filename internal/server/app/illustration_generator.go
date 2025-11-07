package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"

	agentports "alex/internal/agent/ports"
	builtin "alex/internal/tools/builtin"
	id "alex/internal/utils/id"
)

// IllustrationGenerator produces rendered visuals for article illustrations.
type IllustrationGenerator interface {
	GenerateIllustration(ctx context.Context, prompt string) (*GeneratedIllustration, error)
}

// GeneratedIllustration captures the binary payload and metadata returned by the generator.
type GeneratedIllustration struct {
	Name        string
	MediaType   string
	Data        []byte
	Description string
	Source      string
	RemoteURL   string
}

type seedreamIllustrationGenerator struct {
	tool agentports.ToolExecutor
}

// NewSeedreamIllustrationGenerator constructs an IllustrationGenerator backed by the
// Seedream text-to-image tool. Returns nil when the configuration is incomplete.
func NewSeedreamIllustrationGenerator(config builtin.SeedreamConfig) IllustrationGenerator {
	if strings.TrimSpace(config.AccessKey) == "" || strings.TrimSpace(config.SecretKey) == "" || strings.TrimSpace(config.EndpointID) == "" {
		return nil
	}
	tool := builtin.NewSeedreamTextToImage(config)
	if tool == nil {
		return nil
	}
	return &seedreamIllustrationGenerator{tool: tool}
}

// GenerateIllustration calls the underlying Seedream tool with the provided prompt.
func (g *seedreamIllustrationGenerator) GenerateIllustration(ctx context.Context, prompt string) (*GeneratedIllustration, error) {
	if g == nil || g.tool == nil {
		return nil, errors.New("illustration generator not configured")
	}
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return nil, errors.New("prompt is required")
	}

	call := agentports.ToolCall{
		ID:        id.NewKSUID(),
		Name:      "seedream_text_to_image",
		Arguments: map[string]any{"prompt": trimmed},
	}

	result, err := g.tool.Execute(ctx, call)
	if err != nil {
		return nil, fmt.Errorf("seedream execute: %w", err)
	}
	if result == nil {
		return nil, errors.New("seedream returned no result")
	}
	if len(result.Attachments) == 0 {
		return nil, errors.New("seedream returned no attachments")
	}

	names := make([]string, 0, len(result.Attachments))
	for name := range result.Attachments {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		attachment := result.Attachments[name]
		payload := strings.TrimSpace(attachment.Data)
		if payload == "" {
			// Some Seedream deployments only provide remote URLs. Surface the URL for callers
			// while still returning an error so the caller can decide how to handle it.
			if attachment.URI != "" {
				return &GeneratedIllustration{
					Name:        fallbackIllustrationName(attachment.Name),
					MediaType:   normalizeMediaType(attachment.MediaType),
					Data:        nil,
					Description: strings.TrimSpace(attachment.Description),
					Source:      defaultIllustrationSource(attachment.Source),
					RemoteURL:   attachment.URI,
				}, nil
			}
			continue
		}

		decoded, err := decodeSeedreamPayload(payload)
		if err != nil {
			return nil, fmt.Errorf("decode seedream image: %w", err)
		}

		return &GeneratedIllustration{
			Name:        fallbackIllustrationName(attachment.Name),
			MediaType:   normalizeMediaType(attachment.MediaType),
			Data:        decoded,
			Description: strings.TrimSpace(attachment.Description),
			Source:      defaultIllustrationSource(attachment.Source),
			RemoteURL:   attachment.URI,
		}, nil
	}

	return nil, errors.New("seedream did not provide usable attachments")
}

func decodeSeedreamPayload(payload string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err == nil {
		return decoded, nil
	}
	decoded, err = base64.URLEncoding.DecodeString(payload)
	if err == nil {
		return decoded, nil
	}
	return nil, err
}

func fallbackIllustrationName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("illustration-%s.png", id.NewKSUID())
}

func normalizeMediaType(mediaType string) string {
	trimmed := strings.TrimSpace(strings.ToLower(mediaType))
	if trimmed == "" {
		return "image/png"
	}
	return trimmed
}

func defaultIllustrationSource(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed != "" {
		return trimmed
	}
	return "seedream"
}
