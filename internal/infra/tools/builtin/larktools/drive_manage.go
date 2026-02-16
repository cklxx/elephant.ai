package larktools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	larkapi "alex/internal/infra/lark"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// larkDriveManage handles drive file/folder operations via the unified channel tool.
type larkDriveManage struct{}

func (t *larkDriveManage) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	rawClient := shared.LarkClientFromContext(ctx)
	if rawClient == nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "drive operations require a Lark chat context.",
			Error:   fmt.Errorf("lark client not available in context"),
		}, nil
	}
	sdkClient, ok := rawClient.(*lark.Client)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "invalid lark client type in context.",
			Error:   fmt.Errorf("invalid lark client type: %T", rawClient),
		}, nil
	}
	client := larkapi.Wrap(sdkClient)

	action := strings.ToLower(strings.TrimSpace(shared.StringArg(call.Arguments, "action")))
	switch action {
	case "list_files":
		return t.listFiles(ctx, client, call)
	case "create_folder":
		return t.createFolder(ctx, client, call)
	case "copy_file":
		return t.copyFile(ctx, client, call)
	case "delete_file":
		return t.deleteFile(ctx, client, call)
	default:
		err := fmt.Errorf("unsupported drive action: %s", action)
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
}

func (t *larkDriveManage) listFiles(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	folderToken := shared.StringArg(call.Arguments, "folder_token")
	if folderToken == "" {
		folderToken = "root"
	}
	pageSize, _ := shared.IntArg(call.Arguments, "page_size")
	pageToken := shared.StringArg(call.Arguments, "page_token")

	resp, err := client.Drive().ListFiles(ctx, larkapi.ListFilesRequest{
		FolderToken: folderToken,
		PageSize:    pageSize,
		PageToken:   pageToken,
	})
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to list files: %v", err),
			Error:   err,
		}, nil
	}

	if len(resp.Files) == 0 {
		return &ports.ToolResult{CallID: call.ID, Content: "No files found."}, nil
	}

	payload, _ := json.MarshalIndent(resp.Files, "", "  ")
	metadata := map[string]any{"file_count": len(resp.Files)}
	if resp.HasMore {
		metadata["has_more"] = true
		metadata["page_token"] = resp.PageToken
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  fmt.Sprintf("Found %d files:\n%s", len(resp.Files), string(payload)),
		Metadata: metadata,
	}, nil
}

func (t *larkDriveManage) createFolder(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	folderToken := shared.StringArg(call.Arguments, "folder_token")
	if folderToken == "" {
		folderToken = "root"
	}
	name, errResult := shared.RequireStringArg(call.Arguments, call.ID, "name")
	if errResult != nil {
		return errResult, nil
	}

	folder, err := client.Drive().CreateFolder(ctx, folderToken, name)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to create folder: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "Folder created successfully.",
		Metadata: map[string]any{
			"token": folder.Token,
			"url":   folder.URL,
		},
	}, nil
}

func (t *larkDriveManage) copyFile(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	fileToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "file_token")
	if errResult != nil {
		return errResult, nil
	}
	targetFolder, errResult := shared.RequireStringArg(call.Arguments, call.ID, "folder_token")
	if errResult != nil {
		return errResult, nil
	}
	newName, errResult := shared.RequireStringArg(call.Arguments, call.ID, "name")
	if errResult != nil {
		return errResult, nil
	}

	fileType := shared.StringArg(call.Arguments, "file_type")
	if fileType == "" {
		fileType = "file"
	}

	copied, err := client.Drive().CopyFile(ctx, fileToken, targetFolder, newName, fileType)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to copy file: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "File copied successfully.",
		Metadata: map[string]any{
			"token": copied.Token,
			"url":   copied.URL,
		},
	}, nil
}

func (t *larkDriveManage) deleteFile(ctx context.Context, client *larkapi.Client, call ports.ToolCall) (*ports.ToolResult, error) {
	fileToken, errResult := shared.RequireStringArg(call.Arguments, call.ID, "file_token")
	if errResult != nil {
		return errResult, nil
	}

	fileType := shared.StringArg(call.Arguments, "file_type")
	if fileType == "" {
		fileType = "file"
	}

	err := client.Drive().DeleteFile(ctx, fileToken, fileType)
	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Failed to delete file: %v", err),
			Error:   err,
		}, nil
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: "File deleted successfully.",
		Metadata: map[string]any{
			"file_token": fileToken,
		},
	}, nil
}
