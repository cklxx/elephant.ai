package larktools

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

// larkTestServer creates a test HTTP server that handles Lark SDK auth and
// dispatches other requests to the provided handler. Returns the server and
// a context pre-loaded with the Lark client.
func larkTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, context.Context) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tenant_access_token") ||
			strings.Contains(r.URL.Path, "app_access_token") {
			w.Header().Set("Content-Type", "application/json")
			resp, _ := json.Marshal(map[string]any{
				"code":                0,
				"msg":                 "ok",
				"tenant_access_token": "test-token",
				"app_access_token":    "test-token",
				"expire":              7200,
			})
			if _, err := w.Write(resp); err != nil {
				t.Fatalf("failed to write auth response: %v", err)
			}
			return
		}
		handler(w, r)
	}))

	client := lark.NewClient("test-app-id", "test-app-secret",
		lark.WithOpenBaseUrl(srv.URL),
	)
	ctx := shared.WithLarkClient(context.Background(), client)
	ctx = shared.WithLarkBaseDomain(ctx, srv.URL)
	return srv, ctx
}

func writeJSON(t *testing.T, w http.ResponseWriter, code int, msg string, data any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]any{"code": code, "msg": msg}
	if data != nil {
		resp["data"] = data
	}
	b, _ := json.Marshal(resp)
	if _, err := w.Write(b); err != nil {
		t.Fatalf("failed to write response: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Direct docx_manage tests
// ---------------------------------------------------------------------------

func TestDocxManage_NoLarkClient(t *testing.T) {
	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "t1", Arguments: map[string]any{"action": "read"}}
	result, err := dm.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when lark client is missing")
	}
}

func TestDocxManage_InvalidAction(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API for invalid action")
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "t2", Arguments: map[string]any{"action": "unknown"}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported action")
	}
	if !strings.Contains(result.Content, "unsupported docx action") {
		t.Fatalf("unexpected error message: %s", result.Content)
	}
}

func TestDocxManage_CreateDoc(t *testing.T) {
	var createCalled bool
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/docx/v1/documents") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)
		if !strings.Contains(body, `"title":"My New Doc"`) {
			t.Fatalf("expected title in create body, got: %s", body)
		}
		createCalled = true
		writeJSON(t, w, 0, "ok", map[string]any{
			"document": map[string]any{
				"document_id": "doc_create_001",
				"title":       "My New Doc",
				"revision_id": 1,
			},
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "c1", Arguments: map[string]any{
		"action": "create",
		"title":  "My New Doc",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !createCalled {
		t.Fatal("expected create document API call")
	}
	if !strings.Contains(result.Content, "doc_create_001") {
		t.Fatalf("expected document_id in content, got: %s", result.Content)
	}
	if result.Metadata["document_id"] != "doc_create_001" {
		t.Fatalf("expected metadata document_id=doc_create_001, got %v", result.Metadata["document_id"])
	}
	urlVal, ok := result.Metadata["url"].(string)
	if !ok || urlVal == "" {
		t.Fatal("expected non-empty url in metadata")
	}
	if !strings.Contains(urlVal, "/docx/doc_create_001") {
		t.Fatalf("expected url to contain /docx/doc_create_001, got %s", urlVal)
	}
	if !strings.Contains(result.Content, "URL:") {
		t.Fatal("expected URL in content")
	}
}

func TestDocxManage_CreateDoc_WithInitialContent(t *testing.T) {
	var convertCalled bool
	var createDescCalled bool
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/docx/v1/documents"):
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_init_001",
					"title":       "设计说明",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/docx/v1/documents/blocks/convert"):
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "这是正文第一段") {
				t.Fatalf("expected initial content in convert body, got: %s", body)
			}
			convertCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"first_level_block_ids": []string{"tmp_blk_001"},
				"blocks": []map[string]any{
					{"block_id": "tmp_blk_001", "block_type": 2, "parent_id": "doc_init_001"},
				},
			})
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/docx/v1/documents/doc_init_001/blocks/doc_init_001/descendant"):
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "tmp_blk_001") {
				t.Fatalf("expected converted block id in create-descendant body, got: %s", body)
			}
			createDescCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document_revision_id": 3,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "c1b", Arguments: map[string]any{
		"action":  "create",
		"title":   "设计说明",
		"content": "这是正文第一段",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !convertCalled {
		t.Fatal("expected markdown convert call")
	}
	if !createDescCalled {
		t.Fatal("expected create descendant blocks call")
	}
	if result.Metadata["content_written"] != true {
		t.Fatalf("expected content_written=true, got %v", result.Metadata["content_written"])
	}
}

func TestDocxManage_CreateDoc_WithFolder(t *testing.T) {
	var gotPath string
	var gotBody string
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		bodyBytes, _ := io.ReadAll(r.Body)
		gotBody = string(bodyBytes)
		writeJSON(t, w, 0, "ok", map[string]any{
			"document": map[string]any{
				"document_id": "doc_folder_001",
				"title":       "Folder Doc",
				"revision_id": 1,
			},
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "c2", Arguments: map[string]any{
		"action":       "create",
		"title":        "Folder Doc",
		"folder_token": "fldr_abc",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.HasSuffix(gotPath, "/docx/v1/documents") {
		t.Fatalf("unexpected path for create doc with folder: %s", gotPath)
	}
	if !strings.Contains(gotBody, `"folder_token":"fldr_abc"`) {
		t.Fatalf("expected folder_token in request body, got: %s", gotBody)
	}
	if result.Metadata["title"] != "Folder Doc" {
		t.Fatalf("expected title=Folder Doc, got %v", result.Metadata["title"])
	}
}

func TestDocxManage_CreateDoc_APIError(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 99991, "permission denied", nil)
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "c3", Arguments: map[string]any{
		"action": "create",
		"title":  "No Permission",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for API failure")
	}
	if !strings.Contains(result.Content, "Failed to create document") {
		t.Fatalf("expected failure message, got: %s", result.Content)
	}
}

func TestDocxManage_ReadDoc(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "doc_read_001") {
			t.Fatalf("expected doc_read_001 in path, got %s", r.URL.Path)
		}
		writeJSON(t, w, 0, "ok", map[string]any{
			"document": map[string]any{
				"document_id": "doc_read_001",
				"title":       "Read Me",
				"revision_id": 42,
			},
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "r1", Arguments: map[string]any{
		"action":      "read",
		"document_id": "doc_read_001",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Read Me") {
		t.Fatalf("expected title in content, got: %s", result.Content)
	}
	if result.Metadata["document_id"] != "doc_read_001" {
		t.Fatalf("expected metadata document_id, got %v", result.Metadata["document_id"])
	}
	urlVal, ok := result.Metadata["url"].(string)
	if !ok || urlVal == "" {
		t.Fatal("expected non-empty url in read metadata")
	}
	if !strings.Contains(urlVal, "/docx/doc_read_001") {
		t.Fatalf("expected url to contain /docx/doc_read_001, got %s", urlVal)
	}
}

func TestDocxManage_ReadDoc_MissingID(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API when document_id is missing")
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "r2", Arguments: map[string]any{
		"action": "read",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing document_id")
	}
}

func TestDocxManage_ReadDoc_APIError(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 91001, "document not found", nil)
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "r3", Arguments: map[string]any{
		"action":      "read",
		"document_id": "doc_nonexistent",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for API failure")
	}
}

func TestDocxManage_ReadContent(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 0, "ok", map[string]any{
			"content": "Hello, this is the document content.\nSecond line.",
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "rc1", Arguments: map[string]any{
		"action":      "read_content",
		"document_id": "doc_content_001",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Hello, this is the document content.") {
		t.Fatalf("expected document content, got: %s", result.Content)
	}
	if result.Metadata["content_length"].(int) == 0 {
		t.Fatal("expected non-zero content_length")
	}
}

func TestDocxManage_ReadContent_Empty(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 0, "ok", map[string]any{
			"content": "",
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "rc2", Arguments: map[string]any{
		"action":      "read_content",
		"document_id": "doc_empty",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Content != "(empty document)" {
		t.Fatalf("expected '(empty document)', got: %s", result.Content)
	}
}

func TestDocxManage_ReadContent_MissingID(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API")
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "rc3", Arguments: map[string]any{
		"action": "read_content",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing document_id")
	}
}

func TestDocxManage_ListBlocks(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hasMore := false
		writeJSON(t, w, 0, "ok", map[string]any{
			"items": []map[string]any{
				{"block_id": "blk_1", "block_type": 1, "parent_id": "doc_root"},
				{"block_id": "blk_2", "block_type": 2, "parent_id": "blk_1", "children": []string{"blk_3"}},
				{"block_id": "blk_3", "block_type": 3, "parent_id": "blk_2"},
			},
			"has_more": hasMore,
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "lb1", Arguments: map[string]any{
		"action":      "list_blocks",
		"document_id": "doc_blocks_001",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "3 blocks") {
		t.Fatalf("expected '3 blocks' in content, got: %s", result.Content)
	}
	if result.Metadata["block_count"] != 3 {
		t.Fatalf("expected block_count=3, got %v", result.Metadata["block_count"])
	}
	if result.Metadata["has_more"] != nil {
		t.Fatalf("expected no has_more when false, got %v", result.Metadata["has_more"])
	}
}

func TestDocxManage_ListBlocks_Paginated(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hasMore := true
		nextToken := "page2_token"
		writeJSON(t, w, 0, "ok", map[string]any{
			"items": []map[string]any{
				{"block_id": "blk_a", "block_type": 1},
			},
			"has_more":   hasMore,
			"page_token": nextToken,
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "lb2", Arguments: map[string]any{
		"action":      "list_blocks",
		"document_id": "doc_paginated",
		"page_size":   1,
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Metadata["has_more"] != true {
		t.Fatalf("expected has_more=true, got %v", result.Metadata["has_more"])
	}
	if result.Metadata["page_token"] != "page2_token" {
		t.Fatalf("expected page_token=page2_token, got %v", result.Metadata["page_token"])
	}
}

func TestDocxManage_ListBlocks_Empty(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hasMore := false
		writeJSON(t, w, 0, "ok", map[string]any{
			"items":    []map[string]any{},
			"has_more": hasMore,
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "lb3", Arguments: map[string]any{
		"action":      "list_blocks",
		"document_id": "doc_empty_blocks",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Content != "No blocks found in document." {
		t.Fatalf("expected empty message, got: %s", result.Content)
	}
}

func TestDocxManage_ListBlocks_MissingID(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API")
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "lb4", Arguments: map[string]any{
		"action": "list_blocks",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for missing document_id")
	}
}

func TestDocxManage_ListBlocks_APIError(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 91002, "access denied", nil)
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "lb5", Arguments: map[string]any{
		"action":      "list_blocks",
		"document_id": "doc_denied",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for API failure")
	}
}

func TestDocxManage_UpdateBlockText(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/docx/v1/documents/doc_update_001/blocks/blk_update_001") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		bodyBytes, _ := io.ReadAll(r.Body)
		body := string(bodyBytes)
		if !strings.Contains(body, `"update_text_elements"`) || !strings.Contains(body, `"updated from tool"`) {
			t.Fatalf("unexpected patch body: %s", body)
		}
		writeJSON(t, w, 0, "ok", map[string]any{
			"block": map[string]any{
				"block_id":   "blk_update_001",
				"block_type": 2,
				"parent_id":  "doc_update_001",
				"text": map[string]any{
					"elements": []map[string]any{
						{
							"text_run": map[string]any{
								"content": "updated from tool",
							},
						},
					},
				},
			},
			"document_revision_id": 118,
			"client_token":         "ctok_update_001",
		})
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "ub1", Arguments: map[string]any{
		"action":      "update_block_text",
		"document_id": "doc_update_001",
		"block_id":    "blk_update_001",
		"content":     "updated from tool",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Block updated successfully.") {
		t.Fatalf("expected success content, got: %s", result.Content)
	}
	if result.Metadata["document_id"] != "doc_update_001" {
		t.Fatalf("expected document_id metadata, got %v", result.Metadata["document_id"])
	}
	if result.Metadata["block_id"] != "blk_update_001" {
		t.Fatalf("expected block_id metadata, got %v", result.Metadata["block_id"])
	}
}

func TestDocxManage_UpdateBlockText_MissingArgs(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not call API when required args are missing")
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "ub2", Arguments: map[string]any{
		"action":      "update_block_text",
		"document_id": "doc_update_001",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when block_id/content missing")
	}
}

func TestDocxManage_UpdateBlockText_APIError(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 1770001, "invalid param", nil)
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "ub3", Arguments: map[string]any{
		"action":      "update_block_text",
		"document_id": "doc_update_001",
		"block_id":    "blk_update_001",
		"content":     "updated from tool",
	}}
	result, err := dm.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for update API failure")
	}
	if !strings.Contains(result.Content, "Failed to update document block text") {
		t.Fatalf("unexpected error content: %s", result.Content)
	}
}

// ---------------------------------------------------------------------------
// Channel integration tests — verify full dispatch path
// ---------------------------------------------------------------------------

func TestChannel_CreateDoc_E2E(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 0, "ok", map[string]any{
			"document": map[string]any{
				"document_id": "doc_e2e_001",
				"title":       "E2E Doc",
				"revision_id": 1,
			},
		})
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e1", Name: "channel", Arguments: map[string]any{
		"action": "create_doc",
		"title":  "E2E Doc",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "doc_e2e_001") {
		t.Fatalf("expected doc_e2e_001 in content, got: %s", result.Content)
	}
	if urlVal, ok := result.Metadata["url"].(string); !ok || !strings.Contains(urlVal, "/docx/doc_e2e_001") {
		t.Fatalf("expected url with /docx/doc_e2e_001, got %v", result.Metadata["url"])
	}
}

func TestChannel_ReadDoc_E2E(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 0, "ok", map[string]any{
			"document": map[string]any{
				"document_id": "doc_e2e_002",
				"title":       "Read E2E",
				"revision_id": 10,
			},
		})
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e2", Name: "channel", Arguments: map[string]any{
		"action":      "read_doc",
		"document_id": "doc_e2e_002",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Read E2E") {
		t.Fatalf("expected title in content, got: %s", result.Content)
	}
}

func TestChannel_ReadDocContent_E2E(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, 0, "ok", map[string]any{
			"content": "End-to-end content test.\nParagraph two.",
		})
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e3", Name: "channel", Arguments: map[string]any{
		"action":      "read_doc_content",
		"document_id": "doc_e2e_003",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "End-to-end content test.") {
		t.Fatalf("expected content text, got: %s", result.Content)
	}
}

func TestChannel_ListDocBlocks_E2E(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hasMore := false
		writeJSON(t, w, 0, "ok", map[string]any{
			"items": []map[string]any{
				{"block_id": "blk_e2e_1", "block_type": 1},
				{"block_id": "blk_e2e_2", "block_type": 2},
			},
			"has_more": hasMore,
		})
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e4", Name: "channel", Arguments: map[string]any{
		"action":      "list_doc_blocks",
		"document_id": "doc_e2e_004",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "2 blocks") {
		t.Fatalf("expected '2 blocks' in content, got: %s", result.Content)
	}
}

func TestChannel_UpdateDocBlock_E2E(t *testing.T) {
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		writeJSON(t, w, 0, "ok", map[string]any{
			"block": map[string]any{
				"block_id":   "blk_e2e_update_1",
				"block_type": 2,
				"parent_id":  "doc_e2e_update_1",
			},
			"document_revision_id": 205,
		})
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e5", Name: "channel", Arguments: map[string]any{
		"action":      "update_doc_block",
		"document_id": "doc_e2e_update_1",
		"block_id":    "blk_e2e_update_1",
		"content":     "channel update text",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Block updated successfully.") {
		t.Fatalf("expected update success content, got: %s", result.Content)
	}
}

// ---------------------------------------------------------------------------
// Full lifecycle test: create → read → read_content → list_blocks
// ---------------------------------------------------------------------------

func TestDocx_FullLifecycle(t *testing.T) {
	callCount := 0
	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		callCount++
		path := r.URL.Path

		switch {
		case r.Method == http.MethodPost && strings.Contains(path, "/docx/v1/documents") && !strings.Contains(path, "raw_content") && !strings.Contains(path, "blocks"):
			// Create or Get document
			if r.Method == http.MethodPost && !strings.Contains(path, "/doc_lifecycle_001") {
				// Create
				writeJSON(t, w, 0, "ok", map[string]any{
					"document": map[string]any{
						"document_id": "doc_lifecycle_001",
						"title":       "Lifecycle Test",
						"revision_id": 1,
					},
				})
				return
			}
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_lifecycle_001",
					"title":       "Lifecycle Test",
					"revision_id": 1,
				},
			})

		case strings.Contains(path, "doc_lifecycle_001") && strings.Contains(path, "raw_content"):
			writeJSON(t, w, 0, "ok", map[string]any{
				"content": "Lifecycle document body text",
			})

		case strings.Contains(path, "doc_lifecycle_001") && strings.Contains(path, "blocks"):
			hasMore := false
			writeJSON(t, w, 0, "ok", map[string]any{
				"items": []map[string]any{
					{"block_id": "blk_lc_1", "block_type": 1},
					{"block_id": "blk_lc_2", "block_type": 3},
				},
				"has_more": hasMore,
			})

		default:
			// Fallback for Get document
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_lifecycle_001",
					"title":       "Lifecycle Test",
					"revision_id": 1,
				},
			})
		}
	})
	defer srv.Close()

	dm := &larkDocxManage{}

	// Step 1: Create
	result, err := dm.Execute(ctx, ports.ToolCall{ID: "lc1", Arguments: map[string]any{
		"action": "create",
		"title":  "Lifecycle Test",
	}})
	if err != nil {
		t.Fatalf("create: unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("create: tool error: %v", result.Error)
	}
	docID, _ := result.Metadata["document_id"].(string)
	if docID != "doc_lifecycle_001" {
		t.Fatalf("create: expected doc_lifecycle_001, got %s", docID)
	}

	// Step 2: Read metadata
	result, err = dm.Execute(ctx, ports.ToolCall{ID: "lc2", Arguments: map[string]any{
		"action":      "read",
		"document_id": docID,
	}})
	if err != nil {
		t.Fatalf("read: unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("read: tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Lifecycle Test") {
		t.Fatalf("read: expected title, got: %s", result.Content)
	}

	// Step 3: Read content
	result, err = dm.Execute(ctx, ports.ToolCall{ID: "lc3", Arguments: map[string]any{
		"action":      "read_content",
		"document_id": docID,
	}})
	if err != nil {
		t.Fatalf("read_content: unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("read_content: tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Lifecycle document body text") {
		t.Fatalf("read_content: expected body, got: %s", result.Content)
	}

	// Step 4: List blocks
	result, err = dm.Execute(ctx, ports.ToolCall{ID: "lc4", Arguments: map[string]any{
		"action":      "list_blocks",
		"document_id": docID,
	}})
	if err != nil {
		t.Fatalf("list_blocks: unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("list_blocks: tool error: %v", result.Error)
	}
	if result.Metadata["block_count"] != 2 {
		t.Fatalf("list_blocks: expected 2 blocks, got %v", result.Metadata["block_count"])
	}

	if callCount < 4 {
		t.Fatalf("expected at least 4 API calls, got %d", callCount)
	}
}
