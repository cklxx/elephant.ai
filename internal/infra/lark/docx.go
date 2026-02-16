package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

// DocxService provides typed access to Lark Docx APIs (v1).
type DocxService struct {
	client *lark.Client
}

// Document is a simplified view of a Lark document.
type Document struct {
	DocumentID string
	Title      string
	RevisionID int
}

// DocumentBlock is a simplified view of a Lark document block.
type DocumentBlock struct {
	BlockID   string
	BlockType int
	ParentID  string
	Children  []string
}

// CreateDocumentRequest defines parameters for creating a document.
type CreateDocumentRequest struct {
	Title    string // Document title (optional)
	FolderID string // Target folder token (optional, defaults to root)
}

// CreateDocument creates a new empty document and returns its metadata.
func (s *DocxService) CreateDocument(ctx context.Context, req CreateDocumentRequest, opts ...CallOption) (*Document, error) {
	body := &larkdocx.CreateDocumentReqBody{}
	if req.Title != "" {
		body.Title = &req.Title
	}
	if req.FolderID != "" {
		body.FolderToken = &req.FolderID
	}

	createReq := larkdocx.NewCreateDocumentReqBuilder().
		Body(body).
		Build()

	resp, err := s.client.Docx.V1.Document.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create document: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	return parseDocument(resp.Data.Document), nil
}

// GetDocument retrieves document metadata by document ID.
func (s *DocxService) GetDocument(ctx context.Context, documentID string, opts ...CallOption) (*Document, error) {
	getReq := larkdocx.NewGetDocumentReqBuilder().
		DocumentId(documentID).
		Build()

	resp, err := s.client.Docx.V1.Document.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get document: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	return parseDocument(resp.Data.Document), nil
}

// GetDocumentRawContent retrieves the raw text content of a document.
func (s *DocxService) GetDocumentRawContent(ctx context.Context, documentID string, opts ...CallOption) (string, error) {
	rawReq := larkdocx.NewRawContentDocumentReqBuilder().
		DocumentId(documentID).
		Build()

	resp, err := s.client.Docx.V1.Document.RawContent(ctx, rawReq, buildOpts(opts)...)
	if err != nil {
		return "", fmt.Errorf("get document raw content: %w", err)
	}
	if !resp.Success() {
		return "", &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	if resp.Data == nil || resp.Data.Content == nil {
		return "", nil
	}
	return *resp.Data.Content, nil
}

// ListDocumentBlocks lists blocks in a document.
func (s *DocxService) ListDocumentBlocks(ctx context.Context, documentID string, pageSize int, pageToken string, opts ...CallOption) ([]DocumentBlock, string, bool, error) {
	if pageSize <= 0 {
		pageSize = 50
	}

	builder := larkdocx.NewListDocumentBlockReqBuilder().
		DocumentId(documentID).
		PageSize(pageSize)

	if pageToken != "" {
		builder.PageToken(pageToken)
	}

	resp, err := s.client.Docx.V1.DocumentBlock.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, "", false, fmt.Errorf("list document blocks: %w", err)
	}
	if !resp.Success() {
		return nil, "", false, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	blocks := make([]DocumentBlock, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		blocks = append(blocks, parseDocumentBlock(item))
	}

	var nextPageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		nextPageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return blocks, nextPageToken, hasMore, nil
}

// --- helpers ---

func parseDocument(doc *larkdocx.Document) *Document {
	if doc == nil {
		return &Document{}
	}
	d := &Document{}
	if doc.DocumentId != nil {
		d.DocumentID = *doc.DocumentId
	}
	if doc.Title != nil {
		d.Title = *doc.Title
	}
	if doc.RevisionId != nil {
		d.RevisionID = int(*doc.RevisionId)
	}
	return d
}

func parseDocumentBlock(block *larkdocx.Block) DocumentBlock {
	if block == nil {
		return DocumentBlock{}
	}
	b := DocumentBlock{}
	if block.BlockId != nil {
		b.BlockID = *block.BlockId
	}
	if block.BlockType != nil {
		b.BlockType = *block.BlockType
	}
	if block.ParentId != nil {
		b.ParentID = *block.ParentId
	}
	b.Children = block.Children
	return b
}
