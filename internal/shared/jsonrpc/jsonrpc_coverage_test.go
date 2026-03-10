package jsonrpc

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name   string
		id     any
		method string
		params map[string]any
	}{
		{
			name:   "with string id and params",
			id:     "req-1",
			method: "doSomething",
			params: map[string]any{"key": "value"},
		},
		{
			name:   "with int id and nil params",
			id:     42,
			method: "ping",
			params: nil,
		},
		{
			name:   "with nil id",
			id:     nil,
			method: "notify",
			params: map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.id, tt.method, tt.params)

			assert.Equal(t, JSONRPCVersion, req.JSONRPC)
			assert.Equal(t, tt.id, req.ID)
			assert.Equal(t, tt.method, req.Method)
			assert.Equal(t, tt.params, req.Params)
		})
	}
}

func TestNewNotification(t *testing.T) {
	tests := []struct {
		name   string
		method string
		params map[string]any
	}{
		{
			name:   "with params",
			method: "update",
			params: map[string]any{"status": "ok"},
		},
		{
			name:   "without params",
			method: "heartbeat",
			params: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNotification(tt.method, tt.params)

			assert.Equal(t, JSONRPCVersion, n.JSONRPC)
			assert.Equal(t, tt.method, n.Method)
			assert.Equal(t, tt.params, n.Params)
		})
	}
}

func TestNewResponse(t *testing.T) {
	tests := []struct {
		name   string
		id     any
		result any
	}{
		{
			name:   "string result",
			id:     "1",
			result: "hello",
		},
		{
			name:   "map result",
			id:     2,
			result: map[string]any{"data": "value"},
		},
		{
			name:   "nil result",
			id:     "3",
			result: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewResponse(tt.id, tt.result)

			assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
			assert.Equal(t, tt.id, resp.ID)
			assert.Equal(t, tt.result, resp.Result)
			assert.Nil(t, resp.Error)
		})
	}
}

func TestNewErrorResponse_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		id      any
		code    int
		message string
		data    any
	}{
		{
			name:    "parse error with data",
			id:      "1",
			code:    ParseError,
			message: "parse failed",
			data:    "unexpected token",
		},
		{
			name:    "method not found without data",
			id:      "2",
			code:    MethodNotFound,
			message: "method not found",
			data:    nil,
		},
		{
			name:    "internal error with nil id",
			id:      nil,
			code:    InternalError,
			message: "internal error",
			data:    map[string]any{"detail": "crash"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := NewErrorResponse(tt.id, tt.code, tt.message, tt.data)

			assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
			assert.Equal(t, tt.id, resp.ID)
			assert.Nil(t, resp.Result)
			require.NotNil(t, resp.Error)
			assert.Equal(t, tt.code, resp.Error.Code)
			assert.Equal(t, tt.message, resp.Error.Message)
			assert.Equal(t, tt.data, resp.Error.Data)
		})
	}
}

func TestRPCError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      RPCError
		expected string
	}{
		{
			name:     "without data",
			err:      RPCError{Code: ParseError, Message: "parse failed"},
			expected: "JSON-RPC error -32700: parse failed",
		},
		{
			name:     "with string data",
			err:      RPCError{Code: InternalError, Message: "boom", Data: "extra info"},
			expected: "JSON-RPC error -32603: boom (data: extra info)",
		},
		{
			name:     "with numeric data",
			err:      RPCError{Code: InvalidParams, Message: "bad params", Data: 42},
			expected: "JSON-RPC error -32602: bad params (data: 42)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestRequest_IsNotification(t *testing.T) {
	tests := []struct {
		name     string
		id       any
		expected bool
	}{
		{
			name:     "with string id returns false",
			id:       "1",
			expected: false,
		},
		{
			name:     "with int id returns false",
			id:       42,
			expected: false,
		},
		{
			name:     "with nil id returns true",
			id:       nil,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := NewRequest(tt.id, "test", nil)
			assert.Equal(t, tt.expected, req.IsNotification())
		})
	}
}

func TestResponse_IsError(t *testing.T) {
	tests := []struct {
		name     string
		resp     *Response
		expected bool
	}{
		{
			name:     "with error returns true",
			resp:     NewErrorResponse("1", InternalError, "fail", nil),
			expected: true,
		},
		{
			name:     "without error returns false",
			resp:     NewResponse("1", "ok"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.resp.IsError())
		})
	}
}

func TestUnmarshalResponse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		errCode   int
		checkResp func(t *testing.T, resp *Response)
	}{
		{
			name:  "valid response with result",
			input: `{"jsonrpc":"2.0","id":"1","result":"hello"}`,
			checkResp: func(t *testing.T, resp *Response) {
				assert.Equal(t, JSONRPCVersion, resp.JSONRPC)
				assert.Equal(t, "1", resp.ID)
				assert.Equal(t, "hello", resp.Result)
				assert.Nil(t, resp.Error)
			},
		},
		{
			name:  "valid error response",
			input: `{"jsonrpc":"2.0","id":"2","error":{"code":-32601,"message":"not found"}}`,
			checkResp: func(t *testing.T, resp *Response) {
				require.NotNil(t, resp.Error)
				assert.Equal(t, MethodNotFound, resp.Error.Code)
			},
		},
		{
			name:    "invalid JSON",
			input:   `{invalid`,
			wantErr: true,
			errCode: ParseError,
		},
		{
			name:    "wrong version",
			input:   `{"jsonrpc":"1.0","id":"1","result":"hello"}`,
			wantErr: true,
			errCode: InvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := UnmarshalResponse([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				rpcErr, ok := err.(*RPCError)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, rpcErr.Code)
				assert.Nil(t, resp)
			} else {
				require.NoError(t, err)
				require.NotNil(t, resp)
				tt.checkResp(t, resp)
			}
		})
	}
}

func TestUnmarshalRequest(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		errCode  int
		checkReq func(t *testing.T, req *Request)
	}{
		{
			name:  "valid request with params",
			input: `{"jsonrpc":"2.0","id":"1","method":"doWork","params":{"a":"b"}}`,
			checkReq: func(t *testing.T, req *Request) {
				assert.Equal(t, JSONRPCVersion, req.JSONRPC)
				assert.Equal(t, "1", req.ID)
				assert.Equal(t, "doWork", req.Method)
				assert.Equal(t, "b", req.Params["a"])
			},
		},
		{
			name:  "valid notification (no id)",
			input: `{"jsonrpc":"2.0","method":"notify"}`,
			checkReq: func(t *testing.T, req *Request) {
				assert.Nil(t, req.ID)
				assert.Equal(t, "notify", req.Method)
				assert.True(t, req.IsNotification())
			},
		},
		{
			name:    "invalid JSON",
			input:   `not json`,
			wantErr: true,
			errCode: ParseError,
		},
		{
			name:    "wrong version",
			input:   `{"jsonrpc":"1.1","id":"1","method":"test"}`,
			wantErr: true,
			errCode: InvalidRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := UnmarshalRequest([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				rpcErr, ok := err.(*RPCError)
				require.True(t, ok)
				assert.Equal(t, tt.errCode, rpcErr.Code)
				assert.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				tt.checkReq(t, req)
			}
		})
	}
}

func TestMarshal_Roundtrip(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, data []byte)
	}{
		{
			name:  "roundtrip request",
			input: NewRequest("r1", "test.method", map[string]any{"foo": "bar"}),
			check: func(t *testing.T, data []byte) {
				req, err := UnmarshalRequest(data)
				require.NoError(t, err)
				assert.Equal(t, "r1", req.ID)
				assert.Equal(t, "test.method", req.Method)
				assert.Equal(t, "bar", req.Params["foo"])
			},
		},
		{
			name:  "roundtrip response",
			input: NewResponse("r2", map[string]any{"status": "ok"}),
			check: func(t *testing.T, data []byte) {
				resp, err := UnmarshalResponse(data)
				require.NoError(t, err)
				assert.Equal(t, "r2", resp.ID)
				resultMap, ok := resp.Result.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "ok", resultMap["status"])
			},
		},
		{
			name:  "roundtrip error response",
			input: NewErrorResponse("r3", InternalError, "fail", "detail"),
			check: func(t *testing.T, data []byte) {
				resp, err := UnmarshalResponse(data)
				require.NoError(t, err)
				require.NotNil(t, resp.Error)
				assert.Equal(t, InternalError, resp.Error.Code)
				assert.Equal(t, "fail", resp.Error.Message)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Marshal(tt.input)
			require.NoError(t, err)

			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(data, &raw))
			_, hasVersion := raw["jsonrpc"]
			assert.True(t, hasVersion, "marshaled output should contain jsonrpc field")

			tt.check(t, data)
		})
	}
}

func TestRequestIDGenerator_Sequential(t *testing.T) {
	gen := NewRequestIDGenerator()

	tests := []struct {
		expected string
	}{
		{expected: "1"},
		{expected: "2"},
		{expected: "3"},
	}

	for _, tt := range tests {
		got := gen.Next()
		assert.Equal(t, tt.expected, got)
	}
}

func TestRequestIDGenerator_ConcurrentSafety(t *testing.T) {
	gen := NewRequestIDGenerator()
	const goroutines = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)

	ids := make([]string, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ids[idx] = gen.Next()
		}(i)
	}
	wg.Wait()

	// All IDs must be unique.
	seen := make(map[string]struct{}, goroutines)
	for _, id := range ids {
		assert.NotContains(t, seen, id, "duplicate ID generated: %s", id)
		seen[id] = struct{}{}
	}
	assert.Len(t, seen, goroutines)
}
