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

// UpdateDocumentBlockTextRequest defines parameters for updating a text-like block.
type UpdateDocumentBlockTextRequest struct {
	DocumentID         string // Required document ID.
	BlockID            string // Required block ID.
	Content            string // New text content for the block.
	DocumentRevisionID int    // Optional target revision; defaults to -1 (latest).
	ClientToken        string // Optional idempotency token.
	UserIDType         string // Optional user ID type.
}

// UpdateDocumentBlockTextResult is the simplified patch result for a block update.
type UpdateDocumentBlockTextResult struct {
	Block              DocumentBlock
	DocumentRevisionID int
	ClientToken        string
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
	if resp.Data == nil {
		return nil, fmt.Errorf("create document: unexpected nil data in response")
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
	if resp.Data == nil {
		return nil, fmt.Errorf("get document: unexpected nil data in response")
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
	if resp.Data == nil {
		return nil, "", false, fmt.Errorf("list document blocks: unexpected nil data in response")
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

// UpdateDocumentBlockText updates a text-like block content via update_text_elements.
func (s *DocxService) UpdateDocumentBlockText(ctx context.Context, req UpdateDocumentBlockTextRequest, opts ...CallOption) (*UpdateDocumentBlockTextResult, error) {
	textElement := larkdocx.NewTextElementBuilder().
		TextRun(larkdocx.NewTextRunBuilder().
			Content(req.Content).
			Build()).
		Build()

	updateBody := larkdocx.NewUpdateBlockRequestBuilder().
		UpdateTextElements(larkdocx.NewUpdateTextElementsRequestBuilder().
			Elements([]*larkdocx.TextElement{textElement}).
			Build()).
		Build()

	patchReqBuilder := larkdocx.NewPatchDocumentBlockReqBuilder().
		DocumentId(req.DocumentID).
		BlockId(req.BlockID).
		UpdateBlockRequest(updateBody)

	documentRevisionID := req.DocumentRevisionID
	if documentRevisionID == 0 {
		documentRevisionID = -1
	}
	patchReqBuilder.DocumentRevisionId(documentRevisionID)
	if req.ClientToken != "" {
		patchReqBuilder.ClientToken(req.ClientToken)
	}
	if req.UserIDType != "" {
		patchReqBuilder.UserIdType(req.UserIDType)
	}

	resp, err := s.client.Docx.V1.DocumentBlock.Patch(ctx, patchReqBuilder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("update document block text: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("update document block text: unexpected nil data in response")
	}

	result := &UpdateDocumentBlockTextResult{
		Block: parseDocumentBlock(resp.Data.Block),
	}
	if resp.Data.DocumentRevisionId != nil {
		result.DocumentRevisionID = *resp.Data.DocumentRevisionId
	}
	if resp.Data.ClientToken != nil {
		result.ClientToken = *resp.Data.ClientToken
	}

	return result, nil
}

// ConvertResult holds the result of converting markdown/HTML to document blocks.
type ConvertResult struct {
	FirstLevelBlockIDs []string                // Ordered top-level block IDs.
	Blocks             []*larkdocx.Block       // All blocks with parent-child relationships.
	ImageMappings      []BlockImageMapping     // Block ID → image URL mappings for external images.
}

// BlockImageMapping maps a temporary block ID to its source image URL.
type BlockImageMapping struct {
	BlockID  string
	ImageURL string
}

// ConvertMarkdownToBlocks converts markdown content to document blocks.
func (s *DocxService) ConvertMarkdownToBlocks(ctx context.Context, markdown string, opts ...CallOption) (*ConvertResult, error) {
	contentType := "markdown"
	body := &larkdocx.ConvertDocumentReqBody{
		ContentType: &contentType,
		Content:     &markdown,
	}

	req := larkdocx.NewConvertDocumentReqBuilder().
		Body(body).
		Build()

	resp, err := s.client.Docx.V1.Document.Convert(ctx, req, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("convert markdown to blocks: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("convert markdown to blocks: unexpected nil data in response")
	}

	result := &ConvertResult{
		FirstLevelBlockIDs: resp.Data.FirstLevelBlockIds,
		Blocks:             resp.Data.Blocks,
	}
	for _, m := range resp.Data.BlockIdToImageUrls {
		if m.BlockId != nil && m.ImageUrl != nil {
			result.ImageMappings = append(result.ImageMappings, BlockImageMapping{
				BlockID:  *m.BlockId,
				ImageURL: *m.ImageUrl,
			})
		}
	}
	return result, nil
}

// CreateDescendantBlocksRequest defines parameters for creating descendant blocks.
type CreateDescendantBlocksRequest struct {
	DocumentID  string
	BlockID     string            // Parent block ID (typically the page block).
	ChildrenIDs []string          // IDs of direct children to add under the parent.
	Descendants []*larkdocx.Block // All blocks (children + nested descendants).
	Index       int               // Insertion position among existing children (0 = beginning).
}

// CreateDescendantBlocks creates nested blocks under a parent block in a document.
func (s *DocxService) CreateDescendantBlocks(ctx context.Context, req CreateDescendantBlocksRequest, opts ...CallOption) (int, error) {
	body := larkdocx.NewCreateDocumentBlockDescendantReqBodyBuilder().
		ChildrenId(req.ChildrenIDs).
		Descendants(req.Descendants).
		Index(req.Index).
		Build()

	apiReq := larkdocx.NewCreateDocumentBlockDescendantReqBuilder().
		DocumentId(req.DocumentID).
		BlockId(req.BlockID).
		DocumentRevisionId(-1).
		Body(body).
		Build()

	resp, err := s.client.Docx.V1.DocumentBlockDescendant.Create(ctx, apiReq, buildOpts(opts)...)
	if err != nil {
		return 0, fmt.Errorf("create descendant blocks: %w", err)
	}
	if !resp.Success() {
		return 0, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	var revisionID int
	if resp.Data != nil && resp.Data.DocumentRevisionId != nil {
		revisionID = *resp.Data.DocumentRevisionId
	}
	return revisionID, nil
}

// WriteMarkdown converts markdown to document blocks and inserts them into a document.
// It handles table merge_info stripping and batching (max 1000 blocks per call).
func (s *DocxService) WriteMarkdown(ctx context.Context, documentID, parentBlockID, markdown string, opts ...CallOption) error {
	converted, err := s.ConvertMarkdownToBlocks(ctx, markdown, opts...)
	if err != nil {
		return err
	}
	if len(converted.Blocks) == 0 {
		return nil
	}

	// Strip read-only merge_info from table blocks before insertion.
	for _, block := range converted.Blocks {
		if block.Table != nil && block.Table.Property != nil {
			block.Table.Property.MergeInfo = nil
		}
	}

	// Batch insert: max 1000 blocks per API call.
	const maxBlocksPerBatch = 1000
	if len(converted.Blocks) <= maxBlocksPerBatch {
		_, err = s.CreateDescendantBlocks(ctx, CreateDescendantBlocksRequest{
			DocumentID:  documentID,
			BlockID:     parentBlockID,
			ChildrenIDs: converted.FirstLevelBlockIDs,
			Descendants: converted.Blocks,
			Index:       0,
		}, opts...)
		return err
	}

	// For large documents, split blocks into batches by first-level block boundaries.
	batches := splitBlockBatches(converted.FirstLevelBlockIDs, converted.Blocks, maxBlocksPerBatch)
	insertIndex := 0
	for _, batch := range batches {
		_, err = s.CreateDescendantBlocks(ctx, CreateDescendantBlocksRequest{
			DocumentID:  documentID,
			BlockID:     parentBlockID,
			ChildrenIDs: batch.childrenIDs,
			Descendants: batch.blocks,
			Index:       insertIndex,
		}, opts...)
		if err != nil {
			return fmt.Errorf("batch insert at index %d: %w", insertIndex, err)
		}
		insertIndex += len(batch.childrenIDs)
	}
	return nil
}

// --- helpers ---

type blockBatch struct {
	childrenIDs []string
	blocks      []*larkdocx.Block
}

// splitBlockBatches groups blocks into batches, each containing at most maxBlocks.
// Splits on first-level block boundaries to maintain parent-child integrity.
func splitBlockBatches(firstLevelIDs []string, allBlocks []*larkdocx.Block, maxBlocks int) []blockBatch {
	// Index blocks by parent to find all descendants of each first-level block.
	childrenOf := make(map[string][]*larkdocx.Block)
	blockByID := make(map[string]*larkdocx.Block, len(allBlocks))
	for _, b := range allBlocks {
		if b.BlockId != nil {
			blockByID[*b.BlockId] = b
		}
		if b.ParentId != nil {
			childrenOf[*b.ParentId] = append(childrenOf[*b.ParentId], b)
		}
	}

	// Collect all descendants (BFS) for a given block ID.
	collectDescendants := func(rootID string) []*larkdocx.Block {
		var result []*larkdocx.Block
		if root, ok := blockByID[rootID]; ok {
			result = append(result, root)
		}
		queue := []string{rootID}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, child := range childrenOf[current] {
				result = append(result, child)
				if child.BlockId != nil {
					queue = append(queue, *child.BlockId)
				}
			}
		}
		return result
	}

	var batches []blockBatch
	var currentBatch blockBatch

	for _, flID := range firstLevelIDs {
		descendants := collectDescendants(flID)
		// If adding this block's tree would exceed the limit, flush current batch.
		if len(currentBatch.blocks)+len(descendants) > maxBlocks && len(currentBatch.blocks) > 0 {
			batches = append(batches, currentBatch)
			currentBatch = blockBatch{}
		}
		currentBatch.childrenIDs = append(currentBatch.childrenIDs, flID)
		currentBatch.blocks = append(currentBatch.blocks, descendants...)
	}
	if len(currentBatch.blocks) > 0 {
		batches = append(batches, currentBatch)
	}
	return batches
}

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
