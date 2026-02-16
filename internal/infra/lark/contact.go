package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcontact "github.com/larksuite/oapi-sdk-go/v3/service/contact/v3"
)

// ContactService provides typed access to Lark Contact APIs (v3).
type ContactService struct {
	client *lark.Client
}

// User is a simplified view of a Lark user.
type User struct {
	UserID       string
	OpenID       string
	Name         string
	EnName       string
	Email        string
	Mobile       string
	DepartmentIDs []string
	Avatar       string
	Status       int // 1=active, 2=resigned, etc.
}

// Department is a simplified view of a Lark department.
type Department struct {
	DepartmentID       string
	Name               string
	ParentDepartmentID string
	LeaderUserID       string
	MemberCount        int
	Status             int
}

// GetUser retrieves a user by user ID.
func (s *ContactService) GetUser(ctx context.Context, userID string, userIDType string, opts ...CallOption) (*User, error) {
	if userIDType == "" {
		userIDType = "open_id"
	}

	getReq := larkcontact.NewGetUserReqBuilder().
		UserId(userID).
		UserIdType(userIDType).
		Build()

	resp, err := s.client.Contact.V3.User.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("get user: unexpected nil data in response")
	}

	return parseUser(resp.Data.User), nil
}

// ListUsersRequest defines parameters for listing users.
type ListUsersRequest struct {
	DepartmentID string
	UserIDType   string // open_id | union_id | user_id
	PageSize     int
	PageToken    string
}

// ListUsersResponse contains paginated users.
type ListUsersResponse struct {
	Users     []User
	PageToken string
	HasMore   bool
}

// ListUsers lists users in a department.
func (s *ContactService) ListUsers(ctx context.Context, req ListUsersRequest, opts ...CallOption) (*ListUsersResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}
	userIDType := req.UserIDType
	if userIDType == "" {
		userIDType = "open_id"
	}

	builder := larkcontact.NewListUserReqBuilder().
		DepartmentId(req.DepartmentID).
		UserIdType(userIDType).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Contact.V3.User.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list users: unexpected nil data in response")
	}

	users := make([]User, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		users = append(users, *parseUser(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListUsersResponse{
		Users:     users,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// GetDepartment retrieves a department by ID.
func (s *ContactService) GetDepartment(ctx context.Context, departmentID string, opts ...CallOption) (*Department, error) {
	getReq := larkcontact.NewGetDepartmentReqBuilder().
		DepartmentId(departmentID).
		Build()

	resp, err := s.client.Contact.V3.Department.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get department: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("get department: unexpected nil data in response")
	}

	return parseDepartment(resp.Data.Department), nil
}

// ListDepartmentsRequest defines parameters for listing departments.
type ListDepartmentsRequest struct {
	ParentDepartmentID string
	PageSize           int
	PageToken          string
}

// ListDepartmentsResponse contains paginated departments.
type ListDepartmentsResponse struct {
	Departments []Department
	PageToken   string
	HasMore     bool
}

// ListDepartments lists sub-departments.
func (s *ContactService) ListDepartments(ctx context.Context, req ListDepartmentsRequest, opts ...CallOption) (*ListDepartmentsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	parentID := req.ParentDepartmentID
	if parentID == "" {
		parentID = "0" // root department
	}

	builder := larkcontact.NewListDepartmentReqBuilder().
		ParentDepartmentId(parentID).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Contact.V3.Department.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list departments: unexpected nil data in response")
	}

	depts := make([]Department, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		depts = append(depts, *parseDepartment(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListDepartmentsResponse{
		Departments: depts,
		PageToken:   pageToken,
		HasMore:     hasMore,
	}, nil
}

// --- helpers ---

func parseUser(u *larkcontact.User) *User {
	if u == nil {
		return &User{}
	}
	user := &User{}
	if u.UserId != nil {
		user.UserID = *u.UserId
	}
	if u.OpenId != nil {
		user.OpenID = *u.OpenId
	}
	if u.Name != nil {
		user.Name = *u.Name
	}
	if u.EnName != nil {
		user.EnName = *u.EnName
	}
	if u.Email != nil {
		user.Email = *u.Email
	}
	if u.Mobile != nil {
		user.Mobile = *u.Mobile
	}
	if u.DepartmentIds != nil {
		user.DepartmentIDs = u.DepartmentIds
	}
	if u.Avatar != nil && u.Avatar.AvatarOrigin != nil {
		user.Avatar = *u.Avatar.AvatarOrigin
	}
	if u.Status != nil && u.Status.IsActivated != nil && *u.Status.IsActivated {
		user.Status = 1
	}
	return user
}

func parseDepartment(d *larkcontact.Department) *Department {
	if d == nil {
		return &Department{}
	}
	dept := &Department{}
	if d.DepartmentId != nil {
		dept.DepartmentID = *d.DepartmentId
	}
	if d.Name != nil {
		dept.Name = *d.Name
	}
	if d.ParentDepartmentId != nil {
		dept.ParentDepartmentID = *d.ParentDepartmentId
	}
	if d.LeaderUserId != nil {
		dept.LeaderUserID = *d.LeaderUserId
	}
	if d.MemberCount != nil {
		dept.MemberCount = *d.MemberCount
	}
	return dept
}
