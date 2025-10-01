package mcp

import (
	"testing"
)

func TestRequestIDGenerator(t *testing.T) {
	gen := NewRequestIDGenerator()

	// Test sequential ID generation
	id1 := gen.Next()
	id2 := gen.Next()
	id3 := gen.Next()

	if id1 != 1 {
		t.Errorf("Expected first ID to be 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("Expected second ID to be 2, got %d", id2)
	}
	if id3 != 3 {
		t.Errorf("Expected third ID to be 3, got %d", id3)
	}
}

func TestNewRequest(t *testing.T) {
	req := NewRequest(1, "test_method", map[string]any{"param1": "value1"})

	if req.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version %s, got %s", JSONRPCVersion, req.JSONRPC)
	}
	if req.ID != 1 {
		t.Errorf("Expected ID 1, got %v", req.ID)
	}
	if req.Method != "test_method" {
		t.Errorf("Expected method 'test_method', got %s", req.Method)
	}
	if req.Params["param1"] != "value1" {
		t.Errorf("Expected param1='value1', got %v", req.Params["param1"])
	}
}

func TestNewNotification(t *testing.T) {
	notif := NewNotification("test_notification", map[string]any{"data": "test"})

	if notif.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version %s, got %s", JSONRPCVersion, notif.JSONRPC)
	}
	if notif.Method != "test_notification" {
		t.Errorf("Expected method 'test_notification', got %s", notif.Method)
	}
	if notif.Params["data"] != "test" {
		t.Errorf("Expected data='test', got %v", notif.Params["data"])
	}
}

func TestNewResponse(t *testing.T) {
	resp := NewResponse(1, map[string]any{"result": "success"})

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("Expected no error, got %v", resp.Error)
	}
	if resp.IsError() {
		t.Error("Expected IsError() to return false")
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(1, InvalidParams, "Invalid parameters", "param1 is required")

	if resp.JSONRPC != JSONRPCVersion {
		t.Errorf("Expected JSONRPC version %s, got %s", JSONRPCVersion, resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("Expected ID 1, got %v", resp.ID)
	}
	if resp.Error == nil {
		t.Fatal("Expected error, got nil")
	}
	if resp.Error.Code != InvalidParams {
		t.Errorf("Expected error code %d, got %d", InvalidParams, resp.Error.Code)
	}
	if resp.Error.Message != "Invalid parameters" {
		t.Errorf("Expected message 'Invalid parameters', got %s", resp.Error.Message)
	}
	if !resp.IsError() {
		t.Error("Expected IsError() to return true")
	}
}

func TestRPCError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *RPCError
		expected string
	}{
		{
			name:     "error without data",
			err:      &RPCError{Code: ParseError, Message: "Parse failed"},
			expected: "JSON-RPC error -32700: Parse failed",
		},
		{
			name:     "error with data",
			err:      &RPCError{Code: InvalidRequest, Message: "Invalid request", Data: "missing method"},
			expected: "JSON-RPC error -32600: Invalid request (data: missing method)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestMarshalUnmarshal(t *testing.T) {
	// Test request marshaling and unmarshaling
	req := NewRequest(42, "test_method", map[string]any{"key": "value"})

	data, err := Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	parsed, err := UnmarshalRequest(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if parsed.Method != req.Method {
		t.Errorf("Expected method %s, got %s", req.Method, parsed.Method)
	}

	// ID may be unmarshaled as float64 from JSON
	parsedID, ok := parsed.ID.(float64)
	if !ok {
		t.Errorf("Expected ID to be float64 after unmarshal, got %T", parsed.ID)
	} else if parsedID != 42.0 {
		t.Errorf("Expected ID 42, got %v", parsedID)
	}

	// Test response marshaling and unmarshaling
	resp := NewResponse(42, map[string]any{"status": "ok"})

	data, err = Marshal(resp)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	parsedResp, err := UnmarshalResponse(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	// ID may be unmarshaled as float64 from JSON
	parsedRespID, ok := parsedResp.ID.(float64)
	if !ok {
		t.Errorf("Expected ID to be float64 after unmarshal, got %T", parsedResp.ID)
	} else if parsedRespID != 42.0 {
		t.Errorf("Expected ID 42, got %v", parsedRespID)
	}
}

func TestUnmarshalInvalidJSON(t *testing.T) {
	// Test invalid JSON
	_, err := UnmarshalResponse([]byte("not valid json"))
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Errorf("Expected RPCError, got %T", err)
	}
	if rpcErr.Code != ParseError {
		t.Errorf("Expected ParseError code, got %d", rpcErr.Code)
	}
}

func TestUnmarshalInvalidVersion(t *testing.T) {
	// Test invalid JSON-RPC version
	invalidResp := `{"jsonrpc":"1.0","id":1,"result":"test"}`

	_, err := UnmarshalResponse([]byte(invalidResp))
	if err == nil {
		t.Error("Expected error for invalid version")
	}

	rpcErr, ok := err.(*RPCError)
	if !ok {
		t.Errorf("Expected RPCError, got %T", err)
	}
	if rpcErr.Code != InvalidRequest {
		t.Errorf("Expected InvalidRequest code, got %d", rpcErr.Code)
	}
}

func TestRequest_IsNotification(t *testing.T) {
	// Request with ID
	req := NewRequest(1, "test", nil)
	if req.IsNotification() {
		t.Error("Expected request with ID to not be a notification")
	}

	// Request without ID (notification)
	req.ID = nil
	if !req.IsNotification() {
		t.Error("Expected request without ID to be a notification")
	}
}
