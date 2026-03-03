package larktools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/artifacts"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

type larkUploadFile struct {
	shared.BaseTool
}

const defaultMaxBytes = 20 * 1024 * 1024

var larkSupportedFileTypes = map[string]bool{
	"opus": true, "mp4": true, "pdf": true,
	"doc": true, "xls": true, "ppt": true,
	"stream": true,
}

var larkAudioExtensions = map[string]bool{
	"m4a": true, "mp3": true, "opus": true, "wav": true, "aac": true,
}

var larkImageExtensions = map[string]bool{
	"jpg": true, "jpeg": true, "png": true,
	"gif": true, "webp": true, "bmp": true,
	"ico": true, "tif": true, "tiff": true,
}

type uploadCandidate struct {
	reader   io.Reader
	cleanup  func()
	fileName string
	fileType string
	size     int64
	mimeType string
	meta     map[string]any
}

// NewLarkUploadFile constructs a tool that uploads a file and sends it to the
// current Lark chat (audio files are sent as "audio" messages).
func NewLarkUploadFile() tools.ToolExecutor {
	return &larkUploadFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_upload_file",
				Description: "Upload an actual file attachment to the current Lark chat. Use only when explicit file delivery/attachment transfer is required. Do not use for text-only updates or context retrieval; use lark_send_message or lark_chat_history instead.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"source": {
							Type:        "string",
							Description: "Upload source identifier. If source_kind=path, pass a local file path (must stay within the working directory or a temp directory). If source_kind=attachment, pass an attachment name from current task context.",
						},
						"source_kind": {
							Type:        "string",
							Description: "Source type for source.",
							Enum:        []any{"path", "attachment"},
						},
						"file_name": {
							Type:        "string",
							Description: "Optional override for the uploaded file name.",
						},
						"max_bytes": {
							Type:        "integer",
							Description: "Maximum upload size in bytes (default 20MiB).",
						},
					},
					Required: []string{"source", "source_kind"},
				},
			},
			ports.ToolMetadata{
				Name:        "lark_upload_file",
				Version:     "0.1.0",
				Category:    "lark",
				Tags:        []string{"lark", "chat", "upload", "file"},
				SafetyLevel: ports.SafetyLevelReversible,
			},
		),
	}
}

func (t *larkUploadFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	client, errResult := requireLarkClient(ctx, call.ID)
	if errResult != nil {
		return errResult, nil
	}

	chatID := shared.LarkChatIDFromContext(ctx)
	if chatID == "" {
		return missingChatIDResult(call.ID, "lark_upload_file"), nil
	}

	maxBytes := parseMaxBytes(call.Arguments)
	candidate, errResult := prepareUploadCandidate(ctx, call.ID, call.Arguments, maxBytes)
	if errResult != nil {
		return errResult, nil
	}
	if candidate.cleanup != nil {
		defer candidate.cleanup()
	}

	replyToID := strings.TrimSpace(shared.LarkMessageIDFromContext(ctx))

	var msgType string
	msgContent := ""
	metadata := map[string]any{
		"chat_id":   chatID,
		"file_name": candidate.fileName,
		"file_type": candidate.fileType,
		"mime_type": candidate.mimeType,
		"bytes":     candidate.size,
		"max_bytes": maxBytes,
	}
	if replyToID != "" {
		metadata["reply_to_message_id"] = replyToID
	}

	isImage := isImageFile(candidate.fileName, candidate.mimeType)
	if isImage {
		imageKey, errResult := uploadImage(ctx, call.ID, client, candidate.reader)
		if errResult != nil {
			return errResult, nil
		}
		msgType = "image"
		msgContent = imageContent(imageKey)
		metadata["image_key"] = imageKey
	} else {
		uploadReq := larkim.NewCreateFileReqBuilder().
			Body(larkim.NewCreateFileReqBodyBuilder().
				FileType(candidate.fileType).
				FileName(candidate.fileName).
				File(candidate.reader).
				Build()).
			Build()

		uploadResp, err := client.Im.V1.File.Create(ctx, uploadReq)
		if err != nil {
			return sdkCallErr(call.ID, "lark_upload_file: upload", err), nil
		}
		if !uploadResp.Success() {
			return sdkRespErr(call.ID, "lark_upload_file: upload", uploadResp.Code, uploadResp.Msg), nil
		}
		fileKey := ""
		if uploadResp.Data != nil && uploadResp.Data.FileKey != nil {
			fileKey = strings.TrimSpace(*uploadResp.Data.FileKey)
		}
		if fileKey == "" {
			return &ports.ToolResult{
				CallID:  call.ID,
				Content: "lark_upload_file: upload missing file_key",
				Error:   fmt.Errorf("lark upload missing file_key"),
			}, nil
		}

		if isAudioFile(candidate.fileName, candidate.mimeType) {
			msgType = "audio"
		} else {
			msgType = "file"
		}
		msgContent = fileContent(fileKey)
		metadata["file_key"] = fileKey
	}

	messageID, errResult := sendUploadedMessage(ctx, client, call.ID, chatID, replyToID, msgType, msgContent)
	if errResult != nil {
		return errResult, nil
	}
	metadata["message_id"] = messageID
	metadata["msg_type"] = msgType

	for k, v := range candidate.meta {
		metadata[k] = v
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "File sent successfully.",
		Metadata: metadata,
	}, nil
}

func uploadImage(ctx context.Context, callID string, client *lark.Client, reader io.Reader) (string, *ports.ToolResult) {
	uploadReq := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType(larkim.ImageTypeMessage).
			Image(reader).
			Build()).
		Build()

	uploadResp, err := client.Im.V1.Image.Create(ctx, uploadReq)
	if err != nil {
		return "", sdkCallErr(callID, "upload image", err)
	}
	if !uploadResp.Success() {
		return "", sdkRespErr(callID, "upload image", uploadResp.Code, uploadResp.Msg)
	}
	imageKey := ""
	if uploadResp.Data != nil && uploadResp.Data.ImageKey != nil {
		imageKey = strings.TrimSpace(*uploadResp.Data.ImageKey)
	}
	if imageKey == "" {
		return "", toolErrorResult(callID, "lark upload image missing image_key")
	}
	return imageKey, nil
}

func sendUploadedMessage(ctx context.Context, client *lark.Client, callID, chatID, replyToID, msgType, content string) (string, *ports.ToolResult) {
	if replyToID != "" {
		req := larkim.NewReplyMessageReqBuilder().
			MessageId(replyToID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType(msgType).
				Content(content).
				Build()).
			Build()
		resp, err := client.Im.Message.Reply(ctx, req)
		if err != nil {
			return "", sdkCallErr(callID, "reply message", err)
		}
		if !resp.Success() {
			return "", sdkRespErr(callID, "reply message", resp.Code, resp.Msg)
		}
		if resp.Data != nil && resp.Data.MessageId != nil {
			return *resp.Data.MessageId, nil
		}
		return "", nil
	}

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType("chat_id").
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(chatID).
			MsgType(msgType).
			Content(content).
			Build()).
		Build()
	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		return "", sdkCallErr(callID, "send message", err)
	}
	if !resp.Success() {
		return "", sdkRespErr(callID, "send message", resp.Code, resp.Msg)
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func prepareUploadCandidate(ctx context.Context, callID string, args map[string]any, maxBytes int) (uploadCandidate, *ports.ToolResult) {
	source := strings.TrimSpace(shared.StringArg(args, "source"))
	sourceKind := utils.TrimLower(shared.StringArg(args, "source_kind"))
	if source == "" {
		return uploadCandidate{}, toolErrorResult(callID, "source is required")
	}
	if sourceKind != "path" && sourceKind != "attachment" {
		return uploadCandidate{}, toolErrorResult(callID, "source_kind must be one of: path, attachment")
	}

	fileNameOverride := strings.TrimSpace(shared.StringArg(args, "file_name"))
	if _, ok := args["file_name"]; ok && fileNameOverride == "" {
		return uploadCandidate{}, toolErrorResult(callID, "file_name cannot be empty")
	}

	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	if sourceKind == "path" {
		return preparePathCandidate(ctx, callID, source, fileNameOverride, maxBytes)
	}
	return prepareAttachmentCandidate(ctx, callID, source, fileNameOverride, maxBytes)
}

func preparePathCandidate(ctx context.Context, callID, rawPath, fileNameOverride string, maxBytes int) (uploadCandidate, *ports.ToolResult) {
	resolved, err := pathutil.ResolveLocalPathOrTemp(ctx, rawPath)
	if err != nil {
		return uploadCandidate{}, toolErrorResult(callID, "%s: %w", rawPath, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return uploadCandidate{}, toolErrorResult(callID, "%s: %w", rawPath, err)
	}
	if info.IsDir() {
		return uploadCandidate{}, toolErrorResult(callID, "%s: path is a directory", rawPath)
	}
	if maxBytes > 0 && info.Size() > int64(maxBytes) {
		return uploadCandidate{}, toolErrorResult(callID, "%s: exceeds max size %d bytes", rawPath, maxBytes)
	}

	file, err := os.Open(resolved)
	if err != nil {
		return uploadCandidate{}, toolErrorResult(callID, "%s: %w", rawPath, err)
	}

	fileName := strings.TrimSpace(fileNameOverride)
	if fileName == "" {
		fileName = filepath.Base(resolved)
	}
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" {
		_ = file.Close()
		return uploadCandidate{}, toolErrorResult(callID, "%s: file name missing", rawPath)
	}

	meta := map[string]any{
		"path":          rawPath,
		"resolved_path": resolved,
	}
	return uploadCandidate{
		reader:   file,
		cleanup:  func() { _ = file.Close() },
		fileName: fileName,
		fileType: larkFileType(fileTypeForName(fileName)),
		size:     info.Size(),
		mimeType: detectMimeTypeFromFile(file),
		meta:     meta,
	}, nil
}

func prepareAttachmentCandidate(ctx context.Context, callID, attachmentName, fileNameOverride string, maxBytes int) (uploadCandidate, *ports.ToolResult) {
	ctx = shared.WithAllowLocalFetch(ctx)
	client := artifacts.NewAttachmentHTTPClient(artifacts.AttachmentFetchTimeout, "LarkUploadFile")

	ref := attachmentName
	if !strings.HasPrefix(ref, "[") || !strings.HasSuffix(ref, "]") {
		ref = "[" + attachmentName + "]"
	}
	payload, _, err := artifacts.ResolveAttachmentBytes(ctx, ref, client)
	if err != nil {
		return uploadCandidate{}, toolErrorResult(callID, "%s: %w", attachmentName, err)
	}
	if maxBytes > 0 && len(payload) > maxBytes {
		return uploadCandidate{}, toolErrorResult(callID, "%s: exceeds max size %d bytes", attachmentName, maxBytes)
	}

	fileName := strings.TrimSpace(fileNameOverride)
	if fileName == "" {
		fileName = attachmentFileName(ctx, attachmentName)
	}
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" {
		return uploadCandidate{}, toolErrorResult(callID, "%s: file name missing", attachmentName)
	}

	meta := map[string]any{
		"attachment_name": attachmentName,
	}
	mimeType := normalizeMimeType(attachmentMediaType(ctx, attachmentName))
	if !isAudioMimeType(mimeType) {
		if sniffed := normalizeMimeType(http.DetectContentType(payload)); isAudioMimeType(sniffed) {
			mimeType = sniffed
		}
	}
	if mimeType == "" {
		mimeType = normalizeMimeType(http.DetectContentType(payload))
	}
	return uploadCandidate{
		reader:   bytes.NewReader(payload),
		fileName: fileName,
		fileType: larkFileType(fileTypeForName(fileName)),
		size:     int64(len(payload)),
		mimeType: mimeType,
		meta:     meta,
	}, nil
}

func attachmentFileName(ctx context.Context, name string) string {
	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return name
	}
	if att, ok := findAttachmentCaseInsensitive(attachments, name); ok {
		if resolved := strings.TrimSpace(att.Name); resolved != "" {
			return resolved
		}
	}
	return name
}

func attachmentMediaType(ctx context.Context, name string) string {
	attachments, _ := tools.GetAttachmentContext(ctx)
	if len(attachments) == 0 {
		return ""
	}
	if att, ok := findAttachmentCaseInsensitive(attachments, name); ok {
		return strings.TrimSpace(att.MediaType)
	}
	return ""
}

func findAttachmentCaseInsensitive(attachments map[string]ports.Attachment, name string) (ports.Attachment, bool) {
	if len(attachments) == 0 || utils.IsBlank(name) {
		return ports.Attachment{}, false
	}
	if att, ok := attachments[name]; ok {
		return att, true
	}
	for key, att := range attachments {
		if strings.EqualFold(key, name) || strings.EqualFold(att.Name, name) {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func parseMaxBytes(args map[string]any) int {
	maxBytes := defaultMaxBytes
	if v, ok := shared.IntArg(args, "max_bytes"); ok && v > 0 {
		maxBytes = v
	}
	return maxBytes
}

func fileTypeForName(fileName string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")
	return strings.TrimSpace(ext)
}

func larkFileType(ext string) string {
	lower := utils.TrimLower(ext)
	lower = strings.TrimPrefix(lower, ".")
	if larkSupportedFileTypes[lower] {
		return lower
	}
	return "stream"
}

func normalizeMimeType(mimeType string) string {
	mimeType = utils.TrimLower(mimeType)
	if mimeType == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(mimeType); err == nil {
		return utils.TrimLower(parsed)
	}
	if idx := strings.Index(mimeType, ";"); idx >= 0 {
		mimeType = mimeType[:idx]
	}
	return utils.TrimLower(mimeType)
}

func isAudioMimeType(mimeType string) bool {
	mimeType = normalizeMimeType(mimeType)
	if strings.HasPrefix(mimeType, "audio/") {
		return true
	}
	return mimeType == "application/ogg"
}

func isAudioFile(fileName, mimeType string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	if larkAudioExtensions[ext] {
		return true
	}
	return isAudioMimeType(mimeType)
}

func isImageFile(fileName, mimeType string) bool {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	if larkImageExtensions[ext] {
		return true
	}
	return strings.HasPrefix(normalizeMimeType(mimeType), "image/")
}

func detectMimeTypeFromFile(file *os.File) string {
	buf := make([]byte, 512)
	n, err := file.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return ""
	}
	if n <= 0 {
		return ""
	}
	return normalizeMimeType(http.DetectContentType(buf[:n]))
}

func fileContent(fileKey string) string {
	payload, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return string(payload)
}

func imageContent(imageKey string) string {
	payload, _ := json.Marshal(map[string]string{"image_key": imageKey})
	return string(payload)
}
