package larktools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	larkclient "alex/internal/infra/lark"
)

// newTestClient creates a Lark client pointed at a test server that handles
// token auth automatically, then delegates to the provided handler.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *larkclient.Client) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tenant_access_token") ||
			strings.Contains(r.URL.Path, "app_access_token") {
			w.Header().Set("Content-Type", "application/json")
			b, _ := json.Marshal(map[string]any{
				"code": 0, "msg": "ok",
				"tenant_access_token": "test-token",
				"app_access_token":    "test-token",
				"expire":              7200,
			})
			w.Write(b)
			return
		}
		handler(w, r)
	}))
	client := larkclient.NewTestClient(srv.URL)
	return srv, client
}

func TestUpdateDocBlock(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/docx/v1/documents/doc_abc/blocks/blk_1") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"block":{"block_id":"blk_1","block_type":2,"parent_id":"doc_abc"},"document_revision_id":42,"client_token":"ct1"}}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_1",
		Arguments: map[string]any{
			"action":      "update_doc_block",
			"document_id": "doc_abc",
			"block_id":    "blk_1",
			"content":     "new text",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "更新成功") {
		t.Errorf("expected success message, got: %s", result.Content)
	}
	if result.Metadata["block_id"] != "blk_1" {
		t.Errorf("expected block_id=blk_1, got %v", result.Metadata["block_id"])
	}
	if result.Metadata["document_revision_id"] != 42 {
		t.Errorf("expected revision=42, got %v", result.Metadata["document_revision_id"])
	}
}

func TestUpdateDocBlockMissingParams(t *testing.T) {
	tool := NewChannel(nil)

	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "missing document_id",
			args: map[string]any{"action": "update_doc_block", "block_id": "blk", "content": "x"},
			want: "document_id",
		},
		{
			name: "missing block_id",
			args: map[string]any{"action": "update_doc_block", "document_id": "doc", "content": "x"},
			want: "block_id",
		},
		{
			name: "missing content",
			args: map[string]any{"action": "update_doc_block", "document_id": "doc", "block_id": "blk"},
			want: "content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), ports.ToolCall{
				ID:        "call_err",
				Arguments: tt.args,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Error == nil {
				t.Fatal("expected tool error")
			}
			if !strings.Contains(result.Content, tt.want) {
				t.Errorf("expected error about %q, got: %s", tt.want, result.Content)
			}
		})
	}
}

func TestUpdateDocBlockAPIError(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":1770001,"msg":"invalid param","data":null}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_2",
		Arguments: map[string]any{
			"action":      "update_doc_block",
			"document_id": "doc_x",
			"block_id":    "blk_x",
			"content":     "x",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for API failure")
	}
	if !strings.Contains(result.Content, "update_doc_block failed") {
		t.Errorf("expected failure message, got: %s", result.Content)
	}
}

func TestCreateDoc(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"document":{"document_id":"doc_new","title":"Test","revision_id":1}}}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_3",
		Arguments: map[string]any{
			"action": "create_doc",
			"title":  "Test",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "doc_new") {
		t.Errorf("expected document_id in result, got: %s", result.Content)
	}
}

func TestReadDoc(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"document":{"document_id":"doc_1","title":"My Doc","revision_id":5}}}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_4",
		Arguments: map[string]any{
			"action":      "read_doc",
			"document_id": "doc_1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "My Doc") {
		t.Errorf("expected title in result, got: %s", result.Content)
	}
}

func TestReadDocContent(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"content":"Hello world"}}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_5",
		Arguments: map[string]any{
			"action":      "read_doc_content",
			"document_id": "doc_1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if result.Content != "Hello world" {
		t.Errorf("expected 'Hello world', got: %s", result.Content)
	}
}

func TestListDocBlocks(t *testing.T) {
	srv, client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"msg":"ok","data":{"items":[{"block_id":"blk_1","block_type":1,"parent_id":"root"},{"block_id":"blk_2","block_type":2,"parent_id":"blk_1"}],"has_more":false}}`))
	})
	defer srv.Close()

	tool := NewChannel(client)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_6",
		Arguments: map[string]any{
			"action":      "list_doc_blocks",
			"document_id": "doc_1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "blk_1") || !strings.Contains(result.Content, "blk_2") {
		t.Errorf("expected block IDs in result, got: %s", result.Content)
	}
}

func TestUnsupportedAction(t *testing.T) {
	tool := NewChannel(nil)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call_bad",
		Arguments: map[string]any{
			"action": "nonexistent",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for unsupported action")
	}
	if !strings.Contains(result.Content, "unsupported action") {
		t.Errorf("expected unsupported action error, got: %s", result.Content)
	}
}

func TestMissingAction(t *testing.T) {
	tool := NewChannel(nil)
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID:        "call_no_action",
		Arguments: map[string]any{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected tool error for missing action")
	}
}

func TestActionSafetyLevel(t *testing.T) {
	if got := ActionSafetyLevel(ActionReadDoc); got != ports.SafetyLevelReadOnly {
		t.Errorf("read_doc: expected L1, got %d", got)
	}
	if got := ActionSafetyLevel(ActionUpdateDocBlock); got != ports.SafetyLevelHighImpact {
		t.Errorf("update_doc_block: expected L3, got %d", got)
	}
	if got := ActionSafetyLevel("unknown"); got != ports.SafetyLevelHighImpact {
		t.Errorf("unknown: expected L3 fallback, got %d", got)
	}
}

func TestChannelDefinitionAndMetadata(t *testing.T) {
	tool := NewChannel(nil)
	def := tool.Definition()
	if def.Name != "channel" {
		t.Errorf("expected name=channel, got %s", def.Name)
	}
	if _, ok := def.Parameters.Properties["action"]; !ok {
		t.Error("expected action property in parameters")
	}

	meta := tool.Metadata()
	if meta.Category != "lark" {
		t.Errorf("expected category=lark, got %s", meta.Category)
	}
	if meta.SafetyLevel != ports.SafetyLevelHighImpact {
		t.Errorf("expected safety_level=L3, got %d", meta.SafetyLevel)
	}
}
