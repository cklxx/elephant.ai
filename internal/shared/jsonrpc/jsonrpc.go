package jsonrpc

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// JSON-RPC 2.0 specification: https://www.jsonrpc.org/specification

const JSONRPCVersion = "2.0"

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
	ServerError    = -32000
)

type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("JSON-RPC error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

type Notification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

type RequestIDGenerator struct {
	counter atomic.Int64
}

func NewRequestIDGenerator() *RequestIDGenerator {
	return &RequestIDGenerator{}
}

func (g *RequestIDGenerator) Next() string {
	return fmt.Sprintf("%d", g.counter.Add(1))
}

func NewRequest(id any, method string, params map[string]any) *Request {
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

func NewNotification(method string, params map[string]any) *Notification {
	return &Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

func NewResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func NewErrorResponse(id any, code int, message string, data any) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}

func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func UnmarshalResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, &RPCError{
			Code:    ParseError,
			Message: "Failed to parse JSON-RPC response",
			Data:    err.Error(),
		}
	}

	if resp.JSONRPC != JSONRPCVersion {
		return nil, &RPCError{
			Code:    InvalidRequest,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s", resp.JSONRPC),
		}
	}

	return &resp, nil
}

func UnmarshalRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, &RPCError{
			Code:    ParseError,
			Message: "Failed to parse JSON-RPC request",
			Data:    err.Error(),
		}
	}

	if req.JSONRPC != JSONRPCVersion {
		return nil, &RPCError{
			Code:    InvalidRequest,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s", req.JSONRPC),
		}
	}

	return &req, nil
}

func (r *Request) IsNotification() bool {
	return r.ID == nil
}

func (r *Response) IsError() bool {
	return r.Error != nil
}
