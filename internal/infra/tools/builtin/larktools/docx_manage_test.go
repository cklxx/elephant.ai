package larktools

import (
	"context"
	"encoding/json"
	"fmt"
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

// larkTestServerWithDocxConvertMock extends larkTestServer with a default
// /open-apis/docx/v1/documents/blocks/convert route. It returns a minimal
// successful convert payload so tests that focus on create+write flows don't
// fail when they omit an explicit convert stub.
func larkTestServerWithDocxConvertMock(t *testing.T, handler http.HandlerFunc) (*httptest.Server, context.Context) {
	t.Helper()
	return larkTestServerWithObservedDocxConvertMock(t, nil, handler)
}

// larkTestServerWithObservedDocxConvertMock installs the same default convert
// route as larkTestServerWithDocxConvertMock, but also exposes the request to
// the test so callers can assert the markdown->blocks conversion payload while
// still relying on the shared success stub.
func larkTestServerWithObservedDocxConvertMock(t *testing.T, observe func(path, body string), handler http.HandlerFunc) (*httptest.Server, context.Context) {
	t.Helper()
	return larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && isDocxBlocksConvertRoute(r.URL.Path) {
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			assertDocxConvertMarkdownRequest(t, body)
			if observe != nil {
				observe(r.URL.Path, body)
			}
			writeDocxConvertSuccess(t, w, "tmp_blk_default", "doc_mock_parent")
			return
		}
		handler(w, r)
	})
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

func isDocxCreateDocumentRoute(path string) bool {
	// Match create-document endpoint exactly (optionally with trailing slash).
	normalized := strings.TrimSuffix(path, "/")
	return normalized == "/open-apis/docx/v1/documents" || normalized == "/docx/v1/documents"
}

func isDocxBlocksConvertRoute(path string) bool {
	// Match convert endpoint exactly (optionally with trailing slash).
	normalized := strings.TrimSuffix(path, "/")
	return normalized == "/open-apis/docx/v1/documents/blocks/convert" || normalized == "/docx/v1/documents/blocks/convert"
}

func isSupportedDocxConvertPath(path string) bool {
	return isDocxBlocksConvertRoute(path)
}

func assertDocxConvertMarkdownRequest(t *testing.T, body string) {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("expected valid convert request json, got err=%v body=%s", err, body)
	}
	contentType, _ := payload["content_type"].(string)
	if contentType != "markdown" {
		t.Fatalf("expected markdown content_type in convert body, got: %s", body)
	}
	content, _ := payload["content"].(string)
	if strings.TrimSpace(content) == "" {
		t.Fatalf("expected non-empty markdown content in convert body, got: %s", body)
	}
}

func isDocxDescendantRoute(path, documentID, blockID string) bool {
	openPath := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/descendant", documentID, blockID)
	plainPath := fmt.Sprintf("/docx/v1/documents/%s/blocks/%s/descendant", documentID, blockID)
	return strings.Contains(path, openPath) || strings.Contains(path, plainPath)
}

func writeDocxConvertSuccess(t *testing.T, w http.ResponseWriter, blockID, parentID string) {
	t.Helper()
	// Minimal but structurally real convert response that DocxService.WriteMarkdown
	// can consume through the SDK's ConvertDocumentRespData parser.
	writeJSON(t, w, 0, "ok", map[string]any{
		"first_level_block_ids": []string{blockID},
		"blocks": []map[string]any{
			{
				"block_id":   blockID,
				"block_type": 2,
				"parent_id":  parentID,
				"children":   []string{},
				"text": map[string]any{
					"elements": []map[string]any{
						{"text_run": map[string]any{"content": "converted markdown"}},
					},
				},
			},
		},
		"block_id_to_image_urls": []map[string]any{},
	})
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
		if !isDocxCreateDocumentRoute(r.URL.Path) {
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
	var createCalled bool
	var convertCalled bool
	var convertPath string
	var createDescCalled bool
	srv, ctx := larkTestServerWithObservedDocxConvertMock(t, func(path, body string) {
		convertPath = path
		if !strings.Contains(body, "\"content_type\":\"markdown\"") {
			t.Fatalf("expected markdown content_type in convert body, got: %s", body)
		}
		if !strings.Contains(body, "这是正文第一段") {
			t.Fatalf("expected initial content in convert body, got: %s", body)
		}
		convertCalled = true
	}, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && isDocxCreateDocumentRoute(r.URL.Path):
			createCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_init_001",
					"title":       "设计说明",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && isDocxDescendantRoute(r.URL.Path, "doc_init_001", "doc_init_001"):
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "\"children_id\":[\"tmp_blk_default\"]") {
				t.Fatalf("expected converted children_id in create-descendant body, got: %s", body)
			}
			if !strings.Contains(body, "\"block_id\":\"tmp_blk_default\"") {
				t.Fatalf("expected converted descendant block payload, got: %s", body)
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
	if !createCalled {
		t.Fatal("expected create document API call")
	}
	if !convertCalled {
		t.Fatal("expected markdown convert call")
	}
	if !strings.Contains(convertPath, "/documents/blocks/convert") {
		t.Fatalf("expected convert API path, got %s", convertPath)
	}
	if !isSupportedDocxConvertPath(convertPath) {
		t.Fatalf("expected supported convert route, got %s", convertPath)
	}
	if !createDescCalled {
		t.Fatal("expected create descendant blocks call")
	}
	if result.Metadata["content_written"] != true {
		t.Fatalf("expected content_written=true, got %v", result.Metadata["content_written"])
	}
}

func TestDocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock(t *testing.T) {
	var createCalled bool
	var createDescCalled bool

	srv, ctx := larkTestServerWithDocxConvertMock(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && isDocxCreateDocumentRoute(r.URL.Path):
			createCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_init_002",
					"title":       "默认Convert路由",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && isDocxDescendantRoute(r.URL.Path, "doc_init_002", "doc_init_002"):
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "\"children_id\":[\"tmp_blk_default\"]") {
				t.Fatalf("expected default converted children_id in create-descendant body, got: %s", body)
			}
			createDescCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document_revision_id": 2,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	dm := &larkDocxManage{}
	call := ports.ToolCall{ID: "c1c", Arguments: map[string]any{
		"action":  "create",
		"title":   "默认Convert路由",
		"content": "正文内容",
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
	if !createDescCalled {
		t.Fatal("expected create descendant blocks call")
	}
	if result.Metadata["content_written"] != true {
		t.Fatalf("expected content_written=true, got %v", result.Metadata["content_written"])
	}
}

func TestLarkTestServerWithDocxConvertMock_HandlesConvertRoutes(t *testing.T) {
	srv, _ := larkTestServerWithDocxConvertMock(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("convert route should be intercepted before handler: %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()

	paths := []string{
		"/open-apis/docx/v1/documents/blocks/convert",
		"/open-apis/docx/v1/documents/blocks/convert/",
		"/docx/v1/documents/blocks/convert",
		"/docx/v1/documents/blocks/convert/",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			resp, err := http.Post(srv.URL+path, "application/json", strings.NewReader(`{"content_type":"markdown","content":"hello"}`))
			if err != nil {
				t.Fatalf("post convert route: %v", err)
			}
			defer resp.Body.Close()

			bodyBytes, _ := io.ReadAll(resp.Body)
			body := string(bodyBytes)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", resp.StatusCode, body)
			}
			if !strings.Contains(body, `"code":0`) {
				t.Fatalf("expected success code in body, got %s", body)
			}
			if !strings.Contains(body, `"first_level_block_ids":["tmp_blk_default"]`) {
				t.Fatalf("expected default first_level_block_ids in body, got %s", body)
			}
			if !strings.Contains(body, `"block_id":"tmp_blk_default"`) {
				t.Fatalf("expected converted block_id in body, got %s", body)
			}
			if !strings.Contains(body, `"text_run":{"content":"converted markdown"}`) {
				t.Fatalf("expected converted markdown text_run payload in body, got %s", body)
			}
			if !strings.Contains(body, `"parent_id":"doc_mock_parent"`) {
				t.Fatalf("expected default parent_id in body, got %s", body)
			}
			if !strings.Contains(body, `"block_id_to_image_urls":[]`) {
				t.Fatalf("expected empty block_id_to_image_urls in body, got %s", body)
			}
		})
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

func TestChannel_CreateDoc_WithContent_E2E(t *testing.T) {
	var createCalled bool
	var convertCalled bool
	var convertPath string
	var createDescCalled bool

	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && isDocxCreateDocumentRoute(r.URL.Path):
			createCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_e2e_content_001",
					"title":       "E2E Content Doc",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && isDocxBlocksConvertRoute(r.URL.Path):
			convertCalled = true
			convertPath = r.URL.Path
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "channel create_doc body") {
				t.Fatalf("expected markdown content in convert body, got: %s", body)
			}
			// Match current convert API semantics and return data consumable by CreateDescendantBlocks.
			writeDocxConvertSuccess(t, w, "tmp_blk_e2e_001", "doc_e2e_content_001")
		case r.Method == http.MethodPost && isDocxDescendantRoute(r.URL.Path, "doc_e2e_content_001", "doc_e2e_content_001"):
			createDescCalled = true
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "tmp_blk_e2e_001") {
				t.Fatalf("expected converted block id in descendant body, got: %s", body)
			}
			writeJSON(t, w, 0, "ok", map[string]any{
				"document_revision_id": 2,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e1-content", Name: "channel", Arguments: map[string]any{
		"action":  "create_doc",
		"title":   "E2E Content Doc",
		"content": "# Headline\n\nchannel create_doc body",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !createCalled {
		t.Fatal("expected create document API call")
	}
	if !convertCalled {
		t.Fatal("expected blocks convert API call")
	}
	if !strings.Contains(convertPath, "/documents/blocks/convert") {
		t.Fatalf("expected convert API path, got %s", convertPath)
	}
	if !isSupportedDocxConvertPath(convertPath) {
		t.Fatalf("expected supported convert route, got %s", convertPath)
	}
	if !createDescCalled {
		t.Fatal("expected create descendant blocks API call")
	}
	if contentWritten, ok := result.Metadata["content_written"].(bool); !ok || !contentWritten {
		t.Fatalf("expected content_written=true metadata, got %v", result.Metadata["content_written"])
	}
}

func TestChannel_CreateDoc_WithContent_UsesDefaultConvertMockRoute(t *testing.T) {
	var createCalled bool
	var createDescCalled bool

	srv, ctx := larkTestServerWithDocxConvertMock(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && isDocxCreateDocumentRoute(r.URL.Path):
			createCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_e2e_content_default_convert_001",
					"title":       "E2E Content Doc (default convert mock)",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && isDocxDescendantRoute(r.URL.Path, "doc_e2e_content_default_convert_001", "doc_e2e_content_default_convert_001"):
			createDescCalled = true
			bodyBytes, _ := io.ReadAll(r.Body)
			body := string(bodyBytes)
			if !strings.Contains(body, "\"children_id\":[\"tmp_blk_default\"]") {
				t.Fatalf("expected default converted children_id in descendant body, got: %s", body)
			}
			if !strings.Contains(body, "\"block_id\":\"tmp_blk_default\"") {
				t.Fatalf("expected default converted block payload in descendant body, got: %s", body)
			}
			if !strings.Contains(body, "\"text_run\":{\"content\":\"converted markdown\"}") {
				t.Fatalf("expected default converted markdown payload in descendant body, got: %s", body)
			}
			writeJSON(t, w, 0, "ok", map[string]any{
				"document_revision_id": 2,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e1-content-default-convert", Name: "channel", Arguments: map[string]any{
		"action":  "create_doc",
		"title":   "E2E Content Doc (default convert mock)",
		"content": "# Headline\n\ndefault convert body",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !createCalled {
		t.Fatal("expected create document API call")
	}
	if !createDescCalled {
		t.Fatal("expected create descendant blocks API call")
	}
	if contentWritten, ok := result.Metadata["content_written"].(bool); !ok || !contentWritten {
		t.Fatalf("expected content_written=true metadata, got %v", result.Metadata["content_written"])
	}
}

func TestChannel_CreateDoc_WithContent_ConvertAPIError(t *testing.T) {
	var createCalled bool
	var convertCalled bool

	srv, ctx := larkTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && isDocxCreateDocumentRoute(r.URL.Path):
			createCalled = true
			writeJSON(t, w, 0, "ok", map[string]any{
				"document": map[string]any{
					"document_id": "doc_e2e_content_err_001",
					"title":       "E2E Content Error Doc",
					"revision_id": 1,
				},
			})
		case r.Method == http.MethodPost && isDocxBlocksConvertRoute(r.URL.Path):
			convertCalled = true
			writeJSON(t, w, 99991663, "convert failed", nil)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	defer srv.Close()

	tool := NewLarkChannel()
	call := ports.ToolCall{ID: "e2e1-content-err", Name: "channel", Arguments: map[string]any{
		"action":  "create_doc",
		"title":   "E2E Content Error Doc",
		"content": "# Headline\n\nchannel create_doc body",
	}}
	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !createCalled {
		t.Fatal("expected create document API call")
	}
	if !convertCalled {
		t.Fatal("expected blocks convert API call")
	}
	if result.Error == nil {
		t.Fatal("expected tool error when blocks convert API fails")
	}
	if !strings.Contains(result.Content, "Failed to write initial document content") {
		t.Fatalf("unexpected error content: %s", result.Content)
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

		case r.Method == http.MethodPost && isDocxBlocksConvertRoute(path):
			writeDocxConvertSuccess(t, w, "tmp_blk_lc_1", "doc_lifecycle_001")

		case r.Method == http.MethodPost && isDocxDescendantRoute(path, "doc_lifecycle_001", "doc_lifecycle_001"):
			writeJSON(t, w, 0, "ok", map[string]any{
				"document_revision_id": 2,
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
		"action":  "create",
		"title":   "Lifecycle Test",
		"content": "Lifecycle seeded content",
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
