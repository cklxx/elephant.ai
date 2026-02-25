package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListSpaces(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"space_id": "space_1", "name": "Engineering", "visibility": "private"},
				{"space_id": "space_2", "name": "Product", "visibility": "public"},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Wiki().ListSpaces(context.Background(), ListSpacesRequest{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Spaces) != 2 {
		t.Fatalf("expected 2 spaces, got %d", len(resp.Spaces))
	}
	if resp.Spaces[0].SpaceID != "space_1" {
		t.Errorf("expected space_1, got %s", resp.Spaces[0].SpaceID)
	}
	if resp.Spaces[0].Name != "Engineering" {
		t.Errorf("expected Engineering, got %s", resp.Spaces[0].Name)
	}
}

func TestGetSpace(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"space": map[string]interface{}{
				"space_id":    "space_1",
				"name":        "Engineering",
				"description": "Engineering docs",
				"visibility":  "private",
			},
		}))
	})
	defer srv.Close()

	space, err := client.Wiki().GetSpace(context.Background(), "space_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if space.SpaceID != "space_1" {
		t.Errorf("expected space_1, got %s", space.SpaceID)
	}
	if space.Description != "Engineering docs" {
		t.Errorf("expected 'Engineering docs', got %s", space.Description)
	}
}

func TestListNodes(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"node_token":        "node_abc",
					"space_id":          "space_1",
					"title":             "Getting Started",
					"obj_token":         "docx_abc",
					"obj_type":          "docx",
					"parent_node_token": "",
					"has_child":         true,
					"node_type":         "origin",
				},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Wiki().ListNodes(context.Background(), ListNodesRequest{
		SpaceID:  "space_1",
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(resp.Nodes))
	}
	if resp.Nodes[0].NodeToken != "node_abc" {
		t.Errorf("expected node_abc, got %s", resp.Nodes[0].NodeToken)
	}
	if resp.Nodes[0].ObjType != "docx" {
		t.Errorf("expected docx, got %s", resp.Nodes[0].ObjType)
	}
	if !resp.Nodes[0].HasChild {
		t.Error("expected HasChild=true")
	}
}

func TestCreateNode(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"node": map[string]interface{}{
				"node_token":        "node_new",
				"space_id":          "space_1",
				"title":             "New Doc",
				"obj_token":         "docx_new",
				"obj_type":          "docx",
				"parent_node_token": "node_abc",
				"node_type":         "origin",
			},
		}))
	})
	defer srv.Close()

	node, err := client.Wiki().CreateNode(context.Background(), CreateNodeRequest{
		SpaceID:     "space_1",
		ObjType:     "docx",
		ParentToken: "node_abc",
		Title:       "New Doc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.NodeToken != "node_new" {
		t.Errorf("expected node_new, got %s", node.NodeToken)
	}
	if node.ObjToken != "docx_new" {
		t.Errorf("expected docx_new, got %s", node.ObjToken)
	}
}

func TestGetNode(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"node": map[string]interface{}{
				"node_token": "node_abc",
				"space_id":   "space_1",
				"title":      "Existing Page",
				"obj_type":   "docx",
			},
		}))
	})
	defer srv.Close()

	node, err := client.Wiki().GetNode(context.Background(), "node_abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if node.NodeToken != "node_abc" {
		t.Errorf("expected node_abc, got %s", node.NodeToken)
	}
	if node.Title != "Existing Page" {
		t.Errorf("expected 'Existing Page', got %s", node.Title)
	}
}

func TestListSpacesAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "no permission", nil))
	})
	defer srv.Close()

	_, err := client.Wiki().ListSpaces(context.Background(), ListSpacesRequest{})
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
