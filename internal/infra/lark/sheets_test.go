package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestCreateSpreadsheet(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"spreadsheet": map[string]interface{}{
				"spreadsheet_token": "ss_token_123",
				"title":             "Test Sheet",
				"url":               "https://example.com/sheets/ss_token_123",
				"folder_token":      "folder_abc",
			},
		}))
	})
	defer srv.Close()

	ss, err := client.Sheets().CreateSpreadsheet(context.Background(), CreateSpreadsheetRequest{
		Title:       "Test Sheet",
		FolderToken: "folder_abc",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.SpreadsheetToken != "ss_token_123" {
		t.Errorf("expected ss_token_123, got %s", ss.SpreadsheetToken)
	}
	if ss.Title != "Test Sheet" {
		t.Errorf("expected 'Test Sheet', got %s", ss.Title)
	}
}

func TestGetSpreadsheet(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"spreadsheet": map[string]interface{}{
				"title": "My Sheet",
				"token": "ss_token_456",
				"url":   "https://example.com/sheets/ss_token_456",
			},
		}))
	})
	defer srv.Close()

	ss, err := client.Sheets().GetSpreadsheet(context.Background(), "ss_token_456")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ss.SpreadsheetToken != "ss_token_456" {
		t.Errorf("expected ss_token_456, got %s", ss.SpreadsheetToken)
	}
	if ss.Title != "My Sheet" {
		t.Errorf("expected 'My Sheet', got %s", ss.Title)
	}
}

func TestListSheets(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"sheets": []map[string]interface{}{
				{"sheet_id": "sheet_1", "title": "Sheet1", "index": 0},
				{"sheet_id": "sheet_2", "title": "Sheet2", "index": 1},
			},
		}))
	})
	defer srv.Close()

	sheets, err := client.Sheets().ListSheets(context.Background(), "ss_token_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sheets) != 2 {
		t.Fatalf("expected 2 sheets, got %d", len(sheets))
	}
	if sheets[0].SheetID != "sheet_1" {
		t.Errorf("expected sheet_1, got %s", sheets[0].SheetID)
	}
	if sheets[0].Title != "Sheet1" {
		t.Errorf("expected Sheet1, got %s", sheets[0].Title)
	}
	if sheets[1].SheetID != "sheet_2" {
		t.Errorf("expected sheet_2, got %s", sheets[1].SheetID)
	}
}

func TestSheetsAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jsonResponse(99991, "no permission", nil))
	})
	defer srv.Close()

	_, err := client.Sheets().GetSpreadsheet(context.Background(), "ss_token_123")
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
