package lark

import (
	"context"
	"fmt"
	"strconv"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkmail "github.com/larksuite/oapi-sdk-go/v3/service/mail/v1"
)

// MailService provides typed access to Lark Mail APIs (v1).
type MailService struct {
	client *lark.Client
}

// Mailgroup is a simplified view of a Lark mail group.
type Mailgroup struct {
	MailgroupID        string
	Email              string
	Name               string
	Description        string
	DirectMembersCount int64
}

// ListMailgroupsRequest defines parameters for listing mail groups.
type ListMailgroupsRequest struct {
	PageSize  int
	PageToken string
}

// ListMailgroupsResponse contains paginated mail groups.
type ListMailgroupsResponse struct {
	Mailgroups []Mailgroup
	PageToken  string
	HasMore    bool
}

// ListMailgroups lists mail groups.
func (s *MailService) ListMailgroups(ctx context.Context, req ListMailgroupsRequest, opts ...CallOption) (*ListMailgroupsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkmail.NewListMailgroupReqBuilder().
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Mail.V1.Mailgroup.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list mailgroups: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list mailgroups: unexpected nil data in response")
	}

	groups := make([]Mailgroup, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		groups = append(groups, parseMailgroup(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListMailgroupsResponse{
		Mailgroups: groups,
		PageToken:  pageToken,
		HasMore:    hasMore,
	}, nil
}

// GetMailgroup retrieves a mail group by ID.
func (s *MailService) GetMailgroup(ctx context.Context, mailgroupID string, opts ...CallOption) (*Mailgroup, error) {
	getReq := larkmail.NewGetMailgroupReqBuilder().
		MailgroupId(mailgroupID).
		Build()

	resp, err := s.client.Mail.V1.Mailgroup.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get mailgroup: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("get mailgroup: unexpected nil data in response")
	}

	mg := parseGetMailgroupResp(resp.Data)
	return &mg, nil
}

// CreateMailgroupRequest defines parameters for creating a mail group.
type CreateMailgroupRequest struct {
	Email       string
	Name        string
	Description string
}

// CreateMailgroup creates a new mail group.
func (s *MailService) CreateMailgroup(ctx context.Context, req CreateMailgroupRequest, opts ...CallOption) (*Mailgroup, error) {
	mgBuilder := larkmail.NewMailgroupBuilder()
	if req.Email != "" {
		mgBuilder.Email(req.Email)
	}
	if req.Name != "" {
		mgBuilder.Name(req.Name)
	}
	if req.Description != "" {
		mgBuilder.Description(req.Description)
	}

	createReq := larkmail.NewCreateMailgroupReqBuilder().
		Mailgroup(mgBuilder.Build()).
		Build()

	resp, err := s.client.Mail.V1.Mailgroup.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create mailgroup: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("create mailgroup: unexpected nil data in response")
	}

	mg := parseCreateMailgroupResp(resp.Data)
	return &mg, nil
}

// --- helpers ---

func parseMailgroup(mg *larkmail.Mailgroup) Mailgroup {
	if mg == nil {
		return Mailgroup{}
	}
	m := Mailgroup{}
	if mg.MailgroupId != nil {
		m.MailgroupID = *mg.MailgroupId
	}
	if mg.Email != nil {
		m.Email = *mg.Email
	}
	if mg.Name != nil {
		m.Name = *mg.Name
	}
	if mg.Description != nil {
		m.Description = *mg.Description
	}
	if mg.DirectMembersCount != nil {
		m.DirectMembersCount, _ = strconv.ParseInt(*mg.DirectMembersCount, 10, 64)
	}
	return m
}

func parseGetMailgroupResp(data *larkmail.GetMailgroupRespData) Mailgroup {
	m := Mailgroup{}
	if data.MailgroupId != nil {
		m.MailgroupID = *data.MailgroupId
	}
	if data.Email != nil {
		m.Email = *data.Email
	}
	if data.Name != nil {
		m.Name = *data.Name
	}
	if data.Description != nil {
		m.Description = *data.Description
	}
	if data.DirectMembersCount != nil {
		m.DirectMembersCount, _ = strconv.ParseInt(*data.DirectMembersCount, 10, 64)
	}
	return m
}

func parseCreateMailgroupResp(data *larkmail.CreateMailgroupRespData) Mailgroup {
	m := Mailgroup{}
	if data.MailgroupId != nil {
		m.MailgroupID = *data.MailgroupId
	}
	if data.Email != nil {
		m.Email = *data.Email
	}
	if data.Name != nil {
		m.Name = *data.Name
	}
	if data.Description != nil {
		m.Description = *data.Description
	}
	if data.DirectMembersCount != nil {
		m.DirectMembersCount, _ = strconv.ParseInt(*data.DirectMembersCount, 10, 64)
	}
	return m
}
