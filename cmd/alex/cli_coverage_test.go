package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	jsonrpc "alex/internal/shared/jsonrpc"
)

// ---------------------------------------------------------------------------
// Section 1: ACP HTTP/RPC request lifecycle
// ---------------------------------------------------------------------------

func TestACPHTTPHandler_RPCRejectsGET(t *testing.T) {
	t.Parallel()
	server := newACPServer(nil, "")
	transport := newSSETransport("client-1", nil)
	server.registerTransport("client-1", transport)
	h := newACPHTTPServer(server)

	req := httptest.NewRequest(http.MethodGet, "/acp/rpc?client_id=client-1", nil)
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	require.Contains(t, rec.Body.String(), "method not allowed")
}

func TestACPHTTPHandler_RPCMissingClientID(t *testing.T) {
	t.Parallel()
	h := newACPHTTPServer(newACPServer(nil, ""))
	req := httptest.NewRequest(http.MethodPost, "/acp/rpc", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "client_id")
}

func TestACPHTTPHandler_RPCEmptyClientID(t *testing.T) {
	t.Parallel()
	h := newACPHTTPServer(newACPServer(nil, ""))
	req := httptest.NewRequest(http.MethodPost, "/acp/rpc?client_id=", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestACPHTTPHandler_RPCInvalidJSON(t *testing.T) {
	t.Parallel()
	server := newACPServer(nil, "")
	transport := newSSETransport("client-2", nil)
	server.registerTransport("client-2", transport)
	h := newACPHTTPServer(server)

	req := httptest.NewRequest(http.MethodPost, "/acp/rpc?client_id=client-2", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "invalid json-rpc")
}

func TestACPHTTPHandler_RPCDeliversResponse(t *testing.T) {
	t.Parallel()
	server := newACPServer(nil, "")
	transport := newSSETransport("client-3", nil)
	server.registerTransport("client-3", transport)
	h := newACPHTTPServer(server)

	// Set up a pending request so DeliverResponse has something to deliver to
	respCh := make(chan *jsonrpc.Response, 1)
	transport.pendingMu.Lock()
	transport.pending["42"] = respCh
	transport.pendingMu.Unlock()

	payload := `{"jsonrpc":"2.0","id":"42","result":{"status":"ok"}}`
	req := httptest.NewRequest(http.MethodPost, "/acp/rpc?client_id=client-3", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code)

	select {
	case resp := <-respCh:
		require.Equal(t, "42", fmt.Sprintf("%v", resp.ID))
	case <-time.After(time.Second):
		t.Fatal("response was not delivered to pending channel")
	}
}

func TestACPHTTPHandler_RPCAcceptsNotification(t *testing.T) {
	t.Parallel()
	server := newACPServer(nil, "")
	transport := newSSETransport("client-4", nil)
	server.registerTransport("client-4", transport)
	h := newACPHTTPServer(server)

	// Notification: no ID field
	payload := `{"jsonrpc":"2.0","method":"notifications/initialized"}`
	req := httptest.NewRequest(http.MethodPost, "/acp/rpc?client_id=client-4", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}

func TestACPHTTPHandler_RPCAcceptsRequest(t *testing.T) {
	t.Parallel()
	server := newACPServer(nil, "")
	transport := newSSETransport("client-5", nil)
	server.registerTransport("client-5", transport)
	h := newACPHTTPServer(server)

	payload := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"text":"hello"}}`
	req := httptest.NewRequest(http.MethodPost, "/acp/rpc?client_id=client-5", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)
}

func TestACPHTTPHandler_SSEMissingClientID(t *testing.T) {
	t.Parallel()
	h := newACPHTTPServer(newACPServer(nil, ""))
	req := httptest.NewRequest(http.MethodGet, "/acp/sse", nil)
	rec := httptest.NewRecorder()
	h.Handler().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNewACPHTTPServerNilReturnsNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, newACPHTTPServer(nil))
}

func TestClientIDFromRequest(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		url     string
		wantID  string
		wantErr bool
	}{
		{"valid", "/acp/sse?client_id=abc", "abc", false},
		{"with spaces", "/acp/sse?client_id=xyz", "xyz", false},
		{"empty", "/acp/sse?client_id=", "", true},
		{"missing", "/acp/sse", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.url, nil)
			id, err := clientIDFromRequest(r)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantID, id)
			}
		})
	}
}

func TestClientIDFromRequest_NilRequest(t *testing.T) {
	t.Parallel()
	_, err := clientIDFromRequest(nil)
	require.Error(t, err)
}

func TestSSETransportDeliverResponseNilSafe(t *testing.T) {
	t.Parallel()
	var transport *sseTransport
	require.False(t, transport.DeliverResponse(nil))

	transport = newSSETransport("t1", nil)
	require.False(t, transport.DeliverResponse(nil))
}

func TestSSETransportDeliverResponseUnmatchedID(t *testing.T) {
	t.Parallel()
	transport := newSSETransport("t2", nil)
	resp := jsonrpc.NewResponse("999", map[string]any{"ok": true})
	require.False(t, transport.DeliverResponse(resp))
}

func TestSSETransportCallContextCancellation(t *testing.T) {
	t.Parallel()
	transport := newSSETransport("t3", nil)

	// Start a goroutine that drains the send channel to prevent blocking
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range transport.sendCh {
			// drain
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := transport.Call(ctx, "test_method", nil)
	require.ErrorIs(t, err, context.Canceled)

	close(transport.sendCh)
	<-done
}

func TestSSETransportNotifyNilSafe(t *testing.T) {
	t.Parallel()
	var transport *sseTransport
	err := transport.Notify("test", nil)
	require.Error(t, err)
}

func TestSSETransportSendResponseNilSafe(t *testing.T) {
	t.Parallel()
	var transport *sseTransport
	err := transport.SendResponse(jsonrpc.NewResponse(1, "ok"))
	require.Error(t, err)
}

func TestSSETransportStreamNilSafe(t *testing.T) {
	t.Parallel()
	var transport *sseTransport
	err := transport.Stream(context.Background(), httptest.NewRecorder())
	require.Error(t, err)
}

func TestWriteSSEPayload(t *testing.T) {
	t.Parallel()
	var buf strings.Builder
	err := writeSSEPayload(&buf, []byte(`{"test":true}`))
	require.NoError(t, err)
	require.Equal(t, "data: {\"test\":true}\n\n", buf.String())
}

// ---------------------------------------------------------------------------
// Section 2: Runtime and dev subcommand argument validation
// ---------------------------------------------------------------------------

func TestRunRuntimeCommand_HelpVariants(t *testing.T) {
	t.Parallel()
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			err := runRuntimeCommand([]string{arg})
			require.NoError(t, err)
		})
	}
}

func TestRunRuntimeCommand_EmptyShowsUsage(t *testing.T) {
	t.Parallel()
	err := runRuntimeCommand(nil)
	require.NoError(t, err)
}

func TestRunRuntimeCommand_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := runRuntimeCommand([]string{"bogus"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
	require.Contains(t, err.Error(), "unknown runtime subcommand")
}

func TestRunRuntimeSessionCommand_EmptyShowsUsage(t *testing.T) {
	t.Parallel()
	err := runRuntimeSessionCommand(nil)
	require.NoError(t, err)
}

func TestRunRuntimeSessionCommand_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := runRuntimeSessionCommand([]string{"fly"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
	require.Contains(t, err.Error(), "unknown session subcommand")
}

func TestRunRuntimeStart_MissingGoal(t *testing.T) {
	t.Parallel()
	err := runRuntimeStart(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeStart_InvalidFlag(t *testing.T) {
	t.Parallel()
	err := runRuntimeStart([]string{"--bogus"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeInject_MissingBothArgs(t *testing.T) {
	t.Parallel()
	err := runRuntimeInject(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeInject_MissingMessage(t *testing.T) {
	t.Parallel()
	err := runRuntimeInject([]string{"--id", "sess-1"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeInject_MissingID(t *testing.T) {
	t.Parallel()
	err := runRuntimeInject([]string{"--message", "hello"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeStatus_MissingID(t *testing.T) {
	t.Parallel()
	err := runRuntimeStatus(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeStop_MissingID(t *testing.T) {
	t.Parallel()
	err := runRuntimeStop(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunRuntimeStop_InvalidFlag(t *testing.T) {
	t.Parallel()
	err := runRuntimeStop([]string{"--bogus"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestResolveStoreDir(t *testing.T) {
	// Cannot use t.Parallel — t.Setenv is used.
	t.Setenv("KAKU_STORE_DIR", "")
	tests := []struct {
		name     string
		flag     string
		wantSub  string // substring that must appear
		wantFull string // exact match when set
	}{
		{"explicit flag", "/tmp/my-sessions", "", "/tmp/my-sessions"},
		{"blank flag uses default", "", "sessions", ""},
		{"whitespace flag uses default", "   ", "sessions", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveStoreDir(tt.flag)
			if tt.wantFull != "" {
				require.Equal(t, tt.wantFull, got)
			} else {
				require.Contains(t, got, tt.wantSub)
			}
		})
	}
}

func TestResolveStoreDir_EnvOverride(t *testing.T) {
	t.Setenv("KAKU_STORE_DIR", "/tmp/env-store")
	got := resolveStoreDir("")
	require.Equal(t, "/tmp/env-store", got)
}

func TestExpandHome(t *testing.T) {
	t.Parallel()
	require.Equal(t, "/absolute/path", expandHome("/absolute/path"))
	require.NotEqual(t, "~/foo", expandHome("~/foo")) // should expand ~
	require.True(t, strings.HasSuffix(expandHome("~/foo"), "/foo"))
}

func TestRunDevCommand_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := runDevCommand([]string{"nonexistent"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown dev command")
}

func TestRunDevCommand_AttachMissingName(t *testing.T) {
	t.Parallel()
	err := runDevCommand([]string{"attach"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}

func TestRunDevCommand_CaptureMissingName(t *testing.T) {
	t.Parallel()
	err := runDevCommand([]string{"capture"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "usage")
}

// ---------------------------------------------------------------------------
// Section 3: Lark inject command validation and failure modes
// ---------------------------------------------------------------------------

func TestRunLarkCommand_EmptyArgs(t *testing.T) {
	t.Parallel()
	err := runLarkCommand(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestRunLarkCommand_UnknownSubcommand(t *testing.T) {
	t.Parallel()
	err := runLarkCommand([]string{"send"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
	require.Contains(t, err.Error(), "unknown lark subcommand")
}

func TestRunLarkInjectCommand_MissingMessage(t *testing.T) {
	t.Parallel()
	err := runLarkInjectCommand(nil)
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
	require.Contains(t, err.Error(), "usage")
}

func TestRunLarkInjectCommand_InvalidFlag(t *testing.T) {
	t.Parallel()
	err := runLarkInjectCommand([]string{"--bogus"})
	require.Error(t, err)
	var exitErr *ExitCodeError
	require.True(t, errors.As(err, &exitErr))
	require.Equal(t, 2, exitErr.Code)
}

func TestLarkInjectEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		baseURL string
		port    string
		want    string
	}{
		{"baseURL takes precedence", "http://localhost:1234", "9999", "http://localhost:1234/api/dev/inject"},
		{"baseURL with trailing slash", "http://localhost:1234/", "9999", "http://localhost:1234/api/dev/inject"},
		{"port only", "", "8080", "http://127.0.0.1:8080/api/dev/inject"},
		{"default port when empty", "", "", "http://127.0.0.1:9090/api/dev/inject"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := larkInjectEndpoint(tt.baseURL, tt.port)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestPostLarkInject_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(larkInjectResponse{Error: "boom"})
	}))
	defer srv.Close()

	resp, err := postLarkInject(context.Background(), srv.Client(), srv.URL, larkInjectRequest{Text: "test"})
	require.NoError(t, err)
	require.Equal(t, http.StatusInternalServerError, resp.StatusCode)
	require.Equal(t, "boom", resp.Body.Error)
}

func TestPostLarkInject_SuccessWithReplies(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(larkInjectResponse{
			Replies:    []larkInjectReply{{Method: "send_text", Content: `{"text":"hi"}`}},
			DurationMs: 150,
		})
	}))
	defer srv.Close()

	resp, err := postLarkInject(context.Background(), srv.Client(), srv.URL, larkInjectRequest{Text: "hello"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Len(t, resp.Body.Replies, 1)
	require.Equal(t, "send_text", resp.Body.Replies[0].Method)
	require.Equal(t, int64(150), resp.Body.DurationMs)
}

func TestPostLarkInject_EmptyBody(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	resp, err := postLarkInject(context.Background(), srv.Client(), srv.URL, larkInjectRequest{Text: "test"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Empty(t, resp.Body.Replies)
}

func TestPostLarkInject_InvalidResponseJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`not json`))
	}))
	defer srv.Close()

	_, err := postLarkInject(context.Background(), srv.Client(), srv.URL, larkInjectRequest{Text: "test"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "parse response")
}

func TestPostLarkInject_ContextCancellation(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // block forever
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := postLarkInject(ctx, srv.Client(), srv.URL, larkInjectRequest{Text: "test"})
	require.Error(t, err)
}

func TestIsTransientInjectTransportError(t *testing.T) {
	t.Parallel()
	tests := []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("connection refused"), true},
		{errors.New("connection reset by peer"), true},
		{errors.New("read: eof"), true},
		{errors.New("eof"), true},
		{errors.New("write: broken pipe"), true},
		{errors.New("timeout"), false},
		{errors.New("unknown error"), false},
	}
	for _, tt := range tests {
		label := "nil"
		if tt.err != nil {
			label = tt.err.Error()
		}
		t.Run(label, func(t *testing.T) {
			got := isTransientInjectTransportError(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExtractLarkReplyText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"json text object", `{"text":"hello world"}`, "hello world"},
		{"json empty text", `{"text":""}`, `{"text":""}`},
		{"plain text", "hello world", "hello world"},
		{"invalid json", "{bad", "{bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, extractLarkReplyText(tt.content))
		})
	}
}

func TestDefaultString(t *testing.T) {
	t.Parallel()
	require.Equal(t, "fallback", defaultString("", "fallback"))
	require.Equal(t, "fallback", defaultString("  ", "fallback"))
	require.Equal(t, "value", defaultString("value", "fallback"))
}

func TestMaxInt(t *testing.T) {
	t.Parallel()
	require.Equal(t, 300, maxInt(0, 300))
	require.Equal(t, 300, maxInt(-1, 300))
	require.Equal(t, 42, maxInt(42, 300))
}

func TestFlagProvided(t *testing.T) {
	t.Parallel()
	require.False(t, flagProvided(nil, "port"))
}

// ---------------------------------------------------------------------------
// Section 4: Stream output event bridge — envelope parsing
// ---------------------------------------------------------------------------

func testBase() domain.BaseEvent {
	return domain.NewBaseEvent(agent.LevelCore, "sess-1", "run-1", "", time.Unix(1000, 0))
}

func TestEnvelopeToEvent_Nil(t *testing.T) {
	t.Parallel()
	require.Nil(t, envelopeToEvent(nil))
}

func TestEnvelopeToEvent_UnknownEventType(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     "workflow.unknown.type",
	}
	require.Nil(t, envelopeToEvent(env))
}

func TestEnvelopeToEvent_NodeStarted(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventNodeStarted,
		Payload: map[string]any{
			"iteration":        3,
			"total_iters":      10,
			"step_index":       1,
			"step_description": "planning",
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventNodeStarted, evt.Kind)
	require.Equal(t, 3, evt.Data.Iteration)
}

func TestEnvelopeToEvent_NodeOutputDelta(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventNodeOutputDelta,
		Payload: map[string]any{
			"iteration":     2,
			"message_count": 5,
			"delta":         "Hello world",
			"final":         true,
			"created_at":    ts.Format(time.RFC3339Nano),
			"source_model":  "claude-opus-4-6",
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventNodeOutputDelta, evt.Kind)
	require.Equal(t, "Hello world", evt.Data.Delta)
	require.True(t, evt.Data.Final)
}

func TestEnvelopeToEvent_NodeOutputSummary(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventNodeOutputSummary,
		Payload: map[string]any{
			"iteration":       1,
			"content":         "Analyzed the codebase.",
			"tool_call_count": 3,
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventNodeOutputSummary, evt.Kind)
}

func TestEnvelopeToEvent_ToolStarted(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventToolStarted,
		NodeID:    "call-123",
		Payload: map[string]any{
			"iteration": 1,
			"tool_name": "file_read",
			"arguments": map[string]any{"path": "/tmp/test.txt"},
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventToolStarted, evt.Kind)
	require.Equal(t, "call-123", evt.Data.CallID)
	require.Equal(t, "file_read", evt.Data.ToolName)
}

func TestEnvelopeToEvent_ToolStartedFallbackCallID(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventToolStarted,
		Payload: map[string]any{
			"call_id":   "call-from-payload",
			"tool_name": "web_search",
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, "call-from-payload", evt.Data.CallID)
}

func TestEnvelopeToEvent_ToolCompleted(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventToolCompleted,
		NodeID:    "call-456",
		Payload: map[string]any{
			"tool_name": "file_read",
			"result":    "file contents here",
			"duration":  int64(250),
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventToolCompleted, evt.Kind)
	require.Equal(t, "call-456", evt.Data.CallID)
	require.Equal(t, "file contents here", evt.Data.Result)
}

func TestEnvelopeToEvent_ToolCompletedWithError(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventToolCompleted,
		NodeID:    "call-err",
		Payload: map[string]any{
			"tool_name": "web_fetch",
			"error":     "connection timeout",
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.NotEmpty(t, evt.Data.Error)
	require.Contains(t, evt.Data.Error.Error(), "connection timeout")
}

func TestEnvelopeToEvent_NodeFailed(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventNodeFailed,
		Payload: map[string]any{
			"iteration":   4,
			"phase":       "tool_execution",
			"error":       "rate limited",
			"recoverable": true,
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventNodeFailed, evt.Kind)
	require.Equal(t, "tool_execution", evt.Data.PhaseLabel)
	require.NotNil(t, evt.Data.Error)
}

func TestEnvelopeToEvent_ResultFinal(t *testing.T) {
	t.Parallel()
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: testBase(),
		Event:     types.EventResultFinal,
		Payload: map[string]any{
			"final_answer":     "The answer is 42.",
			"total_iterations": 5,
			"total_tokens":     1024,
			"stop_reason":      "final_answer",
			"duration":         int64(3500),
			"is_streaming":     true,
			"stream_finished":  true,
		},
	}
	evt := envelopeToEvent(env)
	require.NotNil(t, evt)
	require.Equal(t, types.EventResultFinal, evt.Kind)
	require.Equal(t, 5, evt.Data.TotalIterations)
	require.Equal(t, 1024, evt.Data.TotalTokens)
}

func TestEnvelopeBase_NilReturnsDefault(t *testing.T) {
	t.Parallel()
	base := envelopeBase(nil)
	require.NotNil(t, base)
}

// ---------------------------------------------------------------------------
// Section 4b: Payload extraction helpers
// ---------------------------------------------------------------------------

func TestPayloadString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    string
	}{
		{"nil payload", nil, "key", ""},
		{"missing key", map[string]any{"a": "b"}, "c", ""},
		{"string value", map[string]any{"k": "hello"}, "k", "hello"},
		{"int value", map[string]any{"k": 42}, "k", "42"},
		{"int64 value", map[string]any{"k": int64(99)}, "k", "99"},
		{"float64 value", map[string]any{"k": float64(7)}, "k", "7"},
		{"bytes value", map[string]any{"k": []byte("data")}, "k", "data"},
		{"nil value", map[string]any{"k": nil}, "k", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, payloadString(tt.payload, tt.key))
		})
	}
}

func TestPayloadBool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    bool
	}{
		{"nil payload", nil, "k", false},
		{"missing key", map[string]any{}, "k", false},
		{"bool true", map[string]any{"k": true}, "k", true},
		{"bool false", map[string]any{"k": false}, "k", false},
		{"string true", map[string]any{"k": "true"}, "k", true},
		{"string false", map[string]any{"k": "false"}, "k", false},
		{"int 1", map[string]any{"k": 1}, "k", true},
		{"int 0", map[string]any{"k": 0}, "k", false},
		{"float64 nonzero", map[string]any{"k": float64(1)}, "k", true},
		{"nil value", map[string]any{"k": nil}, "k", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, payloadBool(tt.payload, tt.key))
		})
	}
}

func TestPayloadInt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    int
	}{
		{"nil payload", nil, "k", 0},
		{"missing key", map[string]any{}, "k", 0},
		{"int value", map[string]any{"k": 42}, "k", 42},
		{"int64 value", map[string]any{"k": int64(99)}, "k", 99},
		{"float64 value", map[string]any{"k": float64(7.9)}, "k", 7},
		{"string value", map[string]any{"k": "123"}, "k", 123},
		{"bad string", map[string]any{"k": "abc"}, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, payloadInt(tt.payload, tt.key))
		})
	}
}

func TestPayloadInt64(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    int64
	}{
		{"nil payload", nil, "k", 0},
		{"int value", map[string]any{"k": 10}, "k", 10},
		{"int64 value", map[string]any{"k": int64(777)}, "k", 777},
		{"float64 value", map[string]any{"k": float64(3.14)}, "k", 3},
		{"string value", map[string]any{"k": "9999"}, "k", 9999},
		{"bad string", map[string]any{"k": "abc"}, "k", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, payloadInt64(tt.payload, tt.key))
		})
	}
}

func TestPayloadTime(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		payload map[string]any
		key     string
		want    time.Time
	}{
		{"nil payload", nil, "k", time.Time{}},
		{"time.Time", map[string]any{"k": ts}, "k", ts},
		{"RFC3339 string", map[string]any{"k": ts.Format(time.RFC3339Nano)}, "k", ts},
		{"bad string", map[string]any{"k": "not-a-time"}, "k", time.Time{}},
		{"nil value", map[string]any{"k": nil}, "k", time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := payloadTime(tt.payload, tt.key)
			require.True(t, tt.want.Equal(got), "want %v, got %v", tt.want, got)
		})
	}
}

func TestPayloadArgs(t *testing.T) {
	t.Parallel()
	require.Nil(t, payloadArgs(nil, "k"))
	require.Nil(t, payloadArgs(map[string]any{"k": "not-a-map"}, "k"))
	require.Nil(t, payloadArgs(map[string]any{"k": nil}, "k"))

	args := map[string]interface{}{"path": "/tmp"}
	require.Equal(t, args, payloadArgs(map[string]any{"k": args}, "k"))
}

func TestPayloadMap(t *testing.T) {
	t.Parallel()
	require.Nil(t, payloadMap(nil, "k"))
	require.Nil(t, payloadMap(map[string]any{"k": 42}, "k"))

	m := map[string]any{"foo": "bar"}
	require.Equal(t, m, payloadMap(map[string]any{"k": m}, "k"))
}

// ---------------------------------------------------------------------------
// Section 4c: Markdown stream buffer
// ---------------------------------------------------------------------------

func TestMarkdownStreamBuffer_EmptyDelta(t *testing.T) {
	t.Parallel()
	buf := newMarkdownStreamBuffer()
	require.Nil(t, buf.Append(""))
}

func TestMarkdownStreamBuffer_CompleteLine(t *testing.T) {
	t.Parallel()
	buf := newMarkdownStreamBuffer()
	chunks := buf.Append("hello\n")
	require.Len(t, chunks, 1)
	require.Equal(t, "hello\n", chunks[0].content)
	require.True(t, chunks[0].completeLine)
}

func TestMarkdownStreamBuffer_MultipleLines(t *testing.T) {
	t.Parallel()
	buf := newMarkdownStreamBuffer()
	chunks := buf.Append("line1\nline2\ntrailing")
	require.Len(t, chunks, 2)
	require.Equal(t, "line1\n", chunks[0].content)
	require.Equal(t, "line2\n", chunks[1].content)

	// Trailing content is buffered, flushed on FlushAll
	remaining := buf.FlushAll()
	require.Equal(t, "trailing", remaining)
}

func TestMarkdownStreamBuffer_FirstTokenImmediate(t *testing.T) {
	t.Parallel()
	buf := newMarkdownStreamBuffer()
	chunks := buf.Append("H") // no newline, but first token
	require.Len(t, chunks, 1)
	require.Equal(t, "H", chunks[0].content)
	require.False(t, chunks[0].completeLine)
}

func TestMarkdownStreamBuffer_FlushAllEmpty(t *testing.T) {
	t.Parallel()
	buf := newMarkdownStreamBuffer()
	require.Equal(t, "", buf.FlushAll())
}

// ---------------------------------------------------------------------------
// Section 5: ExitCodeError
// ---------------------------------------------------------------------------

func TestExitCodeError_NilErr(t *testing.T) {
	t.Parallel()
	e := &ExitCodeError{Code: 1, Err: nil}
	require.Equal(t, "", e.Error())
	require.Nil(t, e.Unwrap())
}

func TestExitCodeError_WrapsError(t *testing.T) {
	t.Parallel()
	inner := errors.New("boom")
	e := &ExitCodeError{Code: 42, Err: inner}
	require.Equal(t, "boom", e.Error())
	require.ErrorIs(t, e, inner)
}

// ---------------------------------------------------------------------------
// Section 6: stringListFlag
// ---------------------------------------------------------------------------

func TestStringListFlag(t *testing.T) {
	t.Parallel()
	var f stringListFlag
	require.NoError(t, f.Set("a,b,c"))
	require.Equal(t, []string{"a", "b", "c"}, []string(f))
	require.NoError(t, f.Set("d"))
	require.Equal(t, []string{"a", "b", "c", "d"}, []string(f))
	require.Equal(t, "a,b,c,d", f.String())
}

func TestStringListFlag_EmptyAndWhitespace(t *testing.T) {
	t.Parallel()
	var f stringListFlag
	require.NoError(t, f.Set(""))
	require.Empty(t, f)
	require.NoError(t, f.Set("  "))
	require.Empty(t, f)
	require.NoError(t, f.Set("a, , b"))
	require.Equal(t, []string{"a", "b"}, []string(f))
}
