package builtin

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
	materialports "alex/internal/materials/ports"
	"alex/internal/sandbox"
)

const sandboxAssetHTTPTimeout = 45 * time.Second

type sandboxAttachmentSpec struct {
	Path                string
	Name                string
	MediaType           string
	Description         string
	Kind                string
	Format              string
	PreviewProfile      string
	RetentionTTLSeconds uint64
}

var (
	sandboxUploaderOnce sync.Once
	sandboxUploader     materialports.Migrator
	sandboxUploaderErr  error
)

func sandboxAttachmentUploader() (materialports.Migrator, error) {
	sandboxUploaderOnce.Do(func() {
		sandboxUploader, sandboxUploaderErr = buildAttachmentStoreMigrator("SandboxUpload", sandboxAssetHTTPTimeout)
	})
	return sandboxUploader, sandboxUploaderErr
}

func parseSandboxAttachmentSpecs(args map[string]any) ([]sandboxAttachmentSpec, error) {
	if args == nil {
		return nil, nil
	}

	raw, ok := args["attachments"]
	if !ok {
		paths := stringSliceArg(args, "output_files")
		if len(paths) == 0 {
			return nil, nil
		}
		specs := make([]sandboxAttachmentSpec, 0, len(paths))
		for _, path := range paths {
			specs = append(specs, sandboxAttachmentSpec{Path: strings.TrimSpace(path)})
		}
		return specs, nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("attachments must be an array")
	}

	specs := make([]sandboxAttachmentSpec, 0, len(list))
	for _, item := range list {
		switch typed := item.(type) {
		case string:
			path := strings.TrimSpace(typed)
			if path == "" {
				return nil, fmt.Errorf("attachment path cannot be empty")
			}
			specs = append(specs, sandboxAttachmentSpec{Path: path})
		case map[string]any:
			spec, err := parseSandboxAttachmentSpec(typed)
			if err != nil {
				return nil, err
			}
			specs = append(specs, spec)
		default:
			return nil, fmt.Errorf("attachments entries must be strings or objects")
		}
	}

	return specs, nil
}

func parseSandboxAttachmentSpec(raw map[string]any) (sandboxAttachmentSpec, error) {
	path := strings.TrimSpace(stringArg(raw, "path"))
	if path == "" {
		path = strings.TrimSpace(stringArg(raw, "file"))
	}
	if path == "" {
		return sandboxAttachmentSpec{}, fmt.Errorf("attachment path is required")
	}

	spec := sandboxAttachmentSpec{
		Path:                path,
		Name:                strings.TrimSpace(stringArg(raw, "name")),
		MediaType:           strings.TrimSpace(stringArg(raw, "media_type")),
		Description:         strings.TrimSpace(stringArg(raw, "description")),
		Kind:                strings.TrimSpace(stringArg(raw, "kind")),
		Format:              strings.TrimSpace(stringArg(raw, "format")),
		PreviewProfile:      strings.TrimSpace(stringArg(raw, "preview_profile")),
		RetentionTTLSeconds: uint64Arg(raw, "retention_ttl_seconds"),
	}
	return spec, nil
}

func downloadSandboxAttachments(
	ctx context.Context,
	client *sandbox.Client,
	sessionID string,
	specs []sandboxAttachmentSpec,
	origin string,
) (map[string]ports.Attachment, []string) {
	if client == nil || len(specs) == 0 {
		return nil, nil
	}

	attachments := make(map[string]ports.Attachment, len(specs))
	var errs []string

	for _, spec := range specs {
		if !strings.HasPrefix(spec.Path, "/") {
			errs = append(errs, fmt.Sprintf("attachment path must be absolute: %s", spec.Path))
			continue
		}

		payload, err := readSandboxFileAsBase64(ctx, client, sessionID, spec.Path)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", spec.Path, err))
			continue
		}

		name := strings.TrimSpace(spec.Name)
		if name == "" {
			name = filepath.Base(spec.Path)
		}
		if name == "" {
			errs = append(errs, fmt.Sprintf("attachment name missing for %s", spec.Path))
			continue
		}

		mediaType := strings.TrimSpace(spec.MediaType)
		if mediaType == "" {
			mediaType = guessMediaType(name)
		}
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}

		attachment := ports.Attachment{
			Name:                name,
			MediaType:           mediaType,
			Data:                payload,
			Source:              origin,
			Description:         spec.Description,
			Kind:                spec.Kind,
			Format:              spec.Format,
			PreviewProfile:      spec.PreviewProfile,
			RetentionTTLSeconds: spec.RetentionTTLSeconds,
		}
		if attachment.PreviewProfile == "" && (attachment.MediaType != "" || attachment.Format != "") {
			attachment.PreviewProfile = previewProfile(attachment.MediaType, attachment.Format)
		}

		attachments[name] = attachment
	}

	if len(attachments) == 0 {
		return nil, errs
	}

	return attachments, errs
}

func readSandboxFileAsBase64(ctx context.Context, client *sandbox.Client, sessionID, path string) (string, error) {
	payload, err := readSandboxFile(ctx, client, sessionID, path, "base64")
	if err == nil && strings.TrimSpace(payload) != "" {
		return strings.TrimSpace(payload), nil
	}

	raw, err := readSandboxFile(ctx, client, sessionID, path, "")
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(raw) == "" {
		return "", fmt.Errorf("sandbox file read returned empty payload")
	}

	return base64.StdEncoding.EncodeToString([]byte(raw)), nil
}

func readSandboxFile(ctx context.Context, client *sandbox.Client, sessionID, path, encoding string) (string, error) {
	request := map[string]any{"file": path}
	if strings.TrimSpace(encoding) != "" {
		request["encoding"] = encoding
	}

	var response sandbox.Response[sandbox.FileReadResult]
	if err := client.DoJSON(ctx, httpMethodPost, "/v1/file/read", request, sessionID, &response); err != nil {
		return "", err
	}
	if !response.Success {
		return "", fmt.Errorf("sandbox file read failed: %s", response.Message)
	}
	if response.Data == nil {
		return "", fmt.Errorf("sandbox file read returned empty payload")
	}

	return response.Data.Content, nil
}

func guessMediaType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return ""
	}
	return mime.TypeByExtension(ext)
}

func normalizeSandboxAttachments(
	ctx context.Context,
	attachments map[string]ports.Attachment,
	uploader materialports.Migrator,
	origin string,
) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return attachments
	}

	if uploader == nil {
		if resolved, err := sandboxAttachmentUploader(); err == nil && resolved != nil {
			uploader = resolved
		}
	}
	if uploader == nil {
		return attachments
	}

	if ctx == nil {
		ctx = context.Background()
	}

	normalized, err := uploader.Normalize(ctx, materialports.MigrationRequest{
		Attachments: attachments,
		Status:      materialapi.MaterialStatusFinal,
		Origin:      origin,
	})
	if err != nil || len(normalized) == 0 {
		return attachments
	}
	return normalized
}

func formatAttachmentList(attachments map[string]ports.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	names := make([]string, 0, len(attachments))
	for name := range attachments {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
