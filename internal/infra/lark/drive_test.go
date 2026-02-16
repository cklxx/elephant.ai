package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListDriveFiles(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"files": []map[string]interface{}{
				{"token": "file_1", "name": "report.pdf", "type": "file"},
				{"token": "folder_1", "name": "Documents", "type": "folder"},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Drive().ListFiles(context.Background(), ListFilesRequest{
		FolderToken: "root",
		PageSize:    20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(resp.Files))
	}
	if resp.Files[0].Token != "file_1" {
		t.Errorf("expected file_1, got %s", resp.Files[0].Token)
	}
	if resp.Files[0].Name != "report.pdf" {
		t.Errorf("expected report.pdf, got %s", resp.Files[0].Name)
	}
	if resp.Files[1].Type != "folder" {
		t.Errorf("expected folder, got %s", resp.Files[1].Type)
	}
}

func TestCreateDriveFolder(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"token": "folder_new",
			"url":   "https://example.com/folder",
		}))
	})
	defer srv.Close()

	folder, err := client.Drive().CreateFolder(context.Background(), "root", "New Folder")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if folder.Token != "folder_new" {
		t.Errorf("expected folder_new, got %s", folder.Token)
	}
	if folder.Type != "folder" {
		t.Errorf("expected folder, got %s", folder.Type)
	}
}

func TestCopyDriveFile(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"file": map[string]interface{}{
				"token": "file_copy",
				"url":   "https://example.com/copy",
			},
		}))
	})
	defer srv.Close()

	copied, err := client.Drive().CopyFile(context.Background(), "file_1", "folder_1", "copy.pdf", "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if copied.Token != "file_copy" {
		t.Errorf("expected file_copy, got %s", copied.Token)
	}
}

func TestDeleteDriveFile(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", nil))
	})
	defer srv.Close()

	err := client.Drive().DeleteFile(context.Background(), "file_1", "file")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDriveAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(99991, "access denied", nil))
	})
	defer srv.Close()

	_, err := client.Drive().ListFiles(context.Background(), ListFilesRequest{FolderToken: "root"})
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
