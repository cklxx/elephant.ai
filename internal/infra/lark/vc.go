package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkvc "github.com/larksuite/oapi-sdk-go/v3/service/vc/v1"
)

// VCService provides typed access to Lark VC (Video Conference) APIs (v1).
type VCService struct {
	client *lark.Client
}

// Meeting is a simplified view of a Lark video meeting.
type Meeting struct {
	MeetingID string
	Topic     string
	URL       string
	StartTime string
	EndTime   string
	HostUser  string
	Status    int // 1=ongoing, 2=ended
}

// MeetingRoom is a simplified view of a meeting room.
type MeetingRoom struct {
	RoomID      string
	Name        string
	Capacity    int
	Description string
}

// ListMeetingsRequest defines parameters for listing meetings.
type ListMeetingsRequest struct {
	StartTime string // Unix timestamp
	EndTime   string // Unix timestamp
	PageSize  int
	PageToken string
}

// ListMeetingsResponse contains paginated meetings.
type ListMeetingsResponse struct {
	Meetings  []Meeting
	PageToken string
	HasMore   bool
}

// ListMeetings lists meetings in a time range.
func (s *VCService) ListMeetings(ctx context.Context, req ListMeetingsRequest, opts ...CallOption) (*ListMeetingsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkvc.NewGetMeetingListReqBuilder().
		StartTime(req.StartTime).
		EndTime(req.EndTime).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Vc.V1.MeetingList.Get(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list meetings: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list meetings: unexpected nil data in response")
	}

	meetings := make([]Meeting, 0, len(resp.Data.MeetingList))
	for _, item := range resp.Data.MeetingList {
		meetings = append(meetings, parseMeeting(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListMeetingsResponse{
		Meetings:  meetings,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// GetMeeting retrieves a meeting by ID.
func (s *VCService) GetMeeting(ctx context.Context, meetingID string, opts ...CallOption) (*Meeting, error) {
	getReq := larkvc.NewGetMeetingReqBuilder().
		MeetingId(meetingID).
		Build()

	resp, err := s.client.Vc.V1.Meeting.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get meeting: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("get meeting: unexpected nil data in response")
	}

	meeting := parseMeetingDetail(resp.Data.Meeting)
	return &meeting, nil
}

// ListRoomsRequest defines parameters for listing meeting rooms.
type ListRoomsRequest struct {
	RoomLevelID string
	PageSize    int
	PageToken   string
}

// ListRoomsResponse contains paginated meeting rooms.
type ListRoomsResponse struct {
	Rooms     []MeetingRoom
	PageToken string
	HasMore   bool
}

// ListRooms lists meeting rooms.
func (s *VCService) ListRooms(ctx context.Context, req ListRoomsRequest, opts ...CallOption) (*ListRoomsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkvc.NewListRoomReqBuilder().
		PageSize(pageSize)

	if req.RoomLevelID != "" {
		builder.RoomLevelId(req.RoomLevelID)
	}
	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Vc.V1.Room.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list rooms: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list rooms: unexpected nil data in response")
	}

	rooms := make([]MeetingRoom, 0, len(resp.Data.Rooms))
	for _, item := range resp.Data.Rooms {
		rooms = append(rooms, parseMeetingRoom(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListRoomsResponse{
		Rooms:     rooms,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// --- helpers ---

func parseMeeting(m *larkvc.MeetingInfo) Meeting {
	if m == nil {
		return Meeting{}
	}
	meeting := Meeting{}
	if m.MeetingId != nil {
		meeting.MeetingID = *m.MeetingId
	}
	if m.MeetingTopic != nil {
		meeting.Topic = *m.MeetingTopic
	}
	if m.MeetingStartTime != nil {
		meeting.StartTime = *m.MeetingStartTime
	}
	if m.MeetingEndTime != nil {
		meeting.EndTime = *m.MeetingEndTime
	}
	return meeting
}

func parseMeetingDetail(m *larkvc.Meeting) Meeting {
	if m == nil {
		return Meeting{}
	}
	meeting := Meeting{}
	if m.Id != nil {
		meeting.MeetingID = *m.Id
	}
	if m.Topic != nil {
		meeting.Topic = *m.Topic
	}
	if m.Url != nil {
		meeting.URL = *m.Url
	}
	if m.StartTime != nil {
		meeting.StartTime = *m.StartTime
	}
	if m.EndTime != nil {
		meeting.EndTime = *m.EndTime
	}
	if m.HostUser != nil && m.HostUser.Id != nil {
		meeting.HostUser = *m.HostUser.Id
	}
	if m.Status != nil {
		meeting.Status = *m.Status
	}
	return meeting
}

func parseMeetingRoom(r *larkvc.Room) MeetingRoom {
	if r == nil {
		return MeetingRoom{}
	}
	room := MeetingRoom{}
	if r.RoomId != nil {
		room.RoomID = *r.RoomId
	}
	if r.Name != nil {
		room.Name = *r.Name
	}
	if r.Capacity != nil {
		room.Capacity = *r.Capacity
	}
	if r.Description != nil {
		room.Description = *r.Description
	}
	return room
}
