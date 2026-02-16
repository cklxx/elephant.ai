package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkbitable "github.com/larksuite/oapi-sdk-go/v3/service/bitable/v1"
)

// BitableService provides typed access to Lark Bitable APIs (v1).
type BitableService struct {
	client *lark.Client
}

// BitableApp is a simplified view of a Lark Bitable application.
type BitableApp struct {
	AppToken string
	Name     string
	URL      string
}

// BitableTable is a simplified view of a Bitable table.
type BitableTable struct {
	TableID  string
	Name     string
	Revision int
}

// BitableRecord is a simplified view of a Bitable record.
type BitableRecord struct {
	RecordID string
	Fields   map[string]interface{}
}

// BitableField is a simplified view of a Bitable field (column).
type BitableField struct {
	FieldID   string
	FieldName string
	FieldType int // 1=Text, 2=Number, 3=SingleSelect, etc.
}

// --- App operations ---

// GetApp retrieves a Bitable app by token.
func (s *BitableService) GetApp(ctx context.Context, appToken string, opts ...CallOption) (*BitableApp, error) {
	getReq := larkbitable.NewGetAppReqBuilder().
		AppToken(appToken).
		Build()

	resp, err := s.client.Bitable.V1.App.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get bitable app: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	return parseBitableDisplayApp(resp.Data.App), nil
}

// --- Table operations ---

// ListTablesRequest defines parameters for listing tables.
type ListTablesRequest struct {
	AppToken  string
	PageSize  int
	PageToken string
}

// ListTablesResponse contains paginated tables.
type ListTablesResponse struct {
	Tables    []BitableTable
	PageToken string
	HasMore   bool
}

// ListTables lists tables in a Bitable app.
func (s *BitableService) ListTables(ctx context.Context, req ListTablesRequest, opts ...CallOption) (*ListTablesResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkbitable.NewListAppTableReqBuilder().
		AppToken(req.AppToken).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Bitable.V1.AppTable.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list bitable tables: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	tables := make([]BitableTable, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		tables = append(tables, parseBitableTable(item))
	}

	var pageToken string
	var hasMore bool
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListTablesResponse{
		Tables:    tables,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// CreateTable creates a new table in a Bitable app.
func (s *BitableService) CreateTable(ctx context.Context, appToken, tableName string, opts ...CallOption) (*BitableTable, error) {
	table := &larkbitable.ReqTable{
		Name: &tableName,
	}

	createReq := larkbitable.NewCreateAppTableReqBuilder().
		AppToken(appToken).
		Body(larkbitable.NewCreateAppTableReqBodyBuilder().
			Table(table).
			Build()).
		Build()

	resp, err := s.client.Bitable.V1.AppTable.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create bitable table: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	result := &BitableTable{}
	if resp.Data.TableId != nil {
		result.TableID = *resp.Data.TableId
	}
	result.Name = tableName
	return result, nil
}

// --- Record operations ---

// ListRecordsRequest defines parameters for listing records.
type ListRecordsRequest struct {
	AppToken  string
	TableID   string
	PageSize  int
	PageToken string
}

// ListRecordsResponse contains paginated records.
type ListRecordsResponse struct {
	Records   []BitableRecord
	Total     int
	PageToken string
	HasMore   bool
}

// ListRecords lists records in a Bitable table.
func (s *BitableService) ListRecords(ctx context.Context, req ListRecordsRequest, opts ...CallOption) (*ListRecordsResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	builder := larkbitable.NewListAppTableRecordReqBuilder().
		AppToken(req.AppToken).
		TableId(req.TableID).
		PageSize(pageSize)

	if req.PageToken != "" {
		builder.PageToken(req.PageToken)
	}

	resp, err := s.client.Bitable.V1.AppTableRecord.List(ctx, builder.Build(), buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list bitable records: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	records := make([]BitableRecord, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		records = append(records, parseBitableRecord(item))
	}

	var total int
	var pageToken string
	var hasMore bool
	if resp.Data.Total != nil {
		total = *resp.Data.Total
	}
	if resp.Data.PageToken != nil {
		pageToken = *resp.Data.PageToken
	}
	if resp.Data.HasMore != nil {
		hasMore = *resp.Data.HasMore
	}

	return &ListRecordsResponse{
		Records:   records,
		Total:     total,
		PageToken: pageToken,
		HasMore:   hasMore,
	}, nil
}

// CreateRecord creates a new record in a Bitable table.
func (s *BitableService) CreateRecord(ctx context.Context, appToken, tableID string, fields map[string]interface{}, opts ...CallOption) (*BitableRecord, error) {
	record := &larkbitable.AppTableRecord{
		Fields: fields,
	}

	createReq := larkbitable.NewCreateAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		AppTableRecord(record).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create bitable record: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	result := parseBitableRecord(resp.Data.Record)
	return &result, nil
}

// UpdateRecord updates fields of an existing record.
func (s *BitableService) UpdateRecord(ctx context.Context, appToken, tableID, recordID string, fields map[string]interface{}, opts ...CallOption) (*BitableRecord, error) {
	record := &larkbitable.AppTableRecord{
		Fields: fields,
	}

	updateReq := larkbitable.NewUpdateAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		RecordId(recordID).
		AppTableRecord(record).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Update(ctx, updateReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("update bitable record: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	result := parseBitableRecord(resp.Data.Record)
	return &result, nil
}

// DeleteRecord deletes a record from a Bitable table.
func (s *BitableService) DeleteRecord(ctx context.Context, appToken, tableID, recordID string, opts ...CallOption) error {
	deleteReq := larkbitable.NewDeleteAppTableRecordReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		RecordId(recordID).
		Build()

	resp, err := s.client.Bitable.V1.AppTableRecord.Delete(ctx, deleteReq, buildOpts(opts)...)
	if err != nil {
		return fmt.Errorf("delete bitable record: %w", err)
	}
	if !resp.Success() {
		return &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	return nil
}

// --- Field operations ---

// ListFields lists fields (columns) in a Bitable table.
func (s *BitableService) ListFields(ctx context.Context, appToken, tableID string, opts ...CallOption) ([]BitableField, error) {
	listReq := larkbitable.NewListAppTableFieldReqBuilder().
		AppToken(appToken).
		TableId(tableID).
		Build()

	resp, err := s.client.Bitable.V1.AppTableField.List(ctx, listReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list bitable fields: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}

	fields := make([]BitableField, 0, len(resp.Data.Items))
	for _, item := range resp.Data.Items {
		fields = append(fields, parseBitableFieldForList(item))
	}
	return fields, nil
}

// --- helpers ---

func parseBitableDisplayApp(app *larkbitable.DisplayApp) *BitableApp {
	if app == nil {
		return &BitableApp{}
	}
	a := &BitableApp{}
	if app.AppToken != nil {
		a.AppToken = *app.AppToken
	}
	if app.Name != nil {
		a.Name = *app.Name
	}
	return a
}

func parseBitableTable(table *larkbitable.AppTable) BitableTable {
	if table == nil {
		return BitableTable{}
	}
	t := BitableTable{}
	if table.TableId != nil {
		t.TableID = *table.TableId
	}
	if table.Name != nil {
		t.Name = *table.Name
	}
	if table.Revision != nil {
		t.Revision = *table.Revision
	}
	return t
}

func parseBitableRecord(record *larkbitable.AppTableRecord) BitableRecord {
	if record == nil {
		return BitableRecord{}
	}
	r := BitableRecord{
		Fields: record.Fields,
	}
	if record.RecordId != nil {
		r.RecordID = *record.RecordId
	}
	return r
}

func parseBitableFieldForList(field *larkbitable.AppTableFieldForList) BitableField {
	if field == nil {
		return BitableField{}
	}
	f := BitableField{}
	if field.FieldName != nil {
		f.FieldName = *field.FieldName
	}
	if field.Type != nil {
		f.FieldType = *field.Type
	}
	return f
}
