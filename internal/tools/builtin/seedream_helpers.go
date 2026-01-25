package builtin

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/jsonx"
	"alex/internal/materials"
	materialapi "alex/internal/materials/api"
	materialports "alex/internal/materials/ports"

	arkm "github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func applyImageRequestOptions(req *arkm.GenerateImagesRequest, args map[string]any) {
	sizeValue := seedreamDefaultImageSize
	if size, ok := args["size"].(string); ok && strings.TrimSpace(size) != "" {
		sizeValue = strings.TrimSpace(size)
	} else if width, ok := readInt(args, "width"); ok {
		if height, okh := readInt(args, "height"); okh && width > 0 && height > 0 {
			sizeValue = fmt.Sprintf("%dx%d", width, height)
		}
	}
	req.Size = volcengine.String(normalizeSeedreamSize(sizeValue))
	if seed, ok := readInt(args, "seed"); ok {
		req.Seed = volcengine.Int64(int64(seed))
	}
	if cfgScale, ok := readFloat(args, "cfg_scale"); ok {
		req.GuidanceScale = volcengine.Float64(sanitizeSeedreamGuidanceScale(cfgScale))
	}
	if optimize, ok := args["optimize_prompt"].(bool); ok {
		req.OptimizePrompt = volcengine.Bool(optimize)
	}
}

func normalizeSeedreamSize(raw string) string {
	width, height, ok := parseSeedreamSize(raw)
	if !ok {
		return seedreamDefaultImageSize
	}

	area := width * height
	if area >= seedreamMinImagePixels {
		return fmt.Sprintf("%dx%d", width, height)
	}

	scale := math.Sqrt(float64(seedreamMinImagePixels) / float64(area))
	scaledWidth := int(math.Ceil(float64(width) * scale))
	scaledHeight := int(math.Ceil(float64(height) * scale))
	return fmt.Sprintf("%dx%d", scaledWidth, scaledHeight)
}

func parseSeedreamSize(raw string) (int, int, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(raw)), "x")
	if len(parts) != 2 {
		return 0, 0, false
	}

	width, err := strconv.Atoi(parts[0])
	if err != nil || width <= 0 {
		return 0, 0, false
	}

	height, err := strconv.Atoi(parts[1])
	if err != nil || height <= 0 {
		return 0, 0, false
	}

	return width, height, true
}

func sanitizeSeedreamGuidanceScale(value float64) float64 {
	if math.IsNaN(value) {
		return seedreamDefaultGuidanceScale
	}
	if value < seedreamMinGuidanceScale || value > seedreamMaxGuidanceScale {
		return seedreamDefaultGuidanceScale
	}
	return value
}

func seedreamMissingConfigMessage(config SeedreamConfig) string {
	missing := []string{}
	if strings.TrimSpace(config.APIKey) == "" {
		missing = append(missing, "ARK_API_KEY")
	}
	if strings.TrimSpace(config.Model) == "" {
		label := "Seedream model identifier"
		if config.ModelEnvVar != "" {
			label = strings.ToUpper(config.ModelEnvVar)
		}
		missing = append(missing, label)
	}
	if len(missing) == 0 {
		return ""
	}

	toolName := config.ModelDescriptor
	if toolName == "" {
		toolName = "Seedream"
	}

	builder := &strings.Builder{}
	fmt.Fprintf(builder, "%s is not configured. Missing values: %s.\n\n", toolName, strings.Join(missing, ", "))
	builder.WriteString("Provide the following settings via ~/.alex/config.yaml (supports ${ENV} interpolation):\n\n")
	builder.WriteString("- ARK_API_KEY from the Volcano Engine Ark console\n")
	if config.ModelEnvVar != "" {
		fmt.Fprintf(builder, "- %s to select the desired Seedream model\n", strings.ToUpper(config.ModelEnvVar))
	} else {
		builder.WriteString("- Seedream model identifier (e.g. doubao-seedream-4-5-251128)\n")
	}
	return builder.String()
}

func formatSeedreamResponse(resp *arkm.ImagesResponse, descriptor, prompt string) (string, map[string]any, map[string]ports.Attachment) {
	return formatSeedreamResponseWithContext(context.Background(), resp, descriptor, prompt)
}

func formatSeedreamResponseWithContext(ctx context.Context, resp *arkm.ImagesResponse, descriptor, prompt string) (string, map[string]any, map[string]ports.Attachment) {
	if resp == nil {
		return "Seedream returned an empty response.", nil, nil
	}

	images := make([]map[string]any, 0, len(resp.Data))
	attachments := make(map[string]ports.Attachment)
	placeholders := make([]string, 0, len(resp.Data))

	requestID := seedreamAttachmentPrefix(resp.Model, resp.Created)

	trimmedPrompt := strings.TrimSpace(prompt)
	attachmentDescription := trimmedPrompt
	if attachmentDescription == "" {
		attachmentDescription = strings.TrimSpace(descriptor)
	}
	if attachmentDescription == "" {
		attachmentDescription = "Seedream image"
	}

	for idx, item := range resp.Data {
		if item == nil {
			continue
		}
		entry := map[string]any{"index": idx}
		if strings.TrimSpace(item.Size) != "" {
			entry["size"] = strings.TrimSpace(item.Size)
		}

		placeholder := fmt.Sprintf("%s_%d.png", requestID, idx)
		entry["placeholder"] = placeholder

		urlStr := strings.TrimSpace(safeDeref(item.Url))
		if urlStr != "" {
			entry["url"] = urlStr
		}

		b64 := strings.TrimSpace(safeDeref(item.B64Json))
		mimeType := inferMediaTypeFromURL(urlStr, "image/png")

		switch {
		case b64 != "":
			attachments[placeholder] = ports.Attachment{
				Name:        placeholder,
				MediaType:   mimeType,
				Data:        b64,
				URI:         fmt.Sprintf("data:%s;base64,%s", mimeType, b64),
				Source:      "seedream",
				Description: attachmentDescription,
			}
			entry["source"] = "base64"
		case urlStr != "":
			attachments[placeholder] = ports.Attachment{
				Name:        placeholder,
				MediaType:   mimeType,
				URI:         urlStr,
				Source:      "seedream",
				Description: attachmentDescription,
			}
			entry["source"] = "url"
		default:
			entry["warning"] = "missing image payload"
			images = append(images, entry)
			continue
		}

		images = append(images, entry)
		placeholders = append(placeholders, placeholder)
	}

	metadata := map[string]any{
		"model":   resp.Model,
		"created": resp.Created,
		"images":  images,
	}
	if resp.Usage != nil {
		metadata["usage"] = resp.Usage
	}
	if descriptor != "" {
		metadata["model_descriptor"] = descriptor
	}
	if trimmedPrompt != "" {
		metadata["prompt"] = trimmedPrompt
		metadata["description"] = trimmedPrompt
	} else if descriptor != "" {
		metadata["description"] = descriptor
	}

	var builder strings.Builder
	title := strings.TrimSpace(descriptor)
	if title == "" {
		title = "Seedream image generation"
	}
	fmt.Fprintf(&builder, "%s complete.\n", title)
	if trimmedPrompt != "" {
		fmt.Fprintf(&builder, "Prompt: %s\n", trimmedPrompt)
	}
	if len(placeholders) > 0 {
		builder.WriteString("Attachments:\n")
		for idx, name := range placeholders {
			fmt.Fprintf(&builder, "%d. [%s]\n", idx+1, name)
		}
	}

	content := strings.TrimSpace(builder.String())

	if len(attachments) > 0 {
		if uploader, err := seedreamAttachmentUploader(); err == nil && uploader != nil {
			if ctx == nil {
				ctx = context.Background()
			}
			normalized, err := uploader.Normalize(ctx, materialports.MigrationRequest{
				Attachments: attachments,
				Status:      materialapi.MaterialStatusFinal,
				Origin:      "seedream",
			})
			if err == nil && len(normalized) > 0 {
				attachments = normalized
			}
		}
	}

	return content, metadata, attachments
}

func safeDeref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

var (
	seedreamUploaderOnce sync.Once
	seedreamUploader     *materials.AttachmentStoreMigrator
	seedreamUploaderErr  error
)

func seedreamAttachmentUploader() (*materials.AttachmentStoreMigrator, error) {
	seedreamUploaderOnce.Do(func() {
		seedreamUploader, seedreamUploaderErr = buildAttachmentStoreMigrator("SeedreamUpload", seedreamAssetHTTPTimeout)
	})
	return seedreamUploader, seedreamUploaderErr
}

func normalizeSeedreamInitImage(raw string) (string, string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", "", errors.New("init_image parameter must be provided (base64 or URL)")
	}

	if strings.HasPrefix(trimmed, "data:") {
		if _, err := extractBase64Payload(trimmed); err != nil {
			return "", "", fmt.Errorf("invalid init_image data URI: %w", err)
		}
		return trimmed, classifySeedreamInitImageKind(trimmed), nil
	}

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed, classifySeedreamInitImageKind(trimmed), nil
	}

	if strings.Contains(trimmed, "://") {
		return "", "", fmt.Errorf("init_image must be an HTTPS URL or data URI")
	}

	// Assume bare base64 PNG blobs and wrap in a generic data URI.
	encoded := fmt.Sprintf("data:image/png;base64,%s", trimmed)
	return encoded, classifySeedreamInitImageKind(encoded), nil
}

func coalesceSeedreamFirstFrameSource(urlValue, base64Value, mimeType string) (string, string, string, error) {
	trimmedURL := strings.TrimSpace(urlValue)
	trimmedBase := strings.TrimSpace(base64Value)
	normalizedMime := normalizeSeedreamMimeType(mimeType)

	if trimmedBase == "" {
		if trimmedURL == "" {
			return "", "", "", nil
		}
		if !strings.HasPrefix(trimmedURL, "http://") && !strings.HasPrefix(trimmedURL, "https://") && !strings.HasPrefix(trimmedURL, "data:") {
			return "", "", "", errors.New("first_frame_url must be an HTTPS URL or data URI")
		}
		return trimmedURL, classifySeedreamInitImageKind(trimmedURL), normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedURL)), nil
	}

	if strings.HasPrefix(trimmedBase, "http://") || strings.HasPrefix(trimmedBase, "https://") {
		// Allow callers to pass a URL via the base64 field for convenience.
		return trimmedBase, classifySeedreamInitImageKind(trimmedBase), normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedBase)), nil
	}

	if strings.HasPrefix(trimmedBase, "data:") {
		payload, err := extractBase64Payload(trimmedBase)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid base64 data URI: %w", err)
		}
		canonical, err := canonicalizeSeedreamBase64(payload)
		if err != nil {
			return "", "", "", fmt.Errorf("invalid base64 payload: %w", err)
		}
		if normalizedMime == "" {
			normalizedMime = normalizeSeedreamMimeType(extractSeedreamDataURIMime(trimmedBase))
		}
		if normalizedMime == "" {
			normalizedMime = "image/png"
		}
		uri := buildSeedreamDataURI(normalizedMime, canonical)
		return uri, "data_uri", normalizedMime, nil
	}

	canonical, err := canonicalizeSeedreamBase64(trimmedBase)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid base64 payload: %w", err)
	}
	if normalizedMime == "" {
		normalizedMime = "image/png"
	}
	uri := buildSeedreamDataURI(normalizedMime, canonical)
	return uri, "data_uri", normalizedMime, nil
}

func canonicalizeSeedreamBase64(raw string) (string, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.ReplaceAll(cleaned, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	if cleaned == "" {
		return "", errors.New("base64 data must not be empty")
	}

	decoded, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		decoded, err = base64.RawStdEncoding.DecodeString(cleaned)
		if err != nil {
			return "", err
		}
	}
	return base64.StdEncoding.EncodeToString(decoded), nil
}

func buildSeedreamDataURI(mimeType, payload string) string {
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, payload)
}

func normalizeSeedreamMimeType(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch trimmed {
	case "", "default":
		return ""
	case ".png", "png":
		return "image/png"
	case ".jpg", "jpg", ".jpeg", "jpeg":
		return "image/jpeg"
	case ".webp", "webp":
		return "image/webp"
	case ".gif", "gif":
		return "image/gif"
	default:
		return trimmed
	}
}

func extractSeedreamDataURIMime(dataURI string) string {
	if !strings.HasPrefix(dataURI, "data:") {
		return ""
	}
	comma := strings.Index(dataURI, ",")
	if comma == -1 {
		return ""
	}
	header := dataURI[len("data:"):comma]
	if header == "" {
		return ""
	}
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return ""
	}
	if strings.Contains(parts[0], "/") {
		return parts[0]
	}
	return ""
}

func summarizeSeedreamImageValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "(empty)"
	}

	const previewLen = 32

	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		prefix := trimmed
		if len(prefix) > previewLen {
			prefix = prefix[:previewLen] + "..."
		}
		return fmt.Sprintf("url(len=%d,prefix=%q)", len(trimmed), prefix)
	}

	if strings.HasPrefix(trimmed, "data:") {
		meta := trimmed
		dataIdx := strings.Index(meta, ",")
		if dataIdx != -1 {
			header := meta[:dataIdx]
			payload := meta[dataIdx+1:]
			if len(payload) > previewLen {
				payload = payload[:previewLen] + "..."
			}
			return fmt.Sprintf("data_uri(header=%q,len=%d,payload_prefix=%q)", header, len(trimmed), payload)
		}
		if len(meta) > previewLen {
			meta = meta[:previewLen] + "..."
		}
		return fmt.Sprintf("data_uri(len=%d,prefix=%q)", len(trimmed), meta)
	}

	payload := trimmed
	if len(payload) > previewLen {
		payload = payload[:previewLen] + "..."
	}
	return fmt.Sprintf("base64(len=%d,prefix=%q)", len(trimmed), payload)
}

func classifySeedreamInitImageKind(value string) string {
	switch {
	case strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://"):
		return "url"
	case strings.HasPrefix(value, "data:"):
		return "data_uri"
	default:
		return "base64"
	}
}

func extractBase64Payload(dataURI string) (string, error) {
	comma := strings.Index(dataURI, ",")
	if !strings.HasPrefix(dataURI, "data:") || comma == -1 {
		return "", errors.New("invalid data URI format")
	}
	payload := dataURI[comma+1:]
	if payload == "" {
		return "", errors.New("missing data payload")
	}
	return payload, nil
}

func readInt(args map[string]any, key string) (int, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case jsonx.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func readFloat(args map[string]any, key string) (float64, bool) {
	value, ok := args[key]
	if !ok {
		return 0, false
	}
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case jsonx.Number:
		if f, err := v.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func readStringSlice(value any) []string {
	switch v := value.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func stringFromArgs(args map[string]any, key string) string {
	if value, ok := args[key]; ok {
		switch v := value.(type) {
		case string:
			return v
		case jsonx.Number:
			return v.String()
		case fmt.Stringer:
			return v.String()
		}
	}
	return ""
}

func readBoolWithDefault(args map[string]any, key string, def bool) bool {
	value, ok := args[key]
	if !ok {
		return def
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(v))
		switch trimmed {
		case "", "default":
			return def
		case "true", "1", "yes", "y", "on":
			return true
		case "false", "0", "no", "n", "off":
			return false
		}
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case jsonx.Number:
		if i, err := v.Int64(); err == nil {
			return i != 0
		}
	}
	return def
}

func readDurationWithDefault(args map[string]any, key string, def int) time.Duration {
	seconds, ok := readInt(args, key)
	if !ok {
		return time.Duration(def) * time.Second
	}
	if seconds <= 0 {
		return time.Duration(def) * time.Second
	}
	return time.Duration(seconds) * time.Second
}

func resolveSeedreamInitImagePlaceholder(ctx context.Context, raw string) (string, string, bool) {
	placeholder := strings.TrimSpace(raw)
	if placeholder == "" {
		return "", "", false
	}
	name, ok := extractPlaceholderIdentifier(placeholder)
	if !ok {
		return "", "", false
	}

	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return "", "", false
	}

	if att, exists := attachments[name]; exists {
		if resolved := attachmentReferenceValueForTool(att); resolved != "" {
			return resolved, name, true
		}
	}

	lowerName := strings.ToLower(name)
	for key, att := range attachments {
		if strings.ToLower(strings.TrimSpace(key)) != lowerName {
			continue
		}
		if resolved := attachmentReferenceValueForTool(att); resolved != "" {
			return resolved, strings.TrimSpace(key), true
		}
	}

	return "", "", false
}

func extractPlaceholderIdentifier(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 {
		return "", false
	}
	if trimmed[0] != '[' || trimmed[len(trimmed)-1] != ']' {
		return "", false
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

func attachmentReferenceValueForTool(att ports.Attachment) string {
	data := strings.TrimSpace(att.Data)
	if data != "" {
		if strings.HasPrefix(data, "data:") {
			return data
		}
		mediaType := strings.TrimSpace(att.MediaType)
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
	}
	if uri := strings.TrimSpace(att.URI); uri != "" {
		return uri
	}
	return ""
}

func seedreamAttachmentPrefix(model string, created int64) string {
	prefix := strings.TrimSpace(model)
	if prefix != "" {
		prefix = strings.ReplaceAll(prefix, "/", "_")
	} else {
		prefix = "seedream"
	}
	if suffix := strings.TrimSpace(seedreamPlaceholderNonce()); suffix != "" {
		return fmt.Sprintf("%s_%s", prefix, suffix)
	}
	if created > 0 {
		return fmt.Sprintf("%s_%d", prefix, created)
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func inferMediaTypeFromURL(rawURL, defaultType string) string {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return defaultType
	}
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".mov"):
		return "video/quicktime"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".zip"):
		return "application/zip"
	case strings.HasSuffix(lower, ".tar") || strings.HasSuffix(lower, ".tar.gz"):
		return "application/x-tar"
	}
	return defaultType
}
