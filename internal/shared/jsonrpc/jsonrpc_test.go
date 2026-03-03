package jsonrpc

import "testing"

func TestRequestIDGenerator(t *testing.T) {
	gen := NewRequestIDGenerator()
	if got := gen.Next(); got != "1" {
		t.Fatalf("first id = %s", got)
	}
	if got := gen.Next(); got != "2" {
		t.Fatalf("second id = %s", got)
	}
}

func TestNewErrorResponse(t *testing.T) {
	resp := NewErrorResponse(1, InvalidParams, "bad", "x")
	if resp.Error == nil || resp.Error.Code != InvalidParams {
		t.Fatalf("unexpected error response: %#v", resp)
	}
	if !resp.IsError() {
		t.Fatal("expected IsError true")
	}
}

func TestMarshalUnmarshalRequest(t *testing.T) {
	req := NewRequest(42, "m", map[string]any{"k": "v"})
	payload, err := Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	parsed, err := UnmarshalRequest(payload)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Method != "m" {
		t.Fatalf("method = %s", parsed.Method)
	}
}
