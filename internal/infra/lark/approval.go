package lark

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkapproval "github.com/larksuite/oapi-sdk-go/v3/service/approval/v4"
)

// ApprovalService provides typed access to Lark Approval APIs (v4).
type ApprovalService struct {
	client *lark.Client
}

// Approval returns the approval sub-service.
func (c *Client) Approval() *ApprovalService {
	return &ApprovalService{client: c.raw}
}

// ApprovalInstance is a simplified view of a Lark approval instance.
type ApprovalInstance struct {
	InstanceID   string            `json:"instance_id"`
	ApprovalCode string            `json:"approval_code"`
	Status       string            `json:"status"`                 // PENDING/APPROVED/REJECTED/CANCELED/DELETED
	StartTime    time.Time         `json:"start_time"`
	EndTime      time.Time         `json:"end_time,omitempty"`
	Initiator    string            `json:"initiator"`              // user_id of initiator
	FormValues   map[string]string `json:"form_values"`            // key-value form data
	TaskList     []ApprovalTask    `json:"task_list"`              // approval node tasks
}

// ApprovalTask represents a single approval task node.
type ApprovalTask struct {
	TaskID   string `json:"task_id"`
	UserID   string `json:"user_id"`
	Status   string `json:"status"`    // PENDING/APPROVED/REJECTED/TRANSFERRED
	NodeName string `json:"node_name"`
}

// --- Query options ---

// ApprovalQueryOption configures the QueryApprovalInstances call.
type ApprovalQueryOption func(*approvalQueryOptions)

type approvalQueryOptions struct {
	status    string
	startTime *time.Time
	endTime   *time.Time
	pageSize  int
	pageToken string
}

// WithApprovalStatus filters instances by status (e.g. "PENDING", "APPROVED", "REJECTED", "CANCELED", "DELETED").
func WithApprovalStatus(status string) ApprovalQueryOption {
	return func(o *approvalQueryOptions) {
		o.status = status
	}
}

// WithApprovalTimeRange filters instances by creation time range.
func WithApprovalTimeRange(start, end time.Time) ApprovalQueryOption {
	return func(o *approvalQueryOptions) {
		o.startTime = &start
		o.endTime = &end
	}
}

// WithApprovalPageSize sets the page size for the query.
func WithApprovalPageSize(size int) ApprovalQueryOption {
	return func(o *approvalQueryOptions) {
		o.pageSize = size
	}
}

// WithApprovalPageToken sets the page token for pagination.
func WithApprovalPageToken(token string) ApprovalQueryOption {
	return func(o *approvalQueryOptions) {
		o.pageToken = token
	}
}

// --- Create options ---

// ApprovalCreateOption configures the CreateApprovalInstance call.
type ApprovalCreateOption func(*approvalCreateOptions)

type approvalCreateOptions struct {
	nodeApprovers []*larkapproval.NodeApprover
	uuid          string
}

// WithNodeApprovers sets approvers for self-selected approval nodes.
func WithNodeApprovers(nodeApprovers []*larkapproval.NodeApprover) ApprovalCreateOption {
	return func(o *approvalCreateOptions) {
		o.nodeApprovers = nodeApprovers
	}
}

// WithApprovalUUID sets a UUID for idempotent instance creation.
func WithApprovalUUID(uuid string) ApprovalCreateOption {
	return func(o *approvalCreateOptions) {
		o.uuid = uuid
	}
}

// --- Methods ---

// QueryApprovalInstances lists approval instances matching the given approval definition code and optional filters.
func (s *ApprovalService) QueryApprovalInstances(ctx context.Context, approvalCode string, opts ...ApprovalQueryOption) ([]ApprovalInstance, error) {
	var qo approvalQueryOptions
	for _, fn := range opts {
		fn(&qo)
	}

	pageSize := qo.pageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	searchBuilder := larkapproval.NewInstanceSearchBuilder().
		ApprovalCode(approvalCode)

	if qo.status != "" {
		searchBuilder.InstanceStatus(qo.status)
	}
	if qo.startTime != nil {
		searchBuilder.InstanceStartTimeFrom(fmt.Sprintf("%d", qo.startTime.UnixMilli()))
	}
	if qo.endTime != nil {
		searchBuilder.InstanceStartTimeTo(fmt.Sprintf("%d", qo.endTime.UnixMilli()))
	}

	reqBuilder := larkapproval.NewQueryInstanceReqBuilder().
		PageSize(pageSize).
		InstanceSearch(searchBuilder.Build())

	if qo.pageToken != "" {
		reqBuilder.PageToken(qo.pageToken)
	}

	resp, err := s.client.Approval.Instance.Query(ctx, reqBuilder.Build())
	if err != nil {
		return nil, fmt.Errorf("query approval instances: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	instances := make([]ApprovalInstance, 0, len(resp.Data.InstanceList))
	for _, item := range resp.Data.InstanceList {
		instances = append(instances, parseInstanceSearchItem(item))
	}
	return instances, nil
}

// GetApprovalInstance retrieves a single approval instance by its instance ID.
func (s *ApprovalService) GetApprovalInstance(ctx context.Context, instanceID string, opts ...CallOption) (*ApprovalInstance, error) {
	req := larkapproval.NewGetInstanceReqBuilder().
		InstanceId(instanceID).
		Build()

	resp, err := s.client.Approval.Instance.Get(ctx, req, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get approval instance: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	inst := parseGetInstanceData(resp.Data)
	return &inst, nil
}

// CreateApprovalInstance creates a new approval instance and returns the instance ID.
func (s *ApprovalService) CreateApprovalInstance(ctx context.Context, approvalCode string, formValues map[string]string, opts ...ApprovalCreateOption) (string, error) {
	var co approvalCreateOptions
	for _, fn := range opts {
		fn(&co)
	}

	formJSON, err := marshalFormValues(formValues)
	if err != nil {
		return "", fmt.Errorf("marshal form values: %w", err)
	}

	instanceBuilder := larkapproval.NewInstanceCreateBuilder().
		ApprovalCode(approvalCode).
		Form(formJSON)

	if len(co.nodeApprovers) > 0 {
		instanceBuilder.NodeApproverUserIdList(co.nodeApprovers)
	}
	if co.uuid != "" {
		instanceBuilder.Uuid(co.uuid)
	}

	req := larkapproval.NewCreateInstanceReqBuilder().
		InstanceCreate(instanceBuilder.Build()).
		Build()

	resp, err := s.client.Approval.Instance.Create(ctx, req)
	if err != nil {
		return "", fmt.Errorf("create approval instance: %w", err)
	}
	if !resp.Success() {
		return "", &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	if resp.Data.InstanceCode == nil {
		return "", fmt.Errorf("create approval instance: instance_code not returned")
	}
	return *resp.Data.InstanceCode, nil
}

// ApproveTask approves a single approval task node.
func (s *ApprovalService) ApproveTask(ctx context.Context, approvalCode, instanceID, taskID, userID, comment string, opts ...CallOption) error {
	body := larkapproval.NewTaskApproveBuilder().
		ApprovalCode(approvalCode).
		InstanceCode(instanceID).
		TaskId(taskID).
		UserId(userID).
		Comment(comment).
		Build()

	req := larkapproval.NewApproveTaskReqBuilder().
		TaskApprove(body).
		Build()

	resp, err := s.client.Approval.Task.Approve(ctx, req, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("approve task: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// RejectTask rejects a single approval task node.
func (s *ApprovalService) RejectTask(ctx context.Context, approvalCode, instanceID, taskID, userID, comment string, opts ...CallOption) error {
	body := larkapproval.NewTaskApproveBuilder().
		ApprovalCode(approvalCode).
		InstanceCode(instanceID).
		TaskId(taskID).
		UserId(userID).
		Comment(comment).
		Build()

	req := larkapproval.NewRejectTaskReqBuilder().
		TaskApprove(body).
		Build()

	resp, err := s.client.Approval.Task.Reject(ctx, req, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("reject task: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// --- helpers ---

// marshalFormValues converts a map of form key-value pairs to the JSON array
// format expected by the Lark Approval API: [{"id":"key","type":"input","value":"val"}, ...].
func marshalFormValues(values map[string]string) (string, error) {
	type formControl struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Value string `json:"value"`
	}
	controls := make([]formControl, 0, len(values))
	for k, v := range values {
		controls = append(controls, formControl{ID: k, Type: "input", Value: v})
	}
	b, err := json.Marshal(controls)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// parseGetInstanceData converts the SDK GetInstanceRespData into our ApprovalInstance type.
func parseGetInstanceData(data *larkapproval.GetInstanceRespData) ApprovalInstance {
	if data == nil {
		return ApprovalInstance{}
	}
	inst := ApprovalInstance{
		FormValues: make(map[string]string),
	}
	if data.InstanceCode != nil {
		inst.InstanceID = *data.InstanceCode
	}
	if data.ApprovalCode != nil {
		inst.ApprovalCode = *data.ApprovalCode
	}
	if data.Status != nil {
		inst.Status = *data.Status
	}
	if data.UserId != nil {
		inst.Initiator = *data.UserId
	}
	if data.StartTime != nil {
		inst.StartTime = parseMilliTimestamp(*data.StartTime)
	}
	if data.EndTime != nil {
		inst.EndTime = parseMilliTimestamp(*data.EndTime)
	}
	if data.Form != nil {
		inst.FormValues = parseFormJSON(*data.Form)
	}

	tasks := make([]ApprovalTask, 0, len(data.TaskList))
	for _, t := range data.TaskList {
		tasks = append(tasks, parseInstanceTask(t))
	}
	inst.TaskList = tasks

	return inst
}

// parseInstanceTask converts an SDK InstanceTask into our ApprovalTask type.
func parseInstanceTask(t *larkapproval.InstanceTask) ApprovalTask {
	if t == nil {
		return ApprovalTask{}
	}
	at := ApprovalTask{}
	if t.Id != nil {
		at.TaskID = *t.Id
	}
	if t.UserId != nil {
		at.UserID = *t.UserId
	}
	if t.Status != nil {
		at.Status = *t.Status
	}
	if t.NodeName != nil {
		at.NodeName = *t.NodeName
	}
	return at
}

// parseInstanceSearchItem converts an SDK InstanceSearchItem into our ApprovalInstance type.
func parseInstanceSearchItem(item *larkapproval.InstanceSearchItem) ApprovalInstance {
	if item == nil {
		return ApprovalInstance{}
	}
	inst := ApprovalInstance{
		FormValues: make(map[string]string),
	}
	if item.Instance != nil {
		if item.Instance.Code != nil {
			inst.InstanceID = *item.Instance.Code
		}
		if item.Instance.Status != nil {
			inst.Status = *item.Instance.Status
		}
		if item.Instance.UserId != nil {
			inst.Initiator = *item.Instance.UserId
		}
		if item.Instance.StartTime != nil {
			inst.StartTime = parseMilliTimestamp(*item.Instance.StartTime)
		}
		if item.Instance.EndTime != nil {
			inst.EndTime = parseMilliTimestamp(*item.Instance.EndTime)
		}
	}
	if item.Approval != nil {
		inst.ApprovalCode = parseSearchApprovalCode(item.Approval)
	}
	return inst
}

// parseSearchApprovalCode extracts the approval code from an InstanceSearchApproval.
func parseSearchApprovalCode(a *larkapproval.InstanceSearchApproval) string {
	if a == nil || a.Code == nil {
		return ""
	}
	return *a.Code
}

// parseFormJSON parses the Lark form JSON array into a key-value map.
// The form JSON looks like: [{"id":"key","type":"input","value":"val"}, ...].
func parseFormJSON(formStr string) map[string]string {
	result := make(map[string]string)
	if formStr == "" {
		return result
	}
	var controls []struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(formStr), &controls); err != nil {
		return result
	}
	for _, c := range controls {
		key := c.ID
		if key == "" {
			key = c.Name
		}
		if key != "" {
			result[key] = c.Value
		}
	}
	return result
}

// parseMilliTimestamp parses a string millisecond timestamp into time.Time.
func parseMilliTimestamp(ts string) time.Time {
	if ts == "" || ts == "0" {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
