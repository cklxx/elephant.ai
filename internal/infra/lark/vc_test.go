package lark

import (
	"context"
	"net/http"
	"testing"
)

func TestListMeetings(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"meeting_list": []map[string]interface{}{
				{
					"meeting_id":         "m_1",
					"meeting_topic":      "Standup",
					"meeting_start_time": "1700000000",
					"meeting_end_time":   "1700003600",
				},
				{
					"meeting_id":    "m_2",
					"meeting_topic": "Retro",
				},
			},
			"page_token": "next",
			"has_more":   true,
		}))
	})
	defer srv.Close()

	resp, err := client.VC().ListMeetings(context.Background(), ListMeetingsRequest{
		StartTime: "1700000000",
		EndTime:   "1700090000",
		PageSize:  20,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Meetings) != 2 {
		t.Fatalf("expected 2 meetings, got %d", len(resp.Meetings))
	}
	if resp.Meetings[0].MeetingID != "m_1" {
		t.Errorf("expected m_1, got %s", resp.Meetings[0].MeetingID)
	}
	if resp.Meetings[0].Topic != "Standup" {
		t.Errorf("expected Standup, got %s", resp.Meetings[0].Topic)
	}
	if resp.Meetings[0].StartTime != "1700000000" {
		t.Errorf("expected 1700000000, got %s", resp.Meetings[0].StartTime)
	}
	if !resp.HasMore {
		t.Error("expected has_more=true")
	}
}

func TestGetMeeting(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"meeting": map[string]interface{}{
				"id":         "m_123",
				"topic":      "Design Review",
				"url":        "https://vc.feishu.cn/j/123",
				"start_time": "1700000000",
				"end_time":   "1700003600",
				"host_user":  map[string]interface{}{"id": "user_host"},
				"status":     1,
			},
		}))
	})
	defer srv.Close()

	m, err := client.VC().GetMeeting(context.Background(), "m_123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.MeetingID != "m_123" {
		t.Errorf("expected m_123, got %s", m.MeetingID)
	}
	if m.Topic != "Design Review" {
		t.Errorf("expected 'Design Review', got %s", m.Topic)
	}
	if m.URL != "https://vc.feishu.cn/j/123" {
		t.Errorf("expected URL, got %s", m.URL)
	}
	if m.HostUser != "user_host" {
		t.Errorf("expected user_host, got %s", m.HostUser)
	}
	if m.Status != 1 {
		t.Errorf("expected status 1, got %d", m.Status)
	}
}

func TestListRooms(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(0, "ok", map[string]interface{}{
			"rooms": []map[string]interface{}{
				{
					"room_id":     "room_1",
					"name":        "Meeting Room A",
					"capacity":    10,
					"description": "Floor 3, Building A",
				},
			},
			"has_more": false,
		}))
	})
	defer srv.Close()

	resp, err := client.VC().ListRooms(context.Background(), ListRoomsRequest{PageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(resp.Rooms))
	}
	if resp.Rooms[0].RoomID != "room_1" {
		t.Errorf("expected room_1, got %s", resp.Rooms[0].RoomID)
	}
	if resp.Rooms[0].Name != "Meeting Room A" {
		t.Errorf("expected 'Meeting Room A', got %s", resp.Rooms[0].Name)
	}
	if resp.Rooms[0].Capacity != 10 {
		t.Errorf("expected capacity 10, got %d", resp.Rooms[0].Capacity)
	}
	if resp.Rooms[0].Description != "Floor 3, Building A" {
		t.Errorf("expected description, got %s", resp.Rooms[0].Description)
	}
}

func TestVCAPIError(t *testing.T) {
	srv, client := testServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mustWrite(t, w, jsonResponse(99991, "no permission", nil))
	})
	defer srv.Close()

	_, err := client.VC().GetMeeting(context.Background(), "m_123")
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
