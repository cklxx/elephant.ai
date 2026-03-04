package lark

import (
	"context"
	"net/http"
	"strings"
	"testing"

	larkdocx "github.com/larksuite/oapi-sdk-go/v3/service/docx/v1"
)

func TestCreateDocument(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"document": map[string]interface{}{
				"document_id": "doc_abc123",
				"title":       "Test Document",
				"revision_id": 1,
			},
		}))
	})
	defer srv.Close()

	doc, err := client.Docx().CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "Test Document",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocumentID != "doc_abc123" {
		t.Errorf("expected doc_abc123, got %s", doc.DocumentID)
	}
	if doc.Title != "Test Document" {
		t.Errorf("expected 'Test Document', got %s", doc.Title)
	}
}

func TestGetDocument(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"document": map[string]interface{}{
				"document_id": "doc_xyz",
				"title":       "My Doc",
				"revision_id": 5,
			},
		}))
	})
	defer srv.Close()

	doc, err := client.Docx().GetDocument(context.Background(), "doc_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.DocumentID != "doc_xyz" {
		t.Errorf("expected doc_xyz, got %s", doc.DocumentID)
	}
	if doc.RevisionID != 5 {
		t.Errorf("expected revision 5, got %d", doc.RevisionID)
	}
}

func TestGetDocumentRawContent(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"content": "Hello, world!",
		}))
	})
	defer srv.Close()

	content, err := client.Docx().GetDocumentRawContent(context.Background(), "doc_xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %s", content)
	}
}

func TestListDocumentBlocks(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hasMore := false
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"block_id": "blk_1", "block_type": 1, "parent_id": "doc_root"},
				{"block_id": "blk_2", "block_type": 2, "parent_id": "blk_1"},
			},
			"has_more": hasMore,
		}))
	})
	defer srv.Close()

	blocks, _, hasMore, err := client.Docx().ListDocumentBlocks(context.Background(), "doc_xyz", 10, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(blocks))
	}
	if blocks[0].BlockID != "blk_1" {
		t.Errorf("expected blk_1, got %s", blocks[0].BlockID)
	}
	if hasMore {
		t.Error("expected hasMore=false")
	}
}

func TestCreateDocumentAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "permission denied", nil))
	})
	defer srv.Close()

	_, err := client.Docx().CreateDocument(context.Background(), CreateDocumentRequest{
		Title: "No Permission",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != 99991 {
		t.Errorf("expected code 99991, got %d", apiErr.Code)
	}
}

func TestUpdateDocumentBlockText(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/docx/v1/documents/doc_xyz/blocks/blk_123") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("document_revision_id"); got != "-1" {
			t.Fatalf("expected default document_revision_id=-1, got %q", got)
		}
		body := string(readBody(r))
		if !strings.Contains(body, `"update_text_elements"`) || !strings.Contains(body, `"updated block text"`) {
			t.Fatalf("unexpected patch body: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"block": map[string]interface{}{
				"block_id":   "blk_123",
				"block_type": 2,
				"parent_id":  "doc_xyz",
				"text": map[string]interface{}{
					"elements": []map[string]interface{}{
						{
							"text_run": map[string]interface{}{
								"content": "updated block text",
							},
						},
					},
				},
			},
			"document_revision_id": 117,
			"client_token":         "ctok_123",
		}))
	})
	defer srv.Close()

	result, err := client.Docx().UpdateDocumentBlockText(context.Background(), UpdateDocumentBlockTextRequest{
		DocumentID: "doc_xyz",
		BlockID:    "blk_123",
		Content:    "updated block text",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Block.BlockID != "blk_123" {
		t.Fatalf("expected block_id=blk_123, got %s", result.Block.BlockID)
	}
	if result.DocumentRevisionID != 117 {
		t.Fatalf("expected revision=117, got %d", result.DocumentRevisionID)
	}
	if result.ClientToken != "ctok_123" {
		t.Fatalf("expected client token ctok_123, got %s", result.ClientToken)
	}
}

func TestConvertMarkdownToBlocks(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/docx/v1/documents/blocks/convert") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body := string(readBody(r))
		if !strings.Contains(body, `"content_type":"markdown"`) {
			t.Fatalf("expected content_type=markdown in body: %s", body)
		}

		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"first_level_block_ids": []string{"tmp_blk_1", "tmp_blk_2"},
			"blocks": []map[string]interface{}{
				{"block_id": "tmp_blk_1", "block_type": 3, "parent_id": "root"},
				{"block_id": "tmp_blk_2", "block_type": 2, "parent_id": "root"},
			},
			"block_id_to_image_urls": []map[string]interface{}{
				{"block_id": "tmp_img_1", "image_url": "https://example.com/img.png"},
			},
		}))
	})
	defer srv.Close()

	result, err := client.Docx().ConvertMarkdownToBlocks(context.Background(), "# Hello\nWorld")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.FirstLevelBlockIDs) != 2 {
		t.Fatalf("expected 2 first-level IDs, got %d", len(result.FirstLevelBlockIDs))
	}
	if len(result.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result.Blocks))
	}
	if len(result.ImageMappings) != 1 {
		t.Fatalf("expected 1 image mapping, got %d", len(result.ImageMappings))
	}
	if result.ImageMappings[0].BlockID != "tmp_img_1" {
		t.Errorf("expected image block_id=tmp_img_1, got %s", result.ImageMappings[0].BlockID)
	}
}

func TestConvertMarkdownToBlocksAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99001, "invalid content", nil))
	})
	defer srv.Close()

	_, err := client.Docx().ConvertMarkdownToBlocks(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != 99001 {
		t.Errorf("expected code 99001, got %d", apiErr.Code)
	}
}

func TestSplitBlockBatches(t *testing.T) {
	makeBlock := func(id, parentID string) *larkdocx.Block {
		b := &larkdocx.Block{}
		b.BlockId = &id
		b.ParentId = &parentID
		return b
	}

	t.Run("single batch", func(t *testing.T) {
		blocks := []*larkdocx.Block{
			makeBlock("a", "root"),
			makeBlock("b", "root"),
		}
		batches := splitBlockBatches([]string{"a", "b"}, blocks, 1000)
		if len(batches) != 1 {
			t.Fatalf("expected 1 batch, got %d", len(batches))
		}
		if len(batches[0].childrenIDs) != 2 {
			t.Errorf("expected 2 children, got %d", len(batches[0].childrenIDs))
		}
	})

	t.Run("split on boundary", func(t *testing.T) {
		// 3 first-level blocks each with 1 child = 6 total blocks
		// max 4 per batch → should split into [a+child, b+child] and [c+child]
		blocks := []*larkdocx.Block{
			makeBlock("a", "root"),
			makeBlock("a1", "a"),
			makeBlock("b", "root"),
			makeBlock("b1", "b"),
			makeBlock("c", "root"),
			makeBlock("c1", "c"),
		}
		batches := splitBlockBatches([]string{"a", "b", "c"}, blocks, 4)
		if len(batches) != 2 {
			t.Fatalf("expected 2 batches, got %d", len(batches))
		}
		if len(batches[0].childrenIDs) != 2 || batches[0].childrenIDs[0] != "a" {
			t.Errorf("first batch children: %v", batches[0].childrenIDs)
		}
		if len(batches[1].childrenIDs) != 1 || batches[1].childrenIDs[0] != "c" {
			t.Errorf("second batch children: %v", batches[1].childrenIDs)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		batches := splitBlockBatches(nil, nil, 1000)
		if len(batches) != 0 {
			t.Errorf("expected 0 batches, got %d", len(batches))
		}
	})
}

func TestUpdateDocumentBlockTextAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(1770001, "invalid param", nil))
	})
	defer srv.Close()

	_, err := client.Docx().UpdateDocumentBlockText(context.Background(), UpdateDocumentBlockTextRequest{
		DocumentID: "doc_xyz",
		BlockID:    "blk_123",
		Content:    "x",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Code != 1770001 {
		t.Fatalf("expected code 1770001, got %d", apiErr.Code)
	}
}
