package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

// artifactManifestTool emits a structured artifact manifest for executor runs.
type artifactManifestTool struct{}

// NewArtifactManifest constructs the artifact_manifest tool executor.
func NewArtifactManifest() ports.ToolExecutor {
	return &artifactManifestTool{}
}

func (t *artifactManifestTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "artifact_manifest",
		Version:  "1.0.0",
		Category: "attachments",
		Tags:     []string{"artifact", "manifest"},
	}
}

func (t *artifactManifestTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "artifact_manifest",
		Description: "Emit a structured manifest of executor artifacts (diffs, reports, binaries, logs).",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"items": {
					Type:        "array",
					Description: "Artifact entries describing outputs (kind/path/command/checksum/etc).",
					Items: &ports.Property{
						Type: "object",
					},
				},
				"summary": {
					Type:        "string",
					Description: "Optional manifest summary.",
				},
				"environment_fingerprint": {
					Type:        "string",
					Description: "Executor environment fingerprint (image/runtime identifiers).",
				},
				"attachment_name": {
					Type:        "string",
					Description: "Optional attachment name for the manifest payload.",
				},
			},
			Required: []string{"items"},
		},
	}
}

func (t *artifactManifestTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	items, ok := call.Arguments["items"].([]any)
	if !ok || len(items) == 0 {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("items is required")}, nil
	}

	summary := strings.TrimSpace(stringArg(call.Arguments, "summary"))
	fingerprint := strings.TrimSpace(stringArg(call.Arguments, "environment_fingerprint"))
	name := strings.TrimSpace(stringArg(call.Arguments, "attachment_name"))
	if name == "" {
		name = fmt.Sprintf("artifact-manifest-%d.json", time.Now().Unix())
	}

	payload := map[string]any{
		"items":      items,
		"generated":  time.Now().UTC().Format(time.RFC3339Nano),
	}
	if summary != "" {
		payload["summary"] = summary
	}
	if fingerprint != "" {
		payload["environment_fingerprint"] = fingerprint
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("encode manifest: %w", err)}, nil
	}

	attachment := ports.Attachment{
		Name:        name,
		MediaType:   "application/json",
		Data:        base64.StdEncoding.EncodeToString(encoded),
		URI:         fmt.Sprintf("data:application/json;base64,%s", base64.StdEncoding.EncodeToString(encoded)),
		Kind:        "artifact",
		Format:      "manifest",
		Source:      call.Name,
		Description: "Executor artifact manifest",
	}

	result := &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("Recorded %d artifact(s).", len(items)),
		Metadata:    map[string]any{"artifact_manifest": payload},
		Attachments: map[string]ports.Attachment{name: attachment},
	}
	return result, nil
}
