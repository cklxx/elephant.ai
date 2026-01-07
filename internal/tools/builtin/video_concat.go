package builtin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

const defaultConcatOutput = "seedream_concat.mp4"

type videoConcatTool struct {
	httpClient *http.Client
}

func NewVideoConcat() ports.ToolExecutor {
	return &videoConcatTool{
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

func (t *videoConcatTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "video_concat",
		Version:  "1.0.0",
		Category: "media",
		Tags:     []string{"video", "concat", "editing", "seedream"},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"video/mp4", "video/webm", "video/quicktime"},
			Produces: []string{"video/mp4"},
		},
	}
}

func (t *videoConcatTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "video_concat",
		Description: "Concatenate multiple videos (e.g. Seedream clips) into a single MP4 file.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"videos": {
					Type:        "array",
					Description: "Ordered list of videos (data URI, HTTPS URL, base64 string, or attachment placeholder like `[clip.mp4]`).",
				},
				"ffmpeg_path": {
					Type:        "string",
					Description: "Optional path to the ffmpeg binary (defaults to ffmpeg in PATH).",
				},
				"output_name": {
					Type:        "string",
					Description: "Output filename (default: seedream_concat.mp4).",
				},
				"description": {
					Type:        "string",
					Description: "Optional description for the resulting video attachment.",
				},
			},
			Required: []string{"videos"},
		},
		MaterialCapabilities: ports.ToolMaterialCapabilities{
			Consumes: []string{"video/mp4", "video/webm", "video/quicktime"},
			Produces: []string{"video/mp4"},
		},
	}
}

func (t *videoConcatTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	videos := stringSliceArg(call.Arguments, "videos")
	if len(videos) < 2 {
		err := errors.New("videos must include at least two entries")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	ffmpegBin := strings.TrimSpace(stringArg(call.Arguments, "ffmpeg_path"))
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	if _, err := exec.LookPath(ffmpegBin); err != nil {
		wrapped := fmt.Errorf("ffmpeg binary %q not found in PATH", ffmpegBin)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	outputName := strings.TrimSpace(stringArg(call.Arguments, "output_name"))
	if outputName == "" {
		outputName = defaultConcatOutput
	}
	outputName = filepath.Base(outputName)
	if outputName == "." || outputName == "/" || outputName == "" {
		outputName = defaultConcatOutput
	}
	if !strings.HasSuffix(strings.ToLower(outputName), ".mp4") {
		outputName += ".mp4"
	}

	description := strings.TrimSpace(stringArg(call.Arguments, "description"))
	if description == "" {
		description = fmt.Sprintf("Concatenated video with %d segment(s)", len(videos))
	}

	workdir, err := os.MkdirTemp("", "video-concat-")
	if err != nil {
		wrapped := fmt.Errorf("create workdir: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}
	defer os.RemoveAll(workdir)

	segmentPaths := make([]string, 0, len(videos))
	for idx, ref := range videos {
		segmentPath, err := t.resolveVideoToFile(ctx, workdir, idx, ref)
		if err != nil {
			wrapped := fmt.Errorf("resolve videos[%d]: %w", idx, err)
			return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
		}
		segmentPaths = append(segmentPaths, segmentPath)
	}

	manifestPath := filepath.Join(workdir, "segments.txt")
	if err := writeConcatManifest(manifestPath, segmentPaths); err != nil {
		wrapped := fmt.Errorf("write manifest: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	outputPath := filepath.Join(workdir, outputName)
	cmd := exec.CommandContext(ctx, ffmpegBin, "-y", "-f", "concat", "-safe", "0", "-i", manifestPath, "-c", "copy", "-movflags", "+faststart", outputPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		wrapped := fmt.Errorf("ffmpeg concat failed: %w: %s", err, strings.TrimSpace(string(out)))
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	outputBytes, err := os.ReadFile(outputPath)
	if err != nil {
		wrapped := fmt.Errorf("read output: %w", err)
		return &ports.ToolResult{CallID: call.ID, Content: wrapped.Error(), Error: wrapped}, nil
	}

	encoded := base64.StdEncoding.EncodeToString(outputBytes)
	attachment := ports.Attachment{
		Name:           outputName,
		MediaType:      "video/mp4",
		Data:           encoded,
		URI:            fmt.Sprintf("data:video/mp4;base64,%s", encoded),
		Source:         call.Name,
		Description:    description,
		Kind:           "artifact",
		Format:         "mp4",
		PreviewProfile: "video",
	}

	attachments := map[string]ports.Attachment{outputName: attachment}
	mutations := map[string]any{
		"attachment_mutations": map[string]any{
			"add": attachments,
		},
	}

	return &ports.ToolResult{
		CallID:      call.ID,
		Content:     fmt.Sprintf("Created %s from %d segments.", outputName, len(videos)),
		Metadata:    mutations,
		Attachments: attachments,
	}, nil
}

func (t *videoConcatTool) resolveVideoToFile(ctx context.Context, workdir string, index int, ref string) (string, error) {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return "", errors.New("empty video reference")
	}

	if path, err := t.resolveVideoFromAttachment(ctx, workdir, index, trimmed); err == nil && path != "" {
		return path, nil
	} else if err != nil {
		return "", err
	}

	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "data:") {
		bytes, mimeType, err := decodeDataURI(trimmed)
		if err != nil {
			return "", err
		}
		return writeVideoBytes(workdir, index, bytes, mimeType)
	}

	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return t.downloadVideo(ctx, workdir, index, trimmed, "")
	}

	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) > 0 {
		return writeVideoBytes(workdir, index, decoded, http.DetectContentType(decoded))
	}

	return "", fmt.Errorf("unsupported video reference %q", trimmed)
}

func (t *videoConcatTool) resolveVideoFromAttachment(ctx context.Context, workdir string, index int, ref string) (string, error) {
	attachments, _ := ports.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return "", nil
	}

	name := normalizePlaceholder(ref)
	if name == "" {
		name = ref
	}
	att, ok := lookupAttachmentCaseInsensitive(attachments, name)
	if !ok {
		return "", nil
	}

	if data := strings.TrimSpace(att.Data); data != "" {
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return "", fmt.Errorf("decode attachment payload: %w", err)
		}
		return writeVideoBytes(workdir, index, decoded, att.MediaType)
	}

	uri := strings.TrimSpace(att.URI)
	if uri == "" {
		return "", errors.New("attachment has no data or uri")
	}

	lower := strings.ToLower(uri)
	if strings.HasPrefix(lower, "data:") {
		decoded, mimeType, err := decodeDataURI(uri)
		if err != nil {
			return "", err
		}
		if mimeType == "" {
			mimeType = att.MediaType
		}
		return writeVideoBytes(workdir, index, decoded, mimeType)
	}

	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return t.downloadVideo(ctx, workdir, index, uri, att.MediaType)
	}

	return "", fmt.Errorf("unsupported attachment uri %q", uri)
}

func (t *videoConcatTool) downloadVideo(ctx context.Context, workdir string, index int, url string, hintedType string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := t.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = hintedType
	}

	filePath := buildSegmentPath(workdir, index, contentType)
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", err
	}

	return filePath, nil
}

func writeVideoBytes(workdir string, index int, payload []byte, mimeType string) (string, error) {
	if len(payload) == 0 {
		return "", errors.New("empty video payload")
	}
	filePath := buildSegmentPath(workdir, index, mimeType)
	if err := os.WriteFile(filePath, payload, 0o644); err != nil {
		return "", err
	}
	return filePath, nil
}

func buildSegmentPath(workdir string, index int, mimeType string) string {
	ext := extensionForVideoMIME(mimeType)
	if ext == "" {
		ext = "mp4"
	}
	filename := fmt.Sprintf("segment_%02d.%s", index+1, ext)
	return filepath.Join(workdir, filename)
}

func extensionForVideoMIME(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "video/mp4":
		return "mp4"
	case "video/webm":
		return "webm"
	case "video/quicktime":
		return "mov"
	default:
		return ""
	}
}

func writeConcatManifest(path string, segments []string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	for _, segment := range segments {
		escaped := strings.ReplaceAll(segment, "'", "'\\''")
		if _, err := fmt.Fprintf(file, "file '%s'\n", escaped); err != nil {
			return err
		}
	}
	return nil
}
