package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListMailgroups(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"mailgroup_id":         "mg_1",
					"email":                "team@example.com",
					"name":                 "Team",
					"description":          "Team mail group",
					"direct_members_count": "15",
				},
				{
					"mailgroup_id": "mg_2",
					"email":        "all@example.com",
					"name":         "All",
				},
			},
			"page_token": "next",
			"has_more":   true,
		}))
	})
	defer srv.Close()

	resp, err := client.Mail().ListMailgroups(context.Background(), ListMailgroupsRequest{PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Mailgroups) != 2 {
		t.Fatalf("expected 2 mailgroups, got %d", len(resp.Mailgroups))
	}
	if resp.Mailgroups[0].MailgroupID != "mg_1" {
		t.Errorf("expected mg_1, got %s", resp.Mailgroups[0].MailgroupID)
	}
	if resp.Mailgroups[0].Email != "team@example.com" {
		t.Errorf("expected team@example.com, got %s", resp.Mailgroups[0].Email)
	}
	if resp.Mailgroups[0].DirectMembersCount != 15 {
		t.Errorf("expected 15, got %d", resp.Mailgroups[0].DirectMembersCount)
	}
	if !resp.HasMore {
		t.Error("expected has_more=true")
	}
}

func TestGetMailgroup(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"mailgroup_id":         "mg_1",
			"email":                "team@example.com",
			"name":                 "Team",
			"description":          "Team mail group",
			"direct_members_count": "10",
		}))
	})
	defer srv.Close()

	mg, err := client.Mail().GetMailgroup(context.Background(), "mg_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mg.MailgroupID != "mg_1" {
		t.Errorf("expected mg_1, got %s", mg.MailgroupID)
	}
	if mg.Name != "Team" {
		t.Errorf("expected Team, got %s", mg.Name)
	}
	if mg.DirectMembersCount != 10 {
		t.Errorf("expected 10, got %d", mg.DirectMembersCount)
	}
}

func TestCreateMailgroup(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"mailgroup_id": "mg_new",
			"email":        "new@example.com",
			"name":         "New Group",
		}))
	})
	defer srv.Close()

	mg, err := client.Mail().CreateMailgroup(context.Background(), CreateMailgroupRequest{
		Email: "new@example.com",
		Name:  "New Group",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if mg.MailgroupID != "mg_new" {
		t.Errorf("expected mg_new, got %s", mg.MailgroupID)
	}
	if mg.Email != "new@example.com" {
		t.Errorf("expected new@example.com, got %s", mg.Email)
	}
}

func TestMailAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "permission denied", nil))
	})
	defer srv.Close()

	_, err := client.Mail().GetMailgroup(context.Background(), "mg_1")
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
