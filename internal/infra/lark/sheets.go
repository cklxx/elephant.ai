package lark

import (
	"context"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larksheets "github.com/larksuite/oapi-sdk-go/v3/service/sheets/v3"
)

// SheetsService provides typed access to Lark Sheets APIs (v3).
type SheetsService struct {
	client *lark.Client
}

// Spreadsheet is a simplified view of a Lark spreadsheet.
type Spreadsheet struct {
	SpreadsheetToken string
	Title            string
	URL              string
	FolderToken      string
}

// Sheet is a simplified view of a sheet tab within a spreadsheet.
type Sheet struct {
	SheetID string
	Title   string
	Index   int
}

// CreateSpreadsheetRequest defines parameters for creating a spreadsheet.
type CreateSpreadsheetRequest struct {
	Title       string
	FolderToken string
}

// CreateSpreadsheet creates a new spreadsheet.
func (s *SheetsService) CreateSpreadsheet(ctx context.Context, req CreateSpreadsheetRequest, opts ...CallOption) (*Spreadsheet, error) {
	ssBuilder := larksheets.NewSpreadsheetBuilder()
	if req.Title != "" {
		ssBuilder.Title(req.Title)
	}
	if req.FolderToken != "" {
		ssBuilder.FolderToken(req.FolderToken)
	}

	createReq := larksheets.NewCreateSpreadsheetReqBuilder().
		Spreadsheet(ssBuilder.Build()).
		Build()

	resp, err := s.client.Sheets.V3.Spreadsheet.Create(ctx, createReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("create spreadsheet: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("create spreadsheet: unexpected nil data in response")
	}

	return parseSpreadsheet(resp.Data.Spreadsheet), nil
}

// GetSpreadsheet retrieves spreadsheet metadata.
func (s *SheetsService) GetSpreadsheet(ctx context.Context, spreadsheetToken string, opts ...CallOption) (*Spreadsheet, error) {
	getReq := larksheets.NewGetSpreadsheetReqBuilder().
		SpreadsheetToken(spreadsheetToken).
		Build()

	resp, err := s.client.Sheets.V3.Spreadsheet.Get(ctx, getReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("get spreadsheet: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("get spreadsheet: unexpected nil data in response")
	}

	return parseGetSpreadsheet(resp.Data.Spreadsheet), nil
}

// ListSheets lists sheet tabs in a spreadsheet.
func (s *SheetsService) ListSheets(ctx context.Context, spreadsheetToken string, opts ...CallOption) ([]Sheet, error) {
	queryReq := larksheets.NewQuerySpreadsheetSheetReqBuilder().
		SpreadsheetToken(spreadsheetToken).
		Build()

	resp, err := s.client.Sheets.V3.SpreadsheetSheet.Query(ctx, queryReq, buildOpts(opts)...)
	if err != nil {
		return nil, fmt.Errorf("list sheets: %w", err)
	}
	if !resp.Success() {
		return nil, &APIError{Code: resp.Code, Msg: resp.Msg}
	}
	if resp.Data == nil {
		return nil, fmt.Errorf("list sheets: unexpected nil data in response")
	}

	return parseSheets(resp.Data.Sheets), nil
}

// --- helpers ---

func parseSpreadsheet(ss *larksheets.Spreadsheet) *Spreadsheet {
	if ss == nil {
		return &Spreadsheet{}
	}
	result := &Spreadsheet{}
	if ss.SpreadsheetToken != nil {
		result.SpreadsheetToken = *ss.SpreadsheetToken
	}
	if ss.Title != nil {
		result.Title = *ss.Title
	}
	if ss.Url != nil {
		result.URL = *ss.Url
	}
	if ss.FolderToken != nil {
		result.FolderToken = *ss.FolderToken
	}
	return result
}

func parseGetSpreadsheet(ss *larksheets.GetSpreadsheet) *Spreadsheet {
	if ss == nil {
		return &Spreadsheet{}
	}
	result := &Spreadsheet{}
	if ss.Token != nil {
		result.SpreadsheetToken = *ss.Token
	}
	if ss.Title != nil {
		result.Title = *ss.Title
	}
	if ss.Url != nil {
		result.URL = *ss.Url
	}
	return result
}

func parseSheets(sheets []*larksheets.Sheet) []Sheet {
	result := make([]Sheet, 0, len(sheets))
	for _, sheet := range sheets {
		if sheet == nil {
			continue
		}
		s := Sheet{}
		if sheet.SheetId != nil {
			s.SheetID = *sheet.SheetId
		}
		if sheet.Title != nil {
			s.Title = *sheet.Title
		}
		if sheet.Index != nil {
			s.Index = *sheet.Index
		}
		result = append(result, s)
	}
	return result
}
