package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestGetBitableApp(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"app": map[string]interface{}{
				"app_token": "app_abc123",
				"name":      "Project Tracker",
				"revision":  3,
			},
		}))
	})
	defer srv.Close()

	app, err := client.Bitable().GetApp(context.Background(), "app_abc123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if app.AppToken != "app_abc123" {
		t.Errorf("expected app_abc123, got %s", app.AppToken)
	}
	if app.Name != "Project Tracker" {
		t.Errorf("expected 'Project Tracker', got %s", app.Name)
	}
}

func TestListBitableTables(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"table_id": "tbl_1", "name": "Tasks", "revision": 10},
				{"table_id": "tbl_2", "name": "Bugs", "revision": 5},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Bitable().ListTables(context.Background(), ListTablesRequest{
		AppToken: "app_abc123",
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(resp.Tables))
	}
	if resp.Tables[0].TableID != "tbl_1" {
		t.Errorf("expected tbl_1, got %s", resp.Tables[0].TableID)
	}
	if resp.Tables[0].Name != "Tasks" {
		t.Errorf("expected Tasks, got %s", resp.Tables[0].Name)
	}
}

func TestCreateBitableTable(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"table_id": "tbl_new",
		}))
	})
	defer srv.Close()

	table, err := client.Bitable().CreateTable(context.Background(), "app_abc123", "New Table")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if table.TableID != "tbl_new" {
		t.Errorf("expected tbl_new, got %s", table.TableID)
	}
}

func TestListBitableRecords(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		total := 2
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"record_id": "rec_1", "fields": map[string]interface{}{"Name": "Alice", "Age": 30}},
				{"record_id": "rec_2", "fields": map[string]interface{}{"Name": "Bob", "Age": 25}},
			},
			"total":    total,
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Bitable().ListRecords(context.Background(), ListRecordsRequest{
		AppToken: "app_abc123",
		TableID:  "tbl_1",
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(resp.Records))
	}
	if resp.Records[0].RecordID != "rec_1" {
		t.Errorf("expected rec_1, got %s", resp.Records[0].RecordID)
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}
}

func TestCreateBitableRecord(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"record": map[string]interface{}{
				"record_id": "rec_new",
				"fields":    map[string]interface{}{"Name": "Charlie"},
			},
		}))
	})
	defer srv.Close()

	record, err := client.Bitable().CreateRecord(context.Background(), "app_abc123", "tbl_1",
		map[string]interface{}{"Name": "Charlie"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.RecordID != "rec_new" {
		t.Errorf("expected rec_new, got %s", record.RecordID)
	}
}

func TestUpdateBitableRecord(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"record": map[string]interface{}{
				"record_id": "rec_1",
				"fields":    map[string]interface{}{"Name": "Alice Updated"},
			},
		}))
	})
	defer srv.Close()

	record, err := client.Bitable().UpdateRecord(context.Background(), "app_abc123", "tbl_1", "rec_1",
		map[string]interface{}{"Name": "Alice Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.RecordID != "rec_1" {
		t.Errorf("expected rec_1, got %s", record.RecordID)
	}
}

func TestDeleteBitableRecord(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", nil))
	})
	defer srv.Close()

	err := client.Bitable().DeleteRecord(context.Background(), "app_abc123", "tbl_1", "rec_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListBitableFields(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"field_name": "Name", "type": 1},
				{"field_name": "Age", "type": 2},
			},
		}))
	})
	defer srv.Close()

	fields, err := client.Bitable().ListFields(context.Background(), "app_abc123", "tbl_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(fields))
	}
	if fields[0].FieldName != "Name" {
		t.Errorf("expected Name, got %s", fields[0].FieldName)
	}
	if fields[0].FieldType != 1 {
		t.Errorf("expected type 1, got %d", fields[0].FieldType)
	}
}

func TestBitableAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "no permission", nil))
	})
	defer srv.Close()

	_, err := client.Bitable().GetApp(context.Background(), "app_abc123")
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
