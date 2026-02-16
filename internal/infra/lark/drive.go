package lark

import (
	"context"
	"fmt"
	"io"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdrive "github.com/larksuite/oapi-sdk-go/v3/service/drive/v1"
)

// DriveService provides typed access to Lark Drive APIs (v1).
type DriveService struct {
	client *lark.Client
}

// DriveFile is a simplified view of a Lark Drive file.
type DriveFile struct {
	Token     string
	Name      string
	Type      string // doc | sheet | bitable | folder | file | docx | slides
	ParentID  string
	URL       string
	CreatedAt int64
	UpdatedAt int64
}

// --- Folder / File listing ---

// ListFilesRequest defines parameters for listing files in a folder.
type ListFilesRequest struct {
	FolderToken string // Use "root" for root folder
	PageSize    int
	PageToken   string
	OrderBy     string // optional: "EditedTime" or "CreatedTime"
}

// ListFilesResponse contains paginated files.
type ListFilesResponse struct {
	Files     []DriveFile
	PageToken string
	HasMore   bool
}

// ListFiles lists files and folders in a specified folder.
func (s *DriveService) ListFiles(ctx context.Context, req ListFilesRequest, opts ...CallOption) (*ListFilesResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larkdrive.NewListFileReqBuilder().
		PageSize(pageSize)

	if req.FolderToken != "" {
		builder.FolderToken(req.FolderToken)
	}
	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}
	if req.OrderBy != "" {
		builder.OrderBy(req.OrderBy)
	}

	resp, err := s.client.Drive.V1.File.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list drive files: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	files := make([]DriveFile, 0, len(resp.Data.Files))
	for _, item := range resp.Data.Files {
		files = append(files, parseDriveFile(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.NextPageToken != nil {
		pageToken = *resp.Data.NextPageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListFilesResponse{
		Files:     files,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// --- Folder creation ---

// CreateFolder creates a new folder under the specified parent.
func (s *DriveService) CreateFolder(ctx context.Context, folderToken, name string, opts ...CallOption) (*DriveFile, error) {
	body := larkdrive.NewCreateFolderFileReqBodyBuilder().
		Name(name).
		FolderToken(folderToken).
		Build()

	createReq := larkdrive.NewCreateFolderFileReqBuilder().
		Body(body).
		Build()

	resp, err := s.client.Drive.V1.File.CreateFolder(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create drive folder: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	result := &DriveFile{Name: name, Type: "folder"}
	if resp.Data.Token != nil {
		result.Token = *resp.Data.Token
	}
	if resp.Data.Url != nil {
		result.URL = *resp.Data.Url
	}
	return result, nil
}

// --- File operations ---

// CopyFile copies a file to a target folder.
func (s *DriveService) CopyFile(ctx context.Context, fileToken, targetFolder, newName, fileType string, opts ...CallOption) (*DriveFile, error) {
	body := larkdrive.NewCopyFileReqBodyBuilder().
		Name(newName).
		FolderToken(targetFolder).
		Type(fileType).
		Build()

	copyReq := larkdrive.NewCopyFileReqBuilder().
		FileToken(fileToken).
		Body(body).
		Build()

	resp, err := s.client.Drive.V1.File.Copy(ctx, copyReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("copy drive file: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	result := &DriveFile{Name: newName, Type: fileType}
	if resp.Data.File != nil {
		if resp.Data.File.Token != nil {
			result.Token = *resp.Data.File.Token
		}
		if resp.Data.File.Url != nil {
			result.URL = *resp.Data.File.Url
		}
	}
	return result, nil
}

// MoveFile moves a file to a different folder.
func (s *DriveService) MoveFile(ctx context.Context, fileToken, targetFolder, fileType string, opts ...CallOption) error {
	body := larkdrive.NewMoveFileReqBodyBuilder().
		FolderToken(targetFolder).
		Type(fileType).
		Build()

	moveReq := larkdrive.NewMoveFileReqBuilder().
		FileToken(fileToken).
		Body(body).
		Build()

	resp, err := s.client.Drive.V1.File.Move(ctx, moveReq, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("move drive file: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// DeleteFile deletes a file or folder.
func (s *DriveService) DeleteFile(ctx context.Context, fileToken, fileType string, opts ...CallOption) error {
	deleteReq := larkdrive.NewDeleteFileReqBuilder().
		FileToken(fileToken).
		Type(fileType).
		Build()

	resp, err := s.client.Drive.V1.File.Delete(ctx, deleteReq, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("delete drive file: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// --- Upload ---

// UploadFileRequest defines parameters for uploading a file.
type UploadFileRequest struct {
	FileName    string
	ParentType  string // explorer (Drive folder)
	ParentToken string // folder token
	Size        int
	Body        io.Reader
}

// UploadFile uploads a file to Drive.
func (s *DriveService) UploadFile(ctx context.Context, req UploadFileRequest, opts ...CallOption) (string, error) {
	uploadReq := larkdrive.NewUploadAllMediaReqBuilder().
		Body(larkdrive.NewUploadAllMediaReqBodyBuilder().
			FileName(req.FileName).
			ParentType(req.ParentType).
			ParentNode(req.ParentToken).
			Size(req.Size).
			File(req.Body).
			Build()).
		Build()

	resp, err := s.client.Drive.V1.Media.UploadAll(ctx, uploadReq, buildOpts(opts)...)
	if err != nil {
		return "", fmt.Errorf("upload drive file: %w", err)
	}
	if !resp.Success() {
		return "", &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	if resp.Data.FileToken != nil {
		return *resp.Data.FileToken, nil
	}
	return "", nil
}

// --- Download ---

// DownloadFile downloads a file from Drive and returns the reader.
func (s *DriveService) DownloadFile(ctx context.Context, fileToken string, opts ...CallOption) (io.Reader, string, error) {
	downloadReq := larkdrive.NewDownloadMediaReqBuilder().
		FileToken(fileToken).
		Build()

	resp, err := s.client.Drive.V1.Media.Download(ctx, downloadReq, buildOpts(opts)...)
	if err != nil {
		return nil, "", fmt.Errorf("download drive file: %w", err)
	}
	if !resp.Success() {
		return nil, "", &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	return resp.File, resp.FileName, nil
}

// --- helpers ---

func parseDriveFile(file *larkdrive.File) DriveFile {
	if file == nil {
		return DriveFile{}
	}
	f := DriveFile{}
	if file.Token != nil {
		f.Token = *file.Token
	}
	if file.Name != nil {
		f.Name = *file.Name
	}
	if file.Type != nil {
		f.Type = *file.Type
	}
	if file.ParentToken != nil {
		f.ParentID = *file.ParentToken
	}
	if file.Url != nil {
		f.URL = *file.Url
	}
	if file.CreatedTime != nil {
		f.CreatedAt = parseTimestampInt(*file.CreatedTime)
	}
	if file.ModifiedTime != nil {
		f.UpdatedAt = parseTimestampInt(*file.ModifiedTime)
	}
	return f
}

func parseTimestampInt(ts string) int64 {
	if ts == "" || ts == "0" {
		return 0
	}
	var result int64
	fmt.Sscanf(ts, "%d", &result)
	return result
}
