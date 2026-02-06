package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// --- test helpers ---

func approvalInstanceJSON(code, status, userID string, startMS, endMS int64, tasks []map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"instance": map[string]interface{}{
			"code":       code,
			"status":     status,
			"user_id":    userID,
			"start_time": fmt.Sprintf("%d", startMS),
			"end_time":   fmt.Sprintf("%d", endMS),
		},
		"approval": map[string]interface{}{
			"code": "APPROVAL-001",
			"name": "Test Approval",
		},
	}
}

func approvalTaskJSON(taskID, userID, status, nodeName string) map[string]interface{} {
	return map[string]interface{}{
		"id":        taskID,
		"user_id":   userID,
		"status":    status,
		"node_name": nodeName,
	}
}

// --- QueryApprovalInstances tests ---

func TestQueryApprovalInstances_Success(t *testing.T) {
	now := time.Now()
	startMS := now.UnixMilli()
	endMS := now.Add(1 * time.Hour).UnixMilli()

	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/approval/v4/instances/query") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		items := []map[string]interface{}{
			approvalInstanceJSON("INST-001", "PENDING", "user-1", startMS, endMS, nil),
			approvalInstanceJSON("INST-002", "APPROVED", "user-2", startMS, endMS, nil),
		}
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"instance_list": items,
			"page_token":    "",
			"has_more":      false,
			"count":         2,
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	instances, err := client.Approval().QueryApprovalInstances(ctx(), "APPROVAL-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	if instances[0].InstanceID != "INST-001" {
		t.Errorf("instance[0].InstanceID = %q, want %q", instances[0].InstanceID, "INST-001")
	}
	if instances[0].Status != "PENDING" {
		t.Errorf("instance[0].Status = %q, want %q", instances[0].Status, "PENDING")
	}
	if instances[1].InstanceID != "INST-002" {
		t.Errorf("instance[1].InstanceID = %q, want %q", instances[1].InstanceID, "INST-002")
	}
	if instances[1].ApprovalCode != "APPROVAL-001" {
		t.Errorf("instance[1].ApprovalCode = %q, want %q", instances[1].ApprovalCode, "APPROVAL-001")
	}
}

func TestQueryApprovalInstances_Empty(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"instance_list": []map[string]interface{}{},
			"page_token":    "",
			"has_more":      false,
			"count":         0,
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	instances, err := client.Approval().QueryApprovalInstances(ctx(), "APPROVAL-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(instances) != 0 {
		t.Fatalf("expected 0 instances, got %d", len(instances))
	}
}

// --- GetApprovalInstance tests ---

func TestGetApprovalInstance_Success(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/approval/v4/instances/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		now := time.Now()
		formJSON := `[{"id":"field-1","type":"input","value":"hello"},{"id":"field-2","type":"input","value":"world"}]`
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"instance_code": "INST-001",
			"approval_code": "APPROVAL-001",
			"status":        "PENDING",
			"user_id":       "user-1",
			"start_time":    fmt.Sprintf("%d", now.UnixMilli()),
			"end_time":      "0",
			"form":          formJSON,
			"task_list": []map[string]interface{}{
				approvalTaskJSON("task-1", "approver-1", "PENDING", "Manager Approval"),
				approvalTaskJSON("task-2", "approver-2", "APPROVED", "VP Approval"),
			},
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	inst, err := client.Approval().GetApprovalInstance(ctx(), "INST-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if inst.InstanceID != "INST-001" {
		t.Errorf("InstanceID = %q, want %q", inst.InstanceID, "INST-001")
	}
	if inst.ApprovalCode != "APPROVAL-001" {
		t.Errorf("ApprovalCode = %q, want %q", inst.ApprovalCode, "APPROVAL-001")
	}
	if inst.Status != "PENDING" {
		t.Errorf("Status = %q, want %q", inst.Status, "PENDING")
	}
	if inst.Initiator != "user-1" {
		t.Errorf("Initiator = %q, want %q", inst.Initiator, "user-1")
	}
	if inst.StartTime.IsZero() {
		t.Error("StartTime should not be zero")
	}
	if !inst.EndTime.IsZero() {
		t.Errorf("EndTime should be zero for end_time=0, got %v", inst.EndTime)
	}

	// Verify form values.
	if inst.FormValues["field-1"] != "hello" {
		t.Errorf("FormValues[field-1] = %q, want %q", inst.FormValues["field-1"], "hello")
	}
	if inst.FormValues["field-2"] != "world" {
		t.Errorf("FormValues[field-2] = %q, want %q", inst.FormValues["field-2"], "world")
	}

	// Verify task list.
	if len(inst.TaskList) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(inst.TaskList))
	}
	if inst.TaskList[0].TaskID != "task-1" {
		t.Errorf("TaskList[0].TaskID = %q, want %q", inst.TaskList[0].TaskID, "task-1")
	}
	if inst.TaskList[0].NodeName != "Manager Approval" {
		t.Errorf("TaskList[0].NodeName = %q, want %q", inst.TaskList[0].NodeName, "Manager Approval")
	}
	if inst.TaskList[1].Status != "APPROVED" {
		t.Errorf("TaskList[1].Status = %q, want %q", inst.TaskList[1].Status, "APPROVED")
	}
}

func TestGetApprovalInstance_NotFound(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(60011, "instance not found", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	_, err := client.Approval().GetApprovalInstance(ctx(), "NONEXISTENT")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != 60011 {
		t.Errorf("APIError.Code = %d, want %d", apiErr.Code, 60011)
	}
}

// --- CreateApprovalInstance tests ---

func TestCreateApprovalInstance_Success(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/approval/v4/instances") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}

		// Verify the request body contains the form data.
		body := readBody(r)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if reqBody["approval_code"] != "APPROVAL-001" {
			t.Errorf("approval_code = %v, want %q", reqBody["approval_code"], "APPROVAL-001")
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"instance_code": "INST-NEW-001",
			"instance_link": "https://example.com/approval/INST-NEW-001",
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	instanceID, err := client.Approval().CreateApprovalInstance(ctx(), "APPROVAL-001", map[string]string{
		"reason": "Business trip",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if instanceID != "INST-NEW-001" {
		t.Errorf("instanceID = %q, want %q", instanceID, "INST-NEW-001")
	}
}

// --- ApproveTask tests ---

func TestApproveTask_Success(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/approval/v4/tasks/approve") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body := readBody(r)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if reqBody["approval_code"] != "APPROVAL-001" {
			t.Errorf("approval_code = %v, want %q", reqBody["approval_code"], "APPROVAL-001")
		}
		if reqBody["instance_code"] != "INST-001" {
			t.Errorf("instance_code = %v, want %q", reqBody["instance_code"], "INST-001")
		}
		if reqBody["task_id"] != "task-1" {
			t.Errorf("task_id = %v, want %q", reqBody["task_id"], "task-1")
		}
		if reqBody["comment"] != "Looks good" {
			t.Errorf("comment = %v, want %q", reqBody["comment"], "Looks good")
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	err := client.Approval().ApproveTask(ctx(), "APPROVAL-001", "INST-001", "task-1", "approver-1", "Looks good")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- RejectTask tests ---

func TestRejectTask_Success(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/approval/v4/tasks/reject") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		body := readBody(r)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}
		if reqBody["approval_code"] != "APPROVAL-001" {
			t.Errorf("approval_code = %v, want %q", reqBody["approval_code"], "APPROVAL-001")
		}
		if reqBody["comment"] != "Insufficient details" {
			t.Errorf("comment = %v, want %q", reqBody["comment"], "Insufficient details")
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", nil)); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	err := client.Approval().RejectTask(ctx(), "APPROVAL-001", "INST-001", "task-1", "approver-1", "Insufficient details")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- ApprovalQueryOptions test ---

func TestApprovalQueryOptions(t *testing.T) {
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now

	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request body carries the options.
		body := readBody(r)
		var reqBody map[string]interface{}
		if err := json.Unmarshal(body, &reqBody); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		if reqBody["approval_code"] != "APPROVAL-002" {
			t.Errorf("approval_code = %v, want %q", reqBody["approval_code"], "APPROVAL-002")
		}
		if reqBody["instance_status"] != "APPROVED" {
			t.Errorf("instance_status = %v, want %q", reqBody["instance_status"], "APPROVED")
		}
		if reqBody["instance_start_time_from"] != fmt.Sprintf("%d", start.UnixMilli()) {
			t.Errorf("instance_start_time_from = %v, want %q", reqBody["instance_start_time_from"], fmt.Sprintf("%d", start.UnixMilli()))
		}
		if reqBody["instance_start_time_to"] != fmt.Sprintf("%d", end.UnixMilli()) {
			t.Errorf("instance_start_time_to = %v, want %q", reqBody["instance_start_time_to"], fmt.Sprintf("%d", end.UnixMilli()))
		}

		// Verify page_size query parameter.
		pageSize := r.URL.Query().Get("page_size")
		if pageSize != "5" {
			t.Errorf("page_size = %q, want %q", pageSize, "5")
		}

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(jsonResponse(0, "ok", map[string]interface{}{
			"instance_list": []map[string]interface{}{},
			"count":         0,
		})); err != nil {
			t.Fatalf("write response: %v", err)
		}
	})
	defer srv.Close()

	_, err := client.Approval().QueryApprovalInstances(ctx(), "APPROVAL-002",
		WithApprovalStatus("APPROVED"),
		WithApprovalTimeRange(start, end),
		WithApprovalPageSize(5),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ctx returns a background context for tests.
func ctx() context.Context {
	return context.Background()
}
