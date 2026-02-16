package lark

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateDocument(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
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
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
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
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
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
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
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
		w.Write(jsonResponse(99991, "permission denied", nil))
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

// docxJSON is a test helper (unused import guard).
var _ = json.Marshal
