package mcp

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// JSON-RPC 2.0 specification: https://www.jsonrpc.org/specification

// JSONRPCVersion is the JSON-RPC version used by MCP
const JSONRPCVersion = "2.0"

// Standard JSON-RPC error codes
const (
	ParseError     = -32700 // Invalid JSON was received
	InvalidRequest = -32600 // The JSON sent is not a valid Request object
	MethodNotFound = -32601 // The method does not exist / is not available
	InvalidParams  = -32602 // Invalid method parameter(s)
	InternalError  = -32603 // Internal JSON-RPC error
	ServerError    = -32000 // Generic server error
)

// Request represents a JSON-RPC 2.0 request
type Request struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"` // String, number, or null
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// Response represents a JSON-RPC 2.0 response
type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id"`
	Result  any       `json:"result,omitempty"`
	Error   *RPCError `json:"error,omitempty"`
}

// RPCError represents a JSON-RPC 2.0 error object
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// Error implements the error interface
func (e *RPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("JSON-RPC error %d: %s (data: %v)", e.Code, e.Message, e.Data)
	}
	return fmt.Sprintf("JSON-RPC error %d: %s", e.Code, e.Message)
}

// Notification represents a JSON-RPC 2.0 notification (no ID)
type Notification struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
}

// RequestIDGenerator generates unique request IDs
type RequestIDGenerator struct {
	counter atomic.Int64
}

// NewRequestIDGenerator creates a new ID generator
func NewRequestIDGenerator() *RequestIDGenerator {
	return &RequestIDGenerator{}
}

// Next generates the next request ID
func (g *RequestIDGenerator) Next() string {
	return fmt.Sprintf("%d", g.counter.Add(1))
}

// NewRequest creates a new JSON-RPC request
func NewRequest(id any, method string, params map[string]any) *Request {
	return &Request{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewNotification creates a new JSON-RPC notification (no response expected)
func NewNotification(method string, params map[string]any) *Notification {
	return &Notification{
		JSONRPC: JSONRPCVersion,
		Method:  method,
		Params:  params,
	}
}

// NewResponse creates a successful JSON-RPC response
func NewResponse(id any, result any) *Response {
	return &Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse creates a JSON-RPC error response
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

// Marshal converts a request/notification to JSON bytes
func Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// UnmarshalResponse parses a JSON-RPC response
func UnmarshalResponse(data []byte) (*Response, error) {
	var resp Response
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, &RPCError{
			Code:    ParseError,
			Message: "Failed to parse JSON-RPC response",
			Data:    err.Error(),
		}
	}

	// Validate JSON-RPC version
	if resp.JSONRPC != JSONRPCVersion {
		return nil, &RPCError{
			Code:    InvalidRequest,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s", resp.JSONRPC),
		}
	}

	return &resp, nil
}

// UnmarshalRequest parses a JSON-RPC request
func UnmarshalRequest(data []byte) (*Request, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, &RPCError{
			Code:    ParseError,
			Message: "Failed to parse JSON-RPC request",
			Data:    err.Error(),
		}
	}

	// Validate JSON-RPC version
	if req.JSONRPC != JSONRPCVersion {
		return nil, &RPCError{
			Code:    InvalidRequest,
			Message: fmt.Sprintf("Invalid JSON-RPC version: %s", req.JSONRPC),
		}
	}

	return &req, nil
}

// IsNotification checks if a request is a notification (no ID)
func (r *Request) IsNotification() bool {
	return r.ID == nil
}

// IsError checks if a response contains an error
func (r *Response) IsError() bool {
	return r.Error != nil
}
