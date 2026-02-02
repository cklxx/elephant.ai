package aliases

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
)

type attachmentSpec struct {
	Path                string
	Name                string
	MediaType           string
	Description         string
	Kind                string
	Format              string
	PreviewProfile      string
	RetentionTTLSeconds uint64
}

func parseAttachmentSpecs(args map[string]any) ([]attachmentSpec, error) {
	if args == nil {
		return nil, nil
	}
	raw, ok := args["attachments"]
	if !ok {
		paths := shared.StringSliceArg(args, "output_files")
		if len(paths) == 0 {
			return nil, nil
		}
		specs := make([]attachmentSpec, 0, len(paths))
		for _, path := range paths {
			specs = append(specs, attachmentSpec{Path: strings.TrimSpace(path)})
		}
		return specs, nil
	}

	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("attachments must be an array")
	}
	specs := make([]attachmentSpec, 0, len(list))
	for _, item := range list {
		switch typed := item.(type) {
		case string:
			path := strings.TrimSpace(typed)
			if path == "" {
				return nil, fmt.Errorf("attachment path cannot be empty")
			}
			specs = append(specs, attachmentSpec{Path: path})
		case map[string]any:
			spec, err := parseAttachmentSpec(typed)
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

func parseAttachmentSpec(raw map[string]any) (attachmentSpec, error) {
	path := strings.TrimSpace(shared.StringArg(raw, "path"))
	if path == "" {
		path = strings.TrimSpace(shared.StringArg(raw, "file"))
	}
	if path == "" {
		return attachmentSpec{}, fmt.Errorf("attachment path is required")
	}

	spec := attachmentSpec{
		Path:                path,
		Name:                strings.TrimSpace(shared.StringArg(raw, "name")),
		MediaType:           strings.TrimSpace(shared.StringArg(raw, "media_type")),
		Description:         strings.TrimSpace(shared.StringArg(raw, "description")),
		Kind:                strings.TrimSpace(shared.StringArg(raw, "kind")),
		Format:              strings.TrimSpace(shared.StringArg(raw, "format")),
		PreviewProfile:      strings.TrimSpace(shared.StringArg(raw, "preview_profile")),
		RetentionTTLSeconds: shared.Uint64Arg(raw, "retention_ttl_seconds"),
	}
	return spec, nil
}

func buildAttachmentsFromSpecs(ctx context.Context, specs []attachmentSpec, cfg shared.AutoUploadConfig) (map[string]ports.Attachment, []string) {
	if len(specs) == 0 {
		return nil, nil
	}
	attachments := make(map[string]ports.Attachment, len(specs))
	var errs []string
	for _, spec := range specs {
		att, err := buildAttachmentFromPath(ctx, spec, cfg)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		attachments[att.Name] = att
	}
	if len(attachments) == 0 {
		return nil, errs
	}
	return attachments, errs
}

func buildAttachmentFromPath(ctx context.Context, spec attachmentSpec, cfg shared.AutoUploadConfig) (ports.Attachment, error) {
	resolved, err := pathutil.ResolveLocalPath(ctx, spec.Path)
	if err != nil {
		return ports.Attachment{}, fmt.Errorf("%s: %w", spec.Path, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return ports.Attachment{}, fmt.Errorf("%s: %w", spec.Path, err)
	}
	if info.IsDir() {
		return ports.Attachment{}, fmt.Errorf("%s: attachment path is a directory", spec.Path)
	}
	if cfg.MaxBytes > 0 && info.Size() > int64(cfg.MaxBytes) {
		return ports.Attachment{}, fmt.Errorf("%s: exceeds max size %d bytes", spec.Path, cfg.MaxBytes)
	}

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = filepath.Base(resolved)
	}
	if name == "" {
		return ports.Attachment{}, fmt.Errorf("%s: attachment name missing", spec.Path)
	}

	ext := strings.ToLower(filepath.Ext(name))
	if !allowExtension(ext, cfg.AllowExts) {
		return ports.Attachment{}, fmt.Errorf("%s: extension %q not allowed", spec.Path, ext)
	}

	payload, err := os.ReadFile(resolved)
	if err != nil {
		return ports.Attachment{}, fmt.Errorf("%s: %w", spec.Path, err)
	}
	encoded := base64.StdEncoding.EncodeToString(payload)

	mediaType := strings.TrimSpace(spec.MediaType)
	if mediaType == "" {
		mediaType = mime.TypeByExtension(ext)
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}

	attachment := ports.Attachment{
		Name:                name,
		MediaType:           mediaType,
		Data:                encoded,
		Source:              "lark_local",
		Description:         spec.Description,
		Kind:                spec.Kind,
		Format:              spec.Format,
		PreviewProfile:      spec.PreviewProfile,
		RetentionTTLSeconds: spec.RetentionTTLSeconds,
	}
	if attachment.PreviewProfile == "" && (attachment.MediaType != "" || attachment.Format != "") {
		attachment.PreviewProfile = shared.PreviewProfile(attachment.MediaType, attachment.Format)
	}
	return attachment, nil
}

func allowExtension(ext string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	for _, item := range allowlist {
		if strings.ToLower(strings.TrimSpace(item)) == ext {
			return true
		}
	}
	return false
}

func formatAttachmentList(attachments map[string]ports.Attachment) string {
	if len(attachments) == 0 {
		return ""
	}
	names := make([]string, 0, len(attachments))
	for name := range attachments {
		names = append(names, name)
	}
	sortStrings(names)
	return strings.Join(names, ", ")
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	sort.Strings(values)
}
