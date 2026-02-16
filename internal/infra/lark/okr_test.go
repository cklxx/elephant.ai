package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListOKRPeriods(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "period_1", "zh_name": "2026 Q1", "status": 0},
				{"id": "period_2", "zh_name": "2025 Q4", "status": 0},
			},
			"page_token": "next_page",
			"has_more":   true,
		}))
	})
	defer srv.Close()

	resp, err := client.OKR().ListPeriods(context.Background(), ListPeriodsRequest{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Periods) != 2 {
		t.Fatalf("expected 2 periods, got %d", len(resp.Periods))
	}
	if resp.Periods[0].PeriodID != "period_1" {
		t.Errorf("expected period_1, got %s", resp.Periods[0].PeriodID)
	}
	if resp.Periods[0].Name != "2026 Q1" {
		t.Errorf("expected '2026 Q1', got %s", resp.Periods[0].Name)
	}
	if !resp.HasMore {
		t.Error("expected has_more=true")
	}
	if resp.PageToken != "next_page" {
		t.Errorf("expected 'next_page', got %s", resp.PageToken)
	}
}

func TestListUserOKRs(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"okr_list": []map[string]interface{}{
				{
					"id":        "okr_1",
					"name":      "Q1 OKR",
					"period_id": "period_1",
					"objective_list": []map[string]interface{}{
						{
							"id":      "obj_1",
							"content": "Increase revenue",
							"score":   80,
							"kr_list": []map[string]interface{}{
								{"id": "kr_1", "content": "Close 10 deals"},
							},
						},
					},
				},
			},
		}))
	})
	defer srv.Close()

	okrs, err := client.OKR().ListUserOKRs(context.Background(), "user_123", "period_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(okrs) != 1 {
		t.Fatalf("expected 1 OKR, got %d", len(okrs))
	}
	if okrs[0].OKRID != "okr_1" {
		t.Errorf("expected okr_1, got %s", okrs[0].OKRID)
	}
	if okrs[0].Name != "Q1 OKR" {
		t.Errorf("expected 'Q1 OKR', got %s", okrs[0].Name)
	}
	if len(okrs[0].ObjectiveList) != 1 {
		t.Fatalf("expected 1 objective, got %d", len(okrs[0].ObjectiveList))
	}
	obj := okrs[0].ObjectiveList[0]
	if obj.Content != "Increase revenue" {
		t.Errorf("expected 'Increase revenue', got %s", obj.Content)
	}
	if obj.Progress != 80 {
		t.Errorf("expected progress 80, got %d", obj.Progress)
	}
	if len(obj.KeyResults) != 1 {
		t.Fatalf("expected 1 KR, got %d", len(obj.KeyResults))
	}
	if obj.KeyResults[0].Content != "Close 10 deals" {
		t.Errorf("expected 'Close 10 deals', got %s", obj.KeyResults[0].Content)
	}
}

func TestBatchGetOKRs(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"okr_list": []map[string]interface{}{
				{"id": "okr_1", "name": "OKR A", "period_id": "p1"},
				{"id": "okr_2", "name": "OKR B", "period_id": "p1"},
			},
		}))
	})
	defer srv.Close()

	okrs, err := client.OKR().BatchGetOKRs(context.Background(), []string{"okr_1", "okr_2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(okrs) != 2 {
		t.Fatalf("expected 2 OKRs, got %d", len(okrs))
	}
	if okrs[0].OKRID != "okr_1" {
		t.Errorf("expected okr_1, got %s", okrs[0].OKRID)
	}
	if okrs[1].Name != "OKR B" {
		t.Errorf("expected 'OKR B', got %s", okrs[1].Name)
	}
}

func TestOKRAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "permission denied", nil))
	})
	defer srv.Close()

	_, err := client.OKR().ListPeriods(context.Background(), ListPeriodsRequest{})
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
