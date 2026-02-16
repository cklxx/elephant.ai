package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestGetUser(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"user": map[string]interface{}{
				"user_id":        "user_123",
				"open_id":        "ou_abc",
				"name":           "Alice",
				"en_name":        "Alice Smith",
				"email":          "alice@example.com",
				"mobile":         "+86-1234567890",
				"department_ids": []string{"dept_1", "dept_2"},
				"status": map[string]interface{}{
					"is_activated": true,
				},
			},
		}))
	})
	defer srv.Close()

	user, err := client.Contact().GetUser(context.Background(), "ou_abc", "open_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user.OpenID != "ou_abc" {
		t.Errorf("expected ou_abc, got %s", user.OpenID)
	}
	if user.Name != "Alice" {
		t.Errorf("expected Alice, got %s", user.Name)
	}
	if user.Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", user.Email)
	}
	if user.Status != 1 {
		t.Errorf("expected status 1, got %d", user.Status)
	}
	if len(user.DepartmentIDs) != 2 {
		t.Errorf("expected 2 department IDs, got %d", len(user.DepartmentIDs))
	}
}

func TestListUsers(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"user_id": "user_1", "open_id": "ou_1", "name": "Alice"},
				{"user_id": "user_2", "open_id": "ou_2", "name": "Bob"},
			},
			"page_token": "next",
			"has_more":   true,
		}))
	})
	defer srv.Close()

	resp, err := client.Contact().ListUsers(context.Background(), ListUsersRequest{
		DepartmentID: "dept_1",
		PageSize:     50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(resp.Users))
	}
	if resp.Users[0].Name != "Alice" {
		t.Errorf("expected Alice, got %s", resp.Users[0].Name)
	}
	if !resp.HasMore {
		t.Error("expected has_more=true")
	}
}

func TestGetDepartment(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"department": map[string]interface{}{
				"department_id":        "dept_1",
				"name":                 "Engineering",
				"parent_department_id": "0",
				"leader_user_id":       "user_lead",
				"member_count":         42,
			},
		}))
	})
	defer srv.Close()

	dept, err := client.Contact().GetDepartment(context.Background(), "dept_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dept.DepartmentID != "dept_1" {
		t.Errorf("expected dept_1, got %s", dept.DepartmentID)
	}
	if dept.Name != "Engineering" {
		t.Errorf("expected Engineering, got %s", dept.Name)
	}
	if dept.MemberCount != 42 {
		t.Errorf("expected 42, got %d", dept.MemberCount)
	}
}

func TestListDepartments(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"department_id": "dept_1", "name": "Engineering"},
				{"department_id": "dept_2", "name": "Product"},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.Contact().ListDepartments(context.Background(), ListDepartmentsRequest{
		ParentDepartmentID: "0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Departments) != 2 {
		t.Fatalf("expected 2 departments, got %d", len(resp.Departments))
	}
	if resp.Departments[1].Name != "Product" {
		t.Errorf("expected Product, got %s", resp.Departments[1].Name)
	}
}

func TestContactAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "no permission", nil))
	})
	defer srv.Close()

	_, err := client.Contact().GetUser(context.Background(), "ou_abc", "open_id")
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
