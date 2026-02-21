package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkokr "github.com/larksuite/oapi-sdk-go/v3/service/okr/v1"
)

// OKRService provides typed access to Lark OKR APIs (v1).
type OKRService struct {
	client *lark.Client
}

// OKR is a simplified view of a Lark OKR.
type OKR struct {
	OKRID      string
	Name       string
	PeriodID   string
	ObjectiveList []Objective
}

// Objective is a simplified OKR objective.
type Objective struct {
	ObjectiveID string
	Content     string
	Progress    int // 0-100
	KeyResults  []KeyResult
}

// KeyResult is a simplified OKR key result.
type KeyResult struct {
	KRID     string
	Content  string
	Progress int
}

// OKRPeriod is a simplified OKR period.
type OKRPeriod struct {
	PeriodID string
	Name     string
	Status   int // 0=normal, 1=hidden, 2=deleted
}

// ListPeriodsRequest defines parameters for listing OKR periods.
type ListPeriodsRequest struct {
	PageSize  int
	PageToken string
}

// ListPeriodsResponse contains paginated OKR periods.
type ListPeriodsResponse struct {
	Periods   []OKRPeriod
	PageToken string
	HasMore   bool
}

// ListPeriods lists OKR periods.
func (s *OKRService) ListPeriods(ctx context.Context, req ListPeriodsRequest, opts ...CallOption) (*ListPeriodsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkokr.NewListPeriodReqBuilder().
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Okr.V1.Period.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list okr periods: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list okr periods: unexpected nil data in response")
	}

	periods := make([]OKRPeriod, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		periods = append(periods, parseOKRPeriod(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListPeriodsResponse{
		Periods:   periods,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// ListUserOKRs lists OKRs for a specific user in a period.
func (s *OKRService) ListUserOKRs(ctx context.Context, userID string, periodID string, opts ...CallOption) ([]OKR, error) {
	builder := larkokr.NewListUserOkrReqBuilder().
		UserId(userID).
		UserIdType("open_id")

	if periodID != "" {
		builder.PeriodIds([]string{periodID})
	}

	resp, err := s.client.Okr.V1.UserOkr.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list user okrs: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list user okrs: unexpected nil data in response")
	}

	okrs := make([]OKR, 0, len(resp.Data.OkrList))
	for _, item := range resp.Data.OkrList {
		okrs = append(okrs, parseOKR(item))
	}
	return okrs, nil
}

// BatchGetOKRs retrieves OKRs by IDs.
func (s *OKRService) BatchGetOKRs(ctx context.Context, okrIDs []string, opts ...CallOption) ([]OKR, error) {
	builder := larkokr.NewBatchGetOkrReqBuilder().
		OkrIds(okrIDs).
		UserIdType("open_id")

	resp, err := s.client.Okr.V1.Okr.BatchGet(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("batch get okrs: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("batch get okrs: unexpected nil data in response")
	}

	okrs := make([]OKR, 0, len(resp.Data.OkrList))
	for _, item := range resp.Data.OkrList {
		okrs = append(okrs, parseOKR(item))
	}
	return okrs, nil
}

// --- helpers ---

func parseOKRPeriod(p *larkokr.Period) OKRPeriod {
	if p == nil {
		return OKRPeriod{}
	}
	period := OKRPeriod{}
	if p.Id != nil {
		period.PeriodID = *p.Id
	}
	if p.ZhName != nil {
		period.Name = *p.ZhName
	}
	if p.Status != nil {
		period.Status = *p.Status
	}
	return period
}

func parseOKR(o *larkokr.OkrBatch) OKR {
	if o == nil {
		return OKR{}
	}
	okr := OKR{}
	if o.Id != nil {
		okr.OKRID = *o.Id
	}
	if o.Name != nil {
		okr.Name = *o.Name
	}
	if o.PeriodId != nil {
		okr.PeriodID = *o.PeriodId
	}
	for _, obj := range o.ObjectiveList {
		okr.ObjectiveList = append(okr.ObjectiveList, parseObjective(obj))
	}
	return okr
}

func parseObjective(obj *larkokr.OkrObjective) Objective {
	if obj == nil {
		return Objective{}
	}
	o := Objective{}
	if obj.Id != nil {
		o.ObjectiveID = *obj.Id
	}
	if obj.Content != nil {
		o.Content = *obj.Content
	}
	if obj.Score != nil {
		o.Progress = *obj.Score
	}
	for _, kr := range obj.KrList {
		o.KeyResults = append(o.KeyResults, parseKeyResult(kr))
	}
	return o
}

func parseKeyResult(kr *larkokr.OkrObjectiveKr) KeyResult {
	if kr == nil {
		return KeyResult{}
	}
	k := KeyResult{}
	if kr.Id != nil {
		k.KRID = *kr.Id
	}
	if kr.Content != nil {
		k.Content = *kr.Content
	}
	return k
}
