package larktools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/tools/builtin/artifacts"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"

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

type uploadCandidate struct {
	reader   io.Reader
	cleanup  func()
	fileName string
	fileType string
	size     int64
	meta     map[string]any
}

// NewLarkUploadFile constructs a tool that uploads a file and sends it to the
// current Lark chat as a "file" message.
func NewLarkUploadFile() tools.ToolExecutor {
	return &larkUploadFile{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "lark_upload_file",
				Description: "Upload a file (from local path or task attachment) and send it to the current Lark chat as a file message. Only available inside a Lark chat context.",
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"path": {
							Type:        "string",
							Description: "Local file path (must stay within the working directory). Provide exactly one of path or attachment_name.",
						},
						"attachment_name": {
							Type:        "string",
							Description: "Attachment name from the current task context. Provide exactly one of path or attachment_name.",
						},
						"file_name": {
							Type:        "string",
							Description: "Optional override for the uploaded file name.",
						},
						"reply_to_message_id": {
							Type:        "string",
							Description: "Optional message ID to reply to (threaded reply).",
						},
						"max_bytes": {
							Type:        "integer",
							Description: "Maximum upload size in bytes (default 20MiB).",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "lark_upload_file",
				Version:  "0.1.0",
				Category: "lark",
				Tags:     []string{"lark", "chat", "upload", "file"},
			},
		),
	}
}

func (t *larkUploadFile) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_upload_file is only available inside a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	client, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_upload_file: invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}

	chatID := shared.LarkChatIDFromContext(ctx)
	if chatID == "" {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "lark_upload_file: no chat_id available in context.",
			Error:   fmt.Errorf("chat_id not available in context"),
		}, nil
	}

	maxBytes := parseMaxBytes(call.Arguments)
	candidate, errResult := prepareUploadCandidate(ctx, call.ID, call.Arguments, maxBytes)
	if errResult != nil {
		return errResult, nil
	}
	if candidate.cleanup != nil {
		defer candidate.cleanup()
	}

	uploadReq := larkim.NewCreateFileReqBuilder().
		Body(larkim.NewCreateFileReqBodyBuilder().
			FileType(candidate.fileType).
			FileName(candidate.fileName).
			File(candidate.reader).
			Build()).
		Build()

	uploadResp, err := client.Im.V1.File.Create(ctx, uploadReq)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_upload_file: upload API call failed: %v", err),
			Error:   fmt.Errorf("lark upload API call failed: %w", err),
		}, nil
	}
	if !uploadResp.Success() {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("lark_upload_file: upload API error code=%d msg=%s", uploadResp.Code, uploadResp.Msg),
			Error:   fmt.Errorf("lark upload API error: code=%d msg=%s", uploadResp.Code, uploadResp.Msg),
		}, nil
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

	replyToID := strings.TrimSpace(shared.StringArg(call.Arguments, "reply_to_message_id"))
	msgContent := fileContent(fileKey)

	messageID, errResult := sendFileMessage(ctx, client, call.ID, chatID, replyToID, msgContent)
	if errResult != nil {
		return errResult, nil
	}

	metadata := map[string]any{
		"chat_id":    chatID,
		"message_id": messageID,
		"file_key":   fileKey,
		"file_name":  candidate.fileName,
		"file_type":  candidate.fileType,
		"bytes":      candidate.size,
		"max_bytes":  maxBytes,
	}
	for k, v := range candidate.meta {
		metadata[k] = v
	}
	if replyToID != "" {
		metadata["reply_to_message_id"] = replyToID
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "File sent successfully.",
		Metadata: metadata,
	}, nil
}

func sendFileMessage(ctx context.Context, client *lark.Client, callID, chatID, replyToID, content string) (string, *ports.ToolResult) {
	if replyToID != "" {
		req := larkim.NewReplyMessageReqBuilder().
			MessageId(replyToID).
			Body(larkim.NewReplyMessageReqBodyBuilder().
				MsgType("file").
				Content(content).
				Build()).
			Build()
		resp, err := client.Im.Message.Reply(ctx, req)
		if err != nil {
			err := fmt.Errorf("lark reply message API call failed: %w", err)
			return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
		}
		if !resp.Success() {
			err := fmt.Errorf("lark reply message API error: code=%d msg=%s", resp.Code, resp.Msg)
			return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
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
			MsgType("file").
			Content(content).
			Build()).
		Build()
	resp, err := client.Im.Message.Create(ctx, req)
	if err != nil {
		err := fmt.Errorf("lark send message API call failed: %w", err)
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if !resp.Success() {
		err := fmt.Errorf("lark send message API error: code=%d msg=%s", resp.Code, resp.Msg)
		return "", &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if resp.Data != nil && resp.Data.MessageId != nil {
		return *resp.Data.MessageId, nil
	}
	return "", nil
}

func prepareUploadCandidate(ctx context.Context, callID string, args map[string]any, maxBytes int) (uploadCandidate, *ports.ToolResult) {
	path := strings.TrimSpace(shared.StringArg(args, "path"))
	attachmentName := strings.TrimSpace(shared.StringArg(args, "attachment_name"))

	if path == "" && attachmentName == "" {
		err := fmt.Errorf("either 'path' or 'attachment_name' is required")
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if path != "" && attachmentName != "" {
		err := fmt.Errorf("provide exactly one of 'path' or 'attachment_name'")
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	fileNameOverride := strings.TrimSpace(shared.StringArg(args, "file_name"))
	if _, ok := args["file_name"]; ok && fileNameOverride == "" {
		err := fmt.Errorf("file_name cannot be empty")
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}

	if path != "" {
		return preparePathCandidate(ctx, callID, path, fileNameOverride, maxBytes)
	}
	return prepareAttachmentCandidate(ctx, callID, attachmentName, fileNameOverride, maxBytes)
}

func preparePathCandidate(ctx context.Context, callID, rawPath, fileNameOverride string, maxBytes int) (uploadCandidate, *ports.ToolResult) {
	resolved, err := pathutil.ResolveLocalPath(ctx, rawPath)
	if err != nil {
		err := fmt.Errorf("%s: %w", rawPath, err)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		err := fmt.Errorf("%s: %w", rawPath, err)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if info.IsDir() {
		err := fmt.Errorf("%s: path is a directory", rawPath)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if maxBytes > 0 && info.Size() > int64(maxBytes) {
		err := fmt.Errorf("%s: exceeds max size %d bytes", rawPath, maxBytes)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	file, err := os.Open(resolved)
	if err != nil {
		err := fmt.Errorf("%s: %w", rawPath, err)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	fileName := strings.TrimSpace(fileNameOverride)
	if fileName == "" {
		fileName = filepath.Base(resolved)
	}
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" {
		_ = file.Close()
		err := fmt.Errorf("%s: file name missing", rawPath)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
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
		err := fmt.Errorf("%s: %w", attachmentName, err)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}
	if maxBytes > 0 && len(payload) > maxBytes {
		err := fmt.Errorf("%s: exceeds max size %d bytes", attachmentName, maxBytes)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	fileName := strings.TrimSpace(fileNameOverride)
	if fileName == "" {
		fileName = attachmentFileName(ctx, attachmentName)
	}
	fileName = strings.TrimSpace(filepath.Base(fileName))
	if fileName == "" {
		err := fmt.Errorf("%s: file name missing", attachmentName)
		return uploadCandidate{}, &ports.ToolResult{CallID: callID, Content: err.Error(), Error: err}
	}

	meta := map[string]any{
		"attachment_name": attachmentName,
	}
	return uploadCandidate{
		reader:   bytes.NewReader(payload),
		fileName: fileName,
		fileType: larkFileType(fileTypeForName(fileName)),
		size:     int64(len(payload)),
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

func findAttachmentCaseInsensitive(attachments map[string]ports.Attachment, name string) (ports.Attachment, bool) {
	if len(attachments) == 0 || strings.TrimSpace(name) == "" {
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
	lower := strings.ToLower(strings.TrimSpace(ext))
	lower = strings.TrimPrefix(lower, ".")
	if larkSupportedFileTypes[lower] {
		return lower
	}
	return "stream"
}

func fileContent(fileKey string) string {
	payload, _ := json.Marshal(map[string]string{"file_key": fileKey})
	return string(payload)
}
